//go:build !windows

package launcher

import (
	"os"
	"syscall"
)

// processAlive reports whether pid is a live process on Unix. Signal 0
// performs no action but returns ESRCH if the process does not exist.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	return proc.Signal(syscall.Signal(0)) == nil
}
