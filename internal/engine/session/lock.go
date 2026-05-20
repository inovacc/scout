package session

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// LockGuard represents an acquired session lock. Call Release to drop it.
// Releases are idempotent.
type LockGuard struct {
	lockDir  string
	released bool
}

// Release drops the lock by removing the lock directory. Safe to call
// multiple times.
func (g *LockGuard) Release() {
	if g == nil || g.released {
		return
	}

	g.released = true

	if err := os.RemoveAll(g.lockDir); err != nil {
		slog.Warn("scout: lock release failed",
			"dir", g.lockDir,
			"err", err,
		)
	}
}

const (
	lockTimeout    = 5 * time.Second
	lockPollWait   = 50 * time.Millisecond
	lockPIDMarker  = "pid"
	lockDirSubpath = ".lock"
)

// AcquireLock takes an exclusive cross-process lock on a session by atomically
// creating a `.lock` subdirectory under the session dir. The current PID is
// written inside so a subsequent acquirer can detect a stale lock left by a
// crashed process (PID no longer alive).
//
// Blocks up to lockTimeout (5s), polling at lockPollWait intervals. Returns
// an error if the timeout expires.
//
// Hardening M3 — prevents read-modify-write races on `scout.pid` between two
// scout processes both claiming the same reusable session.
func AcquireLock(id string) (*LockGuard, error) {
	dir := Dir(id)

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("scout: lock: create session dir: %w", err)
	}

	lockDir := filepath.Join(dir, lockDirSubpath)
	deadline := time.Now().Add(lockTimeout)

	for {
		err := os.Mkdir(lockDir, 0o700)
		if err == nil {
			_ = os.WriteFile(
				filepath.Join(lockDir, lockPIDMarker),
				[]byte(strconv.Itoa(os.Getpid())),
				0o600,
			)

			return &LockGuard{lockDir: lockDir}, nil
		}

		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("scout: lock: %w", err)
		}

		// Lock exists — check if the holder is still alive.
		if isStaleLock(lockDir) {
			_ = os.RemoveAll(lockDir)

			continue
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("scout: lock: session %s busy after %s", id, lockTimeout)
		}

		time.Sleep(lockPollWait)
	}
}

// isStaleLock reports whether the lock directory was left behind by a process
// that no longer exists. A lock without a readable PID file is considered
// fresh (some other acquirer is mid-write).
func isStaleLock(lockDir string) bool {
	data, err := os.ReadFile(filepath.Join(lockDir, lockPIDMarker))
	if err != nil {
		return false
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return false
	}

	return !ProcessAlive(pid)
}
