package session

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	idpkg "github.com/inovacc/scout/pkg/id"
)

// writeSession writes a scout.pid for a fresh random session ID and returns it.
// The session ID's Reusable attr is derived from info.Reusable so that ReadInfo
// (which overwrites the binary field from idpkg.Parse) returns the correct value.
func writeSession(t *testing.T, info *SessionInfo) string {
	t.Helper()

	sid, err := idpkg.New(idpkg.Attrs{Reusable: info.Reusable})
	if err != nil {
		t.Fatalf("idpkg.New: %v", err)
	}
	if err := WriteInfo(sid, info); err != nil {
		t.Fatalf("WriteInfo: %v", err)
	}

	return sid
}

func TestReapOnce_RemovesOrphanWithDeadScout(t *testing.T) {
	dir := t.TempDir()
	orig := SessionsDir
	SessionsDir = func() string { return dir }
	t.Cleanup(func() { SessionsDir = orig })

	// Non-reusable session whose recorded scout PID is definitely dead.
	now := time.Now()
	sid := writeSession(t, &SessionInfo{
		ScoutPID:   deadPID(t),
		BrowserPID: 0, // no live browser → only dir removal exercised
		Reusable:   false,
		CreatedAt:  now,
		LastUsed:   now,
		Browser:    "chrome",
	})

	stats := ReapOnce()

	if stats.Scanned < 1 {
		t.Fatalf("Scanned = %d, want >= 1", stats.Scanned)
	}
	if stats.Removed < 1 {
		t.Fatalf("Removed = %d, want >= 1", stats.Removed)
	}
	if _, err := os.Stat(Dir(sid)); !os.IsNotExist(err) {
		t.Fatalf("session dir %q still present after reap (err=%v)", Dir(sid), err)
	}
}

func TestReapOnce_PreservesOwnedSession(t *testing.T) {
	dir := t.TempDir()
	orig := SessionsDir
	SessionsDir = func() string { return dir }
	t.Cleanup(func() { SessionsDir = orig })

	now := time.Now()
	// ScoutPID = our own PID. IsScoutProcess(os.Getpid()) is true under the
	// test binary only if gops can classify it as a scout exec; it cannot
	// (the test binary is not named scout), so this folder is NOT owned and
	// would normally be reaped. To assert the *preserve* path independent of
	// process identity, use a reusable, not-yet-expired session — that branch
	// short-circuits before any liveness check.
	sid := writeSession(t, &SessionInfo{
		ScoutPID:  deadPID(t),
		Reusable:  true,
		ExpiresAt: now.Add(1 * time.Hour),
		CreatedAt: now,
		LastUsed:  now,
		Browser:   "chrome",
	})

	stats := ReapOnce()

	if stats.Removed != 0 {
		t.Fatalf("Removed = %d, want 0 (reusable unexpired must be preserved)", stats.Removed)
	}
	if _, err := os.Stat(Dir(sid)); err != nil {
		t.Fatalf("reusable session dir %q was removed: %v", Dir(sid), err)
	}
}

func TestReapOnce_RemovesCorruptDir(t *testing.T) {
	dir := t.TempDir()
	orig := SessionsDir
	SessionsDir = func() string { return dir }
	t.Cleanup(func() { SessionsDir = orig })

	// Fabricate a session dir with a corrupt (non-binary) scout.pid — this is
	// the un-killable-zombie case: ReadInfo fails, so classification by
	// metadata is impossible and the dir must still be reaped.
	sid, err := idpkg.New(idpkg.Attrs{})
	if err != nil {
		t.Fatalf("idpkg.New: %v", err)
	}
	corrupt := Dir(sid)
	if err := os.MkdirAll(filepath.Join(corrupt, "data"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(corrupt, "scout.pid"), []byte("not-binary-garbage"), 0o600); err != nil {
		t.Fatalf("write corrupt pid: %v", err)
	}

	stats := ReapOnce()

	if stats.Removed < 1 {
		t.Fatalf("Removed = %d, want >= 1 (corrupt dir must be reaped)", stats.Removed)
	}
	if _, err := os.Stat(corrupt); !os.IsNotExist(err) {
		t.Fatalf("corrupt session dir %q still present after reap", corrupt)
	}
}

func TestReapOnce_RemovesMissingPidDir(t *testing.T) {
	dir := t.TempDir()
	orig := SessionsDir
	SessionsDir = func() string { return dir }
	t.Cleanup(func() { SessionsDir = orig })

	// Orphan dir with no scout.pid at all.
	sid, err := idpkg.New(idpkg.Attrs{})
	if err != nil {
		t.Fatalf("idpkg.New: %v", err)
	}
	orphan := Dir(sid)
	if err := os.MkdirAll(filepath.Join(orphan, "data"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	stats := ReapOnce()

	if stats.Removed < 1 {
		t.Fatalf("Removed = %d, want >= 1 (missing-pid dir must be reaped)", stats.Removed)
	}
	if _, err := os.Stat(orphan); !os.IsNotExist(err) {
		t.Fatalf("orphan session dir %q still present after reap", orphan)
	}
}

// deadPID returns a PID that is guaranteed not to be alive: it spawns a
// trivial child, waits for it to exit, and returns its (now reusable but
// currently dead) PID. ProcessAlive/IsScoutProcess on it return false.
func deadPID(t *testing.T) int {
	t.Helper()

	// A process that exits immediately. `cmd /c exit` on Windows, `true`
	// elsewhere — both are universally present.
	pid := spawnAndReapExited(t)
	if ProcessAlive(pid) {
		t.Fatalf("expected dead PID, %d is still alive", pid)
	}

	return pid
}

func TestCleanStaleSessions_WrapsReapOnce(t *testing.T) {
	dir := t.TempDir()
	orig := SessionsDir
	SessionsDir = func() string { return dir }
	t.Cleanup(func() { SessionsDir = orig })

	now := time.Now()
	sid := writeSession(t, &SessionInfo{
		ScoutPID:  deadPID(t),
		Reusable:  false,
		CreatedAt: now,
		LastUsed:  now,
		Browser:   "chrome",
	})

	n, err := CleanStaleSessions()
	if err != nil {
		t.Fatalf("CleanStaleSessions: %v", err)
	}
	if n < 1 {
		t.Fatalf("CleanStaleSessions returned %d, want >= 1 (Removed count)", n)
	}
	if _, statErr := os.Stat(Dir(sid)); !os.IsNotExist(statErr) {
		t.Fatalf("session dir still present after CleanStaleSessions")
	}
}

func TestCleanOrphans_WrapsReapOnce(t *testing.T) {
	dir := t.TempDir()
	orig := SessionsDir
	SessionsDir = func() string { return dir }
	t.Cleanup(func() { SessionsDir = orig })

	now := time.Now()
	// Dead scout, no live browser → 0 killed but dir removed. CleanOrphans
	// must report the Killed count (0 here) and not error.
	_ = writeSession(t, &SessionInfo{
		ScoutPID:  deadPID(t),
		Reusable:  false,
		CreatedAt: now,
		LastUsed:  now,
		Browser:   "chrome",
	})

	killed, err := CleanOrphans()
	if err != nil {
		t.Fatalf("CleanOrphans: %v", err)
	}
	if killed != 0 {
		t.Fatalf("CleanOrphans killed = %d, want 0 (no live browser)", killed)
	}
}

func spawnAndReapExited(t *testing.T) int {
	t.Helper()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "exit")
	} else {
		cmd = exec.Command("true")
	}
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot spawn helper process: %v", err)
	}
	pid := cmd.Process.Pid
	_ = cmd.Wait()
	return pid
}
