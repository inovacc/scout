//go:build probe

package engine

import (
	"testing"
	"time"
)

// stealthExpectations documents what each probe should return with stealth enabled.
// true = should pass (not detected), false = known to still fail (needs GPU, timezone, etc.)
//
// When stealth mode is enabled via WithStealth(), the following evasions are active:
// - go-rod/stealth: patches navigator.webdriver, navigator.plugins, window.chrome, disable-blink-features=AutomationControlled
// - stealth_extra section 10 (document.$cdc_, window.callPhantom, window.__nightmare, window.domAutomation)
// - stealth_extra section 11 (navigator.vendor, navigator.hardwareConcurrency, navigator.deviceMemory)
// - stealth_extra section 4 (navigator.connection)
// - stealth_extra section 5 (Notification.permission)
// - stealth_extra section 12 (document.hasFocus())
// - stealth_extra section 13 (window.outerWidth/Height)
// - stealth_extra section 14 (toString prototype, if implemented)
// - canvas noise injection (stealth_extra)
// - WebGL vendor spoofing to Intel Inc. (stealth_extra)
//
// Known failures are primarily due to hardware limitations (GPU, timezone) not JS fixable via stealth.
var stealthExpectations = map[string]bool{
	// WEBDRIVER DETECTION — all should pass with stealth
	"navigator.webdriver":           true, // patched by go-rod/stealth
	"webdriver in navigator":        true, // patched by go-rod/stealth
	"webdriver property descriptor": true, // patched by go-rod/stealth
	"document.$cdc_":                true, // fixed in stealth_extra section 10
	"window.callPhantom":            true, // fixed in stealth_extra section 10
	"window.__nightmare":            true, // fixed in stealth_extra section 10
	"window.domAutomation":          true, // fixed in stealth_extra section 10

	// NAVIGATOR PROPERTIES — most should pass
	"navigator.languages":            true, // should be populated with stealth
	"navigator.plugins.length":       true, // patched by go-rod/stealth
	"navigator.mimeTypes.length":     true, // patched by go-rod/stealth
	"navigator.hardwareConcurrency":  true, // fixed in stealth_extra section 11
	"navigator.deviceMemory":         true, // fixed in stealth_extra section 11
	"navigator.platform consistency": true, // consistent user agent + platform
	"navigator.vendor":               true, // fixed in stealth_extra section 11 to Google Inc.
	"navigator.maxTouchPoints":       true, // should be consistent with device
	"navigator.connection":           true, // stealth_extra section 4
	"navigator.getBattery":           true, // browser API availability varies
	"window.chrome":                  true, // patched by go-rod/stealth
	"window.chrome.runtime":          true, // patched by go-rod/stealth (when extension loaded)

	// CANVAS FINGERPRINTING — noise added to prevent detection
	"canvas toDataURL":  true, // noise injection
	"canvas pixel data": true, // noise injection

	// WEBGL SIGNALS — spoofed or software rendering visible
	// NOTE: SwiftShader is still identifiable through various heuristics even with spoofing
	"webgl support":          true,  // should be available
	"webgl vendor":           true,  // spoofed to Intel Inc. via stealth_extra
	"webgl renderer":         false, // SwiftShader still visible (hardware limitation)
	"webgl max texture size": true,  // should be >= 4096
	"webgl line width range": false, // software renderer limitation (max=1 exposed)

	// AUDIO CONTEXT — may be suspended or unavailable in headless
	"AudioContext state":      true, // state varies but available
	"AudioContext sampleRate": true, // should be 44100 or 48000

	// SCREEN / VIEWPORT DIMENSIONS
	"screen dimensions":        true, // should be > 0
	"screen.colorDepth":        true, // should be >= 24
	"window.outerWidth/Height": true, // fixed in stealth_extra section 13
	"screen avail vs total":    true, // avail should be <= total
	"devicePixelRatio":         true, // should be >= 1

	// TIMING SIGNALS
	"page load time":            true, // should be > 0
	"performance.now precision": true, // informational, always passes

	// WEBRTC
	"RTCPeerConnection": true, // should be available in Chrome

	// PERMISSIONS
	"Notification.permission": true, // fixed in stealth_extra section 5

	// DOM API INTEGRITY
	"iframe contentWindow":    true, // should be accessible
	"toString prototype":      true, // fixed in stealth_extra section 14 (if implemented)
	"eval toString integrity": true, // eval should show [native code]

	// BEHAVIOR / ENVIRONMENT
	"window.innerWidth/Height":     true,  // should be > 0
	"document.hasFocus()":          true,  // fixed in stealth_extra section 12
	"Intl.DateTimeFormat timezone": false, // requires --timezone flag, not JS fixable

	// HTTP SIGNALS
	"User-Agent HeadlessChrome": true, // patched by disable-blink-features=AutomationControlled
	"User-Agent consistency":    true, // Chrome UA should include Safari token

	// FINGERPRINT CONSISTENCY
	"screen vs viewport ratio":    true, // viewport should never exceed screen
	"consistent platform signals": true, // platform, oscpu, userAgent should match
}

// TestStealthBaseline_Expectations validates that the stealthExpectations map
// is reasonably comprehensive by checking the count of documented probes.
// This test should pass on all systems without requiring a browser.
func TestStealthBaseline_Expectations(t *testing.T) {
	// We have 45+ probes in probeJS. stealthExpectations should cover most of them.
	minExpectations := 40
	if len(stealthExpectations) < minExpectations {
		t.Errorf("stealthExpectations has only %d entries, expected at least %d", len(stealthExpectations), minExpectations)
	}

	// Count expected pass/fail split
	pass, fail := 0, 0
	for name, shouldPass := range stealthExpectations {
		if shouldPass {
			pass++
		} else {
			fail++
			t.Logf("KNOWN FAILURE (documented): %s", name)
		}
	}
	t.Logf("Stealth baseline coverage: %d expected pass, %d known failures, %d total", pass, fail, pass+fail)

	// Ensure we're not 100% expected pass (unrealistic) or 0% (incomplete baseline)
	if pass == 0 {
		t.Errorf("baseline has no expected passes (baseline incomplete?)")
	}
	if fail == 0 {
		t.Logf("baseline expects all probes to pass (may be unrealistic if GPU/timezone involved)")
	}
}

// TestStealthBaseline_Score checks that stealth mode meets or exceeds a minimum
// pass rate compared to the baseline expectations.
//
// This test runs on a blank page (fast, no network dependency) and validates:
// 1. No regressions: probes expected to pass still pass
// 2. Possible improvements: probes marked as known failures now passing
// 3. Overall score >= 80%
//
// Run with: go test -v -run TestStealthBaseline_Score ./pkg/scout
func TestStealthBaseline_Score(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires browser")
	}

	b, err := New(
		WithHeadless(true),
		WithNoSandbox(),
		WithTimeout(30*time.Second),
		WithStealth(),
		WithoutBridge(),
	)
	if err != nil {
		t.Skipf("browser unavailable: %v", err)
	}
	defer func() { _ = b.Close() }()

	page, err := b.NewPage("about:blank")
	if err != nil {
		t.Skipf("new page: %v", err)
	}
	defer func() { _ = page.Close() }()

	report, err := runProbe(page)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}

	// Compare against baseline
	regressions := 0
	improvements := 0
	unknownProbes := 0

	for _, r := range report.Results {
		expected, ok := stealthExpectations[r.Name]
		if !ok {
			unknownProbes++
			t.Logf("NEW PROBE (no baseline): %s detected=%v value=%s", r.Name, r.Detected, r.Value)
			continue
		}

		actuallyPassed := !r.Detected

		if expected && !actuallyPassed {
			// Expected to pass but failed — regression
			t.Errorf("REGRESSION: %s was expected to pass but FAILED (value=%s)", r.Name, r.Value)
			regressions++
		} else if !expected && actuallyPassed {
			// Expected to fail but passed — improvement
			t.Logf("IMPROVEMENT: %s was expected to fail but PASSED (value=%s)", r.Name, r.Value)
			improvements++
		}
	}

	score := float64(report.Passed) / float64(report.TotalProbes) * 100
	t.Logf("")
	t.Logf("═══════════════════════════════════════════════════")
	t.Logf("  Stealth Baseline Score")
	t.Logf("═══════════════════════════════════════════════════")
	t.Logf("  Total Probes:      %d", report.TotalProbes)
	t.Logf("  Detected (Failed): %d", report.Detected)
	t.Logf("  Passed:            %d", report.Passed)
	t.Logf("  Score:             %.1f%%", score)
	t.Logf("═══════════════════════════════════════════════════")
	t.Logf("  Regressions:       %d (ERRORS)", regressions)
	t.Logf("  Improvements:      %d (INFO)", improvements)
	t.Logf("  Unknown Probes:    %d (NEW)", unknownProbes)
	t.Logf("═══════════════════════════════════════════════════")

	// Fail if there are regressions
	if regressions > 0 {
		t.Errorf("detected %d regression(s) from baseline", regressions)
	}

	// Warn if score is below 80%
	minimumScore := 80.0
	if score < minimumScore {
		t.Errorf("stealth score %.1f%% is below %.1f%% minimum threshold", score, minimumScore)
	}
}

// TestStealthBaseline_DetailedReport runs the baseline test and prints a detailed
// category-by-category breakdown comparing against expectations.
//
// Run with: go test -v -run TestStealthBaseline_DetailedReport ./pkg/scout
func TestStealthBaseline_DetailedReport(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires browser")
	}

	b, err := New(
		WithHeadless(true),
		WithNoSandbox(),
		WithTimeout(30*time.Second),
		WithStealth(),
		WithoutBridge(),
	)
	if err != nil {
		t.Skipf("browser unavailable: %v", err)
	}
	defer func() { _ = b.Close() }()

	page, err := b.NewPage("about:blank")
	if err != nil {
		t.Skipf("new page: %v", err)
	}
	defer func() { _ = page.Close() }()

	report, err := runProbe(page)
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	report.Stealth = true

	// Print full detailed report
	printReport(t, report)

	// Print baseline comparison
	t.Logf("")
	t.Logf("═══════════════════════════════════════════════════════════════════════")
	t.Logf("  BASELINE COMPARISON")
	t.Logf("═══════════════════════════════════════════════════════════════════════")
	t.Logf("")
	t.Logf("  %-40s | Expected | Actual | Status", "Probe Name")
	t.Logf("  %s", "────────────────────────────────────────────────────────────────────")

	for _, r := range report.Results {
		expected, ok := stealthExpectations[r.Name]
		if !ok {
			continue
		}

		expectedStr := "PASS"
		if !expected {
			expectedStr = "FAIL"
		}

		actualStr := "PASS"
		if r.Detected {
			actualStr = "FAIL"
		}

		status := "✓"
		if (expected && r.Detected) || (!expected && !r.Detected) {
			status = "✗"
		}

		t.Logf("  %-40s | %8s | %6s | %s", truncStr(r.Name, 40), expectedStr, actualStr, status)
	}
}

// TestStealthBaseline_CompareModes runs probes across three configurations:
// 1. No stealth (bare)
// 2. Stealth only
// 3. Stealth + random fingerprint
//
// This provides a comparison view of stealth effectiveness vs baseline.
//
// Run with: go test -v -run TestStealthBaseline_CompareModes ./pkg/scout
func TestStealthBaseline_CompareModes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires browser")
	}

	type modeConfig struct {
		Name string
		Opts []Option
	}

	modes := []modeConfig{
		{
			Name: "bare (no stealth)",
			Opts: []Option{WithHeadless(true), WithNoSandbox(), WithTimeout(30 * time.Second), WithoutBridge()},
		},
		{
			Name: "stealth only",
			Opts: []Option{WithHeadless(true), WithNoSandbox(), WithTimeout(30 * time.Second), WithStealth(), WithoutBridge()},
		},
		{
			Name: "stealth+fingerprint",
			Opts: []Option{WithHeadless(true), WithNoSandbox(), WithTimeout(30 * time.Second), WithStealth(), WithRandomFingerprint(), WithoutBridge()},
		},
	}

	reports := make([]*ProbeReport, len(modes))
	scores := make([]float64, len(modes))

	for i, mode := range modes {
		t.Run(mode.Name, func(t *testing.T) {
			b, err := New(mode.Opts...)
			if err != nil {
				t.Skipf("browser unavailable: %v", err)
			}
			defer func() { _ = b.Close() }()

			page, err := b.NewPage("about:blank")
			if err != nil {
				t.Skipf("new page: %v", err)
			}
			defer func() { _ = page.Close() }()

			report, err := runProbe(page)
			if err != nil {
				t.Fatalf("probe: %v", err)
			}
			reports[i] = report
			scores[i] = float64(report.Passed) / float64(report.TotalProbes) * 100
		})
	}

	// Print summary comparison table
	t.Logf("")
	t.Logf("╔═══════════════════════════════════════════════════════════════════╗")
	t.Logf("║            STEALTH BASELINE MODE COMPARISON                      ║")
	t.Logf("╠═══════════════════════════════════════════════════════════════════╣")
	t.Logf("║  Mode                  │ Total │ Detected │ Passed │ Score      ║")
	t.Logf("╠═══════════════════════════════════════════════════════════════════╣")
	for i, mode := range modes {
		r := reports[i]
		if r == nil {
			t.Logf("║  %-22s │  ---  │   ---    │  ---   │  skipped   ║", mode.Name)
			continue
		}
		t.Logf("║  %-22s │  %3d  │   %3d    │  %3d   │  %5.1f%%     ║", mode.Name, r.TotalProbes, r.Detected, r.Passed, scores[i])
	}
	t.Logf("╚═══════════════════════════════════════════════════════════════════╝")

	// Calculate stealth improvement
	if reports[0] != nil && reports[1] != nil {
		improvement := scores[1] - scores[0]
		t.Logf("")
		t.Logf("Stealth improvement over bare: %.1f%% → %.1f%% (+%.1f%% improvement)", scores[0], scores[1], improvement)
	}

	// Print per-probe comparison for stealth mode vs baseline
	t.Logf("")
	t.Logf("Per-probe baseline alignment for stealth mode:")
	t.Logf("")
	if reports[1] != nil {
		t.Logf("  %-40s | Baseline | Actual | Match", "Probe Name")
		t.Logf("  %s", "────────────────────────────────────────────────────────────────────")
		for _, r := range reports[1].Results {
			expected, ok := stealthExpectations[r.Name]
			if !ok {
				continue
			}

			expectedStr := "PASS"
			if !expected {
				expectedStr = "FAIL"
			}

			actualStr := "PASS"
			if r.Detected {
				actualStr = "FAIL"
			}

			match := "YES"
			if (expected && r.Detected) || (!expected && !r.Detected) {
				match = "NO"
			}

			t.Logf("  %-40s | %8s | %6s | %s", truncStr(r.Name, 40), expectedStr, actualStr, match)
		}
	}
}
