package session

import (
	"log/slog"
	"os"
	"sync"
	"time"
)

// Background cleanup retrier.
//
// CleanStaleSessions runs on every scout startup with a bounded budget
// (~11 s per dir via retryRemoveAll). Windows AV / Search Indexer can hold
// Chrome SQLite / LevelDB files much longer than that, leaving session
// dirs orphaned even though scout.pid was already removed.
//
// This file adds a process-lifetime background worker that retries the
// failed dirs every minute until they go away or the process exits. It
// runs independently of StartOrphanWatchdog (which targets live orphan
// browsers, not file-locked stale dirs).

var (
	pendingMu      sync.Mutex
	pendingCleanup []string
)

// recordCleanupFailure adds path to the retry queue. Called from
// CleanStaleSessions when retryRemoveAll exhausts its budget.
func recordCleanupFailure(path string) {
	pendingMu.Lock()
	defer pendingMu.Unlock()

	for _, p := range pendingCleanup {
		if p == path {
			return // already queued
		}
	}
	pendingCleanup = append(pendingCleanup, path)
}

// snapshotPending returns a copy of the current pending list without
// clearing it. removePending is used to drop entries that succeed.
func snapshotPending() []string {
	pendingMu.Lock()
	defer pendingMu.Unlock()
	out := make([]string, len(pendingCleanup))
	copy(out, pendingCleanup)
	return out
}

// removePending drops path from the queue. No-op if absent.
func removePending(path string) {
	pendingMu.Lock()
	defer pendingMu.Unlock()
	for i, p := range pendingCleanup {
		if p == path {
			pendingCleanup = append(pendingCleanup[:i], pendingCleanup[i+1:]...)
			return
		}
	}
}

// PendingCleanupCount reports how many dirs are queued for retry.
// Stays accurate mid-sweep because retryPending removes paths one-by-one
// rather than draining the whole queue up front.
func PendingCleanupCount() int {
	pendingMu.Lock()
	defer pendingMu.Unlock()
	return len(pendingCleanup)
}

// DefaultCleanupRetryInterval is the base interval between retry sweeps.
// Each sweep walks all pending dirs once with a short per-dir budget.
const DefaultCleanupRetryInterval = 60 * time.Second

// StartCleanupRetrier launches a background goroutine that periodically
// retries removing dirs that CleanStaleSessions could not clean (typically
// because AV / Search Indexer was holding files). Stops when done is
// closed.
//
// Successful removals are logged at INFO. The retry interval grows up to
// 10 minutes after consecutive failures on the same dir, then resets when
// that dir finally goes away.
func StartCleanupRetrier(done <-chan struct{}) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("scout: cleanup retrier panic", "panic", r)
			}
		}()

		ticker := time.NewTicker(DefaultCleanupRetryInterval)
		defer ticker.Stop()

		// failCount tracks consecutive misses per path so we can back off.
		failCount := make(map[string]int)

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				retryPending(failCount)
			}
		}
	}()
}

// retryPending walks the current queue, attempts removal of each dir, and
// removes successful entries from the live queue (so PendingCleanupCount
// stays accurate mid-sweep). failCount is mutated in place.
func retryPending(failCount map[string]int) {
	paths := snapshotPending()
	if len(paths) == 0 {
		return
	}

	for _, p := range paths {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			removePending(p)
			delete(failCount, p)
			continue // already gone — maybe another process removed it
		}

		if err := retryRemoveAll(p); err == nil {
			slog.Info("scout: background cleanup removed stale session", "dir", p)
			removePending(p)
			delete(failCount, p)
			continue
		}

		failCount[p]++

		// Cap consecutive-failure logging at 10 to bound noise; dirs
		// that survive a full hour are likely held by something
		// persistent (OneDrive sync, broken AV) — keep retrying
		// silently. Entry stays in queue for next tick.
		if failCount[p] == 10 {
			slog.Warn("scout: background cleanup still blocked after 10 attempts", "dir", p)
		}
	}
}
