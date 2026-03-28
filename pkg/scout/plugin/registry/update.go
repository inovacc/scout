package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// UpdateInfo describes an available update for an installed plugin.
type UpdateInfo struct {
	Name           string
	CurrentVersion string
	LatestVersion  string
	Repo           string
}

// CheckUpdates compares installed plugins against the registry index
// and returns a list of plugins with available updates.
func CheckUpdates(indexURL string) ([]UpdateInfo, error) {
	lock, err := LoadLockFile()
	if err != nil {
		return nil, fmt.Errorf("scout: plugin update check: %w", err)
	}

	if len(lock.Plugins) == 0 {
		return nil, nil
	}

	index, err := FetchIndex(indexURL)
	if err != nil {
		return nil, fmt.Errorf("scout: plugin update check: fetch index: %w", err)
	}

	var updates []UpdateInfo
	for _, installed := range lock.Plugins {
		for _, entry := range index.Plugins {
			if entry.Name == installed.Name && entry.Latest != installed.Version {
				updates = append(updates, UpdateInfo{
					Name:           installed.Name,
					CurrentVersion: installed.Version,
					LatestVersion:  entry.Latest,
					Repo:           entry.Repo,
				})
			}
		}
	}

	return updates, nil
}

// CheckUpdatesWithLock compares a provided lock file against the given index
// and returns a list of plugins with available updates. This variant is useful
// for testing where the lock file and index are constructed in-memory.
func CheckUpdatesWithLock(lock *LockFile, index *Index) []UpdateInfo {
	var updates []UpdateInfo
	for _, installed := range lock.Plugins {
		for _, entry := range index.Plugins {
			if entry.Name == installed.Name && entry.Latest != installed.Version {
				updates = append(updates, UpdateInfo{
					Name:           installed.Name,
					CurrentVersion: installed.Version,
					LatestVersion:  entry.Latest,
					Repo:           entry.Repo,
				})
			}
		}
	}

	return updates
}

// LastCheckFile returns the path to the file tracking when updates were last checked.
func LastCheckFile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, ".scout", "plugins", ".last-update-check")
}

// ShouldCheck returns true if enough time has passed since the last update check.
func ShouldCheck(interval time.Duration) bool {
	path := LastCheckFile()
	if path == "" {
		return true
	}

	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return true // never checked or unreadable
	}

	t, err := time.Parse(time.RFC3339, strings.TrimSpace(string(data)))
	if err != nil {
		return true
	}

	return time.Since(t) > interval
}

// MarkChecked writes the current time to the last-check file.
func MarkChecked() error {
	path := LastCheckFile()
	if path == "" {
		return fmt.Errorf("scout: plugin update check: cannot determine home directory")
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("scout: plugin update check: %w", err)
	}

	return os.WriteFile(path, []byte(time.Now().UTC().Format(time.RFC3339)), 0o644) //nolint:gosec
}
