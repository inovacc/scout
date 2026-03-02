//go:build windows

package scout

import (
	"os/exec"
	"strings"
)

// probeBrowserVersion uses PowerShell to read the file version without opening a GUI.
func probeBrowserVersion(path string) string {
	out, err := exec.Command("powershell", "-NoProfile", "-WindowStyle", "Hidden", "-Command",
		`(Get-Item '`+path+`').VersionInfo.ProductVersion`).Output()
	if err != nil {
		return ""
	}

	return ParseBrowserVersion(strings.TrimSpace(string(out)))
}
