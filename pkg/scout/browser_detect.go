package scout

import (
	"fmt"
	"regexp"
	"sort"
)

// DetectedBrowser represents a browser found on the system.
type DetectedBrowser struct {
	Name    string      `json:"name"`    // "Google Chrome", "Brave Browser", "Microsoft Edge", "Chromium"
	Type    BrowserType `json:"type"`    // BrowserChrome, BrowserBrave, BrowserEdge
	Path    string      `json:"path"`    // executable path
	Version string      `json:"version"` // e.g. "120.0.6099.109"
}

// browserCandidate is a potential browser location returned by platform-specific code.
type browserCandidate struct {
	Name string
	Type BrowserType
	Path string
}

// browserTypePriority defines preference order for sorting (lower = better).
var browserTypePriority = map[BrowserType]int{
	BrowserChrome: 0,
	BrowserBrave:  1,
	BrowserEdge:   2,
}

var versionRe = regexp.MustCompile(`(\d+\.\d+\.\d+\.\d+)`)

// DetectBrowsers scans common install paths for all Chromium-based browsers.
// Returns detected browsers sorted by preference (Chrome > Brave > Edge > Chromium).
func DetectBrowsers() []DetectedBrowser {
	candidates := detectBrowserPaths()
	var results []DetectedBrowser

	for _, c := range candidates {
		if !fileExists(c.Path) {
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

// WithAutoDetect picks the best available browser (Chrome > Brave > Edge > Chromium).
// If no browser is found, falls back to rod's default auto-detection.
// This is ignored if WithExecPath or WithBrowser is also set.
func WithAutoDetect() Option {
	return func(o *options) { o.autoDetect = true }
}

// bestDetectedBrowser returns the path and type of the highest-priority detected browser.
func bestDetectedBrowser() (string, BrowserType, error) {
	browsers := DetectBrowsers()
	if len(browsers) == 0 {
		return "", BrowserChrome, fmt.Errorf("scout: no browsers detected")
	}

	return browsers[0].Path, browsers[0].Type, nil
}
