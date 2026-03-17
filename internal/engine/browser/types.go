package browser

// BrowserType identifies a Chromium-based browser for auto-detection.
type BrowserType string

const (
	// Chrome selects Google Chrome for Testing.
	Chrome BrowserType = "chrome"
	// Chromium selects open-source Chromium (rod default).
	Chromium BrowserType = "chromium"
	// Brave selects Brave Browser.
	Brave BrowserType = "brave"
	// Edge selects Microsoft Edge.
	Edge BrowserType = "edge"
	// Electron selects Electron runtime for app automation.
	Electron BrowserType = "electron"
)

// DetectedBrowser represents a browser found on the system.
type DetectedBrowser struct {
	Name    string      `json:"name"`    // "Google Chrome", "Brave Browser", "Microsoft Edge", "Chromium"
	Type    BrowserType `json:"type"`    // Chrome, Chromium, Brave, Edge
	Path    string      `json:"path"`    // executable path
	Version string      `json:"version"` // e.g. "120.0.6099.109"
}

// browserCandidate is a potential browser location returned by platform-specific code.
type browserCandidate struct {
	Name string
	Type BrowserType
	Path string
}

// BrowserEntry describes a single downloaded browser in the registry.
type BrowserEntry struct {
	Name      string `json:"name"`      // e.g. "chrome", "chromium", "brave", "edge"
	Version   string `json:"version"`   // e.g. "146.0.7680.31", "1593111"
	Binary    string `json:"binary"`    // absolute path to executable
	Platform  string `json:"platform"`  // e.g. "windows_amd64"
	Installed string `json:"installed"` // RFC 3339 timestamp
}

// DownloadedBrowser describes a browser found in ~/.scout/browsers/.
type DownloadedBrowser struct {
	Name     string   // e.g. "chromium", "brave", "electron"
	Versions []string // e.g. ["1592198"], ["1.87.191"]
}
