package browser

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

	"github.com/inovacc/scout/pkg/scout/archive"
)

// ChromiumRevisionDefault is the pinned Chromium snapshot revision from browser.json.
var ChromiumRevisionDefault = mustLoadManifest().DefaultRevision()

// chromiumBins maps GOOS to the executable name within the extracted archive.
var chromiumBins = map[string]string{
	"darwin":  filepath.Join("Chromium.app", "Contents", "MacOS", "Chromium"),
	"linux":   "chrome",
	"windows": "chrome.exe",
}

// ChromiumDownloadURLs returns candidate download URLs for the given revision.
func ChromiumDownloadURLs(revision int) []string {
	m := mustLoadManifest()

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

	p := mustLoadManifest().Platform()

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
	url := mustLoadManifest().LatestAPI()
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

// chromeCfTBinPath returns the relative binary path for Chrome for Testing from browser.json.
func chromeCfTBinPath() string {
	return mustLoadManifest().Chrome.BinaryPath("chrome")
}

// chromeCfTPlatformID returns the CfT platform identifier for the current OS/arch from browser.json.
func chromeCfTPlatformID() string {
	p := mustLoadManifest().Chrome.BrowserPlatform()
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
	apiURL := mustLoadManifest().Chrome.BrowserAPI("latest_stable")
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
