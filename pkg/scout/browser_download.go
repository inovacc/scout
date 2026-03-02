package scout

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// browserDownloadTimeout is the HTTP timeout for downloading browser archives.
const browserDownloadTimeout = 5 * time.Minute

// braveAssets maps GOOS_GOARCH to the GitHub release asset filename pattern.
// The %s placeholder is replaced with the version number (without "v" prefix).
var braveAssets = map[string]string{
	"windows_amd64": "brave-v%s-win32-x64.zip",
	"windows_arm64": "brave-v%s-win32-arm64.zip",
	"darwin_amd64":  "brave-v%s-darwin-x64.zip",
	"darwin_arm64":  "brave-v%s-darwin-arm64.zip",
	"linux_amd64":   "brave-browser-%s-linux-amd64.zip",
	"linux_arm64":   "brave-browser-%s-linux-arm64.zip",
}

// braveBins maps GOOS to the executable path within the extracted archive.
var braveBins = map[string]string{
	"windows": "brave.exe",
	"darwin":  "Brave Browser.app/Contents/MacOS/Brave Browser",
	"linux":   "brave",
}

// BrowserCacheDir returns the path to ~/.scout/browsers/, creating it if needed.
func BrowserCacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("scout: user home dir: %w", err)
	}

	dir := filepath.Join(home, ".scout", "browsers")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("scout: create browsers dir: %w", err)
	}

	return dir, nil
}

// DownloadBrave downloads the latest Brave browser release from GitHub
// and extracts it to ~/.scout/browsers/brave-<version>/. Returns the
// path to the executable.
func DownloadBrave(ctx context.Context) (string, error) {
	version, err := latestBraveVersion(ctx)
	if err != nil {
		return "", err
	}

	cacheDir, err := BrowserCacheDir()
	if err != nil {
		return "", err
	}

	destDir := filepath.Join(cacheDir, "brave-"+version)
	binPath := filepath.Join(destDir, braveBinPath())

	// Already downloaded.
	if fileExists(binPath) {
		return binPath, nil
	}

	asset := braveAssetName(version)
	if asset == "" {
		return "", fmt.Errorf("scout: no Brave release available for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	url := fmt.Sprintf("https://github.com/brave/brave-browser/releases/download/v%s/%s", version, asset)

	data, err := downloadFile(ctx, url)
	if err != nil {
		return "", fmt.Errorf("scout: download brave: %w", err)
	}

	// Clean and recreate dest dir.
	if err := os.RemoveAll(destDir); err != nil {
		return "", fmt.Errorf("scout: clean brave dir: %w", err)
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("scout: create brave dir: %w", err)
	}

	if err := extractZipArchive(data, destDir); err != nil {
		return "", fmt.Errorf("scout: extract brave: %w", err)
	}

	// Make binary executable on Unix.
	if runtime.GOOS != "windows" {
		if err := os.Chmod(binPath, 0o755); err != nil {
			return "", fmt.Errorf("scout: chmod brave binary: %w", err)
		}
	}

	if !fileExists(binPath) {
		return "", fmt.Errorf("scout: brave binary not found at %s after extraction", binPath)
	}

	return binPath, nil
}

// ListDownloadedBrowsers returns info about browsers in ~/.scout/browsers/.
func ListDownloadedBrowsers() ([]string, error) {
	cacheDir, err := BrowserCacheDir()
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

	var browsers []string

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		browsers = append(browsers, entry.Name())
	}

	return browsers, nil
}

// latestBraveVersion fetches the latest Brave release tag from GitHub API.
func latestBraveVersion(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.github.com/repos/brave/brave-browser/releases/latest", nil)
	if err != nil {
		return "", fmt.Errorf("scout: create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 30 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("scout: fetch brave version: %w", err)
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

	// Tag is "vX.Y.Z", strip the "v" prefix.
	return strings.TrimPrefix(release.TagName, "v"), nil
}

// braveAssetName returns the zip filename for the current platform and version.
func braveAssetName(version string) string {
	key := runtime.GOOS + "_" + runtime.GOARCH

	pattern, ok := braveAssets[key]
	if !ok {
		return ""
	}

	return fmt.Sprintf(pattern, version)
}

// braveBinPath returns the relative path to the Brave executable within the extracted archive.
func braveBinPath() string {
	bin, ok := braveBins[runtime.GOOS]
	if !ok {
		return "brave"
	}

	return bin
}

// downloadFile fetches a URL and returns the response body.
func downloadFile(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("scout: create request: %w", err)
	}

	client := &http.Client{Timeout: browserDownloadTimeout}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	return io.ReadAll(resp.Body)
}

// extractZipArchive extracts a zip archive to destDir.
func extractZipArchive(data []byte, destDir string) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}

	for _, f := range zr.File {
		target := filepath.Join(destDir, f.Name) //nolint:gosec // trusted archive from GitHub releases

		// Zip slip protection.
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("zip slip detected: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("create dir %s: %w", f.Name, err)
			}

			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("create parent dir: %w", err)
		}

		if err := extractZipFile(f, target); err != nil {
			return err
		}
	}

	return nil
}

// resolveBrowser tries local lookup first, then falls back to auto-download.
func resolveBrowser(ctx context.Context, bt BrowserType) (string, error) {
	path, err := lookupBrowser(bt)
	if err == nil {
		return path, nil
	}

	if !isNotFound(err) {
		return "", err
	}

	switch bt { //nolint:exhaustive
	case BrowserBrave:
		return DownloadBrave(ctx)
	case BrowserEdge:
		return "", fmt.Errorf("%w: edge — download from https://www.microsoft.com/edge/download", ErrBrowserNotFound)
	default:
		return "", err
	}
}

// isNotFound checks if the error wraps ErrBrowserNotFound.
func isNotFound(err error) bool {
	return errors.Is(err, ErrBrowserNotFound)
}
