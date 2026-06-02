// Hand-written hardening re-exports.
// pkg/scout/scout.go is generated (DO NOT EDIT). The session-hardening
// primitives below are added here so the generated facade stays untouched.
// Every symbol in this file is verified ABSENT from scout.go (duplicate =
// build break).

package scout

import (
	"time"

	"github.com/inovacc/scout/internal/engine"
)

// ReapStats re-exports engine.ReapStats — the tally from one reaper pass.
type ReapStats = engine.ReapStats

// CloseAllLive closes every live browser tracked in the process-wide registry
// using the given per-browser drain timeout. Returns the number of browsers
// closed.
func CloseAllLive(timeout time.Duration) int {
	return engine.CloseAllLive(timeout)
}

// ReapOnce performs one canonical reaping pass and returns tallied stats.
func ReapOnce() ReapStats {
	return engine.ReapOnce()
}

// StartReaperWatchdog starts a process-wide goroutine that runs ReapOnce
// every interval until done is closed. Pass 0 to use the default interval.
func StartReaperWatchdog(interval time.Duration, done <-chan struct{}) {
	engine.StartReaperWatchdog(interval, done)
}

// RecordCleanupFailure enqueues path for background retry cleanup.
func RecordCleanupFailure(path string) {
	engine.RecordCleanupFailure(path)
}

// PendingCleanup returns a snapshot of dirs queued for background retry.
func PendingCleanup() []string {
	return engine.PendingCleanup()
}

// PendingCleanupCount returns the number of dirs currently queued for
// background retry.
func PendingCleanupCount() int {
	return engine.PendingCleanupCount()
}

// ReapSession force-reaps a single session dir by id: kills any browser
// holding its data dir (recorded PID + path-bounded scan, self-pid-guarded)
// and removes the dir. For the CLI enforcement path (audit --kill / doctor --fix).
func ReapSession(id string) ReapStats {
	return engine.ReapSession(id)
}

// FindBrowsersUsingDataDir scans running browsers for processes whose
// --user-data-dir is under dataDir. Returns the matching PIDs.
func FindBrowsersUsingDataDir(dataDir string) []int {
	return engine.FindBrowsersUsingDataDir(dataDir)
}
