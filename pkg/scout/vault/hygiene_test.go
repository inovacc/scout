package vault

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestNoSecretStringConversion guards the "secrets never become string" rule:
// no non-test .go file in this package may convert a LockedBuffer's bytes to a
// string. The one sanctioned exception is the transient CDP header map in
// inject.go (CDP requires string header values), which is annotated below.
func TestNoSecretStringConversion(t *testing.T) {
	bad := regexp.MustCompile(`string\(\s*\w+\.Bytes\(\)\s*\)`)
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(".", name))
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			if bad.MatchString(line) && !strings.Contains(line, "vault:allow-string") {
				t.Errorf("%s:%d converts secret bytes to string: %s", name, i+1, strings.TrimSpace(line))
			}
		}
	}
}
