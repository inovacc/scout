//go:build !windows

package browser

import (
	"os/exec"
	"strings"
)

// probeBrowserVersionPlatform runs --version on Unix (safe, prints to stdout).
func probeBrowserVersionPlatform(path string) string {
	out, err := exec.Command(path, "--version").Output()
	if err != nil {
		return ""
	}

	return ParseVersion(strings.TrimSpace(string(out)))
}
