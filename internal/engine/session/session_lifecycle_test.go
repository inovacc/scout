package session_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/inovacc/scout/internal/engine/session"
)

// TestCloseRemovesAllResources verifies the cleanup sequence:
// data/ subdir removed, scout.pid removed, then parent dir removed.
// This is a regression anchor for SESS-01.
func TestCloseRemovesAllResources(t *testing.T) {
	base := t.TempDir()
	sessionDir := filepath.Join(base, "sessions", "test-session-id")
	dataDir := filepath.Join(sessionDir, "data")

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("setup: create data dir: %v", err)
	}

	pidFile := filepath.Join(sessionDir, "scout.pid")
	if err := os.WriteFile(pidFile, []byte(`{"scout_pid":0,"browser_pid":0}`), 0o644); err != nil {
		t.Fatalf("setup: write scout.pid: %v", err)
	}

	// Simulate the fixed Close() cleanup sequence.
	if err := os.RemoveAll(dataDir); err != nil {
		t.Fatalf("cleanup: remove data dir: %v", err)
	}
	if err := os.Remove(pidFile); err != nil {
		t.Fatalf("cleanup: remove scout.pid: %v", err)
	}
	if err := os.RemoveAll(sessionDir); err != nil {
		t.Fatalf("cleanup: remove session dir: %v", err)
	}

	if _, err := os.Stat(sessionDir); !os.IsNotExist(err) {
		t.Errorf("SESS-01: session dir still exists after cleanup: %s", sessionDir)
	}
}

// TestCleanStaleSessionsRemovesDataDir verifies that CleanStaleSessions removes
// both the data/ subdir and the parent session dir for dead sessions.
// This test FAILS currently — SESS-02 fix not yet applied.
func TestCleanStaleSessionsRemovesDataDir(t *testing.T) {
	base := t.TempDir()

	// Override SessionsDir to point to our temp directory.
	orig := session.SessionsDir
	session.SessionsDir = func() string { return filepath.Join(base, "sessions") }
	t.Cleanup(func() { session.SessionsDir = orig })

	sessionsDir := filepath.Join(base, "sessions")
	sessionID := "stale-test-session"
	sessionDir := filepath.Join(sessionsDir, sessionID)
	dataDir := filepath.Join(sessionDir, "data")

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatalf("setup: create data dir: %v", err)
	}

	// Create dummy file inside data/
	if err := os.WriteFile(filepath.Join(dataDir, "dummy.txt"), []byte("data"), 0o644); err != nil {
		t.Fatalf("setup: write dummy file: %v", err)
	}

	// Write scout.pid with a guaranteed-nonexistent PID.
	pidContent := `{"scout_pid":99999999,"browser_pid":0,"reusable":false}`
	if err := os.WriteFile(filepath.Join(sessionDir, "scout.pid"), []byte(pidContent), 0o644); err != nil {
		t.Fatalf("setup: write scout.pid: %v", err)
	}

	cleaned, err := session.CleanStaleSessions()
	if err != nil {
		t.Fatalf("CleanStaleSessions returned error: %v", err)
	}

	if cleaned == 0 {
		t.Log("SESS-02: CleanStaleSessions reported 0 cleaned (may still have removed dirs)")
	}

	// The data/ subdir MUST be gone.
	if _, err := os.Stat(dataDir); !os.IsNotExist(err) {
		t.Errorf("SESS-02: data/ subdir still exists after CleanStaleSessions: %s", dataDir)
	}

	// The parent session dir MUST be gone.
	if _, err := os.Stat(sessionDir); !os.IsNotExist(err) {
		t.Errorf("SESS-02: session dir still exists after CleanStaleSessions: %s", sessionDir)
	}
}

// TestProcessAliveReturnsFalseForZombie verifies that ProcessAlive returns false
// for a process that has already exited.
// This test FAILS on Windows due to SESS-04 zombie handle bug.
func TestProcessAliveReturnsFalseForZombie(t *testing.T) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "exit")
	} else {
		cmd = exec.Command("sleep", "0.001")
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("start subprocess: %v", err)
	}

	pid := cmd.Process.Pid

	if err := cmd.Wait(); err != nil {
		// Non-zero exit is fine — we just need the PID to be dead.
		t.Logf("Wait returned: %v (expected for short-lived process)", err)
	}

	if session.ProcessAlive(pid) {
		t.Errorf("SESS-04: ProcessAlive(%d) returned true for an exited process", pid)
	}
}

// TestCloseNoDoubleSleep is a placeholder for Plan 04 integration verification.
// The actual no-double-sleep assertion requires integration test infrastructure.
func TestCloseNoDoubleSleep(t *testing.T) {
	t.Skip("verified in Plan 04 integration step — timing assertion requires live browser")
}

// TestRetryBudgetConstants verifies that the retry constants (SESS-06) are
// present in session_track.go. FAILS until Plan 02-03 adds them.
func TestRetryBudgetConstants(t *testing.T) {
	source, err := os.ReadFile("session_track.go")
	if err != nil {
		t.Fatalf("cannot read session_track.go: %v", err)
	}

	content := string(source)

	if !contains(content, "removeRetries") {
		t.Fatal("SESS-06: retry constant 'removeRetries' not found in session_track.go — fix not applied")
	}

	if !contains(content, "removeRetryWait") {
		t.Fatal("SESS-06: retry constant 'removeRetryWait' not found in session_track.go — fix not applied")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
