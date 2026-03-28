//go:build !windows

package session

import (
	"os"
	"strings"
	"syscall"

	"github.com/google/gops/goprocess"
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

// IsScoutProcess checks if a PID belongs to a running scout (Go) process using gops.
// This is more reliable than OS-level ProcessAlive for scout PIDs because it
// avoids false positives from PID reuse by confirming the process is a Go binary
// whose executable name contains "scout".
func IsScoutProcess(pid int) bool {
	if pid <= 0 {
		return false
	}

	p, found, err := goprocess.Find(pid)
	if err != nil || !found {
		return false
	}

	return strings.Contains(strings.ToLower(p.Exec), "scout")
}

// ScoutProcessInfo returns gops info for a scout PID, or nil if not found.
func ScoutProcessInfo(pid int) *goprocess.P {
	if pid <= 0 {
		return nil
	}

	p, found, err := goprocess.Find(pid)
	if err != nil || !found {
		return nil
	}

	return &p
}
