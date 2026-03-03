//go:build !windows

package engine

import (
	"os/exec"
	"strings"
)

// probeBrowserVersion runs "<path> --version" and parses the version string.
func probeBrowserVersion(path string) string {
	out, err := exec.Command(path, "--version").Output()
	if err != nil {
		return ""
	}

	return ParseBrowserVersion(strings.TrimSpace(string(out)))
}
