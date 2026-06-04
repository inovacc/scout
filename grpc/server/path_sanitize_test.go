package server

import "testing"

// TestSanitizeSessionRel pins the path-traversal guard on CreateSession's
// HarOut/HijackOut: only a bare filename (or empty) is accepted; absolute paths
// and any `..` / separator are rejected so a gRPC client cannot steer the HAR
// artifact write outside the per-session directory.
func TestSanitizeSessionRel(t *testing.T) {
	good := []string{
		"",                // empty → caller defaults
		"har.json",        // canonical
		"hijack.jsonl",    // canonical
		"my-har_2026.bin", // ordinary filename
	}
	for _, p := range good {
		got, err := sanitizeSessionRel(p)
		if err != nil {
			t.Errorf("sanitizeSessionRel(%q) = error %v, want ok", p, err)
		}
		if got != p {
			t.Errorf("sanitizeSessionRel(%q) = %q, want passthrough", p, got)
		}
	}

	bad := []string{
		"/etc/passwd",          // absolute (unix)
		`C:\Windows\evil.dll`,  // absolute (windows)
		"../../etc/passwd",     // parent traversal
		`..\..\evil`,           // parent traversal (windows seps)
		"..",                   // bare parent
		".",                    // bare current
		"sub/dir.json",         // nested (forward slash)
		`sub\dir.json`,         // nested (backslash)
		"a/../../../x",         // mixed traversal
		"\\\\server\\share\\x", // UNC path
	}
	for _, p := range bad {
		if _, err := sanitizeSessionRel(p); err == nil {
			t.Errorf("sanitizeSessionRel(%q) = nil error, want rejected", p)
		}
	}
}
