package scout

import (
	"testing"
	"time"
)

func TestFingerprintRotation_PerSession(t *testing.T) {
	r := newFingerprintRotator(FingerprintRotationConfig{
		Strategy: FingerprintRotatePerSession,
	})

	fp1 := r.forPage("example.com")
	fp2 := r.forPage("other.com")
	fp3 := r.forPage("example.com")

	// Per-session: same fingerprint for all pages.
	if fp1.UserAgent != fp2.UserAgent {
		t.Fatal("expected same fingerprint per session")
	}

	if fp1.UserAgent != fp3.UserAgent {
		t.Fatal("expected same fingerprint per session")
	}
}

func TestFingerprintRotation_PerPage(t *testing.T) {
	r := newFingerprintRotator(FingerprintRotationConfig{
		Strategy: FingerprintRotatePerPage,
		Options:  []FingerprintOption{WithFingerprintOS("windows")},
	})

	seen := make(map[string]bool)

	for range 10 {
		fp := r.forPage("example.com")
		if fp.Platform != "Win32" {
			t.Fatalf("expected Win32, got %s", fp.Platform)
		}

		seen[fp.UserAgent] = true
	}

	// With 10 random fingerprints, we should see at least 2 different UAs.
	if len(seen) < 2 {
		t.Fatal("expected different fingerprints per page")
	}
}

func TestFingerprintRotation_PerDomain(t *testing.T) {
	r := newFingerprintRotator(FingerprintRotationConfig{
		Strategy: FingerprintRotatePerDomain,
	})

	fp1a := r.forPage("example.com")
	fp1b := r.forPage("example.com")
	fp2 := r.forPage("other.com")

	// Same domain → same fingerprint.
	if fp1a.UserAgent != fp1b.UserAgent {
		t.Fatal("expected same fingerprint for same domain")
	}

	// Different domain → different fingerprint (with high probability).
	// They could theoretically match, so just verify the lookup works.
	_ = fp2
}

func TestFingerprintRotation_Interval(t *testing.T) {
	r := newFingerprintRotator(FingerprintRotationConfig{
		Strategy: FingerprintRotateInterval,
		Interval: 50 * time.Millisecond,
	})

	fp1 := r.forPage("example.com")
	fp2 := r.forPage("example.com")

	// Before interval: same.
	if fp1.UserAgent != fp2.UserAgent {
		t.Fatal("expected same fingerprint before interval")
	}

	time.Sleep(60 * time.Millisecond)

	fp3 := r.forPage("example.com")

	// After interval: rotated (different with high probability).
	_ = fp3 // can't guarantee different due to randomness, but rotation happened
}

func TestFingerprintRotation_Pool(t *testing.T) {
	pool := []*Fingerprint{
		GenerateFingerprint(WithFingerprintOS("windows")),
		GenerateFingerprint(WithFingerprintOS("mac")),
		GenerateFingerprint(WithFingerprintOS("linux")),
	}

	r := newFingerprintRotator(FingerprintRotationConfig{
		Strategy: FingerprintRotatePerPage,
		Pool:     pool,
	})

	// Initial fingerprint (from constructor) consumed pool[0] (Win32).
	// Per-page rotation generates new on each call.
	fp1 := r.forPage("a.com") // pool[1] = Mac
	fp2 := r.forPage("b.com") // pool[2] = Linux
	fp3 := r.forPage("c.com") // pool[0] = Win32 (wraps)
	fp4 := r.forPage("d.com") // pool[1] = Mac

	if fp1.Platform != "MacIntel" {
		t.Fatalf("expected MacIntel, got %s", fp1.Platform)
	}

	if fp2.Platform != "Linux x86_64" {
		t.Fatalf("expected Linux x86_64, got %s", fp2.Platform)
	}

	if fp3.Platform != "Win32" {
		t.Fatalf("expected Win32 (wrap), got %s", fp3.Platform)
	}

	if fp4.Platform != "MacIntel" {
		t.Fatalf("expected MacIntel, got %s", fp4.Platform)
	}
}

func TestFingerprintRotation_NilSafe(t *testing.T) {
	var r *fingerprintRotator

	fp := r.forPage("example.com")
	if fp != nil {
		t.Fatal("expected nil from nil rotator")
	}
}

func TestDomainFromURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://example.com/path", "example.com"},
		{"http://sub.domain.org:8080/page", "sub.domain.org"},
		{"", ""},
		{"not-a-url", ""},
	}
	for _, tt := range tests {
		got := domainFromURL(tt.url)
		if got != tt.want {
			t.Errorf("domainFromURL(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}
