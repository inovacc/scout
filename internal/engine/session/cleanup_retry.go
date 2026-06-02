package session

import (
	"log/slog"
	"os"
	"path/filepath"
	"slices"
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

	// removeAllFn indirects os.RemoveAll so tests can inject a deterministic
	// failing remover for the normal retry path. The force-break path
	// (forceBreakDir) deliberately calls os.RemoveAll directly so it is never
	// short-circuited by this seam.
	removeAllFn = os.RemoveAll
)

// recordCleanupFailure adds path to the retry queue. Called from
// CleanStaleSessions when retryRemoveAll exhausts its budget.
func recordCleanupFailure(path string) {
	pendingMu.Lock()
	defer pendingMu.Unlock()

	if slices.Contains(pendingCleanup, path) {
		return // already queued
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

// RecordCleanupFailure enqueues path for background retry. Exported wrapper
// over recordCleanupFailure so package engine's Browser.Close can enqueue a
// session dir whose single-shot RemoveAll lost to a Windows file lock,
// instead of leaking it. Safe to call from any goroutine.
func RecordCleanupFailure(path string) {
	recordCleanupFailure(path)
}

// PendingCleanup returns a snapshot of the dirs currently queued for
// background retry. Backs `scout session list --pending`. The returned slice
// is a copy and safe to retain.
func PendingCleanup() []string {
	return snapshotPending()
}

// DefaultCleanupRetryInterval is the base interval between retry sweeps.
// Each sweep walks all pending dirs once with a short per-dir budget.
const DefaultCleanupRetryInterval = 60 * time.Second

// forceBreakThreshold is the number of consecutive retry misses on the same
// dir after which the retrier escalates from polite RemoveAll to a
// best-effort force removal (chmod-walk + RemoveAll). At the 60 s default
// interval this is ~20 minutes — long enough that a genuinely-busy holder
// has almost certainly exited, short enough that a stuck dir does not leak
// for the whole process lifetime.
const forceBreakThreshold = 20

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

		if err := removeAllFn(p); err == nil {
			slog.Info("scout: background cleanup removed stale session", "dir", p)
			removePending(p)
			delete(failCount, p)

			continue
		}

		failCount[p]++

		// Cap consecutive-failure logging at 10 to bound noise; dirs
		// that survive ~10 min are likely held by something persistent
		// (OneDrive sync, broken AV). Entry stays in queue for next tick.
		if failCount[p] == 10 {
			slog.Warn("scout: background cleanup still blocked after 10 attempts", "dir", p)
		}

		// Force-break escalation: once a dir has resisted forceBreakThreshold
		// consecutive sweeps (~20 min), stop waiting and break it
		// aggressively. Best-effort: a still-failing force-break leaves the
		// dir queued for the next tick (we never panic, never block).
		if failCount[p] >= forceBreakThreshold {
			if err := forceBreakDir(p); err == nil {
				slog.Warn("scout: force-broke stuck session dir after threshold",
					"dir", p, "attempts", failCount[p])
				removePending(p)
				delete(failCount, p)

				continue
			} else {
				slog.Warn("scout: force-break of stuck session dir failed",
					"dir", p, "attempts", failCount[p], "err", err)
			}
		}
	}
}

// forceBreakDir aggressively removes a session dir that survived
// forceBreakThreshold normal retry sweeps. It is the terminal escalation: it
// loops os.RemoveAll a few times (clearing read-only attrs between passes),
// then falls back to a low-level platform rmdir walk. Best-effort: it returns
// the last error and never panics. It deliberately calls os.RemoveAll directly
// (not removeAllFn) so the force-break path is never short-circuited by a
// test-injected failing remover.
func forceBreakDir(path string) error {
	// A few direct passes: clearing the read-only bit between passes lets a
	// dir whose entries were marked read-only (common on Windows AV
	// quarantine) finally delete.
	for range 3 {
		_ = clearReadOnly(path)

		if err := os.RemoveAll(path); err == nil {
			return nil
		}
	}

	// Platform last resort: low-level directory removal. Do NOT short-circuit
	// on a nil return from rmdirLowLevel — on non-Windows it delegates to
	// os.RemoveAll which can return nil even when the path still exists (race)
	// or is a no-op. Always let the stat below decide success.
	_ = rmdirLowLevel(path)

	// Authoritative success check: the path is gone regardless of which
	// removal attempt removed it (including a concurrent remover).
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		return nil
	}

	// Return a synthetic sentinel so callers know the dir is still present.
	return &os.PathError{Op: "remove", Path: path, Err: os.ErrPermission}
}

// clearReadOnly best-effort walks path and clears the read-only bit on every
// entry so a subsequent RemoveAll can proceed. Errors are not fatal — the
// caller retries regardless.
func clearReadOnly(path string) error {
	return filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil //nolint:nilerr // skip unreadable entries; keep walking
		}
		// Add owner write bit; harmless on dirs, clears the RO attr on files.
		_ = os.Chmod(p, info.Mode()|0o200)

		return nil
	})
}
