//go:build windows

package vault

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// lockPages best-effort pins b out of the pagefile via VirtualLock.
func lockPages(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	return windows.VirtualLock(uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)))
}

// unlockPages reverses lockPages. Zeroing must happen before this call.
func unlockPages(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	return windows.VirtualUnlock(uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)))
}
