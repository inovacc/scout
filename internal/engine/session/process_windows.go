//go:build windows

package session

import (
	"fmt"

	"github.com/google/gops/goprocess"
	"golang.org/x/sys/windows"
)

// ProcessAlive reports whether the process with the given PID is currently running.
// It uses WaitForSingleObject with a zero timeout, which returns immediately:
//   - windows.WAIT_TIMEOUT (258): process is still running
//   - windows.WAIT_OBJECT_0 (0):  process has exited
//   - windows.WAIT_FAILED:        handle is invalid or access denied
//
// This correctly handles zombie processes (unlike GetExitCodeProcess which returns
// STILL_ACTIVE=259 for zombies that have exited but whose handle is still open).
func ProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	// WaitForSingleObject requires SYNCHRONIZE access right. Opening with
	// PROCESS_QUERY_LIMITED_INFORMATION alone causes WaitForSingleObject to
	// return WAIT_FAILED with "Access is denied" — making ProcessAlive return
	// false for every PID (including the current process). Add SYNCHRONIZE.
	const access = windows.PROCESS_QUERY_LIMITED_INFORMATION | windows.SYNCHRONIZE

	h, err := windows.OpenProcess(access, false, uint32(pid))
	if err != nil {
		return false
	}

	defer func() { _ = windows.CloseHandle(h) }()

	result, _ := windows.WaitForSingleObject(h, 0)

	// WAIT_TIMEOUT (258 / 0x102) means the process has not exited yet.
	// Any other value (WAIT_OBJECT_0=0 or WAIT_FAILED) means exited or invalid.
	return result == uint32(windows.WAIT_TIMEOUT)
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
// at the same PID at different times yield different tokens, so callers can
// detect PID reuse — important on Windows where PID reassignment happens
// quickly after a process exits.
//
// Format: "win:<creation-filetime-nanoseconds>" using GetProcessTimes
// CreationTime.
func ProcessStartToken(pid int) (string, bool) {
	if pid <= 0 {
		return "", false
	}

	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return "", false
	}

	defer func() { _ = windows.CloseHandle(h) }()

	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(h, &creation, &exit, &kernel, &user); err != nil {
		return "", false
	}

	return fmt.Sprintf("win:%d", creation.Nanoseconds()), true
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
