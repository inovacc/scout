package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

// Manifest describes a plugin's capabilities and how to launch it.
type Manifest struct {
	Name         string           `json:"name"`
	Version      string           `json:"version"`
	Description  string           `json:"description"`
	Author       string           `json:"author,omitempty"`
	Command      string           `json:"command"`
	Capabilities []string         `json:"capabilities"`
	Modes        []ModeEntry      `json:"modes,omitempty"`
	Extractors   []ExtractorEntry `json:"extractors,omitempty"`
	Tools        []ToolEntry      `json:"tools,omitempty"`

	// Dir is the directory containing the manifest (set during loading).
	Dir string `json:"-"`
}

// ModeEntry declares a scraper mode provided by the plugin.
type ModeEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ExtractorEntry declares an extractor provided by the plugin.
type ExtractorEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// ToolEntry declares an MCP tool provided by the plugin.
type ToolEntry struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema,omitempty"`
}

// LoadManifest reads and validates a plugin.json from the given directory.
func LoadManifest(dir string) (*Manifest, error) {
	path := filepath.Join(dir, "plugin.json")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("plugin: read manifest: %w", err)
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("plugin: parse manifest %s: %w", path, err)
	}

	if err := m.validate(); err != nil {
		return nil, fmt.Errorf("plugin: invalid manifest %s: %w", path, err)
	}

	m.Dir = dir

	return &m, nil
}

func (m *Manifest) validate() error {
	if m.Name == "" {
		return fmt.Errorf("name is required")
	}

	if m.Version == "" {
		return fmt.Errorf("version is required")
	}

	if m.Command == "" {
		return fmt.Errorf("command is required")
	}

	if len(m.Capabilities) == 0 {
		return fmt.Errorf("at least one capability is required")
	}

	valid := map[string]bool{"scraper_mode": true, "extractor": true, "mcp_tool": true}
	for _, c := range m.Capabilities {
		if !valid[c] {
			return fmt.Errorf("unknown capability: %s", c)
		}
	}

	return nil
}

// HasCapability returns true if the manifest declares the given capability.
func (m *Manifest) HasCapability(capability string) bool {
	return slices.Contains(m.Capabilities, capability)
}

// CommandPath returns the absolute path to the plugin command.
func (m *Manifest) CommandPath() string {
	if filepath.IsAbs(m.Command) {
		return m.Command
	}

	return filepath.Join(m.Dir, m.Command)
}
