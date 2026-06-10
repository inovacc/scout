package devices

import (
	"testing"

	proto2 "github.com/inovacc/scout/internal/engine/lib/proto"
)

// sampleDevice is a real, fully-populated device fixture used across tests.
// Distinct horizontal/vertical sizes let us assert orientation selection.
var sampleDevice = Device{
	Title:          "Test Phone",
	Capabilities:   []string{"touch", "mobile"},
	UserAgent:      "Mozilla/5.0 (Test)",
	AcceptLanguage: "en-US",
	Screen: Screen{
		DevicePixelRatio: 3,
		Horizontal: ScreenSize{Width: 800, Height: 400},
		Vertical:   ScreenSize{Width: 400, Height: 800},
	},
}

func TestHas(t *testing.T) {
	tests := []struct {
		name string
		arr  []string
		str  string
		want bool
	}{
		{name: "present first", arr: []string{"touch", "mobile"}, str: "touch", want: true},
		{name: "present last", arr: []string{"touch", "mobile"}, str: "mobile", want: true},
		{name: "absent", arr: []string{"touch", "mobile"}, str: "stylus", want: false},
		{name: "empty slice", arr: []string{}, str: "touch", want: false},
		{name: "nil slice", arr: nil, str: "touch", want: false},
		{name: "empty needle absent", arr: []string{"touch"}, str: "", want: false},
		{name: "empty needle present", arr: []string{""}, str: "", want: true},
		{name: "case sensitive", arr: []string{"Touch"}, str: "touch", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := has(tt.arr, tt.str); got != tt.want {
				t.Errorf("has(%v, %q) = %v, want %v", tt.arr, tt.str, got, tt.want)
			}
		})
	}
}

func TestDeviceIsClear(t *testing.T) {
	if !Clear.IsClear() {
		t.Error("Clear.IsClear() = false, want true")
	}
	if sampleDevice.IsClear() {
		t.Error("sampleDevice.IsClear() = true, want false")
	}
	// Zero-value device is not clear.
	if (Device{}).IsClear() {
		t.Error("zero Device.IsClear() = true, want false")
	}
}

func TestDeviceLandscape(t *testing.T) {
	got := sampleDevice.Landscape()

	if !got.landscape {
		t.Error("Landscape() did not set landscape flag")
	}
	// Landscape must clone, not mutate the receiver.
	if sampleDevice.landscape {
		t.Error("Landscape() mutated the original device")
	}
	// All other fields must be carried over to the clone.
	if got.Title != sampleDevice.Title || got.UserAgent != sampleDevice.UserAgent {
		t.Errorf("Landscape() lost fields: title=%q ua=%q", got.Title, got.UserAgent)
	}
	if got.Screen != sampleDevice.Screen {
		t.Errorf("Landscape() altered Screen: got %+v", got.Screen)
	}
	// Idempotent: calling twice keeps it landscape.
	if !got.Landscape().landscape {
		t.Error("Landscape().Landscape() lost landscape flag")
	}
}

func TestDeviceMetricsEmulationClear(t *testing.T) {
	if got := Clear.MetricsEmulation(); got != nil {
		t.Errorf("Clear.MetricsEmulation() = %+v, want nil", got)
	}
}

func TestDeviceMetricsEmulationPortrait(t *testing.T) {
	got := sampleDevice.MetricsEmulation()
	if got == nil {
		t.Fatal("MetricsEmulation() = nil, want value")
	}

	// Portrait selects the Vertical screen size.
	if got.Width != 400 || got.Height != 800 {
		t.Errorf("portrait size = %dx%d, want 400x800", got.Width, got.Height)
	}
	if got.DeviceScaleFactor != 3 {
		t.Errorf("DeviceScaleFactor = %v, want 3", got.DeviceScaleFactor)
	}
	if !got.Mobile {
		t.Error("Mobile = false, want true (capability present)")
	}
	if got.ScreenOrientation == nil {
		t.Fatal("ScreenOrientation = nil, want value")
	}
	if got.ScreenOrientation.Angle != 0 {
		t.Errorf("portrait Angle = %d, want 0", got.ScreenOrientation.Angle)
	}
	if got.ScreenOrientation.Type != proto2.EmulationScreenOrientationTypePortraitPrimary {
		t.Errorf("portrait Type = %q, want %q", got.ScreenOrientation.Type, proto2.EmulationScreenOrientationTypePortraitPrimary)
	}
}

func TestDeviceMetricsEmulationLandscape(t *testing.T) {
	got := sampleDevice.Landscape().MetricsEmulation()
	if got == nil {
		t.Fatal("MetricsEmulation() = nil, want value")
	}

	// Landscape selects the Horizontal screen size.
	if got.Width != 800 || got.Height != 400 {
		t.Errorf("landscape size = %dx%d, want 800x400", got.Width, got.Height)
	}
	if got.ScreenOrientation == nil {
		t.Fatal("ScreenOrientation = nil, want value")
	}
	if got.ScreenOrientation.Angle != 90 {
		t.Errorf("landscape Angle = %d, want 90", got.ScreenOrientation.Angle)
	}
	if got.ScreenOrientation.Type != proto2.EmulationScreenOrientationTypeLandscapePrimary {
		t.Errorf("landscape Type = %q, want %q", got.ScreenOrientation.Type, proto2.EmulationScreenOrientationTypeLandscapePrimary)
	}
}

func TestDeviceMetricsEmulationMobileCapability(t *testing.T) {
	// A non-mobile device must report Mobile=false.
	desktop := Device{
		Title:        "Desktop",
		Capabilities: []string{},
		Screen: Screen{
			DevicePixelRatio: 1,
			Vertical:         ScreenSize{Width: 1920, Height: 1080},
			Horizontal:       ScreenSize{Width: 1920, Height: 1080},
		},
	}

	got := desktop.MetricsEmulation()
	if got == nil {
		t.Fatal("MetricsEmulation() = nil, want value")
	}
	if got.Mobile {
		t.Error("Mobile = true, want false (no mobile capability)")
	}
}

func TestDeviceTouchEmulationClear(t *testing.T) {
	got := Clear.TouchEmulation()
	if got == nil {
		t.Fatal("Clear.TouchEmulation() = nil, want disabled struct")
	}
	if got.Enabled {
		t.Error("Clear touch Enabled = true, want false")
	}
	// Clear path must not set MaxTouchPoints.
	if got.MaxTouchPoints != nil {
		t.Errorf("Clear MaxTouchPoints = %v, want nil", got.MaxTouchPoints)
	}
}

func TestDeviceTouchEmulationEnabled(t *testing.T) {
	got := sampleDevice.TouchEmulation()
	if got == nil {
		t.Fatal("TouchEmulation() = nil, want value")
	}
	if !got.Enabled {
		t.Error("Enabled = false, want true (touch capability present)")
	}
	if got.MaxTouchPoints == nil {
		t.Fatal("MaxTouchPoints = nil, want pointer to 5")
	}
	if *got.MaxTouchPoints != 5 {
		t.Errorf("MaxTouchPoints = %d, want 5", *got.MaxTouchPoints)
	}
}

func TestDeviceTouchEmulationDisabledWhenNoCapability(t *testing.T) {
	noTouch := Device{Capabilities: []string{"mobile"}}
	got := noTouch.TouchEmulation()
	if got == nil {
		t.Fatal("TouchEmulation() = nil, want value")
	}
	if got.Enabled {
		t.Error("Enabled = true, want false (no touch capability)")
	}
	// Even when disabled by capability (not clear), MaxTouchPoints is set.
	if got.MaxTouchPoints == nil || *got.MaxTouchPoints != 5 {
		t.Errorf("MaxTouchPoints = %v, want pointer to 5", got.MaxTouchPoints)
	}
}

func TestDeviceUserAgentEmulationClear(t *testing.T) {
	if got := Clear.UserAgentEmulation(); got != nil {
		t.Errorf("Clear.UserAgentEmulation() = %+v, want nil", got)
	}
}

func TestDeviceUserAgentEmulation(t *testing.T) {
	got := sampleDevice.UserAgentEmulation()
	if got == nil {
		t.Fatal("UserAgentEmulation() = nil, want value")
	}
	if got.UserAgent != "Mozilla/5.0 (Test)" {
		t.Errorf("UserAgent = %q, want %q", got.UserAgent, "Mozilla/5.0 (Test)")
	}
	if got.AcceptLanguage != "en-US" {
		t.Errorf("AcceptLanguage = %q, want %q", got.AcceptLanguage, "en-US")
	}
}

// TestRealDeviceFromList exercises the emulation methods against a real
// generated device fixture (IPhone4) to ensure list.go data drives the
// branching logic end to end.
func TestRealDeviceFromList(t *testing.T) {
	metrics := IPhone4.MetricsEmulation()
	if metrics == nil {
		t.Fatal("IPhone4.MetricsEmulation() = nil")
	}
	// IPhone4 declares both "touch" and "mobile".
	if !metrics.Mobile {
		t.Error("IPhone4 Mobile = false, want true")
	}
	// Default (portrait) picks the Vertical size: 320x480.
	if metrics.Width != IPhone4.Screen.Vertical.Width || metrics.Height != IPhone4.Screen.Vertical.Height {
		t.Errorf("IPhone4 portrait = %dx%d, want %dx%d",
			metrics.Width, metrics.Height,
			IPhone4.Screen.Vertical.Width, IPhone4.Screen.Vertical.Height)
	}

	touch := IPhone4.TouchEmulation()
	if touch == nil || !touch.Enabled {
		t.Errorf("IPhone4 touch enabled = %v, want true", touch)
	}

	ua := IPhone4.UserAgentEmulation()
	if ua == nil || ua.UserAgent != IPhone4.UserAgent {
		t.Errorf("IPhone4 UA mismatch: got %v", ua)
	}
}
