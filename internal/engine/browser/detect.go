package browser

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
)

// browserTypePriority defines preference order for sorting (lower = better).
var browserTypePriority = map[BrowserType]int{
	Chrome: 0,
	Brave:  1,
	Edge:   2,
}

var versionRe = regexp.MustCompile(`(\d+\.\d+\.\d+\.\d+)`)

// DetectBrowsers scans common install paths for all Chromium-based browsers.
// Returns detected browsers sorted by preference (Chrome > Brave > Edge > Chromium).
func DetectBrowsers() []DetectedBrowser {
	candidates := detectBrowserPaths()

	var results []DetectedBrowser

	for _, c := range candidates {
		if !FileExists(c.Path) {
			continue
		}

		version := probeBrowserVersion(c.Path)
		results = append(results, DetectedBrowser{
			Name:    c.Name,
			Type:    c.Type,
			Path:    c.Path,
			Version: version,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		pi := browserTypePriority[results[i].Type]
		pj := browserTypePriority[results[j].Type]

		return pi < pj
	})

	return results
}

// ParseBrowserVersion extracts a version string from browser --version output.
func ParseBrowserVersion(output string) string {
	m := versionRe.FindString(output)
	if m != "" {
		return m
	}

	// Fallback: try to find any version-like pattern (X.Y.Z).
	re2 := regexp.MustCompile(`(\d+\.\d+\.\d+)`)

	return re2.FindString(output)
}

// BestDetected returns the path and type of the highest-priority detected browser.
func BestDetected() (string, BrowserType, error) {
	browsers := DetectBrowsers()
	if len(browsers) == 0 {
		return "", Chrome, fmt.Errorf("scout: no browsers detected")
	}

	return browsers[0].Path, browsers[0].Type, nil
}

// BestCached scans ~/.scout/browsers/ for downloaded browsers and returns
// the best match (chrome > chromium > edge > brave). Returns empty string if none found.
func BestCached() (string, error) {
	// Preference order: chrome (CfT), chromium, edge, brave.
	// Check registry first.
	for _, name := range []string{"chrome", "chromium", "edge", "brave"} {
		if path := LookupRegistryBrowser(name); path != "" {
			return path, nil
		}
	}

	// Fallback: scan cache dirs on disk.
	cacheDir, err := CacheDir()
	if err != nil {
		return "", err
	}

	candidates := []struct {
		subdir  string
		binName string
	}{
		{"chrome", chromeCfTBinPath()},
		{"chromium", chromiumBinPath()},
		{"edge", edgeBinPath()},
		{"brave", braveBinPath()},
	}

	for _, c := range candidates {
		if p := LatestCachedBin(filepath.Join(cacheDir, c.subdir), c.binName); p != "" {
			return p, nil
		}
	}

	return "", fmt.Errorf("scout: no cached browsers found in %s", cacheDir)
}

// BrowserTypePriority returns the priority map for external use in tests.
func BrowserTypePriority() map[BrowserType]int {
	return browserTypePriority
}
