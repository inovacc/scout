package plugin

import "testing"

// TestManifestValidateRejectsUnsafeName guards against path traversal via the
// manifest name, which is used as the on-disk plugin directory.
func TestManifestValidateRejectsUnsafeName(t *testing.T) {
	valid := &Manifest{
		Name:         "scout-diag",
		Version:      "1.0.0",
		Command:      "bin/run",
		Capabilities: []string{"mcp_tool"},
	}
	if err := valid.validate(); err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}

	for _, bad := range []string{"..", "../evil", "../../x", "a/b", `a\b`, "/abs"} {
		m := &Manifest{
			Name:         bad,
			Version:      "1.0.0",
			Command:      "bin/run",
			Capabilities: []string{"mcp_tool"},
		}
		if err := m.validate(); err == nil {
			t.Errorf("name %q accepted, want rejection", bad)
		}
	}
}
