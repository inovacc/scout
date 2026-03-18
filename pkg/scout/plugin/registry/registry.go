// Package registry provides plugin discovery, version management, and integrity
// verification for Scout plugins distributed via GitHub Releases.
package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Index is the plugin registry index (JSON file hosted on GitHub).
type Index struct {
	Version  string        `json:"version"`
	Updated  time.Time     `json:"updated"`
	Plugins  []PluginInfo  `json:"plugins"`
}

// PluginInfo describes a plugin in the registry.
type PluginInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Author      string   `json:"author"`
	Repo        string   `json:"repo"`          // e.g. "inovacc/scout-diag"
	Latest      string   `json:"latest"`        // latest version tag
	Tags        []string `json:"tags,omitempty"` // search tags
}

// LockFile tracks installed plugin versions and checksums.
type LockFile struct {
	Plugins []LockedPlugin `json:"plugins"`
}

// LockedPlugin is a single entry in the lock file.
type LockedPlugin struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Checksum  string `json:"checksum"` // SHA256 of installed binary
	Repo      string `json:"repo,omitempty"`
	Installed string `json:"installed"` // RFC3339
}

// DefaultIndexURL is the default plugin registry URL.
const DefaultIndexURL = "https://raw.githubusercontent.com/inovacc/scout/main/plugins/registry.json"

// FetchIndex downloads and parses the plugin registry index.
func FetchIndex(url string) (*Index, error) {
	if url == "" {
		url = DefaultIndexURL
	}

	resp, err := http.Get(url) //nolint:gosec,noctx
	if err != nil {
		return nil, fmt.Errorf("registry: fetch index: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry: fetch index: HTTP %d", resp.StatusCode)
	}

	var index Index
	if err := json.NewDecoder(resp.Body).Decode(&index); err != nil {
		return nil, fmt.Errorf("registry: parse index: %w", err)
	}

	return &index, nil
}

// Search filters plugins by query (matches name, description, tags).
func (idx *Index) Search(query string) []PluginInfo {
	if query == "" {
		return idx.Plugins
	}

	q := strings.ToLower(query)
	var results []PluginInfo

	for _, p := range idx.Plugins {
		if matches(p, q) {
			results = append(results, p)
		}
	}

	return results
}

func matches(p PluginInfo, q string) bool {
	if strings.Contains(strings.ToLower(p.Name), q) {
		return true
	}

	if strings.Contains(strings.ToLower(p.Description), q) {
		return true
	}

	for _, tag := range p.Tags {
		if strings.Contains(strings.ToLower(tag), q) {
			return true
		}
	}

	return false
}

// archiveExt returns the platform-appropriate archive extension.
func archiveExt() string {
	if runtime.GOOS == "windows" {
		return "zip"
	}

	return "tar.gz"
}

// ReleaseURL constructs the download URL for a plugin release asset.
func ReleaseURL(repo, version, pluginName string) string {
	return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s-%s-%s.%s",
		repo, version, pluginName, runtime.GOOS, runtime.GOARCH, archiveExt())
}

// LatestReleaseURL constructs the latest release download URL.
func LatestReleaseURL(repo, pluginName string) string {
	return fmt.Sprintf("https://github.com/%s/releases/latest/download/%s-%s-%s.%s",
		repo, pluginName, runtime.GOOS, runtime.GOARCH, archiveExt())
}

// LockFilePath returns the path to the plugin lock file.
func LockFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".scout", "plugins", "lock.json"), nil
}

// LoadLockFile reads the lock file from disk.
func LoadLockFile() (*LockFile, error) {
	path, err := LockFilePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		if os.IsNotExist(err) {
			return &LockFile{}, nil
		}

		return nil, fmt.Errorf("registry: read lock: %w", err)
	}

	var lf LockFile
	if err := json.Unmarshal(data, &lf); err != nil {
		return nil, fmt.Errorf("registry: parse lock: %w", err)
	}

	return &lf, nil
}

// Save writes the lock file to disk.
func (lf *LockFile) Save() error {
	path, err := LockFilePath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("registry: create lock dir: %w", err)
	}

	data, err := json.MarshalIndent(lf, "", "  ")
	if err != nil {
		return fmt.Errorf("registry: marshal lock: %w", err)
	}

	return os.WriteFile(path, data, 0o600)
}

// Lock adds or updates a plugin in the lock file.
func (lf *LockFile) Lock(name, version, checksum, repo string) {
	for i, p := range lf.Plugins {
		if p.Name == name {
			lf.Plugins[i].Version = version
			lf.Plugins[i].Checksum = checksum
			lf.Plugins[i].Repo = repo
			lf.Plugins[i].Installed = time.Now().UTC().Format(time.RFC3339)

			return
		}
	}

	lf.Plugins = append(lf.Plugins, LockedPlugin{
		Name:      name,
		Version:   version,
		Checksum:  checksum,
		Repo:      repo,
		Installed: time.Now().UTC().Format(time.RFC3339),
	})
}

// Get returns a locked plugin by name.
func (lf *LockFile) Get(name string) *LockedPlugin {
	for i, p := range lf.Plugins {
		if p.Name == name {
			return &lf.Plugins[i]
		}
	}

	return nil
}

// FileChecksum returns the SHA256 hex digest of a file.
func FileChecksum(path string) (string, error) {
	f, err := os.Open(path) //nolint:gosec
	if err != nil {
		return "", err
	}

	defer func() { _ = f.Close() }()

	h := sha256.New()

	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerifyChecksum checks if a file matches the expected SHA256 checksum.
func VerifyChecksum(path, expected string) error {
	actual, err := FileChecksum(path)
	if err != nil {
		return fmt.Errorf("registry: checksum: %w", err)
	}

	if actual != expected {
		return fmt.Errorf("registry: checksum mismatch: got %s, want %s", actual, expected)
	}

	return nil
}
