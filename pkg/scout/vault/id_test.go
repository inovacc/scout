package vault

import (
	"regexp"
	"testing"
)

func TestNewIDFormatAndUniqueness(t *testing.T) {
	re := regexp.MustCompile(`^[A-Za-z0-9_-]{22}$`)
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := newID()
		if !re.MatchString(id) {
			t.Fatalf("id %q does not match %s", id, re)
		}
		if seen[id] {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = true
	}
}
