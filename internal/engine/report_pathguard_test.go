package engine

import (
	"strings"
	"testing"
)

// TestReportPathRejectsTraversal pins the path-traversal guard on report IDs
// used by ReadReport / ReadReportRaw / DeleteReport.
func TestReportPathRejectsTraversal(t *testing.T) {
	bad := []string{
		"",
		"..",
		"../secret",
		"../../etc/passwd",
		"sub/evil",
		`..\..\windows`,
		"/abs/path",
	}
	for _, id := range bad {
		if _, err := reportPath(id); err == nil {
			t.Errorf("reportPath(%q) should be rejected", id)
		}
	}

	good := "0190a1b2-c3d4-7e5f-89ab-cdef01234567"
	p, err := reportPath(good)
	if err != nil {
		t.Fatalf("reportPath(%q) unexpected error: %v", good, err)
	}
	if !strings.HasSuffix(p, good+".txt") {
		t.Errorf("reportPath(%q) = %q, want suffix %q.txt", good, p, good)
	}
}
