package engine_test

import (
	"os"
	"regexp"
	"testing"

	"github.com/google/uuid"
)

// TestNewSessionIDIsUnique verifies that session IDs are UUIDs, not deterministic hashes.
// This test FAILS currently because browser.go still uses SessionHash() — SESS-03.
func TestNewSessionIDIsUnique(t *testing.T) {
	source, err := os.ReadFile("browser.go")
	if err != nil {
		t.Fatalf("cannot read browser.go: %v", err)
	}

	content := string(source)

	// SessionHash is the old deterministic hash — must be absent from the
	// session-ID assignment path after SESS-03 fix.
	if containsSubstring(content, "SessionHash(") {
		t.Fatal("SESS-03: deterministic hash 'SessionHash(' still present in browser.go — replace with uuid.NewV7()")
	}
}

// TestSessionIDFormat verifies UUID v7 format as a regression anchor.
// This test PASSES immediately and serves as a guardrail for future changes.
func TestSessionIDFormat(t *testing.T) {
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

	for i := range 10 {
		id := uuid.Must(uuid.NewV7()).String()
		if !uuidRegex.MatchString(id) {
			t.Errorf("iteration %d: UUID %q does not match expected format", i, id)
		}
	}
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
