//go:build windows

package session

import "os"

// rmdirLowLevel is the platform last-resort removal. This stub delegates to
// os.RemoveAll; Task 7.2 replaces this with a real low-level implementation
// that calls syscall.RemoveDirectory on each entry to bypass Explorer /
// OneDrive handle leaks that defeat os.RemoveAll.
//
// Deprecated stub: will be replaced by Task 7.2.
func rmdirLowLevel(path string) error {
	return os.RemoveAll(path)
}
