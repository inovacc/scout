//go:build !windows

package session

import (
	"os"
	"syscall"
)

// ProcessAlive checks if a process with the given PID is still running.
func ProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 checks existence without actually signaling.
	return p.Signal(syscall.Signal(0)) == nil
}
