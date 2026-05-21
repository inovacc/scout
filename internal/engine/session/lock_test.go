package session

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAcquireLockBasic verifies AcquireLock creates scout.pid and that
// Release allows a fresh acquire to succeed.
func TestAcquireLockBasic(t *testing.T) {
	dir := t.TempDir()
	orig := SessionsDir
	SessionsDir = func() string { return dir }
	t.Cleanup(func() { SessionsDir = orig })

	g, err := AcquireLock("sess-lock-1")
	if err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}

	pidPath := filepath.Join(dir, "sess-lock-1", "scout.pid")
	if _, err := os.Stat(pidPath); err != nil {
		t.Fatalf("scout.pid not present after acquire: %v", err)
	}

	g.Release()

	// File persists; OS lock released. Re-acquire should succeed.
	g2, err := AcquireLock("sess-lock-1")
	if err != nil {
		t.Fatalf("second AcquireLock after release: %v", err)
	}
	g2.Release()

	// Idempotent release.
	g2.Release()
}

// TestAcquireLockConflict verifies a second exclusive acquire fails fast
// when the lock is held.
func TestAcquireLockConflict(t *testing.T) {
	dir := t.TempDir()
	orig := SessionsDir
	SessionsDir = func() string { return dir }
	t.Cleanup(func() { SessionsDir = orig })

	g1, err := AcquireLock("sess-busy")
	if err != nil {
		t.Fatalf("first AcquireLock: %v", err)
	}
	defer g1.Release()

	if g2, err := AcquireLock("sess-busy"); err == nil {
		g2.Release()
		t.Fatalf("expected second AcquireLock to fail while first is held")
	}
}

// TestAcquireSharedAllowsMultiple verifies shared locks coexist.
func TestAcquireSharedAllowsMultiple(t *testing.T) {
	dir := t.TempDir()
	orig := SessionsDir
	SessionsDir = func() string { return dir }
	t.Cleanup(func() { SessionsDir = orig })

	if err := WriteInfo("sess-shared", &SessionInfo{ScoutPID: 1, Browser: "chrome"}); err != nil {
		t.Fatalf("WriteInfo: %v", err)
	}

	g1, err := AcquireSharedLock("sess-shared")
	if err != nil {
		t.Fatalf("first shared acquire: %v", err)
	}
	defer g1.Release()

	g2, err := AcquireSharedLock("sess-shared")
	if err != nil {
		t.Fatalf("second shared acquire should succeed: %v", err)
	}
	defer g2.Release()
}

// TestExclusiveBlocksShared verifies a held exclusive lock blocks shared
// acquires.
func TestExclusiveBlocksShared(t *testing.T) {
	dir := t.TempDir()
	orig := SessionsDir
	SessionsDir = func() string { return dir }
	t.Cleanup(func() { SessionsDir = orig })

	g, err := AcquireLock("sess-mix")
	if err != nil {
		t.Fatalf("exclusive acquire: %v", err)
	}
	defer g.Release()

	if gs, err := AcquireSharedLock("sess-mix"); err == nil {
		gs.Release()
		t.Fatalf("expected shared acquire to fail while exclusive held")
	}
}
