package browser

import (
	"fmt"
	"os"
	"regexp"
	"sort"
)

// browserCandidate is a potential browser location returned by platform-specific code.
type browserCandidate struct {
	Name string
	Type string
	Path string
}

// typePriority defines preference order for sorting (lower = better).
var typePriority = map[string]int{
	TypeChrome: 0,
	TypeBrave:  1,
	TypeEdge:   2,
}

// Detect scans the system for all installed Chromium-based browsers.
// Returns detected browsers sorted by preference (Chrome > Brave > Edge).
func Detect() ([]BrowserInfo, error) {
	candidates := detectBrowserPaths()
	var results []BrowserInfo

	for _, c := range candidates {
		if !fileExists(c.Path) {
			continue
		}

		version := probeBrowserVersion(c.Path)
		results = append(results, BrowserInfo{
			Name:       c.Name,
			Type:       c.Type,
			Path:       c.Path,
			Version:    version,
			Downloaded: false,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return typePriority[results[i].Type] < typePriority[results[j].Type]
	})

	return results, nil
}

// DetectByType finds a specific browser type on the system.
// Returns ErrNotFound if no browser of that type is installed.
func DetectByType(browserType string) (BrowserInfo, error) {
	browsers, err := Detect()
	if err != nil {
		return BrowserInfo{}, err
	}

	for _, b := range browsers {
		if b.Type == browserType {
			return b, nil
		}
	}

	return BrowserInfo{}, fmt.Errorf("browser: %w: %s", ErrNotFound, browserType)
}

// Best returns the highest-priority detected browser (Chrome > Brave > Edge).
func Best() (BrowserInfo, error) {
	browsers, err := Detect()
	if err != nil {
		return BrowserInfo{}, err
	}

	if len(browsers) == 0 {
		return BrowserInfo{}, fmt.Errorf("browser: %w: no browsers detected", ErrNotFound)
	}

	return browsers[0], nil
}

// ParseVersion extracts a version string from browser --version output.
// It returns the first version-like pattern found (3 or 4 part).
func ParseVersion(output string) string {
	re := regexp.MustCompile(`\d+\.\d+\.\d+(?:\.\d+)?`)
	return re.FindString(output)
}

// probeBrowserVersion extracts the version of a browser at the given path.
// On Windows, running --version can launch the browser GUI, so we use
// platform-specific version detection instead.
func probeBrowserVersion(path string) string {
	return probeBrowserVersionPlatform(path)
}

// fileExists returns true if path exists and is a regular file.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
