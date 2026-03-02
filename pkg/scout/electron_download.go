package scout

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// electronAssets maps GOOS_GOARCH to the GitHub release asset filename pattern.
// The %s placeholder is replaced with the version string (e.g. "v33.2.0").
var electronAssets = map[string]string{
	"windows_amd64": "electron-%s-win32-x64.zip",
	"windows_arm64": "electron-%s-win32-arm64.zip",
	"darwin_amd64":  "electron-%s-darwin-x64.zip",
	"darwin_arm64":  "electron-%s-darwin-arm64.zip",
	"linux_amd64":   "electron-%s-linux-x64.zip",
	"linux_arm64":   "electron-%s-linux-arm64.zip",
}

// electronBins maps GOOS to the executable path within the extracted archive.
var electronBins = map[string]string{
	"windows": "electron.exe",
	"darwin":  "Electron.app/Contents/MacOS/Electron",
	"linux":   "electron",
}

// ElectronCacheDir returns the path to ~/.scout/electron/, creating it if needed.
func ElectronCacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("scout: user home dir: %w", err)
	}

	dir := filepath.Join(home, ".scout", "electron")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("scout: create electron dir: %w", err)
	}

	return dir, nil
}

// latestElectronVersion fetches the latest Electron release tag from GitHub API.
func latestElectronVersion(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.github.com/repos/electron/electron/releases/latest", nil)
	if err != nil {
		return "", fmt.Errorf("scout: create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("scout: fetch electron version: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("scout: github API returned HTTP %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("scout: decode github response: %w", err)
	}

	if release.TagName == "" {
		return "", fmt.Errorf("scout: empty tag_name in github response")
	}

	return release.TagName, nil
}

// electronAssetName returns the zip filename for the current platform and version.
func electronAssetName(version string) string {
	key := runtime.GOOS + "_" + runtime.GOARCH
	pattern, ok := electronAssets[key]
	if !ok {
		return ""
	}

	return fmt.Sprintf(pattern, version)
}

// electronBinPath returns the relative path to the Electron executable within the extracted archive.
func electronBinPath() string {
	bin, ok := electronBins[runtime.GOOS]
	if !ok {
		return "electron"
	}

	return bin
}

// DownloadElectron downloads a specific Electron version from GitHub releases
// and extracts it to ~/.scout/electron/<version>/. Returns the path to the executable.
// The version should include the "v" prefix (e.g. "v33.2.0").
func DownloadElectron(ctx context.Context, version string) (string, error) {
	version = strings.TrimPrefix(version, "v")
	version = "v" + version // normalize to "vX.Y.Z"

	cacheDir, err := ElectronCacheDir()
	if err != nil {
		return "", err
	}

	destDir := filepath.Join(cacheDir, version)
	binPath := filepath.Join(destDir, electronBinPath())

	// Already downloaded.
	if fileExists(binPath) {
		return binPath, nil
	}

	asset := electronAssetName(version)
	if asset == "" {
		return "", fmt.Errorf("scout: no Electron release available for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	url := fmt.Sprintf("https://github.com/electron/electron/releases/download/%s/%s", version, asset)

	data, err := downloadFile(ctx, url)
	if err != nil {
		return "", fmt.Errorf("scout: download electron: %w", err)
	}

	if err := os.RemoveAll(destDir); err != nil {
		return "", fmt.Errorf("scout: clean electron dir: %w", err)
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("scout: create electron dir: %w", err)
	}

	if err := extractZipArchive(data, destDir); err != nil {
		return "", fmt.Errorf("scout: extract electron: %w", err)
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(binPath, 0o755); err != nil {
			return "", fmt.Errorf("scout: chmod electron binary: %w", err)
		}
	}

	if !fileExists(binPath) {
		return "", fmt.Errorf("scout: electron binary not found at %s after extraction", binPath)
	}

	return binPath, nil
}

// DownloadLatestElectron downloads the latest Electron release.
func DownloadLatestElectron(ctx context.Context) (string, error) {
	version, err := latestElectronVersion(ctx)
	if err != nil {
		return "", err
	}

	return DownloadElectron(ctx, version)
}
