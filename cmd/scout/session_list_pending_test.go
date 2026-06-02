package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestSessionListPendingEmpty asserts the --pending path runs and reports the
// empty case cleanly (no panic, no tracked-ID fallthrough). The pending queue
// is process-global and starts empty in a fresh test binary.
func TestSessionListPendingEmpty(t *testing.T) {
	if sessionListCmd.Flags().Lookup("pending") == nil {
		t.Fatal("sessionListCmd is missing the --pending flag")
	}

	var out bytes.Buffer
	sessionListCmd.SetOut(&out)
	sessionListCmd.SetErr(&out)

	if err := sessionListCmd.Flags().Set("pending", "true"); err != nil {
		t.Fatalf("set --pending: %v", err)
	}
	t.Cleanup(func() { _ = sessionListCmd.Flags().Set("pending", "false") })

	if err := sessionListCmd.RunE(sessionListCmd, nil); err != nil {
		t.Fatalf("list --pending: %v", err)
	}

	if !strings.Contains(out.String(), "No pending cleanup") {
		t.Fatalf("expected empty-pending message, got:\n%s", out.String())
	}
}
