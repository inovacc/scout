package session

import (
	"os"
	"path/filepath"
	"testing"
)

// withRemoveAllFn swaps the package removeAllFn seam for the duration of a
// test and restores it on cleanup.
func withRemoveAllFn(t *testing.T, fn func(string) error) {
	t.Helper()
	orig := removeAllFn
	removeAllFn = fn
	t.Cleanup(func() { removeAllFn = orig })
}

// resetPending clears the package-level pending queue so each test starts
// from a known state. White-box access only.
func resetPending(t *testing.T) {
	t.Helper()
	pendingMu.Lock()
	pendingCleanup = nil
	pendingMu.Unlock()
	t.Cleanup(func() {
		pendingMu.Lock()
		pendingCleanup = nil
		pendingMu.Unlock()
	})
}

// TestRetryPendingForceBreakAfterThreshold proves a dir that fails removal on
// every normal sweep is force-broken once failCount reaches
// forceBreakThreshold, and is then dequeued so PendingCleanupCount drops to 0.
func TestRetryPendingForceBreakAfterThreshold(t *testing.T) {
	resetPending(t)

	dir := t.TempDir()
	target := filepath.Join(dir, "stuck-sess")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}

	// removeAllFn fails for every normal sweep; the force-break path uses the
	// real os.RemoveAll via forceBreakDir (NOT removeAllFn) so it can succeed.
	calls := 0
	withRemoveAllFn(t, func(p string) error {
		calls++
		return &os.PathError{Op: "remove", Path: p, Err: os.ErrPermission}
	})

	recordCleanupFailure(target)
	if got := PendingCleanupCount(); got != 1 {
		t.Fatalf("PendingCleanupCount after enqueue = %d, want 1", got)
	}

	failCount := make(map[string]int)

	// Drive sweeps up to (threshold-1): normal removeAllFn keeps failing, no
	// force-break yet, dir stays queued.
	for range forceBreakThreshold - 1 {
		retryPending(failCount)
	}

	if got := PendingCleanupCount(); got != 1 {
		t.Fatalf("before threshold: PendingCleanupCount = %d, want 1", got)
	}
	if got := failCount[target]; got != forceBreakThreshold-1 {
		t.Fatalf("failCount[target] = %d, want %d", got, forceBreakThreshold-1)
	}

	// The threshold-th sweep triggers force-break, which uses forceBreakDir
	// (real os.RemoveAll loop, bypassing the injected removeAllFn) and
	// succeeds since the dir is unlocked in this test.
	retryPending(failCount)

	if got := PendingCleanupCount(); got != 0 {
		t.Fatalf("after force-break: PendingCleanupCount = %d, want 0", got)
	}
	if _, ok := failCount[target]; ok {
		t.Fatalf("failCount still has target after force-break")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target dir still present after force-break: stat err=%v", err)
	}
	if calls == 0 {
		t.Fatalf("removeAllFn was never exercised on the normal path")
	}
}
