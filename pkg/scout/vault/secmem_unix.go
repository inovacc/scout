//go:build !windows

package vault

import "golang.org/x/sys/unix"

// lockPages best-effort pins b out of swap via mlock. RLIMIT_MEMLOCK may reject
// this (EAGAIN) on unprivileged processes; callers treat failure as non-fatal.
func lockPages(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	return unix.Mlock(b)
}

// unlockPages reverses lockPages. Zeroing must happen before this call.
func unlockPages(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	return unix.Munlock(b)
}
