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

// spawnHolder starts a long-lived child whose argv contains
// --user-data-dir=<dataDir> so FindBrowsersUsingDataDir can match it. The
// binary name must look like a browser to isBrowserCmdline; we cannot rename
// the system sleep binary, so this acceptance test is skipped on platforms
// where the holder cannot be made discoverable. On Linux the child's
// /proc/<pid>/cmdline is matched on the data-dir substring AND the browser
// token; we satisfy isBrowserCmdline by invoking through a compiled "chrome"
// shim when available, else skip.
func spawnHolder(t *testing.T, dataDir string) *exec.Cmd {
	t.Helper()

	shim := browserShim(t) // returns a path whose basename matches isBrowserCmdline, or "" to skip
	if shim == "" {
		t.Skipf("no browser-named shim available on %s; skipping holder acceptance", runtime.GOOS)
	}

	// The shim is a tiny sleeper; pass the data-dir flag so the scan matches.
	cmd := exec.Command(shim, "--user-data-dir="+dataDir, "--sleep")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot start holder: %v", err)
	}

	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})

	// Give the OS a beat to publish the cmdline.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(FindBrowsersUsingDataDir(dataDir)) > 0 {
			return cmd
		}

		time.Sleep(50 * time.Millisecond)
	}

	t.Skipf("holder cmdline not observable via FindBrowsersUsingDataDir on %s", runtime.GOOS)

	return cmd
}

func TestReapOnce_Acceptance_KillsHolderAndRemovesDir(t *testing.T) {
	dir := t.TempDir()
	orig := SessionsDir
	SessionsDir = func() string { return dir }
	t.Cleanup(func() { SessionsDir = orig })

	now := time.Now()

	sid, err := idpkg.New(idpkg.Attrs{})
	if err != nil {
		t.Fatalf("idpkg.New: %v", err)
	}

	// Fabricate the leak: dead scout, a live child holding the data dir.
	holder := spawnHolder(t, DataDir(sid))
	holderPID := holder.Process.Pid

	if err := WriteInfo(sid, &SessionInfo{
		ScoutPID:   spawnAndReapExited(t), // definitely dead
		BrowserPID: holderPID,
		Reusable:   false,
		CreatedAt:  now,
		LastUsed:   now,
		Browser:    "chrome",
	}); err != nil {
		t.Fatalf("WriteInfo: %v", err)
	}

	stats := ReapOnce()

	if stats.Killed < 1 {
		t.Fatalf("Killed = %d, want >= 1 (holder must be killed)", stats.Killed)
	}

	// Poll until holder is dead — bounded at 5 s to avoid flakiness.
	deadline := time.Now().Add(5 * time.Second)
	for ProcessAlive(holderPID) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}

	if ProcessAlive(holderPID) {
		t.Fatalf("holder PID %d still alive after reap", holderPID)
	}

	// Dir removed.
	if _, err := os.Stat(Dir(sid)); !os.IsNotExist(err) {
		t.Fatalf("session dir %q still present after reap", Dir(sid))
	}
}

func TestReapOnce_Acceptance_NeverKillsSelf(t *testing.T) {
	dir := t.TempDir()
	orig := SessionsDir
	SessionsDir = func() string { return dir }
	t.Cleanup(func() { SessionsDir = orig })

	now := time.Now()

	sid, err := idpkg.New(idpkg.Attrs{})
	if err != nil {
		t.Fatalf("idpkg.New: %v", err)
	}

	// Owner is dead so the folder would be reaped, but record the CURRENT
	// process as the BrowserPID. reapFolder must skip os.Getpid() in the
	// recorded-PID kill path (guard: browserPID != os.Getpid()) and must not
	// kill the test runner via the scan path either (our cmdline is the test
	// binary, not a browser with this data dir).
	if err := WriteInfo(sid, &SessionInfo{
		ScoutPID:   spawnAndReapExited(t),
		BrowserPID: os.Getpid(),
		Reusable:   false,
		CreatedAt:  now,
		LastUsed:   now,
		Browser:    "chrome",
	}); err != nil {
		t.Fatalf("WriteInfo: %v", err)
	}

	// If reapFolder killed os.Getpid() the test process would die; reaching
	// the assertion below proves it did not.
	_ = ReapOnce()

	if !ProcessAlive(os.Getpid()) {
		t.Fatalf("unreachable: current process killed")
	}
}

// browserShim returns the path to a sleeper binary whose basename matches
// isBrowserCmdline (e.g. "chrome"/"chrome.exe"), built into t.TempDir on the
// fly from a tiny Go program, or "" if it cannot be produced (→ skip).
// TestReapSession_KillsCorruptPidHolder proves that ReapSession kills a live
// browser holding the session data dir even when scout.pid is MISSING/CORRUPT
// (the ReadInfo path returns an error, so BrowserPID is unknown). The old
// scout.ResetSession path only killed the recorded BrowserPID and skipped the
// kill entirely when that was zero — leaving a zombie. ReapSession must kill
// via the path-bounded scan (FindBrowsersUsingDataDir) regardless.
//
// RED before Part A (ReapSession unexported / does not exist).
// GREEN after Part A+B+C.
func TestReapSession_KillsCorruptPidHolder(t *testing.T) {
	dir := t.TempDir()
	orig := SessionsDir
	SessionsDir = func() string { return dir }
	t.Cleanup(func() { SessionsDir = orig })

	sid, err := idpkg.New(idpkg.Attrs{})
	if err != nil {
		t.Fatalf("idpkg.New: %v", err)
	}

	// Create the session data dir but write CORRUPT scout.pid so ReadInfo fails.
	dataDir := DataDir(sid)
	if mkErr := os.MkdirAll(dataDir, 0o700); mkErr != nil {
		t.Fatalf("mkdir data dir: %v", mkErr)
	}
	if wErr := os.WriteFile(filepath.Join(Dir(sid), "scout.pid"), []byte("not-binary-garbage"), 0o600); wErr != nil {
		t.Fatalf("write corrupt pid: %v", wErr)
	}

	// Spawn a real child holding --user-data-dir=<dataDir>. This simulates a
	// zombie chrome that survived because scout.pid was unreadable.
	holder := spawnHolder(t, dataDir)
	holderPID := holder.Process.Pid

	// Sanity: holder must be visible before we call ReapSession.
	if len(FindBrowsersUsingDataDir(dataDir)) == 0 {
		t.Skip("holder not observable via FindBrowsersUsingDataDir; skipping")
	}

	stats := ReapSession(sid)

	// The scan-kill path must have fired.
	if stats.Killed < 1 {
		t.Fatalf("ReapSession Killed = %d, want >= 1 (corrupt-pid zombie must be killed via scan path)", stats.Killed)
	}

	// Poll until the OS confirms the holder is dead (bounded at 5 s).
	deadline := time.Now().Add(5 * time.Second)
	for ProcessAlive(holderPID) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if ProcessAlive(holderPID) {
		t.Fatalf("holder PID %d still alive after ReapSession — corrupt-pid zombie not killed", holderPID)
	}

	// Session dir must be removed.
	if _, statErr := os.Stat(Dir(sid)); !os.IsNotExist(statErr) {
		t.Fatalf("session dir %q still present after ReapSession", Dir(sid))
	}
}

func browserShim(t *testing.T) string {
	t.Helper()

	src := `package main

import (
	"os"
	"time"
)

func main() {
	_ = os.Args
	time.Sleep(60 * time.Second)
}
`

	td := t.TempDir()
	srcPath := filepath.Join(td, "shim.go")

	if err := os.WriteFile(srcPath, []byte(src), 0o600); err != nil {
		return ""
	}

	name := "chrome"
	if runtime.GOOS == "windows" {
		name = "chrome.exe"
	}

	binPath := filepath.Join(td, name)

	build := exec.Command("go", "build", "-o", binPath, srcPath)
	build.Env = os.Environ()

	if out, err := build.CombinedOutput(); err != nil {
		t.Logf("shim build failed (%v): %s", err, out)
		return ""
	}

	return binPath
}
