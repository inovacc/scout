package browser

import (
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

	"github.com/inovacc/scout/pkg/scout/archive"
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

// DownloadChrome downloads Chromium and returns the path to the executable.
// The cacheDir parameter is the base browsers directory (e.g. ~/.scout/browsers/).
// If empty, it defaults to the standard browser cache directory.
func DownloadChrome(cacheDir string) (string, error) {
	return downloadChromium(context.Background(), cacheDir)
}

// downloadChromium implements Chromium download with CDN fallback.
func downloadChromium(ctx context.Context, cacheDir string) (string, error) {
	if cacheDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("browser: user home dir: %w", err)
		}

		cacheDir = filepath.Join(home, ".scout", "browsers")
	}

	revision := 1592198 // pinned Chromium revision

	conf, ok := chromiumHostConf[runtime.GOOS+"_"+runtime.GOARCH]
	if !ok {
		return "", fmt.Errorf("browser: no Chromium download for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	revStr := fmt.Sprintf("%d", revision)
	destDir := filepath.Join(cacheDir, "chromium", revStr)
	binPath := filepath.Join(destDir, chromiumBinPath())

	if fileExists(binPath) {
		return binPath, nil
	}

	urls := chromiumDownloadURLs(revision, conf)

	var (
		data  []byte
		dlErr error
	)

	for _, u := range urls {
		data, dlErr = downloadFile(ctx, u)
		if dlErr == nil {
			break
		}
	}

	if dlErr != nil {
		return "", fmt.Errorf("browser: download chromium: %w", dlErr)
	}

	_ = os.RemoveAll(destDir)

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("browser: create chromium dir: %w", err)
	}

	if err := archive.Extract(data, conf.zipName, destDir); err != nil {
		return "", fmt.Errorf("browser: extract chromium: %w", err)
	}

	if err := stripFirstDir(destDir); err != nil {
		return "", fmt.Errorf("browser: strip chromium dir: %w", err)
	}

	if runtime.GOOS != "windows" {
		_ = os.Chmod(binPath, 0o755)
	}

	if !fileExists(binPath) {
		return "", fmt.Errorf("browser: chromium binary not found at %s", binPath)
	}

	return binPath, nil
}

var chromiumHostConf = map[string]struct {
	urlPrefix string
	zipName   string
}{
	"darwin_amd64":  {"Mac", "chrome-mac.zip"},
	"darwin_arm64":  {"Mac_Arm", "chrome-mac.zip"},
	"linux_amd64":   {"Linux_x64", "chrome-linux.zip"},
	"windows_386":   {"Win", "chrome-win.zip"},
	"windows_amd64": {"Win_x64", "chrome-win.zip"},
}

func chromiumDownloadURLs(revision int, conf struct{ urlPrefix, zipName string }) []string {
	return []string{
		fmt.Sprintf("https://storage.googleapis.com/chromium-browser-snapshots/%s/%d/%s",
			conf.urlPrefix, revision, conf.zipName),
		fmt.Sprintf("https://registry.npmmirror.com/-/binary/chromium-browser-snapshots/%s/%d/%s",
			conf.urlPrefix, revision, conf.zipName),
	}
}

func chromiumBinPath() string {
	return map[string]string{
		"darwin":  filepath.Join("Chromium.app", "Contents", "MacOS", "Chromium"),
		"linux":   "chrome",
		"windows": "chrome.exe",
	}[runtime.GOOS]
}

// stripFirstDir promotes contents of a single top-level directory up one level.
func stripFirstDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	if len(entries) != 1 || !entries[0].IsDir() {
		return nil
	}

	innerDir := filepath.Join(dir, entries[0].Name())

	innerEntries, err := os.ReadDir(innerDir)
	if err != nil {
		return err
	}

	for _, e := range innerEntries {
		if err := os.Rename(filepath.Join(innerDir, e.Name()), filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}

	return os.Remove(innerDir)
}

// DownloadBrave downloads the latest Brave browser release from GitHub
// and extracts it to <cacheDir>/brave-<version>/. Returns the path to the executable.
func DownloadBrave(ctx context.Context, cacheDir string) (string, error) {
	version, err := latestBraveVersion(ctx)
	if err != nil {
		return "", err
	}

	destDir := filepath.Join(cacheDir, "brave", version)
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

	if err := archive.Extract(data, asset, destDir); err != nil {
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
