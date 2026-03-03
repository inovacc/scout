//go:build !windows

package engine

import (
	"os/exec"
	"runtime"
)

func detectBrowserPaths() []browserCandidate {
	var candidates []browserCandidate

	// Google Chrome
	chromePaths := []string{
		"/usr/bin/google-chrome",
		"/usr/bin/google-chrome-stable",
		"/usr/bin/chromium",
		"/usr/bin/chromium-browser",
		"/snap/bin/chromium",
	}
	if runtime.GOOS == "darwin" {
		chromePaths = append(chromePaths,
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		)
	}
	for _, p := range chromePaths {
		candidates = append(candidates, browserCandidate{
			Name: "Google Chrome",
			Type: BrowserChrome,
			Path: p,
		})
	}
	// PATH-based fallback for Chrome.
	if p, err := exec.LookPath("google-chrome"); err == nil {
		candidates = append(candidates, browserCandidate{Name: "Google Chrome", Type: BrowserChrome, Path: p})
	}
	if p, err := exec.LookPath("chromium-browser"); err == nil {
		candidates = append(candidates, browserCandidate{Name: "Chromium", Type: BrowserChrome, Path: p})
	}

	// Brave Browser
	bravePaths := []string{
		"/usr/bin/brave-browser",
		"/usr/bin/brave-browser-stable",
		"/opt/brave.com/brave/brave-browser",
		"/snap/bin/brave",
	}
	if runtime.GOOS == "darwin" {
		bravePaths = append(bravePaths,
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
		)
	}
	for _, p := range bravePaths {
		candidates = append(candidates, browserCandidate{Name: "Brave Browser", Type: BrowserBrave, Path: p})
	}
	if p, err := exec.LookPath("brave-browser"); err == nil {
		candidates = append(candidates, browserCandidate{Name: "Brave Browser", Type: BrowserBrave, Path: p})
	}

	// Microsoft Edge
	edgePaths := []string{
		"/usr/bin/microsoft-edge",
		"/usr/bin/microsoft-edge-stable",
		"/opt/microsoft/msedge/msedge",
	}
	if runtime.GOOS == "darwin" {
		edgePaths = append(edgePaths,
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		)
	}
	for _, p := range edgePaths {
		candidates = append(candidates, browserCandidate{Name: "Microsoft Edge", Type: BrowserEdge, Path: p})
	}
	if p, err := exec.LookPath("microsoft-edge"); err == nil {
		candidates = append(candidates, browserCandidate{Name: "Microsoft Edge", Type: BrowserEdge, Path: p})
	}

	return candidates
}
