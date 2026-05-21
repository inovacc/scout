package engine_test

import (
	"os"
	"regexp"
	"testing"

	"github.com/inovacc/scout/pkg/id"
)

// TestNewSessionIDIsUnique verifies that session IDs are NOT generated from a
// deterministic hash (regression anchor for SESS-03).
func TestNewSessionIDIsUnique(t *testing.T) {
	source, err := os.ReadFile("browser.go")
	if err != nil {
		t.Fatalf("cannot read browser.go: %v", err)
	}

	if containsSubstring(string(source), "SessionHash(") {
		t.Fatal("SESS-03: deterministic hash 'SessionHash(' still present in browser.go — must use session.NewSessionID()")
	}
}

// TestSessionIDFormat verifies the encoded-attribute ID shape: 12 attribute
// chars (version + 6 attrs + 5 reserved) followed by 24 random alpha.
func TestSessionIDFormat(t *testing.T) {
	// Position 0 = '1' (version), positions 1-6 = enum chars, 7-11 = '0',
	// followed by 24 [A-Za-z].
	idRegex := regexp.MustCompile(`^1[CBEXM][HV][PE][SN][BN][VN]0{5}[A-Z]{24}$`)

	for i := range 10 {
		s, err := id.New(id.Attrs{Browser: "chrome"})
		if err != nil {
			t.Fatalf("id.New: %v", err)
		}
		if !idRegex.MatchString(s) {
			t.Errorf("iteration %d: id %q does not match expected format", i, s)
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
