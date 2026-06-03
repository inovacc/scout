//go:build windows

package browser

import (
	"os/exec"
	"strings"
)

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
