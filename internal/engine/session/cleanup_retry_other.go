//go:build !windows

package session

// rmdirLowLevel is the non-Windows fallback. On Unix-like systems os.RemoveAll
// already handles every removable case (there is no AV/indexer lock
// pathology), so this is a no-op that reports success and lets forceBreakDir's
// final stat decide the outcome.
func rmdirLowLevel(_ string) error {
	return nil
}
