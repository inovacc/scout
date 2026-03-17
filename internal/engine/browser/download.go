package browser

import (
	"context"
	"encoding/json"
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

// CacheDir returns the path to ~/.scout/browsers/, creating it if needed.
func CacheDir() (string, error) {
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

// ChromiumRevisionDefault is the pinned Chromium snapshot revision from browser.json.
var ChromiumRevisionDefault = LoadManifest().DefaultRevision()

// chromiumBins maps GOOS to the executable name within the extracted archive.
var chromiumBins = map[string]string{
	"darwin":  filepath.Join("Chromium.app", "Contents", "MacOS", "Chromium"),
	"linux":   "chrome",
	"windows": "chrome.exe",
}

// ChromiumDownloadURLs returns candidate download URLs for the given revision.
func ChromiumDownloadURLs(revision int) []string {
	m := LoadManifest()

	p := m.Platform()
	if p == nil {
		return nil
	}

	rev := fmt.Sprintf("%d", revision)

	var urls []string

	for _, tmpl := range p.URLs {
		urls = append(urls, strings.ReplaceAll(tmpl, "{revision}", rev))
	}

	return urls
}

// DownloadChromium downloads Chromium at the given revision (or default) and
// extracts it to ~/.scout/browsers/chromium/<revision>/. Returns the executable path.
func DownloadChromium(ctx context.Context, revision int) (string, error) {
	if revision <= 0 {
		revision = ChromiumRevisionDefault
	}

	cacheDir, err := CacheDir()
	if err != nil {
		return "", err
	}

	revStr := fmt.Sprintf("%d", revision)
	destDir := filepath.Join(cacheDir, "chromium", revStr)
	binPath := filepath.Join(destDir, chromiumBinPath())

	// Already downloaded.
	if FileExists(binPath) {
		RegisterBrowser("chromium", revStr, binPath)
		return binPath, nil
	}

	// Try LAST_CHANGE fallback if the pinned revision fails.
	urls := ChromiumDownloadURLs(revision)
	if len(urls) == 0 {
		return "", fmt.Errorf("scout: no Chromium download for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	var (
		data  []byte
		dlErr error
	)

	for _, u := range urls {
		data, dlErr = DownloadFile(ctx, u)
		if dlErr == nil {
			break
		}
	}

	if dlErr != nil {
		// Fallback: try latest revision from LAST_CHANGE.
		if latest, ok := latestChromiumRevision(ctx); ok && latest != revision {
			for _, u := range ChromiumDownloadURLs(latest) {
				data, dlErr = DownloadFile(ctx, u)
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

	p := LoadManifest().Platform()

	zipName := ""
	if p != nil {
		zipName = p.Zip
	}

	if err := archive.Extract(data, zipName, destDir); err != nil {
		return "", fmt.Errorf("scout: extract chromium: %w", err)
	}

	// Chromium zips have a top-level dir (e.g. chrome-win/). Strip it.
	if err := stripFirstDir(destDir); err != nil {
		return "", fmt.Errorf("scout: strip chromium dir: %w", err)
	}

	if runtime.GOOS != "windows" {
		_ = os.Chmod(binPath, 0o755)
	}

	if !FileExists(binPath) {
		return "", fmt.Errorf("scout: chromium binary not found at %s after extraction", binPath)
	}

	RegisterBrowser("chromium", revStr, binPath)

	return binPath, nil
}

// latestChromiumRevision queries Google's LAST_CHANGE endpoint.
func latestChromiumRevision(ctx context.Context) (int, bool) {
	url := LoadManifest().LatestAPI()
	if url == "" {
		return 0, false
	}

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

	cacheDir, err := CacheDir()
	if err != nil {
		return "", err
	}

	destDir := filepath.Join(cacheDir, "brave", version)
	binPath := filepath.Join(destDir, braveBinPath())

	// Already downloaded.
	if FileExists(binPath) {
		RegisterBrowser("brave", version, binPath)
		return binPath, nil
	}

	dlURL := LoadManifest().Brave.DownloadURL(version)
	if dlURL == "" {
		return "", fmt.Errorf("scout: no Brave release available for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	asset := braveAssetName(version)

	data, err := DownloadFile(ctx, dlURL)
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

	if !FileExists(binPath) {
		return "", fmt.Errorf("scout: brave binary not found at %s after extraction", binPath)
	}

	RegisterBrowser("brave", version, binPath)

	return binPath, nil
}

// chromeCfTBinPath returns the relative binary path for Chrome for Testing from browser.json.
func chromeCfTBinPath() string {
	return LoadManifest().Chrome.BinaryPath("chrome")
}

// chromeCfTPlatformID returns the CfT platform identifier for the current OS/arch from browser.json.
func chromeCfTPlatformID() string {
	p := LoadManifest().Chrome.BrowserPlatform()
	if p == nil {
		return ""
	}

	return p.PlatformID
}

// DownloadChrome downloads Google Chrome for Testing (latest Stable) and
// extracts it to ~/.scout/browsers/chrome/<version>/. Returns the executable path.
func DownloadChrome(ctx context.Context) (string, error) {
	version, dlURL, err := latestChromeForTesting(ctx)
	if err != nil {
		return "", err
	}

	cacheDir, err := CacheDir()
	if err != nil {
		return "", err
	}

	destDir := filepath.Join(cacheDir, "chrome", version)
	binPath := filepath.Join(destDir, chromeCfTBinPath())

	if FileExists(binPath) {
		RegisterBrowser("chrome", version, binPath)
		return binPath, nil
	}

	data, err := DownloadFile(ctx, dlURL)
	if err != nil {
		return "", fmt.Errorf("scout: download chrome: %w", err)
	}

	if err := os.RemoveAll(destDir); err != nil {
		return "", fmt.Errorf("scout: clean chrome dir: %w", err)
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("scout: create chrome dir: %w", err)
	}

	if err := archive.Extract(data, filepath.Base(dlURL), destDir); err != nil {
		return "", fmt.Errorf("scout: extract chrome: %w", err)
	}

	if runtime.GOOS != "windows" {
		_ = os.Chmod(binPath, 0o755)
	}

	if !FileExists(binPath) {
		return "", fmt.Errorf("scout: chrome binary not found at %s after extraction", binPath)
	}

	RegisterBrowser("chrome", version, binPath)

	return binPath, nil
}

// latestChromeForTesting queries the CfT API for the latest Stable version and download URL.
func latestChromeForTesting(ctx context.Context) (version, downloadURL string, err error) {
	apiURL := LoadManifest().Chrome.BrowserAPI("latest_stable")
	if apiURL == "" {
		return "", "", fmt.Errorf("scout: no Chrome for Testing API URL in browser.json")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("scout: create request: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("scout: fetch chrome versions: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("scout: chrome API returned HTTP %d", resp.StatusCode)
	}

	var result struct {
		Channels struct {
			Stable struct {
				Version   string `json:"version"`
				Downloads struct {
					Chrome []struct {
						Platform string `json:"platform"`
						URL      string `json:"url"`
					} `json:"chrome"`
				} `json:"downloads"`
			} `json:"Stable"`
		} `json:"channels"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", fmt.Errorf("scout: decode chrome response: %w", err)
	}

	stable := result.Channels.Stable
	if stable.Version == "" {
		return "", "", fmt.Errorf("scout: empty version in chrome API response")
	}

	wantPlatform := chromeCfTPlatformID()
	if wantPlatform == "" {
		return "", "", fmt.Errorf("scout: no Chrome for Testing for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	for _, dl := range stable.Downloads.Chrome {
		if dl.Platform == wantPlatform {
			return stable.Version, dl.URL, nil
		}
	}

	return "", "", fmt.Errorf("scout: no Chrome for Testing download for platform %s", wantPlatform)
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

// latestBraveVersion fetches the latest Brave release tag from GitHub API.
func latestBraveVersion(ctx context.Context) (string, error) {
	apiURL := LoadManifest().Brave.BrowserAPI("latest_release")
	if apiURL == "" {
		return "", fmt.Errorf("scout: no Brave API URL in browser.json")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
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

// braveAssetName returns the zip filename for the current platform and version from browser.json.
func braveAssetName(version string) string {
	return LoadManifest().Brave.ZipName(version)
}

// braveBinPath returns the relative path to the Brave executable from browser.json.
func braveBinPath() string {
	return LoadManifest().Brave.BinaryPath("brave")
}

// edgeBinPath returns the relative path to the Edge executable from browser.json.
func edgeBinPath() string {
	return LoadManifest().Edge.BinaryPath("msedge")
}

// DownloadEdge downloads Microsoft Edge Stable from the official updates API
// and extracts it to ~/.scout/browsers/edge/<version>/. Returns the path to the executable.
func DownloadEdge(ctx context.Context) (string, error) {
	if runtime.GOOS == "windows" {
		return downloadEdgeWindows(ctx)
	}

	return downloadEdgeUnix(ctx)
}

// downloadEdgeWindows copies the system-installed Edge into the browser cache.
// This uses lookupBrowser(Edge) intentionally — Windows has no standalone Edge
// download URL, so the only option is to copy from the system install path.
func downloadEdgeWindows(_ context.Context) (string, error) {
	systemPath, err := lookupBrowser(Edge)
	if err != nil {
		return "", fmt.Errorf("scout: edge not installed — download from https://www.microsoft.com/edge/download: %w", err)
	}

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
		return systemPath, nil
	}

	cacheDir, err := CacheDir()
	if err != nil {
		return "", err
	}

	destDir := filepath.Join(cacheDir, "edge", version)
	binPath := filepath.Join(destDir, "msedge.exe")

	if FileExists(binPath) {
		RegisterBrowser("edge", version, binPath)
		return binPath, nil
	}

	srcDir := filepath.Join(appDir, version)

	if err := copyDir(srcDir, destDir); err != nil {
		return "", fmt.Errorf("scout: copy edge to cache: %w", err)
	}

	for _, name := range []string{"msedge.exe", "msedge.dll", "msedge_elf.dll"} {
		src := filepath.Join(appDir, name)
		if FileExists(src) {
			data, err := os.ReadFile(src)
			if err == nil {
				_ = os.WriteFile(filepath.Join(destDir, name), data, 0o755)
			}
		}
	}

	if !FileExists(binPath) {
		return systemPath, nil
	}

	_, _ = fmt.Fprintf(os.Stderr, "scout: cached Edge %s to %s\n", version, destDir)

	RegisterBrowser("edge", version, binPath)

	return binPath, nil
}

// downloadEdgeUnix downloads and extracts Edge for Linux/macOS.
func downloadEdgeUnix(ctx context.Context) (string, error) {
	version, dlURL, err := latestEdgeRelease(ctx)
	if err != nil {
		return "", err
	}

	cacheDir, err := CacheDir()
	if err != nil {
		return "", err
	}

	destDir := filepath.Join(cacheDir, "edge", version)
	binPath := filepath.Join(destDir, edgeBinPath())

	if FileExists(binPath) {
		RegisterBrowser("edge", version, binPath)
		return binPath, nil
	}

	data, err := DownloadFile(ctx, dlURL)
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

	if !FileExists(binPath) {
		return "", fmt.Errorf("scout: edge binary not found at %s after extraction", binPath)
	}

	RegisterBrowser("edge", version, binPath)

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
	apiURL := LoadManifest().Edge.BrowserAPI("updates")
	if apiURL == "" {
		return "", "", fmt.Errorf("scout: no Edge updates API URL in browser.json")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
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

// extractMSI extracts an MSI archive.
func extractMSI(data []byte, destDir string) error {
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("scout-edge-%d.msi", time.Now().UnixNano()))

	if err := os.WriteFile(tmpFile, data, 0o644); err != nil {
		return fmt.Errorf("write temp msi: %w", err)
	}

	defer func() { _ = os.Remove(tmpFile) }()

	if sevenZip, err := exec.LookPath("7z"); err == nil {
		cmd := exec.Command(sevenZip, "x", "-y", "-o"+destDir, tmpFile)

		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("7z extract msi: %w\n%s", err, string(output))
		}

		return nil
	}

	if runtime.GOOS != "windows" {
		return fmt.Errorf("7z not found — install 7-Zip to extract MSI on this platform")
	}

	cmdLine := fmt.Sprintf(`msiexec /a "%s" /qn TARGETDIR="%s"`, filepath.Clean(tmpFile), filepath.Clean(destDir))
	cmd := exec.Command("cmd", "/c", cmdLine)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("msiexec extract: %w\n%s", err, string(output))
	}

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

// DownloadFile fetches a URL and returns the response body.
func DownloadFile(ctx context.Context, url string) ([]byte, error) {
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

// Resolve tries local (system-installed) lookup first, then falls back to download.
// This is the "system browser" resolution path — only called when systemBrowser=true.
func Resolve(ctx context.Context, bt BrowserType) (string, error) {
	path, err := lookupBrowser(bt)
	if err == nil {
		return path, nil
	}

	if !IsNotFound(err) {
		return "", err
	}

	switch bt { //nolint:exhaustive
	case Brave:
		return DownloadBrave(ctx)
	case Edge:
		return DownloadEdge(ctx)
	case Chromium:
		return DownloadChromium(ctx, ChromiumRevisionDefault)
	default:
		return "", err
	}
}

// ResolveCached looks only in ~/.scout/browsers/ for the given browser type.
// If not found in cache, downloads it. Never scans system install paths.
func ResolveCached(ctx context.Context, bt BrowserType) (string, error) {
	// Fast path: check registry first.
	registryNames := browserRegistryNames(bt)
	for _, name := range registryNames {
		if path := LookupRegistryBrowser(name); path != "" {
			return path, nil
		}
	}

	// Fallback: scan cache dirs on disk (handles pre-registry downloads).
	cacheDir, err := CacheDir()
	if err != nil {
		return "", err
	}

	type cacheEntry struct {
		subdir  string
		binName string
	}

	var candidates []cacheEntry

	switch bt { //nolint:exhaustive
	case Brave:
		candidates = []cacheEntry{{"brave", braveBinPath()}}
	case Edge:
		candidates = []cacheEntry{{"edge", edgeBinPath()}}
	case Chrome:
		candidates = []cacheEntry{
			{"chrome", chromeCfTBinPath()},
			{"chromium", chromiumBinPath()},
		}
	case Chromium:
		candidates = []cacheEntry{{"chromium", chromiumBinPath()}}
	default:
		return "", fmt.Errorf("%w: %s", ErrNotFound, bt)
	}

	for _, c := range candidates {
		if path := LatestCachedBin(filepath.Join(cacheDir, c.subdir), c.binName); path != "" {
			return path, nil
		}
	}

	// Not cached — download.
	switch bt { //nolint:exhaustive
	case Brave:
		return DownloadBrave(ctx)
	case Edge:
		return DownloadEdge(ctx)
	case Chrome:
		return DownloadChrome(ctx)
	case Chromium:
		return DownloadChromium(ctx, ChromiumRevisionDefault)
	default:
		return "", fmt.Errorf("%w: %s", ErrNotFound, bt)
	}
}

// browserRegistryNames maps a BrowserType to registry entry names to check.
func browserRegistryNames(bt BrowserType) []string {
	switch bt { //nolint:exhaustive
	case Chrome:
		return []string{"chrome", "chromium"}
	case Chromium:
		return []string{"chromium"}
	case Brave:
		return []string{"brave"}
	case Edge:
		return []string{"edge"}
	default:
		return nil
	}
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
