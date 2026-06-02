package session

import (
	"os"
	"path/filepath"
	"runtime"
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

// TestRetryPendingRemovesBeforeThreshold proves a dir that removes cleanly on
// the first sweep is dequeued immediately and the force-break path is never
// taken (failCount never reaches the threshold).
func TestRetryPendingRemovesBeforeThreshold(t *testing.T) {
	resetPending(t)

	dir := t.TempDir()
	target := filepath.Join(dir, "easy-sess")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}

	// Default removeAllFn (os.RemoveAll) — succeeds on the first try.
	recordCleanupFailure(target)
	if got := PendingCleanupCount(); got != 1 {
		t.Fatalf("PendingCleanupCount after enqueue = %d, want 1", got)
	}

	failCount := make(map[string]int)
	retryPending(failCount)

	if got := PendingCleanupCount(); got != 0 {
		t.Fatalf("after one sweep: PendingCleanupCount = %d, want 0", got)
	}
	if _, ok := failCount[target]; ok {
		t.Fatalf("failCount unexpectedly tracked a successfully-removed dir")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target still present after clean removal: %v", err)
	}
}

// TestRetryPendingForceBreakRealLock exercises the force-break path against a
// genuinely locked file (open handle) on Windows, where os.RemoveAll fails
// while a handle is held. The handle is released just before the threshold
// sweep so the force-break (forceBreakDir -> os.RemoveAll loop / rmdirLowLevel)
// can finally remove the dir. Non-Windows cannot reproduce the lock (open
// handles do not block unlink) and is skipped.
func TestRetryPendingForceBreakRealLock(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("open-handle removal lock is Windows-specific")
	}
	resetPending(t)

	dir := t.TempDir()
	target := filepath.Join(dir, "locked-sess")
	if err := os.MkdirAll(target, 0o700); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	locked := filepath.Join(target, "scout.lock")
	f, err := os.Create(locked)
	if err != nil {
		t.Fatalf("create locked file: %v", err)
	}
	released := false
	releaseOnce := func() {
		if !released {
			_ = f.Close()
			released = true
		}
	}
	t.Cleanup(releaseOnce)

	// Use the real remover so the open handle actually blocks removal.
	recordCleanupFailure(target)
	failCount := make(map[string]int)

	// Sweeps 1..threshold-1: handle held → RemoveAll fails → stays queued.
	for sweep := range forceBreakThreshold - 1 {
		retryPending(failCount)
		if PendingCleanupCount() != 1 {
			t.Fatalf("sweep %d: dir dequeued early while locked", sweep+1)
		}
	}

	// Release the handle, then the threshold-th sweep force-breaks the dir.
	releaseOnce()
	retryPending(failCount)

	if got := PendingCleanupCount(); got != 0 {
		t.Fatalf("after force-break: PendingCleanupCount = %d, want 0", got)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("locked dir survived force-break: %v", err)
	}
}
