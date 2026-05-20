//go:build !windows

package session

import (
	"fmt"
	"os"
	"runtime"
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

	return isScoutExec(p.Exec)
}

// ProcessStartToken returns an opaque platform-specific token uniquely
// identifying the process instance running at the given PID. Two processes
// that ran at the same PID at different times yield different tokens, so
// callers can detect PID reuse.
//
// Linux: parses /proc/<pid>/stat field 22 (starttime in clock ticks since
// boot). Format: "linux:<starttime-ticks>".
//
// Darwin/BSD: returns ("", false) — sysctl-based start time is not
// implemented. verifyProcess degrades to ProcessAlive-only on these.
func ProcessStartToken(pid int) (string, bool) {
	if pid <= 0 {
		return "", false
	}

	if runtime.GOOS != "linux" {
		return "", false
	}

	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return "", false
	}

	// /proc/<pid>/stat: "pid (comm) state ..." — comm may contain spaces
	// and parentheses, so split after the last ')'.
	s := string(data)

	end := strings.LastIndex(s, ")")
	if end < 0 {
		return "", false
	}

	fields := strings.Fields(s[end+1:])
	// After ")" the remaining sequence is state ppid pgrp ... starttime.
	// starttime is the 22nd field overall; the post-')' slice starts at
	// field 3 (state), so starttime is index 19.
	if len(fields) < 20 {
		return "", false
	}

	return "linux:" + fields[19], true
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
