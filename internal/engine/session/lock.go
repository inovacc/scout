package session

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// LockGuard represents an acquired session lock. Call Release to drop it.
// Releases are idempotent. The underlying OS lock is held via an open file
// handle on <session>/scout.pid; closing the handle releases the lock.
//
// Hardening M3 (revised) — replaces the legacy `.lock` mkdir-mutex with an
// OS-level advisory lock (LockFileEx on Windows, flock on Unix). This lets
// the same file carry both metadata (binary SessionInfo) and the lock, and
// the OS reclaims the lock automatically if the holder crashes — no more
// stale lock dirs to garbage-collect.
type LockGuard struct {
	f        *os.File
	released bool
}

// Release drops the lock by closing the file handle. Safe to call multiple
// times. The scout.pid file itself is preserved on disk.
func (g *LockGuard) Release() {
	if g == nil || g.released {
		return
	}
	g.released = true

	// Unlock first so any error from Close doesn't leave the OS lock held
	// (Windows: LockFileEx leaks if the process keeps a handle open even
	// briefly after we've decided to release).
	_ = unlockFile(g.f)

	if err := g.f.Close(); err != nil {
		slog.Warn("scout: lock release close failed",
			"path", g.f.Name(),
			"err", err,
		)
	}
}

// File exposes the underlying scout.pid file handle so callers that hold the
// lock can write to the same fd without re-opening (avoids racing readers).
func (g *LockGuard) File() *os.File {
	if g == nil {
		return nil
	}
	return g.f
}

// AcquireLock takes an exclusive cross-process advisory lock on a session's
// scout.pid file. Creates the file (zero-length) if it does not yet exist
// so callers can lock before writing the first SessionInfo.
//
// Returns immediately on success or contention — no spin-wait loop. If the
// lock is held elsewhere, returns an error with the session id for the
// caller to act on (audit/list use AcquireSharedLock instead).
func AcquireLock(id string) (*LockGuard, error) {
	return acquire(id, false)
}

// AcquireSharedLock takes a shared advisory lock on scout.pid. Multiple
// readers (audit, list) may hold the shared lock concurrently while no
// writer holds the exclusive lock.
func AcquireSharedLock(id string) (*LockGuard, error) {
	return acquire(id, true)
}

func acquire(id string, shared bool) (*LockGuard, error) {
	dir := Dir(id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("scout: lock: create session dir: %w", err)
	}

	path := filepath.Join(dir, "scout.pid")
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("scout: lock: open scout.pid: %w", err)
	}

	if err := lockFile(f, !shared); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("scout: lock: session %s busy: %w", id, err)
	}

	return &LockGuard{f: f}, nil
}
