package browser

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

// Supported browser type constants.
const (
	TypeChrome = "chrome"
	TypeBrave  = "brave"
	TypeEdge   = "edge"
)

// ErrNotFound is returned when the requested browser executable cannot be located.
var ErrNotFound = errors.New("browser not found")

// ErrUnknownType is returned for unsupported browser type strings.
var ErrUnknownType = errors.New("unknown browser type")

// BrowserInfo describes a detected or downloaded browser.
type BrowserInfo struct {
	Name       string `json:"name"`       // Human-readable name, e.g. "Google Chrome"
	Type       string `json:"type"`       // TypeChrome, TypeBrave, TypeEdge
	Path       string `json:"path"`       // Absolute path to the executable
	Version    string `json:"version"`    // Version string, e.g. "120.0.6099.109"
	Downloaded bool   `json:"downloaded"` // True if managed by the cache (downloaded)
}

// Manager provides browser detection, download, and cache management.
type Manager struct {
	cacheDir string
	logger   *slog.Logger
	mu       sync.Mutex
}

// ManagerOption configures a Manager.
type ManagerOption func(*Manager)

// WithCacheDir sets the directory for downloaded browser caches.
// Defaults to ~/.scout/browsers/.
func WithCacheDir(dir string) ManagerOption {
	return func(m *Manager) { m.cacheDir = dir }
}

// WithLogger sets a structured logger for the manager.
func WithLogger(logger *slog.Logger) ManagerOption {
	return func(m *Manager) { m.logger = logger }
}

// NewManager creates a new browser Manager with the given options.
func NewManager(opts ...ManagerOption) *Manager {
	m := &Manager{
		logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})),
	}
	for _, o := range opts {
		o(m)
	}
	return m
}

// ensureCacheDir resolves and creates the cache directory if needed.
func (m *Manager) ensureCacheDir() (string, error) {
	if m.cacheDir != "" {
		if err := os.MkdirAll(m.cacheDir, 0o755); err != nil {
			return "", fmt.Errorf("browser: create cache dir: %w", err)
		}
		return m.cacheDir, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("browser: user home dir: %w", err)
	}

	dir := filepath.Join(home, ".scout", "browsers")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("browser: create cache dir: %w", err)
	}

	m.cacheDir = dir
	return dir, nil
}

// Download downloads a browser of the given type to the cache and returns
// the path to the executable. Supported types: "chrome", "brave", "edge".
func (m *Manager) Download(browserType string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cacheDir, err := m.ensureCacheDir()
	if err != nil {
		return "", err
	}

	switch browserType {
	case TypeChrome:
		return DownloadChrome(cacheDir)
	case TypeBrave:
		return DownloadBrave(context.Background(), cacheDir)
	case TypeEdge:
		return DownloadEdge(cacheDir)
	default:
		return "", fmt.Errorf("browser: download: %w: %q", ErrUnknownType, browserType)
	}
}

// Resolve finds an existing browser of the given type, or downloads it if not found.
// For "chrome", uses rod launcher auto-download. For "brave", downloads from GitHub.
// For "edge", returns an error with a download URL if not installed.
func (m *Manager) Resolve(browserType string) (string, error) {
	switch browserType {
	case TypeChrome, TypeBrave, TypeEdge:
		// ok
	default:
		return "", fmt.Errorf("browser: resolve: %w: %q", ErrUnknownType, browserType)
	}

	// Try local detection first.
	info, err := DetectByType(browserType)
	if err == nil {
		return info.Path, nil
	}

	// Not found locally — try download.
	m.logger.Info("browser not found locally, downloading", "type", browserType)
	return m.Download(browserType)
}

// List returns all detected system browsers and downloaded browsers from the cache.
func (m *Manager) List() ([]BrowserInfo, error) {
	// System-installed browsers.
	detected, err := Detect()
	if err != nil {
		return nil, err
	}

	// Downloaded browsers from cache.
	cacheDir, _ := m.ensureCacheDir()
	if cacheDir == "" {
		return detected, nil
	}

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return detected, nil
		}
		return nil, fmt.Errorf("browser: read cache dir: %w", err)
	}

	// Build a set of detected paths to avoid duplicates.
	detectedPaths := make(map[string]bool, len(detected))
	for _, d := range detected {
		detectedPaths[d.Path] = true
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := entry.Name()
		var browserType, friendlyName string
		switch {
		case len(name) >= 5 && name[:5] == "brave":
			browserType = TypeBrave
			friendlyName = "Brave Browser"
		case len(name) >= 6 && name[:6] == "chrome":
			browserType = TypeChrome
			friendlyName = "Google Chrome"
		default:
			continue
		}

		binPath := guessCachedBinPath(cacheDir, name, browserType)
		if binPath == "" || !fileExists(binPath) {
			continue
		}

		if detectedPaths[binPath] {
			continue
		}

		version := probeBrowserVersion(binPath)
		detected = append(detected, BrowserInfo{
			Name:       friendlyName + " (downloaded)",
			Type:       browserType,
			Path:       binPath,
			Version:    version,
			Downloaded: true,
		})
	}

	return detected, nil
}

// Clean removes all downloaded browsers from the cache directory.
func (m *Manager) Clean() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cacheDir, err := m.ensureCacheDir()
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("browser: read cache dir: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(cacheDir, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("browser: remove %s: %w", entry.Name(), err)
		}
	}

	return nil
}

// guessCachedBinPath returns the expected binary path for a cached browser entry.
func guessCachedBinPath(cacheDir, dirName, browserType string) string {
	base := filepath.Join(cacheDir, dirName)
	switch browserType {
	case TypeBrave:
		return filepath.Join(base, braveBinPath())
	default:
		return ""
	}
}
