//go:build windows

package session

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
)

// rmdirLowLevel performs a depth-first removal of path using the raw Win32
// primitives (SetFileAttributes + DeleteFile + RemoveDirectory). It is the
// terminal escalation invoked by forceBreakDir after os.RemoveAll has failed
// forceBreakThreshold times. Best-effort: it returns the last error but tries
// every entry regardless of individual failures, and never panics.
func rmdirLowLevel(path string) error {
	// childErr accumulates errors from child removals; it is a fallback only —
	// the dir-remove block below always produces a definitive lastErr.
	var childErr error

	entries, err := os.ReadDir(path)
	if err == nil {
		for _, e := range entries {
			child := filepath.Join(path, e.Name())
			if e.IsDir() {
				if rmErr := rmdirLowLevel(child); rmErr != nil {
					childErr = rmErr
				}
				continue
			}
			if delErr := deleteFileLowLevel(child); delErr != nil {
				childErr = delErr
			}
		}
	} else {
		childErr = err
	}

	// Clear attributes on the dir itself, then remove it.
	// lastErr is set unconditionally here (success or failure), so childErr
	// is only used as a fallback if the UTF-16 conversion itself fails.
	var lastErr error
	if p, cvtErr := windows.UTF16PtrFromString(path); cvtErr == nil {
		_ = windows.SetFileAttributes(p, windows.FILE_ATTRIBUTE_NORMAL)
		if rmErr := windows.RemoveDirectory(p); rmErr != nil {
			lastErr = rmErr
		}
		// rmErr == nil → lastErr stays nil (success)
	} else {
		lastErr = childErr // best we can report; cvtErr is an internal detail
	}

	// If the dir is gone despite a reported error, treat as success.
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		return nil
	}
	return lastErr
}

// deleteFileLowLevel clears blocking attributes then DeleteFile's a single
// file via raw Win32. Best-effort.
func deleteFileLowLevel(path string) error {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	// Strip read-only/hidden/system so DeleteFile is permitted.
	_ = windows.SetFileAttributes(p, windows.FILE_ATTRIBUTE_NORMAL)
	if err := windows.DeleteFile(p); err != nil {
		if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
			return nil
		}
		return err
	}
	return nil
}
