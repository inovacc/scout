package session

import (
	"errors"
	"log/slog"
	"os"
	"time"
)

// ReapStats tallies the result of a single ReapOnce pass.
//
//   - Scanned: total session folders examined.
//   - Killed:  browser processes killed (recorded BrowserPID + path-bounded
//     holders found via FindBrowsersUsingDataDir).
//   - Removed: session folders successfully removed.
//   - Pending: folders whose removal failed and were enqueued for the
//     background retrier (RecordCleanupFailure).
type ReapStats struct {
	Scanned int
	Killed  int
	Removed int
	Pending int
}

// ReapOnce performs one canonical reaping pass over GetSessionsDir(). It is the
// single source of truth for "is this folder ownerless, and if so kill any
// browser holding it and remove it." CleanStaleSessions and CleanOrphans are
// thin wrappers over this; the startup path, the watchdog, and the daemon
// reconcile all call it so behavior is identical wherever it runs.
//
// Ownership rule (a folder is PRESERVED iff):
//   - scout.pid parses AND the recorded ScoutPID is a live, identity-verified
//     scout process (IsScoutProcess), OR
//   - the session is reusable and not yet expired (IsExpired() == false).
//
// Every other folder is reaped:
//   - Missing / corrupt / legacy scout.pid (ReadInfo error) — the
//     un-killable-zombie case: we cannot classify by metadata, so we still
//     scan-and-kill any browser whose --user-data-dir is under this folder
//     (path-bounded via FindBrowsersUsingDataDir) and remove the folder.
//   - Owner (ScoutPID) dead or unverifiable — kill recorded BrowserPID (with
//     an immediate-before-kill liveness re-check to shrink the TOCTOU window),
//     scan-and-kill path-bounded holders, remove the folder.
//
// All kills are best-effort and logged; never fatal. Removal failures are
// enqueued via RecordCleanupFailure so the retrier (and eventual force-break)
// reaps them later. The pass is idempotent and safe to run concurrently from
// startup and the watchdog (folder-level guarding via scout.lock).
func ReapOnce() ReapStats {
	var stats ReapStats

	sessDir := GetSessionsDir()

	entries, err := os.ReadDir(sessDir)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("scout: reaper: read sessions dir", "dir", sessDir, "err", err)
		}

		return stats
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		id := e.Name()
		stats.Scanned++

		info, readErr := ReadInfo(id)
		if readErr != nil {
			// Missing / corrupt / legacy / foreign-owned scout.pid. We cannot
			// trust metadata, so treat the folder as an orphan and reap it.
			// This is the un-killable-zombie fix: scan-and-kill any browser
			// holding the data dir even though scout.pid told us nothing.
			if errors.Is(readErr, ErrLegacyFormat) {
				slog.Info("scout: reaper: removing legacy JSON session", "id", id)
			}

			reapFolder(id, 0, "", &stats)

			continue
		}

		// PRESERVE: reusable session still within its lifetime window.
		if info.Reusable && !info.IsExpired() {
			continue
		}

		// PRESERVE: a live, identity-verified scout still owns this folder.
		if info.ScoutPID != 0 && IsScoutProcess(info.ScoutPID) {
			continue
		}

		if info.Reusable {
			slog.Info("scout: reaper: reusable session expired, reaping",
				"id", id, "expires_at", info.ExpiresAt)
		}

		reapFolder(id, info.BrowserPID, info.BrowserStartToken, &stats)
	}

	return stats
}

// reapFolder kills any browser holding the session's data dir, then removes the
// folder, tallying into stats. browserPID/startToken come from scout.pid when
// it was readable (0/"" otherwise). It performs, in order:
//  1. Kill recorded BrowserPID if it verifies AND is still alive immediately
//     before the kill (TOCTOU shrink).
//  2. Path-bounded scan-and-kill: every PID FindBrowsersUsingDataDir reports
//     for DataDir(id). NO identity gate (per design Q2) — but the scan is
//     bounded to GetSessionsDir() by FindBrowsersUsingDataDir itself, which is
//     the single safety floor that never touches the user's real Chrome.
//  3. retryRemoveAll(Dir(id)); on failure RecordCleanupFailure(Dir(id)).
func reapFolder(id string, browserPID int, startToken string, stats *ReapStats) {
	// 1. Recorded BrowserPID, identity-verified + TOCTOU re-check.
	if browserPID != 0 && browserPID != os.Getpid() && verifyProcess(browserPID, startToken) {
		if ProcessAlive(browserPID) {
			if p, err := os.FindProcess(browserPID); err == nil {
				if killErr := p.Kill(); killErr == nil {
					stats.Killed++
				} else {
					slog.Warn("scout: reaper: kill browser failed",
						"id", id, "pid", browserPID, "err", killErr)
				}
			}
		}
	}

	// 2. Path-bounded scan-and-kill of any holder of the data dir. This is the
	// only path that reaches a zombie whose scout.pid was missing/corrupt.
	for _, pid := range FindBrowsersUsingDataDir(DataDir(id)) {
		if pid == os.Getpid() {
			continue // never kill ourselves
		}

		if p, err := os.FindProcess(pid); err == nil {
			if killErr := p.Kill(); killErr == nil {
				stats.Killed++
			} else {
				slog.Warn("scout: reaper: kill holder failed",
					"id", id, "pid", pid, "err", killErr)
			}
		}
	}

	// 3. Remove the folder; enqueue for the retrier on persistent failure.
	dir := Dir(id)
	if err := retryRemoveAll(dir); err != nil {
		RecordCleanupFailure(dir)
		stats.Pending++

		slog.Warn("scout: reaper: removal deferred to background retrier",
			"id", id, "dir", dir, "err", err)

		return
	}

	stats.Removed++
}

// ReapSession force-reaps a single session dir by id: kills any browser
// holding its data dir (recorded PID + path-bounded scan, self-pid-guarded)
// and removes the dir. For the CLI enforcement path (audit --kill / doctor --fix).
//
// scout.pid is read best-effort; if missing/corrupt (the CORRUPT zombie case)
// browserPID is 0 and the kill falls through to the path-bounded scan via
// FindBrowsersUsingDataDir — so the zombie IS killed even when scout.pid is
// unreadable. This is the fix for the defect where the CLI path missed such zombies.
func ReapSession(id string) ReapStats {
	var stats ReapStats

	info, readErr := ReadInfo(id)
	if readErr != nil {
		// Missing / corrupt scout.pid — cannot read BrowserPID.
		// Pass 0/"" so reapFolder skips the recorded-PID kill and falls
		// straight through to the path-bounded scan-and-kill.
		reapFolder(id, 0, "", &stats)
		return stats
	}

	reapFolder(id, info.BrowserPID, info.BrowserStartToken, &stats)
	return stats
}

// DefaultReaperInterval is the default interval for the process-wide reaper
// watchdog. Mirrors the previous per-browser orphan check cadence.
const DefaultReaperInterval = DefaultOrphanCheckInterval

// StartReaperWatchdog starts a single process-wide goroutine that runs ReapOnce
// every interval until done is closed. It REPLACES the per-browser
// StartOrphanWatchdog fan-out — one ticker scans every folder, so N live
// browsers no longer mean N redundant watchdogs. Returns immediately.
func StartReaperWatchdog(interval time.Duration, done <-chan struct{}) {
	if interval <= 0 {
		interval = DefaultReaperInterval
	}

	go func() {
		// Panic recovery so a single bad iteration (filesystem race, gops
		// internal race) does not silently kill the watchdog.
		defer func() {
			if r := recover(); r != nil {
				slog.Error("scout: reaper watchdog panic", "panic", r)
			}
		}()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				_ = ReapOnce()
			}
		}
	}()
}
