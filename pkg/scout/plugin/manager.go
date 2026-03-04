package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/inovacc/scout/pkg/scout/scraper"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Manager discovers, caches, and routes to plugin subprocesses.
type Manager struct {
	dirs      []string
	logger    *slog.Logger
	manifests map[string]*Manifest // keyed by plugin name

	mu      sync.Mutex
	clients map[string]*Client // keyed by plugin name, lazily started
}

// NewManager creates a plugin manager that scans the given directories.
func NewManager(dirs []string, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}

	return &Manager{
		dirs:      dirs,
		logger:    logger,
		manifests: make(map[string]*Manifest),
		clients:   make(map[string]*Client),
	}
}

// DefaultDirs returns the default plugin search directories.
func DefaultDirs() []string {
	var dirs []string

	home, err := os.UserHomeDir()
	if err == nil {
		dirs = append(dirs, filepath.Join(home, ".scout", "plugins"))
	}

	if envPath := os.Getenv("SCOUT_PLUGIN_PATH"); envPath != "" {
		dirs = append(dirs, filepath.SplitList(envPath)...)
	}

	return dirs
}

// Discover scans all configured directories for plugin manifests.
func (m *Manager) Discover() error {
	m.manifests = make(map[string]*Manifest)

	for _, dir := range m.dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}

			m.logger.Warn("plugin: scan dir", "dir", dir, "error", err)
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			pluginDir := filepath.Join(dir, entry.Name())
			manifest, err := LoadManifest(pluginDir)
			if err != nil {
				m.logger.Warn("plugin: load manifest", "dir", pluginDir, "error", err)
				continue
			}

			m.manifests[manifest.Name] = manifest
			m.logger.Info("plugin: discovered", "name", manifest.Name, "version", manifest.Version)
		}
	}

	return nil
}

// Plugins returns all discovered plugin manifests.
func (m *Manager) Plugins() []*Manifest {
	result := make([]*Manifest, 0, len(m.manifests))
	for _, manifest := range m.manifests {
		result = append(result, manifest)
	}

	return result
}

// getClient returns a running client for the given manifest, starting it if needed.
func (m *Manager) getClient(manifest *Manifest) (*Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if client, ok := m.clients[manifest.Name]; ok {
		return client, nil
	}

	client := NewClient(manifest, m.logger)

	ctx := context.Background()
	if err := client.Start(ctx); err != nil {
		return nil, err
	}

	if err := client.Initialize(ctx); err != nil {
		_ = client.Shutdown(ctx)
		return nil, err
	}

	m.clients[manifest.Name] = client

	return client, nil
}

// GetMode returns a scraper.Mode proxy for the given mode name, if any plugin provides it.
func (m *Manager) GetMode(name string) (scraper.Mode, bool) {
	for _, manifest := range m.manifests {
		if !manifest.HasCapability("scraper_mode") {
			continue
		}

		for _, entry := range manifest.Modes {
			if entry.Name == name {
				return &ModeProxy{
					entry:    entry,
					manifest: manifest,
					manager:  m,
				}, true
			}
		}
	}

	return nil, false
}

// GetExtractor returns an Extractor proxy for the given name, if any plugin provides it.
func (m *Manager) GetExtractor(name string) (Extractor, bool) {
	for _, manifest := range m.manifests {
		if !manifest.HasCapability("extractor") {
			continue
		}

		for _, entry := range manifest.Extractors {
			if entry.Name == name {
				return &extractorProxy{
					entry:    entry,
					manifest: manifest,
					manager:  m,
				}, true
			}
		}
	}

	return nil, false
}

// RegisterMCPTools registers all plugin-provided MCP tools with the given server.
func (m *Manager) RegisterMCPTools(server *mcp.Server) {
	for _, manifest := range m.manifests {
		if !manifest.HasCapability("mcp_tool") {
			continue
		}

		for _, entry := range manifest.Tools {
			proxy := &ToolProxy{
				entry:    entry,
				manifest: manifest,
				manager:  m,
			}

			proxy.Register(server)
		}
	}
}

// ListModes returns all plugin-provided mode names.
func (m *Manager) ListModes() []string {
	var names []string
	for _, manifest := range m.manifests {
		for _, entry := range manifest.Modes {
			names = append(names, entry.Name)
		}
	}

	return names
}

// ListExtractors returns all plugin-provided extractor names.
func (m *Manager) ListExtractors() []string {
	var names []string
	for _, manifest := range m.manifests {
		for _, entry := range manifest.Extractors {
			names = append(names, entry.Name)
		}
	}

	return names
}

// Close shuts down all running plugin processes.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	ctx := context.Background()
	for name, client := range m.clients {
		if err := client.Shutdown(ctx); err != nil {
			m.logger.Warn("plugin: shutdown", "plugin", name, "error", err)
		}
	}

	m.clients = make(map[string]*Client)
}

// extractorProxy implements Extractor by forwarding to the plugin subprocess.
type extractorProxy struct {
	entry    ExtractorEntry
	manifest *Manifest
	manager  *Manager
}

func (e *extractorProxy) Name() string        { return e.entry.Name }
func (e *extractorProxy) Description() string  { return e.entry.Description }

func (e *extractorProxy) Extract(ctx context.Context, html, url string, params map[string]any) (any, error) {
	client, err := e.manager.getClient(e.manifest)
	if err != nil {
		return nil, fmt.Errorf("plugin: start %s: %w", e.manifest.Name, err)
	}

	reqParams := map[string]any{
		"name":   e.entry.Name,
		"html":   html,
		"url":    url,
		"params": params,
	}

	result, err := client.Call(ctx, "extract", reqParams)
	if err != nil {
		return nil, fmt.Errorf("plugin: extract %s: %w", e.entry.Name, err)
	}

	var data any
	if err := json.Unmarshal(result, &data); err != nil {
		return nil, fmt.Errorf("plugin: unmarshal extract result: %w", err)
	}

	return data, nil
}
