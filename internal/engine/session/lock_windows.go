//go:build windows

package session

import (
	"os"

	"golang.org/x/sys/windows"
)

// lockFile takes an OS-level advisory lock on f. Exclusive when exclusive=true,
// shared otherwise. Fails immediately (no block) — callers decide whether to
// retry or report busy.
func lockFile(f *os.File, exclusive bool) error {
	var flags uint32 = windows.LOCKFILE_FAIL_IMMEDIATELY
	if exclusive {
		flags |= windows.LOCKFILE_EXCLUSIVE_LOCK
	}
	ol := new(windows.Overlapped)
	// Lock the entire file (length = max uint32 low + max uint32 high).
	return windows.LockFileEx(windows.Handle(f.Fd()), flags, 0, 0xffffffff, 0xffffffff, ol)
}

// unlockFile releases a lock acquired by lockFile. Closing the handle would
// also drop the lock; this call exists for symmetry and to ensure prompt
// release before any subsequent reopen.
func unlockFile(f *os.File) error {
	ol := new(windows.Overlapped)
	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 0xffffffff, 0xffffffff, ol)
}
