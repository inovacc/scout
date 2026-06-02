package main

// Task 5.3 — crash→reap ACCEPTANCE TEST
//
// Proves the user's core invariant end-to-end:
//   "<scouthome>/sessions/ is clean unless a live, verified scout process
//    owns each folder; every folder maps to a verifiable parent (scout) PID,
//    and every scout-launched browser has a live scout parent (no orphans)."
//
// This test operates at the cmd/scout level, exercising:
//   auditAllSessions() + doctorViolations() + enforceAuditCleanup()
//   + session.ReapOnce() + newSessionDoctorCmd()
//
// It deliberately DOES NOT duplicate Task 2.4 (reaper_acceptance_test.go).
// The 2.4 test proves the kill/remove mechanics in the session package.
// This test proves the doctor verdict transitions VIOLATION→CLEAN across
// a reap — the observable invariant guarantee exposed to operators.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/inovacc/scout/internal/engine/session"
	idpkg "github.com/inovacc/scout/pkg/id"
)

// ── helpers ─────────────────────────────────────────────────────────────────

// withTempSessionsAcceptance is the same as withTempSessions but independently
// defined here so this file does not depend on test helpers from other files.
func withTempSessionsAcceptance(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	orig := session.SessionsDir
	session.SessionsDir = func() string { return dir }
	t.Cleanup(func() { session.SessionsDir = orig })

	return dir
}

// spawnDeadPID starts a process and waits for it to exit, returning a PID
// that is guaranteed dead.
func spawnDeadPID(t *testing.T) int {
	t.Helper()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", "exit")
	} else {
		cmd = exec.Command("true")
	}

	if err := cmd.Start(); err != nil {
		t.Skipf("cannot spawn exit helper: %v", err)
	}

	pid := cmd.Process.Pid
	_ = cmd.Wait()

	return pid
}

// buildChromeShim compiles a tiny Go sleeper whose binary is named chrome (or
// chrome.exe on Windows) so FindBrowsersUsingDataDir matches it. Returns "" to
// skip if compilation fails.
func buildChromeShim(t *testing.T) string {
	t.Helper()

	src := `package main

import "time"

func main() {
	// Intentionally do NOT call flag.Parse(): Go's flag package calls
	// os.Exit(2) on unrecognised flags like --user-data-dir=..., which
	// would kill the shim before the reaper test can observe it.
	// Just sleep so the process stays alive with its full cmdline visible.
	time.Sleep(10 * time.Minute)
}
`
	shimDir := t.TempDir()

	srcFile := filepath.Join(shimDir, "main.go")
	if err := os.WriteFile(srcFile, []byte(src), 0o600); err != nil {
		t.Logf("chrome shim: write source: %v — skipping", err)

		return ""
	}

	shimName := "chrome"
	if runtime.GOOS == "windows" {
		shimName = "chrome.exe"
	}

	outBin := filepath.Join(shimDir, shimName)

	buildCmd := exec.Command("go", "build", "-o", outBin, srcFile)
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Logf("chrome shim: go build: %v\n%s — skipping", err, out)

		return ""
	}

	return outBin
}

// spawnHolderProcess starts the chrome shim with --user-data-dir pointing at
// dataDir. It blocks until FindBrowsersUsingDataDir sees the process, or
// skips the test if the cmdline is not observable on this platform.
func spawnHolderProcess(t *testing.T, shim, dataDir string) *exec.Cmd {
	t.Helper()

	cmd := exec.Command(shim, "--user-data-dir="+dataDir, "--some-flag")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot start chrome shim: %v", err)
	}

	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})

	// Poll until the OS publishes the cmdline (bounded at 3 s).
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if len(session.FindBrowsersUsingDataDir(dataDir)) > 0 {
			return cmd
		}

		time.Sleep(50 * time.Millisecond)
	}

	t.Skipf("chrome shim cmdline not observable via FindBrowsersUsingDataDir on %s", runtime.GOOS)

	return cmd
}

// pollUntilDead polls ProcessAlive(pid) for up to timeout, returning true when
// the process has exited.
func pollUntilDead(pid int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for session.ProcessAlive(pid) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}

	return !session.ProcessAlive(pid)
}

// ── acceptance tests ─────────────────────────────────────────────────────────

// TestSessionDoctorAcceptance_ViolationToClean is the Task 5.3 end-to-end proof.
//
// Scenario:
//  1. Fabricate a ZOMBIE session: dead scout PID + live chrome shim holding
//     the session data dir with --user-data-dir.
//  2. Assert doctor reports VIOLATION (invariant broken).
//  3. Run session.ReapOnce() (the enforcement path).
//  4. Assert the chrome shim process is dead (bounded poll ≤ 5 s).
//  5. Assert the session folder is gone.
//  6. Assert doctor now reports CLEAN (invariant restored).
func TestSessionDoctorAcceptance_ViolationToClean(t *testing.T) {
	dir := withTempSessionsAcceptance(t)

	shim := buildChromeShim(t)
	if shim == "" {
		t.Skip("chrome shim could not be compiled; skipping acceptance test")
	}

	now := time.Now()

	sid, err := idpkg.New(idpkg.Attrs{})
	if err != nil {
		t.Fatalf("idpkg.New: %v", err)
	}

	// Make sure the data dir exists (session package needs the dir to store
	// scout.pid; the chrome shim will hold it open on Unix via cmdline).
	if err := os.MkdirAll(session.DataDir(sid), 0o700); err != nil {
		t.Fatalf("mkdir DataDir: %v", err)
	}

	// Spawn the holder. spawnHolderProcess skips on platforms where cmdline
	// is not observable — that is acceptable.
	holder := spawnHolderProcess(t, shim, session.DataDir(sid))
	holderPID := holder.Process.Pid

	// Write the session: dead scout, live holder.
	if err := session.WriteInfo(sid, &session.SessionInfo{
		ScoutPID:   spawnDeadPID(t),
		BrowserPID: holderPID,
		Reusable:   false,
		CreatedAt:  now,
		LastUsed:   now,
		Browser:    "chrome",
	}); err != nil {
		t.Fatalf("WriteInfo: %v", err)
	}

	// ── Step 2: doctor must see a VIOLATION before the reap ─────────────────
	doctorBefore := newSessionDoctorCmd()
	var outBefore bytes.Buffer
	doctorBefore.SetOut(&outBefore)
	doctorBefore.SetErr(&outBefore)
	doctorBefore.SetArgs(nil)

	errBefore := doctorBefore.RunE(doctorBefore, nil)
	if errBefore == nil {
		t.Fatalf("doctor BEFORE reap: expected non-nil error (violation), got nil\noutput:\n%s", outBefore.String())
	}

	if !strings.Contains(outBefore.String(), "VIOLATED") {
		t.Fatalf("doctor BEFORE reap: expected VIOLATED in output, got:\n%s", outBefore.String())
	}

	t.Logf("doctor BEFORE reap (expected VIOLATION):\n%s", outBefore.String())

	// ── Step 3: reap ─────────────────────────────────────────────────────────
	stats := session.ReapOnce()
	t.Logf("ReapOnce stats: scanned=%d killed=%d removed=%d pending=%d",
		stats.Scanned, stats.Killed, stats.Removed, stats.Pending)

	if stats.Killed < 1 {
		t.Fatalf("ReapOnce.Killed = %d, want >= 1 (holder must be killed)", stats.Killed)
	}

	// ── Step 4: holder must die within 5 s ───────────────────────────────────
	if !pollUntilDead(holderPID, 5*time.Second) {
		t.Fatalf("holder PID %d still alive %v after reap — reaper BLOCKED", holderPID, 5*time.Second)
	}

	t.Logf("holder PID %d confirmed dead after reap", holderPID)

	// ── Step 5: session folder must be gone ──────────────────────────────────
	if _, err := os.Stat(session.Dir(sid)); !os.IsNotExist(err) {
		t.Fatalf("session dir %q still present after reap (os.Stat err=%v)", session.Dir(sid), err)
	}

	// ── Step 6: doctor must now report CLEAN ─────────────────────────────────
	doctorAfter := newSessionDoctorCmd()
	var outAfter bytes.Buffer
	doctorAfter.SetOut(&outAfter)
	doctorAfter.SetErr(&outAfter)
	doctorAfter.SetArgs(nil)

	if err := doctorAfter.RunE(doctorAfter, nil); err != nil {
		t.Fatalf("doctor AFTER reap: expected nil (clean), got %v\noutput:\n%s", err, outAfter.String())
	}

	if !strings.Contains(outAfter.String(), "invariant holds") {
		t.Fatalf("doctor AFTER reap: expected 'invariant holds', got:\n%s", outAfter.String())
	}

	t.Logf("doctor AFTER reap (expected CLEAN): %s", strings.TrimSpace(outAfter.String()))

	// Verify temp dir is also empty (nothing leaked).
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("sessions dir not empty after reap: %v", entries)
	}
}

// TestSessionDoctorAcceptance_NegativeControl_ReusablePreserved verifies that a
// reusable, not-yet-expired session is NOT reaped and doctor classifies it as
// REUSABLE (not a violation).
func TestSessionDoctorAcceptance_NegativeControl_ReusablePreserved(t *testing.T) {
	withTempSessionsAcceptance(t)

	now := time.Now()

	sid, err := idpkg.New(idpkg.Attrs{Reusable: true})
	if err != nil {
		t.Fatalf("idpkg.New: %v", err)
	}

	if err := session.WriteInfo(sid, &session.SessionInfo{
		ScoutPID:   spawnDeadPID(t), // dead scout, but reusable so must be preserved
		BrowserPID: 0,
		Reusable:   true,
		ExpiresAt:  now.Add(1 * time.Hour),
		CreatedAt:  now,
		LastUsed:   now,
		Browser:    "chrome",
	}); err != nil {
		t.Fatalf("WriteInfo: %v", err)
	}

	// Reap must NOT remove the reusable session.
	stats := session.ReapOnce()
	if stats.Removed != 0 {
		t.Fatalf("ReapOnce removed reusable session (Removed=%d); must be 0", stats.Removed)
	}

	if _, err := os.Stat(session.Dir(sid)); err != nil {
		t.Fatalf("reusable session dir removed unexpectedly: %v", err)
	}

	// Doctor treats a reusable, not-yet-expired session as non-violating —
	// it classifies as REUSABLE which is not in doctorViolations output.
	cmd := newSessionDoctorCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(nil)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("doctor with reusable session: expected nil (no violation), got %v\noutput:\n%s", err, out.String())
	}

	if !strings.Contains(out.String(), "invariant holds") {
		t.Fatalf("expected 'invariant holds' for reusable session, got:\n%s", out.String())
	}
}

// TestSessionDoctorAcceptance_NegativeControl_OutsideSessionsUntouched verifies
// that a live process whose --user-data-dir is OUTSIDE sessions/ is never
// killed by the reaper.
//
// The reaper's path-bounded scan (FindBrowsersUsingDataDir) is bounded to
// GetSessionsDir(); a holder in an unrelated temp dir is outside that scope
// and must survive the ReapOnce pass.
func TestSessionDoctorAcceptance_NegativeControl_OutsideSessionsUntouched(t *testing.T) {
	withTempSessionsAcceptance(t)

	shim := buildChromeShim(t)
	if shim == "" {
		t.Skip("chrome shim could not be compiled; skipping negative-control test")
	}

	// dataDir is OUTSIDE the sessions root — a completely separate temp dir.
	outsideDir := t.TempDir()
	if err := os.MkdirAll(outsideDir, 0o700); err != nil {
		t.Fatalf("mkdir outsideDir: %v", err)
	}

	// Start holder with --user-data-dir pointing OUTSIDE sessions/.
	cmd := exec.Command(shim, "--user-data-dir="+outsideDir, "--outside")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot start outside holder: %v", err)
	}

	outsidePID := cmd.Process.Pid
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})

	// Give the OS a beat to publish the cmdline (we don't strictly need it
	// observable since the test just proves it isn't killed, but waiting
	// confirms the process is alive).
	time.Sleep(100 * time.Millisecond)

	// Run reap with an EMPTY sessions dir — nothing to reap.
	stats := session.ReapOnce()
	if stats.Killed > 0 {
		t.Fatalf("ReapOnce killed %d processes with empty sessions dir; outside holder should be safe", stats.Killed)
	}

	// The outside process must still be alive.
	if !session.ProcessAlive(outsidePID) {
		t.Fatalf("outside holder PID %d was killed by reaper — path-bounded safety broken", outsidePID)
	}
}
