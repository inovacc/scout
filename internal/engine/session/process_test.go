package session

import (
	"os"
	"runtime"
	"testing"
)

func TestIsScoutExec(t *testing.T) {
	tests := []struct {
		name string
		exec string
		want bool
	}{
		// True positives.
		{"unix bare", "/usr/local/bin/scout", true},
		{"windows bare", `C:\Program Files\scout\scout.exe`, true},
		{"windows lowercase drive", `c:\tools\scout.exe`, true},
		{"uppercase basename", "/usr/local/bin/SCOUT", true},
		{"mixed case .exe", `C:\bin\Scout.EXE`, true},

		// False positives the old strings.Contains check would have hit.
		{"substring boyscout", "/usr/local/bin/boyscout", false},
		{"substring scoutsdk", "/opt/scoutsdk-helper", false},
		{"substring whereisscout", "/tmp/whereisscout", false},
		{"substring scout-helper", "/usr/bin/scout-helper", false},
		{"substring myscout", "/home/user/bin/myscout", false},
		{"substring path contains scout", "/opt/scout-project/runner", false},
		{"directory named scout", "/etc/scout/tool", false},

		// Edge cases.
		{"empty", "", false},
		{"just slash", "/", false},
		{"unrelated", "/bin/bash", false},
		{"scout.exe substring", "/tmp/notscout.exe", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isScoutExec(tc.exec); got != tc.want {
				t.Errorf("isScoutExec(%q) = %v, want %v", tc.exec, got, tc.want)
			}
		})
	}
}

// TestProcessStartTokenSelfStable verifies ProcessStartToken returns the same
// token across repeated calls for the current process (token is stable while
// the process lives).
func TestProcessStartTokenSelfStable(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		t.Skipf("ProcessStartToken unsupported on %s", runtime.GOOS)
	}

	pid := os.Getpid()

	tok1, ok := ProcessStartToken(pid)
	if !ok {
		t.Fatalf("ProcessStartToken(%d) = !ok for current process", pid)
	}

	if tok1 == "" {
		t.Fatalf("ProcessStartToken(%d) = empty token", pid)
	}

	tok2, ok := ProcessStartToken(pid)
	if !ok || tok2 != tok1 {
		t.Errorf("ProcessStartToken instability: %q vs %q", tok1, tok2)
	}
}

// TestProcessStartTokenInvalidPID verifies non-existent PIDs return false.
func TestProcessStartTokenInvalidPID(t *testing.T) {
	if _, ok := ProcessStartToken(0); ok {
		t.Error("ProcessStartToken(0) returned ok=true")
	}

	if _, ok := ProcessStartToken(-1); ok {
		t.Error("ProcessStartToken(-1) returned ok=true")
	}

	// PID 999999999 is almost certainly not a live process. (Linux PID_MAX
	// is typically 2^22; Windows handles 32-bit PIDs but reuses them fast.)
	if _, ok := ProcessStartToken(999999999); ok {
		t.Error("ProcessStartToken(huge) returned ok=true")
	}
}

// TestVerifyProcessDegradesOnEmptyToken verifies verifyProcess preserves
// legacy behavior (ProcessAlive-only) when the recorded token is empty.
func TestVerifyProcessDegradesOnEmptyToken(t *testing.T) {
	pid := os.Getpid()

	if !verifyProcess(pid, "") {
		t.Errorf("verifyProcess(self, empty) = false, want true (degrade to alive-only)")
	}

	if verifyProcess(0, "") {
		t.Errorf("verifyProcess(0, empty) = true, want false")
	}
}

// TestVerifyProcessRejectsMismatchedToken verifies a mismatched token rejects
// even when the PID is alive — the core PID-reuse guard.
func TestVerifyProcessRejectsMismatchedToken(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		t.Skipf("ProcessStartToken unsupported on %s", runtime.GOOS)
	}

	pid := os.Getpid()

	if verifyProcess(pid, "linux:99999999999") {
		t.Errorf("verifyProcess(self, bogus token) = true, want false")
	}

	if verifyProcess(pid, "win:0") {
		t.Errorf("verifyProcess(self, win:0) = true, want false")
	}
}

// TestVerifyProcessAcceptsMatchingToken verifies verifyProcess passes when
// PID is alive and the token matches the live process's actual start.
func TestVerifyProcessAcceptsMatchingToken(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		t.Skipf("ProcessStartToken unsupported on %s", runtime.GOOS)
	}

	pid := os.Getpid()

	tok, ok := ProcessStartToken(pid)
	if !ok {
		t.Fatalf("ProcessStartToken: !ok")
	}

	if !verifyProcess(pid, tok) {
		t.Errorf("verifyProcess(self, self-token) = false, want true")
	}
}
