package main

import "testing"

// TestIsNewer pins the self-update version gate: only a strictly-newer semver
// tag is accepted, so `scout update` cannot be steered into a downgrade by a
// rolled-back or spoofed older release tag.
func TestIsNewer(t *testing.T) {
	cases := []struct {
		current, remote string
		want            bool
	}{
		{"v1.0.0", "v1.1.0", true},  // minor upgrade
		{"v1.0.0", "v1.0.1", true},  // patch upgrade
		{"1.1.0", "1.0.0", false},   // downgrade refused (no 'v' prefix)
		{"v1.1.0", "v1.1.0", false}, // equal refused
		{"v2.0.0", "v1.9.9", false}, // major downgrade refused
		{"dev", "v1.0.0", true},     // dev always updatable
		{"", "v1.0.0", true},        // empty always updatable
	}
	for _, c := range cases {
		if got := isNewer(c.current, c.remote); got != c.want {
			t.Errorf("isNewer(%q, %q) = %v, want %v", c.current, c.remote, got, c.want)
		}
	}
}

// TestIsNewerAllowDowngrade proves the explicit operator opt-in restores
// rollback to any different tag (but never claims an equal tag is newer).
func TestIsNewerAllowDowngrade(t *testing.T) {
	t.Setenv("SCOUT_ALLOW_DOWNGRADE", "1")

	if !isNewer("v1.1.0", "v1.0.0") {
		t.Error("with SCOUT_ALLOW_DOWNGRADE, a different (older) tag should be allowed")
	}
	if isNewer("v1.1.0", "v1.1.0") {
		t.Error("an equal tag is never 'newer', even with allow-downgrade")
	}
}

// TestIsNewerNonSemverFallback covers odd, non-semver version strings: update
// only on a difference (conservative — never silently equal).
func TestIsNewerNonSemverFallback(t *testing.T) {
	if !isNewer("weird-tag", "other-tag") {
		t.Error("non-semver different tags should be updatable (conservative fallback)")
	}
	if isNewer("weird-tag", "weird-tag") {
		t.Error("identical non-semver tags should not be updatable")
	}
}
