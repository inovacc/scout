package session

import (
	"os"
	"path/filepath"
	"testing"
)

// TestReadInfoAcceptsOwnedDir verifies ReadInfo succeeds when the session
// directory is owned by the current user (default validateDirOwner behavior).
func TestReadInfoAcceptsOwnedDir(t *testing.T) {
	dir := t.TempDir()

	orig := SessionsDir
	SessionsDir = func() string { return dir }
	t.Cleanup(func() { SessionsDir = orig })

	info := &SessionInfo{ScoutPID: 42, BrowserPID: 43, Browser: "chrome"}
	if err := WriteInfo("sess-owned", info); err != nil {
		t.Fatalf("WriteInfo: %v", err)
	}

	got, err := ReadInfo("sess-owned")
	if err != nil {
		t.Fatalf("ReadInfo on self-owned dir: %v", err)
	}

	if got.ScoutPID != 42 {
		t.Errorf("ScoutPID = %d, want 42", got.ScoutPID)
	}
}

// TestReadInfoRejectsForeignDir verifies ReadInfo refuses to read from a
// directory marked as non-self-owned. This is the H4 takeover guard: an
// attacker pre-creates ~/.scout/sessions/<predicted>/ with a poisoned
// scout.pid, and ReadInfo must refuse to trust it.
func TestReadInfoRejectsForeignDir(t *testing.T) {
	dir := t.TempDir()

	origSessions := SessionsDir
	SessionsDir = func() string { return dir }
	t.Cleanup(func() { SessionsDir = origSessions })

	// Simulate the attacker-planted dir: create it on disk, plant a valid
	// scout.pid, then stub the owner check to report it as foreign.
	id := "sess-foreign"
	sessionDir := filepath.Join(dir, id)

	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatalf("setup mkdir: %v", err)
	}

	if err := os.WriteFile(
		filepath.Join(sessionDir, "scout.pid"),
		[]byte(`{"scout_pid":1,"browser_pid":2,"browser":"chrome"}`),
		0o644,
	); err != nil {
		t.Fatalf("setup write: %v", err)
	}

	origOwner := dirOwnerCheck
	dirOwnerCheck = func(string) bool { return false }
	t.Cleanup(func() { dirOwnerCheck = origOwner })

	if _, err := ReadInfo(id); err == nil {
		t.Errorf("ReadInfo on foreign dir: want error, got nil")
	}
}

// TestValidateDirOwnerSelf verifies the platform implementation accepts a
// directory created by the current process (i.e., owned by current uid/SID).
func TestValidateDirOwnerSelf(t *testing.T) {
	dir := t.TempDir()

	if !validateDirOwner(dir) {
		t.Errorf("validateDirOwner on self-created TempDir = false, want true")
	}
}

// TestValidateDirOwnerMissing verifies a non-existent path is rejected.
func TestValidateDirOwnerMissing(t *testing.T) {
	if validateDirOwner("/does/not/exist/scout-h4-test") {
		t.Errorf("validateDirOwner on missing path = true, want false")
	}
}
