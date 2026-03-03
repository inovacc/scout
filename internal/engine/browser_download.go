package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/inovacc/scout/pkg/scout/archive"
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

// ChromiumRevisionDefault is the pinned Chromium snapshot revision.
const ChromiumRevisionDefault = 1592198

// chromiumRevisionPlaywright is the Playwright ARM64 Linux revision.
const chromiumRevisionPlaywright = 1124

// chromiumHostConf maps GOOS_GOARCH to Chromium snapshot URL parts.
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

// chromiumBins maps GOOS to the executable name within the extracted archive.
var chromiumBins = map[string]string{
	"darwin":  filepath.Join("Chromium.app", "Contents", "MacOS", "Chromium"),
	"linux":   "chrome",
	"windows": "chrome.exe",
}

// ChromiumDownloadURLs returns candidate download URLs for the given revision.
func ChromiumDownloadURLs(revision int) []string {
	conf, ok := chromiumHostConf[runtime.GOOS+"_"+runtime.GOARCH]
	if !ok {
		return nil
	}

	urls := []string{
		// Google CDN.
		fmt.Sprintf("https://storage.googleapis.com/chromium-browser-snapshots/%s/%d/%s",
			conf.urlPrefix, revision, conf.zipName),
		// NPM mirror.
		fmt.Sprintf("https://registry.npmmirror.com/-/binary/chromium-browser-snapshots/%s/%d/%s",
			conf.urlPrefix, revision, conf.zipName),
	}

	// Playwright CDN for ARM64 Linux.
	if runtime.GOOS == "linux" && runtime.GOARCH == "arm64" {
		urls = append(urls, fmt.Sprintf(
			"https://playwright.azureedge.net/builds/chromium/%d/chromium-linux-arm64.zip",
			chromiumRevisionPlaywright))
	}

	return urls
}

// DownloadChromium downloads Chromium at the given revision (or default) and
// extracts it to ~/.scout/browsers/chromium/<revision>/. Returns the executable path.
func DownloadChromium(ctx context.Context, revision int) (string, error) {
	if revision <= 0 {
		revision = ChromiumRevisionDefault
	}

	cacheDir, err := BrowserCacheDir()
	if err != nil {
		return "", err
	}

	revStr := fmt.Sprintf("%d", revision)
	destDir := filepath.Join(cacheDir, "chromium", revStr)
	binPath := filepath.Join(destDir, chromiumBinPath())

	// Already downloaded.
	if fileExists(binPath) {
		return binPath, nil
	}

	// Try LAST_CHANGE fallback if the pinned revision fails.
	urls := ChromiumDownloadURLs(revision)
	if len(urls) == 0 {
		return "", fmt.Errorf("scout: no Chromium download for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	var data []byte
	var dlErr error

	for _, u := range urls {
		data, dlErr = downloadFile(ctx, u)
		if dlErr == nil {
			break
		}
	}

	if dlErr != nil {
		// Fallback: try latest revision from LAST_CHANGE.
		if latest, ok := latestChromiumRevision(ctx); ok && latest != revision {
			for _, u := range ChromiumDownloadURLs(latest) {
				data, dlErr = downloadFile(ctx, u)
				if dlErr == nil {
					revStr = fmt.Sprintf("%d", latest)
					destDir = filepath.Join(cacheDir, "chromium", revStr)
					binPath = filepath.Join(destDir, chromiumBinPath())

					break
				}
			}
		}

		if dlErr != nil {
			return "", fmt.Errorf("scout: download chromium: %w", dlErr)
		}
	}

	if err := os.RemoveAll(destDir); err != nil {
		return "", fmt.Errorf("scout: clean chromium dir: %w", err)
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("scout: create chromium dir: %w", err)
	}

	conf := chromiumHostConf[runtime.GOOS+"_"+runtime.GOARCH]

	if err := archive.Extract(data, conf.zipName, destDir); err != nil {
		return "", fmt.Errorf("scout: extract chromium: %w", err)
	}

	// Chromium zips have a top-level dir (e.g. chrome-win/). Strip it.
	if err := stripFirstDir(destDir); err != nil {
		return "", fmt.Errorf("scout: strip chromium dir: %w", err)
	}

	if runtime.GOOS != "windows" {
		_ = os.Chmod(binPath, 0o755)
	}

	if !fileExists(binPath) {
		return "", fmt.Errorf("scout: chromium binary not found at %s after extraction", binPath)
	}

	return binPath, nil
}

// latestChromiumRevision queries Google's LAST_CHANGE endpoint.
func latestChromiumRevision(ctx context.Context) (int, bool) {
	conf, ok := chromiumHostConf[runtime.GOOS+"_"+runtime.GOARCH]
	if !ok {
		return 0, false
	}

	url := fmt.Sprintf("https://storage.googleapis.com/chromium-browser-snapshots/%s/LAST_CHANGE", conf.urlPrefix)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, false
	}

	client := &http.Client{Timeout: 15 * time.Second}

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			_ = resp.Body.Close()
		}

		return 0, false
	}

	defer func() { _ = resp.Body.Close() }()

	var buf [20]byte

	n, _ := resp.Body.Read(buf[:])
	body := strings.TrimSpace(string(buf[:n]))

	var rev int
	if _, err := fmt.Sscanf(body, "%d", &rev); err != nil {
		return 0, false
	}

	return rev, true
}

// chromiumBinPath returns the relative path to the Chromium executable.
func chromiumBinPath() string {
	bin, ok := chromiumBins[runtime.GOOS]
	if !ok {
		return "chrome"
	}

	return bin
}

// stripFirstDir removes a single top-level directory, promoting its contents up.
// Used for archives like chrome-win.zip that wrap everything in chrome-win/.
func stripFirstDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	// Only strip if there's exactly one directory entry.
	if len(entries) != 1 || !entries[0].IsDir() {
		return nil
	}

	innerDir := filepath.Join(dir, entries[0].Name())

	innerEntries, err := os.ReadDir(innerDir)
	if err != nil {
		return err
	}

	for _, e := range innerEntries {
		src := filepath.Join(innerDir, e.Name())
		dst := filepath.Join(dir, e.Name())

		if err := os.Rename(src, dst); err != nil {
			return err
		}
	}

	return os.Remove(innerDir)
}

// DownloadBrave downloads the latest Brave browser release from GitHub
// and extracts it to ~/.scout/browsers/brave/<version>/. Returns the
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

	destDir := filepath.Join(cacheDir, "brave", version)
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

	if err := archive.Extract(data, asset, destDir); err != nil {
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

// DownloadedBrowser describes a browser found in ~/.scout/browsers/.
type DownloadedBrowser struct {
	Name     string   // e.g. "chromium", "brave", "electron"
	Versions []string // e.g. ["1592198"], ["1.87.191"]
}

// ListDownloadedBrowsers returns info about browsers in ~/.scout/browsers/.
func ListDownloadedBrowsers() ([]DownloadedBrowser, error) {
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

// edgeUpdatesAPI is the Microsoft Edge update products endpoint.
const edgeUpdatesAPI = "https://edgeupdates.microsoft.com/api/products"

// edgeBins maps GOOS to the executable path within the extracted Edge install.
var edgeBins = map[string]string{
	"windows": filepath.Join("Microsoft", "Edge", "Application", "msedge.exe"),
	"linux":   filepath.Join("opt", "microsoft", "msedge", "msedge"),
	"darwin":  filepath.Join("Microsoft Edge.app", "Contents", "MacOS", "Microsoft Edge"),
}

// DownloadEdge downloads Microsoft Edge Stable from the official updates API
// and extracts it to ~/.scout/browsers/edge/<version>/. Returns the path to the executable.
//
// On Windows, Edge's Enterprise MSI is a bootstrapper (not extractable).
// We copy the locally installed Edge into the cache instead.
// On Linux, the .deb package is downloaded and extracted.
// On macOS, the .pkg is downloaded and extracted via pkgutil.
func DownloadEdge(ctx context.Context) (string, error) {
	if runtime.GOOS == "windows" {
		return downloadEdgeWindows(ctx)
	}

	return downloadEdgeUnix(ctx)
}

// downloadEdgeWindows copies the system-installed Edge into the browser cache.
func downloadEdgeWindows(ctx context.Context) (string, error) {
	systemPath, err := lookupBrowser(BrowserEdge)
	if err != nil {
		return "", fmt.Errorf("scout: edge not installed — download from https://www.microsoft.com/edge/download: %w", err)
	}

	// Edge is at e.g. C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe
	// The version dir is next to msedge.exe, e.g. .../Application/131.0.2903.86/
	appDir := filepath.Dir(systemPath)

	var version string

	entries, err := os.ReadDir(appDir)
	if err != nil {
		return "", fmt.Errorf("scout: read edge dir: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() && len(e.Name()) > 0 && e.Name()[0] >= '0' && e.Name()[0] <= '9' {
			version = e.Name()

			break
		}
	}

	if version == "" {
		// Use the binary directly without caching.
		return systemPath, nil
	}

	cacheDir, err := BrowserCacheDir()
	if err != nil {
		return "", err
	}

	destDir := filepath.Join(cacheDir, "edge", version)
	binPath := filepath.Join(destDir, "msedge.exe")

	if fileExists(binPath) {
		return binPath, nil
	}

	// Copy the entire Edge version directory to cache.
	srcDir := filepath.Join(appDir, version)

	if err := copyDir(srcDir, destDir); err != nil {
		return "", fmt.Errorf("scout: copy edge to cache: %w", err)
	}

	// Also copy msedge.exe and related files from Application/ root.
	for _, name := range []string{"msedge.exe", "msedge.dll", "msedge_elf.dll"} {
		src := filepath.Join(appDir, name)
		if fileExists(src) {
			data, err := os.ReadFile(src)
			if err == nil {
				_ = os.WriteFile(filepath.Join(destDir, name), data, 0o755)
			}
		}
	}

	if !fileExists(binPath) {
		return systemPath, nil
	}

	_, _ = fmt.Fprintf(os.Stderr, "scout: cached Edge %s to %s\n", version, destDir)

	return binPath, nil
}

// downloadEdgeUnix downloads and extracts Edge for Linux/macOS.
func downloadEdgeUnix(ctx context.Context) (string, error) {
	version, dlURL, err := latestEdgeRelease(ctx)
	if err != nil {
		return "", err
	}

	cacheDir, err := BrowserCacheDir()
	if err != nil {
		return "", err
	}

	destDir := filepath.Join(cacheDir, "edge", version)
	binPath := filepath.Join(destDir, edgeBinPath())

	if fileExists(binPath) {
		return binPath, nil
	}

	data, err := downloadFile(ctx, dlURL)
	if err != nil {
		return "", fmt.Errorf("scout: download edge: %w", err)
	}

	if err := os.RemoveAll(destDir); err != nil {
		return "", fmt.Errorf("scout: clean edge dir: %w", err)
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("scout: create edge dir: %w", err)
	}

	if err := extractEdge(data, dlURL, destDir); err != nil {
		return "", fmt.Errorf("scout: extract edge: %w", err)
	}

	if err := os.Chmod(binPath, 0o755); err != nil {
		return "", fmt.Errorf("scout: chmod edge binary: %w", err)
	}

	if !fileExists(binPath) {
		return "", fmt.Errorf("scout: edge binary not found at %s after extraction", binPath)
	}

	return binPath, nil
}

// copyDir recursively copies src directory to dst.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		target := filepath.Join(dst, rel)

		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		return os.WriteFile(target, data, info.Mode())
	})
}

// latestEdgeRelease queries the Edge updates API for the latest Stable version and download URL.
func latestEdgeRelease(ctx context.Context) (version, downloadURL string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, edgeUpdatesAPI, nil)
	if err != nil {
		return "", "", fmt.Errorf("scout: create request: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("scout: fetch edge updates: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("scout: edge updates API returned HTTP %d", resp.StatusCode)
	}

	var products []struct {
		Product  string `json:"Product"`
		Releases []struct {
			Platform       string `json:"Platform"`
			Architecture   string `json:"Architecture"`
			ProductVersion string `json:"ProductVersion"`
			Artifacts      []struct {
				ArtifactName string `json:"ArtifactName"`
				Location     string `json:"Location"`
			} `json:"Artifacts"`
		} `json:"Releases"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&products); err != nil {
		return "", "", fmt.Errorf("scout: decode edge updates: %w", err)
	}

	wantPlatform, wantArch := edgePlatformArch()

	for _, p := range products {
		if p.Product != "Stable" {
			continue
		}

		for _, r := range p.Releases {
			if !strings.EqualFold(r.Platform, wantPlatform) || !strings.EqualFold(r.Architecture, wantArch) {
				continue
			}

			for _, a := range r.Artifacts {
				if isEdgeArtifact(a.ArtifactName) {
					return r.ProductVersion, a.Location, nil
				}
			}
		}
	}

	return "", "", fmt.Errorf("scout: no Edge Stable release for %s/%s", runtime.GOOS, runtime.GOARCH)
}

// edgePlatformArch maps Go runtime to Edge API platform/architecture strings.
func edgePlatformArch() (platform, arch string) {
	switch runtime.GOOS {
	case "windows":
		platform = "Windows"
	case "darwin":
		platform = "MacOS"
	case "linux":
		platform = "Linux"
	default:
		platform = runtime.GOOS
	}

	switch runtime.GOARCH {
	case "amd64":
		arch = "x64"
	case "arm64":
		arch = "arm64"
	case "386":
		arch = "x86"
	default:
		arch = runtime.GOARCH
	}

	// macOS uses universal builds.
	if runtime.GOOS == "darwin" {
		arch = "universal"
	}

	return platform, arch
}

// isEdgeArtifact returns true for the artifact type we can extract on this OS.
func isEdgeArtifact(name string) bool {
	switch runtime.GOOS {
	case "windows":
		return name == "msi"
	case "linux":
		return name == "deb"
	case "darwin":
		return name == "pkg"
	default:
		return false
	}
}

// extractEdge extracts the Edge installer based on file type.
func extractEdge(data []byte, dlURL, destDir string) error {
	lower := strings.ToLower(dlURL)

	switch {
	case strings.HasSuffix(lower, ".msi"):
		return extractMSI(data, destDir)
	case strings.HasSuffix(lower, ".deb"):
		return archive.Extract(data, "edge.deb", destDir)
	case strings.HasSuffix(lower, ".pkg"):
		return extractMacPKG(data, destDir)
	default:
		return fmt.Errorf("unsupported edge installer format: %s", filepath.Base(dlURL))
	}
}

// extractMSI extracts an MSI archive. Tries 7z first (handles MSI natively),
// falls back to msiexec /a on Windows.
func extractMSI(data []byte, destDir string) error {
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("scout-edge-%d.msi", time.Now().UnixNano()))

	if err := os.WriteFile(tmpFile, data, 0o644); err != nil {
		return fmt.Errorf("write temp msi: %w", err)
	}

	defer func() { _ = os.Remove(tmpFile) }()

	// Try 7z first — handles MSI natively and works cross-platform.
	if sevenZip, err := exec.LookPath("7z"); err == nil {
		cmd := exec.Command(sevenZip, "x", "-y", "-o"+destDir, tmpFile)

		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("7z extract msi: %w\n%s", err, string(output))
		}

		return nil
	}

	// Fallback: msiexec /a (Windows only).
	if runtime.GOOS != "windows" {
		return fmt.Errorf("7z not found — install 7-Zip to extract MSI on this platform")
	}

	cmdLine := fmt.Sprintf(`msiexec /a "%s" /qn TARGETDIR="%s"`, filepath.Clean(tmpFile), filepath.Clean(destDir))
	cmd := exec.Command("cmd", "/c", cmdLine)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("msiexec extract: %w\n%s", err, string(output))
	}

	// msiexec copies the MSI into TARGETDIR; remove it.
	_ = os.Remove(filepath.Join(destDir, filepath.Base(tmpFile)))

	return nil
}

// extractMacPKG extracts a macOS .pkg using pkgutil on macOS.
func extractMacPKG(data []byte, destDir string) error {
	tmpFile := filepath.Join(os.TempDir(), "scout-edge-"+fmt.Sprintf("%d", time.Now().UnixNano())+".pkg")

	if err := os.WriteFile(tmpFile, data, 0o644); err != nil {
		return fmt.Errorf("write temp pkg: %w", err)
	}

	defer func() { _ = os.Remove(tmpFile) }()

	cmd := exec.Command("pkgutil", "--expand-full", tmpFile, destDir)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pkgutil extract: %w\n%s", err, string(output))
	}

	return nil
}

// edgeBinPath returns the relative path to the Edge executable within the extracted install.
func edgeBinPath() string {
	bin, ok := edgeBins[runtime.GOOS]
	if !ok {
		return "msedge"
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
		return DownloadEdge(ctx)
	default:
		return "", err
	}
}

// isNotFound checks if the error wraps ErrBrowserNotFound.
func isNotFound(err error) bool {
	return errors.Is(err, ErrBrowserNotFound)
}
