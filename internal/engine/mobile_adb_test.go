package engine

import (
	"testing"
)

func TestParseADBDevicesOutput(t *testing.T) {
	// Test ListADBDevices with a non-existent adb binary.
	// Should return an error about adb not found.
	_, err := ListADBDevices(t.Context(), "/nonexistent/adb")
	if err == nil {
		t.Error("expected error for missing adb binary")
	}
}

func TestMobileConfigDefaults(t *testing.T) {
	cfg := MobileConfig{}

	if cfg.ADBPath != "" {
		t.Error("default ADBPath should be empty")
	}
	if cfg.CDPPort != 0 {
		t.Error("default CDPPort should be 0")
	}
	if cfg.PackageName != "" {
		t.Error("default PackageName should be empty")
	}
	if cfg.DeviceID != "" {
		t.Error("default DeviceID should be empty")
	}
}

func TestWithMobileOption(t *testing.T) {
	cfg := MobileConfig{
		DeviceID: "emulator-5554",
		CDPPort:  9333,
	}

	opt := WithMobile(cfg)
	o := defaults()
	opt(o)

	if o.mobile == nil {
		t.Fatal("mobile config should be set")
	}
	if o.mobile.DeviceID != "emulator-5554" {
		t.Errorf("DeviceID = %q, want emulator-5554", o.mobile.DeviceID)
	}
	if o.mobile.CDPPort != 9333 {
		t.Errorf("CDPPort = %d, want 9333", o.mobile.CDPPort)
	}
}

func TestWithTouchEmulationOption(t *testing.T) {
	opt := WithTouchEmulation()
	o := defaults()
	opt(o)

	if !o.touchEmulation {
		t.Error("touchEmulation should be true")
	}
}

func TestWithMobileAllFields(t *testing.T) {
	cfg := MobileConfig{
		DeviceID:    "abc123",
		ADBPath:     "/usr/bin/adb",
		PackageName: "com.chrome.beta",
		CDPPort:     9876,
	}

	opt := WithMobile(cfg)
	o := defaults()
	opt(o)

	if o.mobile.ADBPath != "/usr/bin/adb" {
		t.Errorf("ADBPath = %q, want /usr/bin/adb", o.mobile.ADBPath)
	}
	if o.mobile.PackageName != "com.chrome.beta" {
		t.Errorf("PackageName = %q, want com.chrome.beta", o.mobile.PackageName)
	}
}

func TestListADBDevicesEmptyPath(t *testing.T) {
	// Empty path defaults to "adb" which likely isn't on PATH in CI.
	// We just verify it doesn't panic and returns an error.
	_, err := ListADBDevices(t.Context(), "")
	if err == nil {
		t.Skip("adb found on PATH; skipping missing-binary test")
	}
}
