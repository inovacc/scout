//go:build windows

package browser

import (
	"os/exec"
	"strings"
)

// psQuote escapes a string for use inside a single-quoted PowerShell string
// literal, where the only metacharacter is the single quote (escaped by
// doubling). Without it a browser path containing a single quote could break out
// of the literal and inject arbitrary PowerShell into the -Command argument.
func psQuote(s string) string { return strings.ReplaceAll(s, "'", "''") }

// probeBrowserVersion uses PowerShell to read the file version without opening a GUI.
func probeBrowserVersion(path string) string {
	out, err := exec.Command("powershell", "-NoProfile", "-WindowStyle", "Hidden", "-Command",
		`(Get-Item '`+psQuote(path)+`').VersionInfo.ProductVersion`).Output()
	if err != nil {
		return ""
	}

	return ParseBrowserVersion(strings.TrimSpace(string(out)))
}
