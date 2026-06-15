package main

import "testing"

// TestSafePluginDirRejectsTraversal ensures a crafted plugin name cannot escape
// the plugins directory (path traversal) at the directory-resolution layer.
func TestSafePluginDirRejectsTraversal(t *testing.T) {
	t.Setenv("SCOUT_HOME", t.TempDir())

	if _, err := safePluginDir("scout-diag"); err != nil {
		t.Fatalf("valid name rejected: %v", err)
	}

	for _, bad := range []string{"..", "../evil", "../../x", "a/b", `a\b`, "/abs", ""} {
		if _, err := safePluginDir(bad); err == nil {
			t.Errorf("name %q accepted, want rejection", bad)
		}
	}
}
