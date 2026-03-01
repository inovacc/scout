//go:build windows

package browser

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
			Type: TypeChrome,
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
			Type: TypeBrave,
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
			Type: TypeEdge,
			Path: filepath.Join(dir, `Microsoft\Edge\Application\msedge.exe`),
		})
	}

	return candidates
}

// probeBrowserVersionPlatform extracts version on Windows using PowerShell
// file version info. Running --version on Windows launches the browser GUI.
func probeBrowserVersionPlatform(path string) string {
	out, err := exec.Command("powershell", "-NoProfile", "-Command",
		`(Get-Item '`+path+`').VersionInfo.ProductVersion`).Output()
	if err != nil {
		return ""
	}
	return ParseVersion(strings.TrimSpace(string(out)))
}
