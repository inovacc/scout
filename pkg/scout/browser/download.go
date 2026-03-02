package browser

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/inovacc/scout/pkg/scout/rod/lib/launcher"
)

// downloadTimeout is the HTTP timeout for downloading browser archives.
const downloadTimeout = 5 * time.Minute

// braveAssets maps GOOS_GOARCH to the GitHub release asset filename pattern.
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

// DownloadChrome downloads Chrome/Chromium using rod's built-in launcher.
// Returns the path to the executable. Rod manages its own cache internally.
func DownloadChrome(_ string) (string, error) {
	binPath := launcher.NewBrowser().MustGet()
	return binPath, nil
}

// DownloadBrave downloads the latest Brave browser release from GitHub
// and extracts it to <cacheDir>/brave-<version>/. Returns the path to the executable.
func DownloadBrave(ctx context.Context, cacheDir string) (string, error) {
	version, err := latestBraveVersion(ctx)
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
		return "", fmt.Errorf("browser: no Brave release for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	url := fmt.Sprintf("https://github.com/brave/brave-browser/releases/download/v%s/%s", version, asset)

	data, err := downloadFile(ctx, url)
	if err != nil {
		return "", fmt.Errorf("browser: download brave: %w", err)
	}

	if err := os.RemoveAll(destDir); err != nil {
		return "", fmt.Errorf("browser: clean brave dir: %w", err)
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("browser: create brave dir: %w", err)
	}

	if err := extractZipArchive(data, destDir); err != nil {
		return "", fmt.Errorf("browser: extract brave: %w", err)
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(binPath, 0o755); err != nil {
			return "", fmt.Errorf("browser: chmod brave binary: %w", err)
		}
	}

	if !fileExists(binPath) {
		return "", fmt.Errorf("browser: brave binary not found at %s after extraction", binPath)
	}

	return binPath, nil
}

// DownloadEdge is a stub that returns an error with a download URL.
// Edge does not offer a programmatic download API.
func DownloadEdge(_ string) (string, error) {
	return "", fmt.Errorf("browser: %w: edge — download manually from https://www.microsoft.com/edge/download", ErrNotFound)
}

// Patch applies common patches to a browser installation to disable
// auto-update, telemetry, and first-run dialogs. Currently a no-op stub
// for future expansion.
func Patch(_ string) error {
	// Reserved for future: disable auto-update, telemetry, first-run flags.
	return nil
}

// latestBraveVersion fetches the latest Brave release tag from GitHub API.
func latestBraveVersion(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.github.com/repos/brave/brave-browser/releases/latest", nil)
	if err != nil {
		return "", fmt.Errorf("browser: create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("browser: fetch brave version: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("browser: github API returned HTTP %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("browser: decode github response: %w", err)
	}

	if release.TagName == "" {
		return "", fmt.Errorf("browser: empty tag_name in github response")
	}

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

// braveBinPath returns the relative path to the Brave executable within the archive.
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
		return nil, fmt.Errorf("browser: create request: %w", err)
	}

	client := &http.Client{Timeout: downloadTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("browser: HTTP %d from %s", resp.StatusCode, url)
	}

	return io.ReadAll(resp.Body)
}

// extractZipArchive extracts a zip archive to destDir.
func extractZipArchive(data []byte, destDir string) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("browser: open zip: %w", err)
	}

	for _, f := range zr.File {
		target := filepath.Join(destDir, f.Name) //nolint:gosec // trusted archive from GitHub releases

		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("browser: zip slip detected: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("browser: create dir %s: %w", f.Name, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("browser: create parent dir: %w", err)
		}

		if err := extractZipFile(f, target); err != nil {
			return err
		}
	}

	return nil
}

// extractZipFile extracts a single file from a zip archive.
func extractZipFile(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("browser: open zip entry %s: %w", f.Name, err)
	}
	defer func() { _ = rc.Close() }()

	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return fmt.Errorf("browser: create file %s: %w", f.Name, err)
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, rc); err != nil {
		return fmt.Errorf("browser: write file %s: %w", f.Name, err)
	}

	return nil
}
