package engine

import (
	"strings"
	"testing"
)

func TestGenerateFingerprint_Default(t *testing.T) {
	fp := GenerateFingerprint()
	if fp.UserAgent == "" {
		t.Fatal("expected non-empty UserAgent")
	}

	if fp.Platform == "" {
		t.Fatal("expected non-empty Platform")
	}

	if fp.Vendor == "" {
		t.Fatal("expected non-empty Vendor")
	}

	if len(fp.Languages) == 0 {
		t.Fatal("expected non-empty Languages")
	}

	if fp.Timezone == "" {
		t.Fatal("expected non-empty Timezone")
	}

	if fp.ScreenWidth == 0 || fp.ScreenHeight == 0 {
		t.Fatalf("expected non-zero screen: %dx%d", fp.ScreenWidth, fp.ScreenHeight)
	}

	if fp.ColorDepth == 0 {
		t.Fatal("expected non-zero ColorDepth")
	}

	if fp.PixelRatio == 0 {
		t.Fatal("expected non-zero PixelRatio")
	}

	if fp.WebGLVendor == "" || fp.WebGLRenderer == "" {
		t.Fatal("expected non-empty WebGL fields")
	}

	if fp.HardwareConcurrency == 0 {
		t.Fatal("expected non-zero HardwareConcurrency")
	}

	if fp.DeviceMemory == 0 {
		t.Fatal("expected non-zero DeviceMemory")
	}
}

func TestGenerateFingerprint_OS(t *testing.T) {
	tests := []struct {
		os       string
		uaMatch  string
		platform string
	}{
		{"windows", "Windows NT", "Win32"},
		{"mac", "Macintosh", "MacIntel"},
		{"linux", "Linux x86_64", "Linux x86_64"},
	}

	for _, tt := range tests {
		t.Run(tt.os, func(t *testing.T) {
			fp := GenerateFingerprint(WithFingerprintOS(tt.os))
			if !strings.Contains(fp.UserAgent, tt.uaMatch) {
				t.Errorf("UA %q does not contain %q", fp.UserAgent, tt.uaMatch)
			}

			if fp.Platform != tt.platform {
				t.Errorf("platform = %q, want %q", fp.Platform, tt.platform)
			}
		})
	}
}

func TestGenerateFingerprint_Mobile(t *testing.T) {
	fp := GenerateFingerprint(WithFingerprintMobile(true))
	if fp.MaxTouchPoints == 0 {
		t.Error("mobile fingerprint should have touch points > 0")
	}

	if !strings.Contains(fp.UserAgent, "Mobile") && !strings.Contains(fp.UserAgent, "CriOS") {
		t.Errorf("expected mobile UA, got %q", fp.UserAgent)
	}

	if fp.ScreenWidth > 500 {
		t.Errorf("expected mobile screen width <= 500, got %d", fp.ScreenWidth)
	}
}

func TestGenerateFingerprint_Locale(t *testing.T) {
	fp := GenerateFingerprint(WithFingerprintLocale("de-DE"))
	if fp.Timezone != "Europe/Berlin" {
		t.Errorf("expected Europe/Berlin, got %q", fp.Timezone)
	}

	found := false

	for _, l := range fp.Languages {
		if l == "de-DE" {
			found = true
		}
	}

	if !found {
		t.Errorf("expected de-DE in languages, got %v", fp.Languages)
	}
}

func TestFingerprintToJS(t *testing.T) {
	fp := GenerateFingerprint(WithFingerprintOS("windows"))

	js := fp.ToJS()
	if js == "" {
		t.Fatal("expected non-empty JS")
	}

	checks := []string{
		"navigator",
		"screen",
		"devicePixelRatio",
		"WEBGL_debug_renderer_info",
		"UNMASKED_VENDOR_WEBGL",
		fp.UserAgent,
		fp.Platform,
		fp.Timezone,
	}
	for _, check := range checks {
		if !strings.Contains(js, check) {
			t.Errorf("JS missing %q", check)
		}
	}
}

func TestFingerprintToProfile(t *testing.T) {
	fp := GenerateFingerprint(WithFingerprintOS("mac"))

	prof := FingerprintToProfile(fp)
	if prof.Identity.UserAgent != fp.UserAgent {
		t.Errorf("profile UA = %q, want %q", prof.Identity.UserAgent, fp.UserAgent)
	}

	if prof.Identity.Timezone != fp.Timezone {
		t.Errorf("profile timezone = %q, want %q", prof.Identity.Timezone, fp.Timezone)
	}

	if prof.Browser.WindowW != fp.ScreenWidth {
		t.Errorf("profile window width = %d, want %d", prof.Browser.WindowW, fp.ScreenWidth)
	}

	if prof.Version != 1 {
		t.Errorf("profile version = %d, want 1", prof.Version)
	}

	if !strings.HasPrefix(prof.Name, "fingerprint-") {
		t.Errorf("profile name = %q, want fingerprint- prefix", prof.Name)
	}
}

func TestFingerprintJSON(t *testing.T) {
	fp := GenerateFingerprint()

	js, err := fp.JSON()
	if err != nil {
		t.Fatalf("JSON() error: %v", err)
	}

	if !strings.Contains(js, "user_agent") {
		t.Error("JSON missing user_agent key")
	}
}

func TestGenerateFingerprint_Deterministic(t *testing.T) {
	fp1 := GenerateFingerprint()
	fp2 := GenerateFingerprint()
	// Not guaranteed to differ in every field, but user agents should differ
	// across many runs with high probability. Check that at least something differs.
	if fp1.UserAgent == fp2.UserAgent &&
		fp1.ScreenWidth == fp2.ScreenWidth &&
		fp1.Timezone == fp2.Timezone &&
		fp1.WebGLRenderer == fp2.WebGLRenderer {
		t.Error("two calls produced identical fingerprints; expected randomness")
	}
}

func TestGenerateFingerprint_WindowsWebGL(t *testing.T) {
	// Verify Windows fingerprints get Windows-appropriate WebGL profiles.
	for range 10 {
		fp := GenerateFingerprint(WithFingerprintOS("windows"))
		if !strings.Contains(fp.WebGLRenderer, "Direct3D11") && !strings.Contains(fp.WebGLRenderer, "ANGLE") {
			t.Errorf("Windows WebGL renderer unexpected: %q", fp.WebGLRenderer)
		}
	}
}

func TestGenerateFingerprint_MacWebGL(t *testing.T) {
	for range 10 {
		fp := GenerateFingerprint(WithFingerprintOS("mac"))
		if !strings.Contains(fp.WebGLRenderer, "Apple") {
			t.Errorf("Mac WebGL renderer should contain Apple: %q", fp.WebGLRenderer)
		}
	}
}
