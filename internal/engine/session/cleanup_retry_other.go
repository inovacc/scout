//go:build !windows

package session

import "os"

// rmdirLowLevel is the non-Windows fallback. On Unix-like systems os.RemoveAll
// already handles every removable case (there is no AV/indexer lock
// pathology). We call os.RemoveAll here so the force-break path makes at
// least one genuine removal attempt on Unix (the caller's stat-based check
// then determines whether it actually succeeded).
func rmdirLowLevel(path string) error {
	return os.RemoveAll(path)
}
