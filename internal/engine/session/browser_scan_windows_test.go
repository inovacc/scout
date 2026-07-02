//go:build windows

package session

import (
	"os/exec"
	"testing"
	"time"
)

// TestFindBrowsersWindows_EmptyDataDir verifies the fast-path guard returns
// immediately without spawning PowerShell.
func TestFindBrowsersWindows_EmptyDataDir(t *testing.T) {
	if got := findBrowsersWindows(""); got != nil {
		t.Fatalf("findBrowsersWindows(\"\") = %v, want nil", got)
	}
}

// TestFindBrowsersWindows_Bounded proves the WMI scan is bounded: even if
// PowerShell is slow, the call must return within browserScanTimeout plus a
// generous margin. Regression for the "hung WMI blocks every command" defect.
func TestFindBrowsersWindows_Bounded(t *testing.T) {
	if _, err := exec.LookPath("powershell"); err != nil {
		t.Skip("powershell not on PATH")
	}

	done := make(chan []int, 1)
	go func() { done <- findBrowsersWindows(t.TempDir()) }()

	select {
	case <-done:
		// returned within the deadline — pass
	case <-time.After(browserScanTimeout + 5*time.Second):
		t.Fatal("findBrowsersWindows exceeded its deadline — scan is not bounded")
	}
}

func TestEscapePowerShellBackslash(t *testing.T) {
	in := `C:\Users\x\AppData\Local\Scout\sessions\a[b]\data`
	got := escapePowerShell(in)

	// Backslashes must be doubled so the value is a literal path inside a
	// PowerShell double-quoted string, and -like wildcard metacharacters
	// ([ and ]) must be back-tick escaped so they are matched literally.
	for _, frag := range []string{"``[", "``]"} {
		if !contains(got, frag) {
			t.Fatalf("escapePowerShell(%q) = %q, missing escaped fragment %q", in, got, frag)
		}
	}

	if !contains(got, `\\`) {
		t.Fatalf("escapePowerShell(%q) = %q, backslashes not doubled", in, got)
	}
}

// contains is a tiny local helper so this test file needs no extra imports
// beyond what the package test build already pulls in.
func contains(haystack, needle string) bool {
	return len(needle) == 0 || indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}

	return -1
}
