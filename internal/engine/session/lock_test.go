package session

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// TestAcquireLockBasic verifies basic acquire and release.
func TestAcquireLockBasic(t *testing.T) {
	dir := t.TempDir()
	orig := SessionsDir
	SessionsDir = func() string { return dir }
	t.Cleanup(func() { SessionsDir = orig })

	g, err := AcquireLock("sess-lock-1")
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}

	lockDir := filepath.Join(dir, "sess-lock-1", ".lock")
	if _, err := os.Stat(lockDir); err != nil {
		t.Errorf("lock dir not created: %v", err)
	}

	g.Release()

	if _, err := os.Stat(lockDir); !os.IsNotExist(err) {
		t.Errorf("lock dir not removed after Release: %v", err)
	}

	// Idempotent release.
	g.Release()
}

// TestAcquireLockConflictTimesOut verifies second acquire returns an error
// when the first lock is held by a live process.
func TestAcquireLockConflictTimesOut(t *testing.T) {
	dir := t.TempDir()
	orig := SessionsDir
	SessionsDir = func() string { return dir }
	t.Cleanup(func() { SessionsDir = orig })

	g1, err := AcquireLock("sess-busy")
	if err != nil {
		t.Fatalf("first AcquireLock: %v", err)
	}

	t.Cleanup(g1.Release)

	// Short timeout for test speed: temporarily lower would require API
	// change. Instead manually craft a busy state and expect failure
	// after the configured timeout. To keep this fast, just assert that
	// a second acquire returns an error eventually.
	type result struct {
		g   *LockGuard
		err error
	}

	ch := make(chan result, 1)

	go func() {
		g, err := AcquireLock("sess-busy")
		ch <- result{g, err}
	}()

	select {
	case r := <-ch:
		if r.err == nil {
			r.g.Release()
			t.Fatalf("expected timeout error; got nil")
		}
	case <-time.After(lockTimeout + 2*time.Second):
		t.Fatalf("AcquireLock did not return within %s", lockTimeout+2*time.Second)
	}
}

// TestAcquireLockStaleHolder verifies that a lock left behind by a dead
// process (PID not alive) is treated as stale and replaced.
func TestAcquireLockStaleHolder(t *testing.T) {
	dir := t.TempDir()
	orig := SessionsDir
	SessionsDir = func() string { return dir }
	t.Cleanup(func() { SessionsDir = orig })

	// Manually plant a stale lock with a guaranteed-dead PID.
	sessionDir := filepath.Join(dir, "sess-stale")
	if err := os.MkdirAll(sessionDir, 0o700); err != nil {
		t.Fatalf("setup mkdir: %v", err)
	}

	lockDir := filepath.Join(sessionDir, ".lock")
	if err := os.Mkdir(lockDir, 0o700); err != nil {
		t.Fatalf("setup mkdir lock: %v", err)
	}

	if err := os.WriteFile(
		filepath.Join(lockDir, "pid"),
		[]byte(strconv.Itoa(999999999)),
		0o600,
	); err != nil {
		t.Fatalf("setup write pid: %v", err)
	}

	// Acquire should clean the stale lock and succeed quickly.
	start := time.Now()

	g, err := AcquireLock("sess-stale")
	if err != nil {
		t.Fatalf("AcquireLock on stale lock: %v", err)
	}

	if time.Since(start) > lockTimeout {
		t.Errorf("stale-lock acquire took too long: %s", time.Since(start))
	}

	g.Release()
}
