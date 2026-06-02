package engine

import (
	"time"

	"github.com/inovacc/scout/internal/engine/session"
)

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

// RecordCleanupFailure enqueues path for background retry. Engine callers
// (e.g. Browser.Close) use this instead of importing session directly.
func RecordCleanupFailure(path string) { session.RecordCleanupFailure(path) }

// PendingCleanup returns a snapshot of dirs currently queued for background
// retry. Backs `scout session list --pending`.
func PendingCleanup() []string { return session.PendingCleanup() }
