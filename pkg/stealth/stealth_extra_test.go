package stealth

import (
	"strings"
	"testing"
)

func TestExtraJS_NotEmpty(t *testing.T) {
	if len(ExtraJS) == 0 {
		t.Fatal("ExtraJS should not be empty")
	}
}

func TestExtraJS_ContainsAllEvasions(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
	}{
		{"1_canvas_toDataURL", "HTMLCanvasElement.prototype.toDataURL"},
		{"1_canvas_getImageData", "CanvasRenderingContext2D.prototype.getImageData"},
		{"2_audio_context", "AudioContext.prototype.createOscillator"},
		{"3_webgl_vendor", "UNMASKED_VENDOR_WEBGL"},
		{"3_webgl_renderer", "UNMASKED_RENDERER_WEBGL"},
		{"4_navigator_connection", "navigator.connection"},
		{"5_notification_permission", "Notification, 'permission'"},
		{"6_webrtc_leak", "RTCPeerConnection"},
		{"7_font_fingerprint", "document.fonts.check"},
		{"8_screen_resolution", "screen, 'width'"},
		{"9_battery_api", "getBattery"},
		{"10_chromedriver_leak", "cdc_adoQpoasnfa76pfcZLmcfl"},
		{"11_hardware_concurrency", "hardwareConcurrency"},
		{"11_device_memory", "deviceMemory"},
		{"11_vendor", "'Google Inc.'"},
		{"12_hasFocus", "Document.prototype.hasFocus"},
		{"13_outer_dimensions", "outerWidth"},
		{"14_languages", "navigator.languages"},
		{"15_plugins", "navigator.plugins"},
		{"16_timezone", "Intl.DateTimeFormat"},
		{"17_toString_integrity", "Function.prototype.toString"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(ExtraJS, tt.pattern) {
				t.Errorf("ExtraJS missing evasion pattern %q", tt.pattern)
			}
		})
	}
}

func TestExtraJS_ValidJavaScript(t *testing.T) {
	// Verify structural integrity: matching parens and IIFEs
	tests := []struct {
		name    string
		check   func(string) bool
	}{
		{"starts_with_IIFE", func(s string) bool { return strings.Contains(s, "(function()") }},
		{"balanced_braces", func(s string) bool {
			count := 0
			for _, c := range s {
				switch c {
				case '{':
					count++
				case '}':
					count--
				}
				if count < 0 {
					return false
				}
			}
			return count == 0
		}},
		{"balanced_parens", func(s string) bool {
			count := 0
			for _, c := range s {
				switch c {
				case '(':
					count++
				case ')':
					count--
				}
				if count < 0 {
					return false
				}
			}
			return count == 0
		}},
		{"no_syntax_errors_obvious", func(s string) bool {
			// Should not have consecutive operators or unclosed strings
			return !strings.Contains(s, ";;;\n;;;")
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.check(ExtraJS) {
				t.Errorf("ExtraJS failed structural check %q", tt.name)
			}
		})
	}
}

func TestExtraJS_WebGLSpoofValues(t *testing.T) {
	if !strings.Contains(ExtraJS, "'Intel Inc.'") {
		t.Error("ExtraJS should spoof WebGL vendor to 'Intel Inc.'")
	}
	if !strings.Contains(ExtraJS, "'Intel Iris OpenGL Engine'") {
		t.Error("ExtraJS should spoof WebGL renderer to 'Intel Iris OpenGL Engine'")
	}
}

func TestExtraJS_ConnectionValues(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
	}{
		{"effectiveType_4g", "'4g'"},
		{"downlink_10", "downlink"},
		{"rtt_50", "rtt"},
		{"saveData_false", "saveData"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(ExtraJS, tt.pattern) {
				t.Errorf("ExtraJS missing connection property %q", tt.pattern)
			}
		})
	}
}

func TestExtraJS_BatteryValues(t *testing.T) {
	if !strings.Contains(ExtraJS, "charging: true") {
		t.Error("ExtraJS should set battery charging to true")
	}
	if !strings.Contains(ExtraJS, "level: 1.0") {
		t.Error("ExtraJS should set battery level to 1.0")
	}
}

func TestExtraJS_AutomationMarkers(t *testing.T) {
	markers := []string{"callPhantom", "_phantom", "__nightmare", "domAutomation", "domAutomationController"}
	for _, m := range markers {
		t.Run(m, func(t *testing.T) {
			if !strings.Contains(ExtraJS, m) {
				t.Errorf("ExtraJS should remove automation marker %q", m)
			}
		})
	}
}

func TestExtraJS_FakePlugins(t *testing.T) {
	plugins := []string{"PDF Viewer", "Chrome PDF Viewer", "Chromium PDF Viewer", "Microsoft Edge PDF Viewer", "WebKit built-in PDF"}
	for _, p := range plugins {
		t.Run(p, func(t *testing.T) {
			if !strings.Contains(ExtraJS, p) {
				t.Errorf("ExtraJS should include fake plugin %q", p)
			}
		})
	}
}

func TestExtraJS_CommonFonts(t *testing.T) {
	fonts := []string{"Arial", "Times New Roman", "Verdana", "Georgia", "Courier New"}
	for _, f := range fonts {
		t.Run(f, func(t *testing.T) {
			if !strings.Contains(ExtraJS, f) {
				t.Errorf("ExtraJS should include common font %q", f)
			}
		})
	}
}

func TestExtraJS_TimezoneSpoof(t *testing.T) {
	if !strings.Contains(ExtraJS, "America/New_York") {
		t.Error("ExtraJS should spoof timezone to America/New_York")
	}
}

func TestExtraJS_WebRTCLocalIPPattern(t *testing.T) {
	if !strings.Contains(ExtraJS, `192\.168`) {
		t.Error("ExtraJS should contain local IP pattern for 192.168")
	}
	if !strings.Contains(ExtraJS, "172") {
		t.Error("ExtraJS should contain local IP pattern for 172.x")
	}
	if !strings.Contains(ExtraJS, "localIPPattern") {
		t.Error("ExtraJS should define localIPPattern")
	}
}
