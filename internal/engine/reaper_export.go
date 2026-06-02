package engine

import (
	"sync"
	"time"

	"github.com/inovacc/scout/internal/engine/session"
)

var reaperWatchdogOnce sync.Once

// EnsureReaperWatchdog starts the single process-wide session reaper watchdog
// exactly once per process. Safe to call repeatedly (idempotent via sync.Once).
func EnsureReaperWatchdog() {
	reaperWatchdogOnce.Do(func() {
		session.StartReaperWatchdog(session.DefaultReaperInterval, nil)
	})
}

// ReapStats re-exports session.ReapStats so engine/facade callers can name the
// return type of ReapOnce without importing the sub-package directly.
type ReapStats = session.ReapStats

// DefaultReaperInterval re-exports session.DefaultReaperInterval.
const DefaultReaperInterval = session.DefaultReaperInterval

// ReapOnce performs one canonical reaping pass and returns tallied stats.
// It is the single source of truth for ownerless-session cleanup; see
// session.ReapOnce for the full ownership rule.
func ReapOnce() ReapStats { return session.ReapOnce() }

// StartReaperWatchdog starts a single process-wide goroutine that runs
// ReapOnce every interval until done is closed. Pass 0 for interval to use
// DefaultReaperInterval.
func StartReaperWatchdog(interval time.Duration, done <-chan struct{}) {
	session.StartReaperWatchdog(interval, done)
}

// ReapSession force-reaps a single session dir: kills any browser holding its
// data dir (recorded PID + path-bounded scan, self-pid-guarded) and removes
// the dir. For the CLI enforcement path (audit --kill / doctor --fix).
func ReapSession(id string) ReapStats { return session.ReapSession(id) }

// RecordCleanupFailure enqueues path for background retry. Engine callers
// (e.g. Browser.Close) use this instead of importing session directly.
func RecordCleanupFailure(path string) { session.RecordCleanupFailure(path) }

// PendingCleanup returns a snapshot of dirs currently queued for background
// retry. Backs `scout session list --pending`.
func PendingCleanup() []string { return session.PendingCleanup() }
