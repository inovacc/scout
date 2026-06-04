package browser

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/inovacc/scout/internal/engine/scouthome"
)

// browserDownloadTimeout is the HTTP timeout for downloading browser archives.
const browserDownloadTimeout = 5 * time.Minute

// CacheDir returns the per-user browsers directory, creating it if needed.
// Resolved via scouthome (H5) so SCOUT_HOME and platform conventions apply.
func CacheDir() (string, error) {
	dir, err := scouthome.Sub("browsers")
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("scout: create browsers dir: %w", err)
	}

	return dir, nil
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

// maxBrowserDownload bounds a browser-archive download. Chromium/Brave/Edge
// builds are well under 1 GiB; the cap stops a hostile or misconfigured mirror
// from exhausting memory via an unbounded response body.
const maxBrowserDownload = 1 << 30 // 1 GiB

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

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBrowserDownload+1))
	if err != nil {
		return nil, err
	}

	if int64(len(data)) > maxBrowserDownload {
		return nil, fmt.Errorf("scout: download exceeds %d-byte limit: %s", maxBrowserDownload, url)
	}

	return data, nil
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
