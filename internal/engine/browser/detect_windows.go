//go:build windows

package browser

import (
	"os"
	"path/filepath"
)

func detectBrowserPaths() []browserCandidate {
	localAppData := os.Getenv("LOCALAPPDATA")
	programFiles := os.Getenv("ProgramFiles")
	programFilesX86 := os.Getenv("ProgramFiles(x86)")

	var candidates []browserCandidate

	// Google Chrome
	for _, dir := range []string{programFiles, programFilesX86, localAppData} {
		if dir == "" {
			continue
		}

		candidates = append(candidates, browserCandidate{
			Name: "Google Chrome",
			Type: Chrome,
			Path: filepath.Join(dir, `Google\Chrome\Application\chrome.exe`),
		})
	}

	// Brave Browser
	for _, dir := range []string{localAppData, programFiles, programFilesX86} {
		if dir == "" {
			continue
		}

		candidates = append(candidates, browserCandidate{
			Name: "Brave Browser",
			Type: Brave,
			Path: filepath.Join(dir, `BraveSoftware\Brave-Browser\Application\brave.exe`),
		})
	}

	// Microsoft Edge
	for _, dir := range []string{programFiles, programFilesX86, localAppData} {
		if dir == "" {
			continue
		}

		candidates = append(candidates, browserCandidate{
			Name: "Microsoft Edge",
			Type: Edge,
			Path: filepath.Join(dir, `Microsoft\Edge\Application\msedge.exe`),
		})
	}

	return candidates
}
