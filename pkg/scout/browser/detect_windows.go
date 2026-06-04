//go:build windows

package browser

import (
	"os/exec"
	"strings"
)

// psQuote escapes a string for use inside a single-quoted PowerShell string
// literal (the single quote is escaped by doubling). Without it a browser path
// containing a single quote could break out and inject PowerShell.
func psQuote(s string) string { return strings.ReplaceAll(s, "'", "''") }

// probeBrowserVersionPlatform extracts version on Windows using PowerShell
// file version info. Running --version on Windows launches the browser GUI.
func probeBrowserVersionPlatform(path string) string {
	out, err := exec.Command("powershell", "-NoProfile", "-Command",
		`(Get-Item '`+psQuote(path)+`').VersionInfo.ProductVersion`).Output()
	if err != nil {
		return ""
	}

	return ParseVersion(strings.TrimSpace(string(out)))
}
