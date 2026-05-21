package launcher

import (
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// clearStaleChromeLock removes Chrome's lockfile when the PID it points to
// is no longer alive. Chrome writes <data_dir>/lockfile on Windows/Linux
// and <data_dir>/SingletonLock on macOS in the form "hostname\nPID" (or
// "hostname-PID"). On crash, the file isn't cleaned up; the next Chrome
// launch refuses to start ("Profile is already in use") and our URLParser
// blocks waiting for a debug URL that never arrives.
//
// Best-effort: removal errors are logged but not fatal.
func clearStaleChromeLock(dataDir string) {
	if dataDir == "" {
		return
	}

	for _, name := range []string{"lockfile", "SingletonLock", "SingletonCookie", "SingletonSocket"} {
		path := filepath.Join(dataDir, name)
		info, err := os.Lstat(path)
		if err != nil {
			continue // file absent — nothing to clean
		}

		// Symlinks (macOS SingletonLock) — read the target to extract PID.
		if info.Mode()&os.ModeSymlink != 0 {
			target, lerr := os.Readlink(path)
			if lerr == nil && !pidAliveFromLockTarget(target) {
				if rerr := os.Remove(path); rerr != nil {
					slog.Warn("scout: stale chrome lock remove failed", "path", path, "err", rerr)
				}
			}
			continue
		}

		// Regular file (Linux/Windows lockfile).
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			continue
		}

		if !pidAliveFromLockTarget(string(data)) {
			if remErr := os.Remove(path); remErr != nil {
				slog.Warn("scout: stale chrome lock remove failed", "path", path, "err", remErr)
			}
		}
	}
}

// pidAliveFromLockTarget extracts the trailing PID from Chrome's lock
// content. Formats observed in the wild:
//
//	"hostname\nPID"   (Linux / Windows lockfile body)
//	"hostname-PID"    (macOS SingletonLock symlink target)
//
// Returns true (assume alive) on parse failure so we don't aggressively
// delete a lock we can't interpret.
func pidAliveFromLockTarget(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}

	// Pull the trailing all-digit run.
	end := len(s)
	start := end
	for start > 0 && s[start-1] >= '0' && s[start-1] <= '9' {
		start--
	}
	if start == end {
		return true
	}

	pid, err := strconv.Atoi(s[start:end])
	if err != nil || pid <= 0 {
		return true
	}

	return processAlive(pid)
}
