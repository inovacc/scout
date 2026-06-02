//go:build !windows

package session

import "os"

// rmdirLowLevel is the platform last-resort removal. On non-Windows platforms
// a plain os.RemoveAll is sufficient; the real low-level syscall escalation
// is only needed on Windows (see cleanup_retry_windows.go, added in Task 7.2).
func rmdirLowLevel(path string) error {
	return os.RemoveAll(path)
}
