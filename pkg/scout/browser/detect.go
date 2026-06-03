package browser

import (
	"fmt"
	"regexp"

	enginebrowser "github.com/inovacc/scout/internal/engine/browser"
)

// engineTypeToPkg maps an internal engine BrowserType to this package's
// string-based browser type. The engine's detection only ever emits
// Chrome, Brave, and Edge; any other type (Chromium, Electron) is folded
// into TypeChrome to preserve this package's historical behavior where a
// Chromium binary is reported as a Chrome-family browser.
func engineTypeToPkg(t enginebrowser.BrowserType) string {
	switch t {
	case enginebrowser.Brave:
		return TypeBrave
	case enginebrowser.Edge:
		return TypeEdge
	case enginebrowser.Chrome, enginebrowser.Chromium, enginebrowser.Electron:
		return TypeChrome
	default:
		return TypeChrome
	}
}

// Detect scans the system for all installed Chromium-based browsers.
// Returns detected browsers sorted by preference (Chrome > Brave > Edge).
func Detect() ([]BrowserInfo, error) {
	// Detection is delegated to internal/engine/browser.DetectBrowsers so the
	// platform-specific path globbing lives in a single place. The engine
	// result is mapped onto this package's public BrowserInfo type.
	detected := enginebrowser.DetectBrowsers()

	results := make([]BrowserInfo, 0, len(detected))

	for _, d := range detected {
		results = append(results, BrowserInfo{
			Name:       d.Name,
			Type:       engineTypeToPkg(d.Type),
			Path:       d.Path,
			Version:    d.Version,
			Downloaded: false,
		})
	}

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
	// NOTE: this intentionally does NOT delegate to
	// internal/engine/browser.ParseBrowserVersion. The engine variant prefers
	// the first 4-part match anywhere in the string, which would extract the
	// wrong version from strings like
	// "Brave Browser 1.62.156 Chromium: 121.0.6167.85" (returning the Chromium
	// version). This package's contract is to return the first version-like
	// token, so the regex is kept local.
	re := regexp.MustCompile(`\d+\.\d+\.\d+(?:\.\d+)?`)
	return re.FindString(output)
}

// probeBrowserVersion extracts the version of a browser at the given path.
// On Windows, running --version can launch the browser GUI, so we use
// platform-specific version detection instead. Used by Manager.List when
// reporting versions of cached/downloaded browsers.
func probeBrowserVersion(path string) string {
	return probeBrowserVersionPlatform(path)
}

// fileExists returns true if path exists and is a regular file. It delegates
// to internal/engine/browser.FileExists so file-existence semantics stay in a
// single place.
func fileExists(path string) bool {
	return enginebrowser.FileExists(path)
}
