//go:build !windows

package session

import (
	"os"
	"syscall"
)

// lockFile takes a non-blocking flock on f. Exclusive when exclusive=true,
// shared otherwise. Returns syscall.EWOULDBLOCK if another process holds an
// incompatible lock.
func lockFile(f *os.File, exclusive bool) error {
	how := syscall.LOCK_SH
	if exclusive {
		how = syscall.LOCK_EX
	}
	return syscall.Flock(int(f.Fd()), how|syscall.LOCK_NB)
}

// unlockFile releases the flock. Closing the fd would also drop it; this
// call exists for symmetry with the Windows path.
func unlockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}
