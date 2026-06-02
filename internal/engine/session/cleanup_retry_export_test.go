package session

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestRecordCleanupFailureAndPendingCleanup(t *testing.T) {
	// Drain any residue from other tests so assertions are deterministic.
	for _, p := range snapshotPending() {
		removePending(p)
	}

	dir := filepath.Join(t.TempDir(), "stuck-session")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	RecordCleanupFailure(dir)

	pending := PendingCleanup()

	if !slices.Contains(pending, dir) {
		t.Fatalf("PendingCleanup() = %v, want to contain %q", pending, dir)
	}

	// Duplicate enqueue must not grow the list.
	RecordCleanupFailure(dir)

	if got := PendingCleanupCount(); got != 1 {
		t.Fatalf("PendingCleanupCount() = %d after duplicate enqueue, want 1", got)
	}

	appearances := 0

	for _, p := range PendingCleanup() {
		if p == dir {
			appearances++
		}
	}

	if appearances != 1 {
		t.Fatalf("dir appears %d times in PendingCleanup(), want 1", appearances)
	}

	removePending(dir)
}

func TestForceBreakThresholdConst(t *testing.T) {
	if forceBreakThreshold < 1 {
		t.Fatalf("forceBreakThreshold = %d, want >= 1", forceBreakThreshold)
	}
}
