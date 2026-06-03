package flow

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestSaveCapturePermsAreOwnerOnly guards the at-rest secrecy of a capture: it
// holds raw request/response headers and bodies (live tokens, cookies, CSRF), so
// it must be written 0o600. POSIX perms are not enforced on Windows, so skip there.
func TestSaveCapturePermsAreOwnerOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file permissions are not enforced on Windows")
	}
	p := filepath.Join(t.TempDir(), "capture.json")
	if err := SaveCapture(p, &Capture{Version: "1"}); err != nil {
		t.Fatalf("SaveCapture: %v", err)
	}
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Fatalf("capture.json perms = %o, want 600", perm)
	}
}
