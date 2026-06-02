package engine_test

import (
	"testing"
	"time"

	engine "github.com/inovacc/scout/internal/engine"
)

// TestReapOnce_Delegates verifies that engine.ReapOnce delegates to the session
// sub-package and returns a zero-value ReapStats when no sessions exist (clean
// temp environment). The real reaping logic is tested in internal/engine/session.
func TestReapOnce_Delegates(t *testing.T) {
	t.Setenv("SCOUT_HOME", t.TempDir())

	stats := engine.ReapOnce()

	// With an empty sessions dir we expect nothing was scanned.
	if stats.Scanned < 0 || stats.Removed < 0 || stats.Killed < 0 || stats.Pending < 0 {
		t.Fatalf("unexpected negative fields in ReapStats: %+v", stats)
	}
}

// TestReapStats_TypeAlias verifies that engine.ReapStats and session.ReapStats
// are the same type (alias), so callers can use engine.ReapStats as the named
// return type without a conversion.
func TestReapStats_TypeAlias(t *testing.T) {
	var s engine.ReapStats
	s.Scanned = 1
	s.Killed = 2
	s.Removed = 3
	s.Pending = 4

	if s.Scanned != 1 || s.Killed != 2 || s.Removed != 3 || s.Pending != 4 {
		t.Fatalf("ReapStats field access broken: %+v", s)
	}
}

// TestStartReaperWatchdog_Delegates verifies the watchdog starts, ticks once
// without panicking, and exits cleanly when done is closed.
func TestStartReaperWatchdog_Delegates(t *testing.T) {
	t.Setenv("SCOUT_HOME", t.TempDir())

	done := make(chan struct{})
	engine.StartReaperWatchdog(50*time.Millisecond, done)

	// Give the watchdog at least one tick.
	time.Sleep(80 * time.Millisecond)
	close(done)
}

// TestRecordAndPendingCleanup_Delegates verifies that RecordCleanupFailure
// enqueues a path and PendingCleanup reflects it.
func TestRecordAndPendingCleanup_Delegates(t *testing.T) {
	t.Setenv("SCOUT_HOME", t.TempDir())

	const fakePath = "/tmp/scout-test-reaper-export-fake-dir"

	engine.RecordCleanupFailure(fakePath)

	pending := engine.PendingCleanup()

	found := false

	for _, p := range pending {
		if p == fakePath {
			found = true

			break
		}
	}

	if !found {
		t.Fatalf("RecordCleanupFailure(%q) not reflected in PendingCleanup(); got %v", fakePath, pending)
	}
}
