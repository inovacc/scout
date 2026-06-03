package browser

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// RegistryFile is the JSON file tracking downloaded browsers.
const RegistryFile = "installed.json"

// LoadRegistry reads ~/.scout/browsers/installed.json.
// Returns an empty slice (not error) if the file doesn't exist.
func LoadRegistry() ([]BrowserEntry, error) {
	cacheDir, err := CacheDir()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(filepath.Join(cacheDir, RegistryFile))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("scout: read browser registry: %w", err)
	}

	var entries []BrowserEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("scout: parse browser registry: %w", err)
	}

	return entries, nil
}

// SaveRegistry writes the registry to ~/.scout/browsers/installed.json.
func SaveRegistry(entries []BrowserEntry) error {
	cacheDir, err := CacheDir()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("scout: marshal browser registry: %w", err)
	}

	return os.WriteFile(filepath.Join(cacheDir, RegistryFile), data, 0o644)
}

// RegisterBrowser adds or updates a browser entry in the registry.
// No-op if the exact entry (name+version+platform+binary) already exists.
func RegisterBrowser(name, version, binary string) {
	entries, _ := LoadRegistry()

	platform := runtime.GOOS + "_" + runtime.GOARCH

	// Check if already registered with same binary path.
	for _, e := range entries {
		if e.Name == name && e.Version == version && e.Platform == platform && e.Binary == binary {
			return // already registered
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)

	// Update existing entry for same name+version+platform, or append.
	found := false

	for i, e := range entries {
		if e.Name == name && e.Version == version && e.Platform == platform {
			entries[i].Binary = binary
			entries[i].Installed = now
			found = true

			break
		}
	}

	if !found {
		entries = append(entries, BrowserEntry{
			Name:      name,
			Version:   version,
			Binary:    binary,
			Platform:  platform,
			Installed: now,
		})
	}

	_ = SaveRegistry(entries)
}

// LookupRegistryBrowser finds the latest entry for a given browser name from the registry.
// Returns the binary path or empty string if not found (or binary no longer exists on disk).
func LookupRegistryBrowser(name string) string {
	entries, err := LoadRegistry()
	if err != nil || len(entries) == 0 {
		return ""
	}

	platform := runtime.GOOS + "_" + runtime.GOARCH

	// Walk backwards — later entries are newer.
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		if e.Name == name && e.Platform == platform && FileExists(e.Binary) {
			return e.Binary
		}
	}

	return ""
}

// ListDownloaded returns info about browsers in ~/.scout/browsers/.
func ListDownloaded() ([]DownloadedBrowser, error) {
	cacheDir, err := CacheDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("scout: read browsers dir: %w", err)
	}

	var browsers []DownloadedBrowser

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		b := DownloadedBrowser{Name: entry.Name()}

		versions, err := os.ReadDir(filepath.Join(cacheDir, entry.Name()))
		if err == nil {
			for _, v := range versions {
				if v.IsDir() {
					b.Versions = append(b.Versions, v.Name())
				}
			}
		}

		browsers = append(browsers, b)
	}

	return browsers, nil
}

// LatestCachedBin scans a browser cache directory for the latest version
// subdirectory containing binName. Returns the full path or empty string.
func LatestCachedBin(browserDir, binName string) string {
	entries, err := os.ReadDir(browserDir)
	if err != nil {
		return ""
	}

	for i := len(entries) - 1; i >= 0; i-- {
		if !entries[i].IsDir() {
			continue
		}

		p := filepath.Join(browserDir, entries[i].Name(), binName)
		if FileExists(p) {
			return p
		}
	}

	return ""
}
