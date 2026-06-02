package scout

import (
	"testing"
	"time"
)

// TestHardeningExportsSurface asserts that every symbol added in
// pkg/scout/hardening_exports.go is reachable from the scout package and
// delegates to the underlying engine primitives without panicking.
func TestHardeningExportsSurface(t *testing.T) {
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
