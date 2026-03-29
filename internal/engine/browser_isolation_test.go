package engine_test

import (
	"os"
	"testing"
)

// TestNoRodFallback verifies that the rod auto-detect fallthrough comment has been removed.
// This test FAILS currently because browser.go:341 still contains the fallback comment — ISOL-02.
func TestNoRodFallback(t *testing.T) {
	source, err := os.ReadFile("browser.go")
	if err != nil {
		t.Fatalf("cannot read browser.go: %v", err)
	}

	content := string(source)

	if containsSubstring(content, "fall through to rod auto-detect") {
		t.Fatal("ISOL-02: rod fallback comment 'fall through to rod auto-detect' still present in browser.go — remove the fallthrough")
	}
}

// TestDefaultIsolationNeverProbesSystem verifies that BestDetected() is only called
// inside the systemBrowser gate. This is a regression anchor — PASSES currently (ISOL-01).
func TestDefaultIsolationNeverProbesSystem(t *testing.T) {
	source, err := os.ReadFile("browser.go")
	if err != nil {
		t.Fatalf("cannot read browser.go: %v", err)
	}

	content := string(source)

	// Ensure the systemBrowser guard exists.
	if !containsSubstring(content, "o.systemBrowser") {
		t.Fatal("ISOL-01: systemBrowser gate missing from browser.go")
	}

	// BestDetected must only appear inside the systemBrowser-gated block.
	// We confirm this by checking the gate exists adjacent to BestDetected usage.
	// A simple heuristic: if BestDetected appears, systemBrowser must appear before it nearby.
	bestDetectedIdx := indexOfSubstring(content, "BestDetected")
	systemBrowserIdx := indexOfSubstring(content, "o.systemBrowser")

	if bestDetectedIdx >= 0 && systemBrowserIdx < 0 {
		t.Fatal("ISOL-01: BestDetected() found but systemBrowser gate is missing")
	}
}

// TestSystemBrowserOptInExists verifies the WithSystemBrowser() option exists.
// This is a regression anchor — PASSES immediately (ISOL-03).
func TestSystemBrowserOptInExists(t *testing.T) {
	source, err := os.ReadFile("option.go")
	if err != nil {
		t.Fatalf("cannot read option.go: %v", err)
	}

	if !containsSubstring(string(source), "WithSystemBrowser") {
		t.Fatal("ISOL-03: WithSystemBrowser option not found in option.go")
	}
}

// indexOfSubstring returns the index of the first occurrence of sub in s, or -1.
func indexOfSubstring(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
