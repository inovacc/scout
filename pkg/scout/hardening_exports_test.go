package scout

import (
	"testing"
	"time"
)

// TestHardeningExportsSurface asserts that every symbol added in
// pkg/scout/hardening_exports.go is reachable from the scout package and
// delegates to the underlying engine primitives without panicking.
func TestHardeningExportsSurface(t *testing.T) {
	// Isolate session state. These re-exports (ReapOnce, PendingCleanup,
	// RecordCleanupFailure) operate on GetSessionsDir(); without this the test
	// reaps the developer's REAL ~/.scout sessions and pays retryRemoveAll's
	// ~11s AV-hardening budget (~17s total). t.Setenv redirects scouthome to an
	// empty temp dir, making the test both hermetic and fast.
	t.Setenv("SCOUT_HOME", t.TempDir())

	// CloseAllLive on an empty registry returns 0.
	if n := CloseAllLive(time.Second); n != 0 {
		t.Fatalf("CloseAllLive on empty registry = %d, want 0", n)
	}

	// ReapOnce returns a ReapStats value (typed re-export).
	stats := ReapOnce()
	_ = stats

	// PendingCleanup returns a slice; PendingCleanupCount returns its length-ish.
	_ = PendingCleanup()
	_ = PendingCleanupCount()

	// FindBrowsersUsingDataDir is callable with an arbitrary path.
	_ = FindBrowsersUsingDataDir(t.TempDir())

	// StartReaperWatchdog accepts (interval, done) and returns immediately.
	done := make(chan struct{})
	StartReaperWatchdog(time.Hour, done)
	close(done)

	// RecordCleanupFailure is callable.
	RecordCleanupFailure(t.TempDir())
}
