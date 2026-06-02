//go:build windows

package session

import (
	"testing"
)

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
