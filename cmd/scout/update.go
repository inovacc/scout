package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.AddCommand(updateCheckCmd)
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update scout to the latest release",
	Long: `Download and install the latest scout release from GitHub.

Checks https://api.github.com/repos/inovacc/scout/releases/latest for the
newest version, compares it with the current build, and replaces the running
binary if a newer version is available.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		current := Version
		out := cmd.OutOrStdout()

		_, _ = fmt.Fprintf(out, "Current version: %s\n", current)
		_, _ = fmt.Fprintf(out, "Checking for updates...\n")

		release, err := fetchLatestRelease()
		if err != nil {
			return fmt.Errorf("scout: update: %w", err)
		}

		_, _ = fmt.Fprintf(out, "Latest version:  %s\n", release.TagName)

		if !isNewer(current, release.TagName) {
			_, _ = fmt.Fprintf(out, "Already up to date.\n")
			return nil
		}

		assetName := buildAssetName()
		assetURL := ""

		for _, a := range release.Assets {
			if a.Name == assetName {
				assetURL = a.BrowserDownloadURL
				break
			}
		}

		if assetURL == "" {
			return fmt.Errorf("scout: update: no release asset %q found for %s", assetName, release.TagName)
		}

		_, _ = fmt.Fprintf(out, "Downloading %s ...\n", assetURL)

		if err := selfReplace(assetURL); err != nil {
			return fmt.Errorf("scout: update: %w", err)
		}

		_, _ = fmt.Fprintf(out, "Updated successfully to %s\n", release.TagName)

		return nil
	},
}

var updateCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check if a newer version is available",
	RunE: func(cmd *cobra.Command, _ []string) error {
		current := Version
		out := cmd.OutOrStdout()

		_, _ = fmt.Fprintf(out, "Current version: %s\n", current)

		release, err := fetchLatestRelease()
		if err != nil {
			return fmt.Errorf("scout: update: check: %w", err)
		}

		_, _ = fmt.Fprintf(out, "Latest version:  %s\n", release.TagName)

		if isNewer(current, release.TagName) {
			_, _ = fmt.Fprintf(out, "Update available! Run 'scout update' to install.\n")
		} else {
			_, _ = fmt.Fprintf(out, "Already up to date.\n")
		}

		return nil
	},
}

// githubRelease is the subset of the GitHub release JSON we care about.
type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

const releaseURL = "https://api.github.com/repos/inovacc/scout/releases/latest"

func fetchLatestRelease() (*githubRelease, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	req, err := http.NewRequest(http.MethodGet, releaseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch release: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api returned %s", resp.Status)
	}

	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("decode release json: %w", err)
	}

	return &rel, nil
}

// buildAssetName returns the expected asset filename for the current platform.
// Convention: scout-{os}-{arch}[.exe]
func buildAssetName() string {
	name := fmt.Sprintf("scout-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}

	return name
}

// isNewer returns true when the remote tag is different from the local version
// and the local version looks like a dev build or is strictly older.
func isNewer(current, remote string) bool {
	remote = strings.TrimPrefix(remote, "v")
	current = strings.TrimPrefix(current, "v")

	// Dev builds are always "updatable".
	if current == "dev" || current == "" {
		return true
	}

	return current != remote
}

// selfReplace downloads the binary from url and replaces the running executable.
func selfReplace(url string) error {
	client := &http.Client{Timeout: 5 * time.Minute}

	resp, err := client.Get(url) //nolint:noctx
	if err != nil {
		return fmt.Errorf("download binary: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download returned %s", resp.Status)
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate current executable: %w", err)
	}

	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}

	// Write to a temp file in the same directory (ensures same filesystem for rename).
	dir := filepath.Dir(exe)

	tmp, err := os.CreateTemp(dir, "scout-update-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	tmpPath := tmp.Name()

	defer func() { _ = os.Remove(tmpPath) }() // clean up on failure

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	// Make executable on Unix.
	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmpPath, 0o755); err != nil {
			return fmt.Errorf("chmod: %w", err)
		}
	}

	// Platform-specific replacement.
	if runtime.GOOS == "windows" {
		return selfReplaceWindows(exe, tmpPath)
	}

	return selfReplaceUnix(exe, tmpPath)
}

// selfReplaceUnix atomically renames the temp file over the current executable.
func selfReplaceUnix(exe, tmpPath string) error {
	if err := os.Rename(tmpPath, exe); err != nil {
		return fmt.Errorf("rename over executable: %w", err)
	}

	return nil
}

// selfReplaceWindows works around the Windows file-lock limitation:
// 1. Rename current executable to .old
// 2. Rename new binary into place
// 3. Best-effort remove .old
func selfReplaceWindows(exe, tmpPath string) error {
	oldPath := exe + ".old"

	// Remove leftover .old from a previous update.
	_ = os.Remove(oldPath)

	if err := os.Rename(exe, oldPath); err != nil {
		return fmt.Errorf("rename current to .old: %w", err)
	}

	if err := os.Rename(tmpPath, exe); err != nil {
		// Try to restore the original.
		_ = os.Rename(oldPath, exe)
		return fmt.Errorf("rename new binary into place: %w", err)
	}

	// Best-effort cleanup; the file may still be locked.
	_ = os.Remove(oldPath)

	return nil
}
