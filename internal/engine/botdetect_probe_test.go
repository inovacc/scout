//go:build probe

package engine

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// DetectionCategory groups related bot-detection signals.
type DetectionCategory string

const (
	CatWebDriver   DetectionCategory = "webdriver"
	CatNavigator   DetectionCategory = "navigator"
	CatCanvas      DetectionCategory = "canvas"
	CatWebGL       DetectionCategory = "webgl"
	CatAudio       DetectionCategory = "audio"
	CatFonts       DetectionCategory = "fonts"
	CatPlugins     DetectionCategory = "plugins"
	CatScreen      DetectionCategory = "screen"
	CatTiming      DetectionCategory = "timing"
	CatWebRTC      DetectionCategory = "webrtc"
	CatPermissions DetectionCategory = "permissions"
	CatDOM         DetectionCategory = "dom"
	CatBehavior    DetectionCategory = "behavior"
	CatHTTP        DetectionCategory = "http"
	CatFingerprint DetectionCategory = "fingerprint"
)

// ProbeResult holds the outcome of a single detection probe.
type ProbeResult struct {
	Category    DetectionCategory `json:"category"`
	Name        string            `json:"name"`
	Detected    bool              `json:"detected"`
	Value       string            `json:"value"`
	Expected    string            `json:"expected"`
	Description string            `json:"description"`
}

// ProbeReport is the full output from probing a page.
type ProbeReport struct {
	URL         string        `json:"url"`
	Stealth     bool          `json:"stealth"`
	TotalProbes int           `json:"total_probes"`
	Detected    int           `json:"detected"`
	Passed      int           `json:"passed"`
	Results     []ProbeResult `json:"results"`
}

// probeJS is the comprehensive JavaScript that checks every known bot detection signal.
// Each check returns {category, name, detected, value, expected, description}.
const probeJS = `() => {
	const results = [];

	function add(category, name, detected, value, expected, desc) {
		results.push({
			category: category,
			name: name,
			detected: !!detected,
			value: String(value),
			expected: String(expected),
			description: desc
		});
	}

	// ═══════════════════ WEBDRIVER ═══════════════════

	add('webdriver', 'navigator.webdriver',
		navigator.webdriver === true,
		String(navigator.webdriver),
		'false/undefined',
		'Chrome sets navigator.webdriver=true for automated sessions');

	add('webdriver', 'webdriver in navigator',
		'webdriver' in navigator && navigator.webdriver !== false,
		String('webdriver' in navigator),
		'false or property absent',
		'Property existence check — some bots fail to remove it');

	// Check for webdriver via Object.getOwnPropertyDescriptor
	(function() {
		var desc = Object.getOwnPropertyDescriptor(navigator, 'webdriver');
		var suspicious = desc && desc.get && /native code/.test(desc.get.toString()) === false;
		add('webdriver', 'webdriver property descriptor',
			suspicious,
			desc ? JSON.stringify({configurable: desc.configurable, enumerable: desc.enumerable}) : 'undefined',
			'native getter or undefined',
			'Overridden webdriver getter has non-native toString');
	})();

	add('webdriver', 'document.$cdc_',
		!!(document.$cdc_asdjflasutopfhvcZLmcfl_ || window.cdc_adoQpoasnfa76pfcZLmcfl_l8),
		String(!!document.$cdc_asdjflasutopfhvcZLmcfl_),
		'false',
		'ChromeDriver injects $cdc_ variables into the document');

	add('webdriver', 'window.callPhantom',
		typeof window.callPhantom !== 'undefined' || typeof window._phantom !== 'undefined',
		typeof window.callPhantom,
		'undefined',
		'PhantomJS ghost variables');

	add('webdriver', 'window.__nightmare',
		typeof window.__nightmare !== 'undefined',
		typeof window.__nightmare,
		'undefined',
		'Nightmare.js automation marker');

	add('webdriver', 'window.domAutomation',
		typeof window.domAutomation !== 'undefined' || typeof window.domAutomationController !== 'undefined',
		typeof window.domAutomation,
		'undefined',
		'Chrome DevTools automation controller');

	// ═══════════════════ NAVIGATOR ═══════════════════

	add('navigator', 'navigator.languages',
		!navigator.languages || navigator.languages.length === 0,
		JSON.stringify(navigator.languages),
		'["en-US","en"] or similar',
		'Headless Chrome often has empty languages array');

	add('navigator', 'navigator.plugins.length',
		navigator.plugins.length === 0,
		String(navigator.plugins.length),
		'>0',
		'Real browsers have plugins (PDF viewer, etc.)');

	add('navigator', 'navigator.mimeTypes.length',
		navigator.mimeTypes.length === 0,
		String(navigator.mimeTypes.length),
		'>0',
		'Real browsers have MIME types registered');

	add('navigator', 'navigator.hardwareConcurrency',
		navigator.hardwareConcurrency === 0 || navigator.hardwareConcurrency === undefined,
		String(navigator.hardwareConcurrency),
		'>=2',
		'Headless may report 0 or undefined cores');

	add('navigator', 'navigator.deviceMemory',
		navigator.deviceMemory === 0 || navigator.deviceMemory === undefined,
		String(navigator.deviceMemory),
		'>=2',
		'Headless may report 0 or undefined device memory');

	add('navigator', 'navigator.platform consistency',
		(function() {
			var ua = navigator.userAgent.toLowerCase();
			var plat = navigator.platform.toLowerCase();
			if (ua.includes('win') && !plat.includes('win')) return true;
			if (ua.includes('mac') && !plat.includes('mac')) return true;
			if (ua.includes('linux') && !plat.includes('linux') && !plat.includes('cros')) return true;
			return false;
		})(),
		navigator.platform + ' vs ' + navigator.userAgent.substring(0, 50),
		'platform matches UA',
		'Mismatched platform and UA suggests spoofing');

	add('navigator', 'navigator.vendor',
		navigator.vendor === '' && /chrome/i.test(navigator.userAgent),
		String(navigator.vendor),
		'Google Inc.',
		'Chrome should have Google Inc. as vendor');

	add('navigator', 'navigator.maxTouchPoints',
		(function() {
			var ua = navigator.userAgent.toLowerCase();
			// Desktop claiming touch or mobile claiming no touch
			if (!ua.includes('mobile') && !ua.includes('android') && navigator.maxTouchPoints > 0) return false; // OK for some laptops
			return false;
		})(),
		String(navigator.maxTouchPoints),
		'consistent with device type',
		'Touch points should match device type');

	add('navigator', 'navigator.connection',
		typeof navigator.connection === 'undefined' && /chrome/i.test(navigator.userAgent),
		typeof navigator.connection,
		'object',
		'Chrome should have NetworkInformation API');

	add('navigator', 'navigator.getBattery',
		typeof navigator.getBattery === 'undefined' && /chrome/i.test(navigator.userAgent) && !navigator.userAgent.includes('Headless'),
		typeof navigator.getBattery,
		'function',
		'Chrome supports Battery API');

	// ═══════════════════ CHROME OBJECT ═══════════════════

	add('navigator', 'window.chrome',
		typeof window.chrome === 'undefined' && /chrome/i.test(navigator.userAgent),
		typeof window.chrome,
		'object',
		'Chrome browser should have window.chrome object');

	add('navigator', 'window.chrome.runtime',
		(function() {
			if (typeof window.chrome === 'undefined') return true;
			if (typeof window.chrome.runtime === 'undefined') return false; // OK for non-extension pages
			return false;
		})(),
		typeof window.chrome === 'object' ? typeof window.chrome.runtime : 'no chrome obj',
		'object (with or without runtime)',
		'chrome.runtime presence check');

	// ═══════════════════ CANVAS ═══════════════════

	(function() {
		try {
			var canvas = document.createElement('canvas');
			canvas.width = 200;
			canvas.height = 50;
			var ctx = canvas.getContext('2d');
			ctx.textBaseline = 'top';
			ctx.font = '14px Arial';
			ctx.fillStyle = '#f60';
			ctx.fillRect(125, 1, 62, 20);
			ctx.fillStyle = '#069';
			ctx.fillText('BotDetect,01onal', 2, 15);
			ctx.fillStyle = 'rgba(102, 204, 0, 0.7)';
			ctx.fillText('BotDetect,01anal', 4, 17);
			var dataURL = canvas.toDataURL();
			var hash = dataURL.length;

			add('canvas', 'canvas toDataURL',
				dataURL === 'data:,' || dataURL.length < 1000,
				'length=' + dataURL.length,
				'length > 1000',
				'Empty or tiny canvas output suggests headless/blocked canvas');

			// Check if canvas is all zeros (GPU disabled)
			var imgData = ctx.getImageData(0, 0, 200, 50);
			var allZero = true;
			for (var i = 0; i < imgData.data.length; i += 100) {
				if (imgData.data[i] !== 0) { allZero = false; break; }
			}
			add('canvas', 'canvas pixel data',
				allZero,
				allZero ? 'all zeros' : 'has pixel data',
				'has pixel data',
				'All-zero canvas means GPU rendering is disabled');
		} catch(e) {
			add('canvas', 'canvas support', true, 'error: ' + e.message, 'working', 'Canvas API threw an error');
		}
	})();

	// ═══════════════════ WEBGL ═══════════════════

	(function() {
		try {
			var canvas = document.createElement('canvas');
			var gl = canvas.getContext('webgl') || canvas.getContext('experimental-webgl');
			if (!gl) {
				add('webgl', 'webgl support', true, 'null', 'WebGLRenderingContext', 'WebGL context unavailable');
				return;
			}

			var ext = gl.getExtension('WEBGL_debug_renderer_info');
			var vendor = ext ? gl.getParameter(ext.UNMASKED_VENDOR_WEBGL) : 'no ext';
			var renderer = ext ? gl.getParameter(ext.UNMASKED_RENDERER_WEBGL) : 'no ext';

			add('webgl', 'webgl vendor',
				vendor === '' || vendor === 'Brian Paul' || vendor.includes('Mesa') || vendor.includes('SwiftShader'),
				vendor,
				'Intel Inc./NVIDIA/AMD',
				'SwiftShader/Mesa/Brian Paul = software rendering (headless)');

			add('webgl', 'webgl renderer',
				renderer.includes('SwiftShader') || renderer.includes('llvmpipe') || renderer === '',
				renderer,
				'real GPU renderer',
				'SwiftShader/llvmpipe = software renderer used by headless Chrome');

			// Check WebGL parameters
			var maxTexSize = gl.getParameter(gl.MAX_TEXTURE_SIZE);
			add('webgl', 'webgl max texture size',
				maxTexSize < 4096,
				String(maxTexSize),
				'>= 4096',
				'Very low max texture size suggests software rendering');

			var aliasedLineWidthRange = gl.getParameter(gl.ALIASED_LINE_WIDTH_RANGE);
			add('webgl', 'webgl line width range',
				aliasedLineWidthRange && aliasedLineWidthRange[1] === 1,
				aliasedLineWidthRange ? '[' + aliasedLineWidthRange[0] + ',' + aliasedLineWidthRange[1] + ']' : 'null',
				'[1, >1]',
				'Line width max=1 is common in software renderers');
		} catch(e) {
			add('webgl', 'webgl support', true, 'error: ' + e.message, 'working', 'WebGL threw an error');
		}
	})();

	// ═══════════════════ AUDIO ═══════════════════

	(function() {
		try {
			var ctx = new (window.AudioContext || window.webkitAudioContext)();
			add('audio', 'AudioContext state',
				ctx.state === 'suspended' || ctx.state === 'closed',
				ctx.state,
				'running or suspended',
				'AudioContext availability check');

			add('audio', 'AudioContext sampleRate',
				ctx.sampleRate === 0,
				String(ctx.sampleRate),
				'44100 or 48000',
				'Zero sample rate means audio is disabled');

			ctx.close();
		} catch(e) {
			add('audio', 'AudioContext support', true, 'error: ' + e.message, 'working', 'AudioContext not available');
		}
	})();

	// ═══════════════════ SCREEN ═══════════════════

	add('screen', 'screen dimensions',
		screen.width === 0 || screen.height === 0,
		screen.width + 'x' + screen.height,
		'>0x>0',
		'Zero screen dimensions indicate headless');

	add('screen', 'screen.colorDepth',
		screen.colorDepth < 24,
		String(screen.colorDepth),
		'24 or 32',
		'Low color depth unusual for modern displays');

	add('screen', 'window.outerWidth/Height',
		window.outerWidth === 0 || window.outerHeight === 0,
		window.outerWidth + 'x' + window.outerHeight,
		'>0x>0',
		'Zero outer dimensions common in headless');

	add('screen', 'screen avail vs total',
		(function() {
			// availWidth/Height should be <= width/height
			return screen.availWidth > screen.width || screen.availHeight > screen.height;
		})(),
		'avail=' + screen.availWidth + 'x' + screen.availHeight + ' total=' + screen.width + 'x' + screen.height,
		'avail <= total',
		'Available dimensions exceeding total is inconsistent');

	add('screen', 'devicePixelRatio',
		window.devicePixelRatio === 0 || window.devicePixelRatio === undefined,
		String(window.devicePixelRatio),
		'>=1',
		'Missing or zero DPR suggests headless');

	// ═══════════════════ TIMING ═══════════════════

	(function() {
		if (!window.performance || !performance.timing) return;
		var t = performance.timing;
		var loadTime = t.loadEventEnd - t.navigationStart;

		add('timing', 'page load time',
			loadTime <= 0 && t.loadEventEnd > 0,
			loadTime + 'ms',
			'>0ms',
			'Zero or negative load time is suspicious');

		// Check if Performance.now() has reduced precision (privacy mode)
		var t1 = performance.now();
		var t2 = performance.now();
		var precision = t2 - t1;
		add('timing', 'performance.now precision',
			false, // informational
			precision.toFixed(6) + 'ms',
			'varies',
			'Reduced timer precision may indicate privacy settings (informational)');
	})();

	// ═══════════════════ WEBRTC ═══════════════════

	add('webrtc', 'RTCPeerConnection',
		typeof RTCPeerConnection === 'undefined' && typeof webkitRTCPeerConnection === 'undefined',
		typeof RTCPeerConnection,
		'function',
		'WebRTC should be available in Chrome');

	// ═══════════════════ PERMISSIONS ═══════════════════

	(function() {
		if (typeof Notification !== 'undefined') {
			add('permissions', 'Notification.permission',
				Notification.permission === 'denied',
				Notification.permission,
				'default',
				'Headless Chrome often has notifications denied');
		}
	})();

	// ═══════════════════ DOM ═══════════════════

	add('dom', 'iframe contentWindow',
		(function() {
			try {
				var iframe = document.createElement('iframe');
				iframe.style.display = 'none';
				document.body.appendChild(iframe);
				var hasContentWindow = !!iframe.contentWindow;
				document.body.removeChild(iframe);
				return !hasContentWindow;
			} catch(e) { return false; }
		})(),
		'',
		'accessible',
		'iframe contentWindow should be accessible');

	add('dom', 'toString prototype',
		(function() {
			// Native functions should produce [native code] in toString
			var nativeStr = Function.prototype.toString.call(navigator.permissions.query);
			return !nativeStr.includes('native code') && !nativeStr.includes('[native code]');
		})(),
		'',
		'[native code]',
		'Overridden native functions leak via toString');

	// Check if common APIs have been tampered with
	add('dom', 'eval toString integrity',
		(function() {
			try {
				var s = eval.toString();
				return !s.includes('native code');
			} catch(e) { return false; }
		})(),
		'',
		'function eval() { [native code] }',
		'eval() toString should show native code');

	// ═══════════════════ BEHAVIOR / ENVIRONMENT ═══════════════════

	add('behavior', 'window.innerWidth/Height',
		window.innerWidth === 0 || window.innerHeight === 0,
		window.innerWidth + 'x' + window.innerHeight,
		'>0x>0',
		'Zero viewport suggests headless/minimized');

	add('behavior', 'document.hasFocus()',
		!document.hasFocus(),
		String(document.hasFocus()),
		'true',
		'Headless pages may not have focus');

	add('behavior', 'Intl.DateTimeFormat timezone',
		(function() {
			var tz = Intl.DateTimeFormat().resolvedOptions().timeZone;
			return !tz || tz === '' || tz === 'UTC';
		})(),
		Intl.DateTimeFormat().resolvedOptions().timeZone || 'empty',
		'real timezone (e.g. America/New_York)',
		'UTC or empty timezone unusual for real users');

	// ═══════════════════ HTTP SIGNALS ═══════════════════

	add('http', 'User-Agent HeadlessChrome',
		/HeadlessChrome/i.test(navigator.userAgent),
		navigator.userAgent.substring(0, 80),
		'no HeadlessChrome',
		'HeadlessChrome in UA is an obvious bot signal');

	add('http', 'User-Agent consistency',
		(function() {
			var ua = navigator.userAgent;
			// Check for contradictions
			if (/Chrome/.test(ua) && !/Safari/.test(ua)) return true; // Chrome UA always has Safari
			return false;
		})(),
		navigator.userAgent.substring(0, 80),
		'Chrome/... Safari/...',
		'Chrome UA must include Safari token');

	// ═══════════════════ FINGERPRINT CONSISTENCY ═══════════════════

	add('fingerprint', 'screen vs viewport ratio',
		(function() {
			if (screen.width === 0 || window.innerWidth === 0) return false;
			// Viewport should never exceed screen
			return window.innerWidth > screen.width || window.innerHeight > screen.height;
		})(),
		'viewport=' + window.innerWidth + 'x' + window.innerHeight + ' screen=' + screen.width + 'x' + screen.height,
		'viewport <= screen',
		'Viewport larger than screen is physically impossible');

	add('fingerprint', 'consistent platform signals',
		(function() {
			var ua = navigator.userAgent.toLowerCase();
			var plat = (navigator.platform || '').toLowerCase();
			var oscpu = (navigator.oscpu || '').toLowerCase();
			// If oscpu exists it should match platform
			if (oscpu && plat) {
				if (plat.includes('win') && !oscpu.includes('win')) return true;
				if (plat.includes('mac') && !oscpu.includes('mac')) return true;
				if (plat.includes('linux') && !oscpu.includes('linux')) return true;
			}
			return false;
		})(),
		navigator.platform + ' / ' + (navigator.oscpu || 'n/a'),
		'matching platform signals',
		'platform, oscpu, and userAgentData should agree');

	return results;
}`

// runProbe executes the probe JS on a page and returns parsed results.
func runProbe(p *Page) (*ProbeReport, error) {
	if err := p.WaitLoad(); err != nil {
		return nil, fmt.Errorf("wait load: %w", err)
	}
	p.WaitDOMStable(500*time.Millisecond, 0.1)

	result, err := p.Eval(probeJS)
	if err != nil {
		return nil, fmt.Errorf("eval probe: %w", err)
	}

	raw, err := json.Marshal(result.Value)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	var results []ProbeResult
	if err := json.Unmarshal(raw, &results); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	report := &ProbeReport{
		URL:         p.page.MustInfo().URL,
		TotalProbes: len(results),
		Results:     results,
	}
	for _, r := range results {
		if r.Detected {
			report.Detected++
		} else {
			report.Passed++
		}
	}
	return report, nil
}

// printReport logs a formatted probe report.
func printReport(t *testing.T, report *ProbeReport) {
	t.Logf("═══════════════════════════════════════════════")
	t.Logf("  URL:      %s", report.URL)
	t.Logf("  Stealth:  %v", report.Stealth)
	t.Logf("  Total:    %d probes", report.TotalProbes)
	t.Logf("  Detected: %d  |  Passed: %d", report.Detected, report.Passed)
	t.Logf("═══════════════════════════════════════════════")

	byCategory := map[DetectionCategory][]ProbeResult{}
	for _, r := range report.Results {
		byCategory[r.Category] = append(byCategory[r.Category], r)
	}

	cats := []DetectionCategory{
		CatWebDriver, CatNavigator, CatCanvas, CatWebGL, CatAudio,
		CatScreen, CatTiming, CatWebRTC, CatPermissions, CatDOM,
		CatBehavior, CatHTTP, CatFingerprint,
	}
	for _, cat := range cats {
		results, ok := byCategory[cat]
		if !ok {
			continue
		}
		t.Logf("")
		t.Logf("  [%s]", cat)
		for _, r := range results {
			status := "PASS"
			if r.Detected {
				status = "FAIL"
			}
			t.Logf("    %s %-35s val=%-30s expected=%s", status, r.Name, truncStr(r.Value, 30), truncStr(r.Expected, 30))
			if r.Detected {
				t.Logf("         ↳ %s", r.Description)
			}
		}
	}
}

func truncStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

// TestBotProbe_NoStealth runs the full detection probe WITHOUT stealth on a blank page
// and on each bot detection site. This establishes a baseline of what gets detected.
func TestBotProbe_NoStealth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires network access")
	}

	b, err := New(
		WithHeadless(true),
		WithNoSandbox(),
		WithTimeout(30*time.Second),
		WithoutBridge(),
	)
	if err != nil {
		t.Skipf("browser unavailable: %v", err)
	}
	defer func() { _ = b.Close() }()

	// Probe on a blank page first (fastest, no network dependency).
	t.Run("blank", func(t *testing.T) {
		page, err := b.NewPage("about:blank")
		if err != nil {
			t.Skipf("new page: %v", err)
		}
		defer func() { _ = page.Close() }()

		report, err := runProbe(page)
		if err != nil {
			t.Fatalf("probe: %v", err)
		}
		report.Stealth = false
		printReport(t, report)
		t.Logf("SUMMARY (no stealth): %d/%d detected", report.Detected, report.TotalProbes)
	})

	// Probe on each bot detection site.
	for _, site := range botCheckSites {
		t.Run(site.Name, func(t *testing.T) {
			page, err := b.NewPage(site.URL)
			if err != nil {
				t.Skipf("navigate: %v", err)
			}
			defer func() { _ = page.Close() }()

			report, err := runProbe(page)
			if err != nil {
				t.Logf("probe failed (site may block): %v", err)
				return
			}
			report.Stealth = false
			printReport(t, report)

			// Also run the site's own check.
			isBot, detail := site.Check(page)
			t.Logf("Site's own verdict: detected=%v detail=%s", isBot, detail)
		})
	}
}

// TestBotProbe_WithStealth runs the same probes WITH stealth enabled.
// Compare results against TestBotProbe_NoStealth to measure stealth effectiveness.
func TestBotProbe_WithStealth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires network access")
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

	t.Run("blank", func(t *testing.T) {
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
		printReport(t, report)
		t.Logf("SUMMARY (stealth): %d/%d detected", report.Detected, report.TotalProbes)
	})

	for _, site := range botCheckSites {
		t.Run(site.Name, func(t *testing.T) {
			page, err := b.NewPage(site.URL)
			if err != nil {
				t.Skipf("navigate: %v", err)
			}
			defer func() { _ = page.Close() }()

			report, err := runProbe(page)
			if err != nil {
				t.Logf("probe failed (site may block): %v", err)
				return
			}
			report.Stealth = true
			printReport(t, report)

			isBot, detail := site.Check(page)
			t.Logf("Site's own verdict: detected=%v detail=%s", isBot, detail)
		})
	}
}

// TestBotProbe_WithStealthAndFingerprint runs probes with stealth + random fingerprint.
// This is the maximum evasion configuration.
func TestBotProbe_WithStealthAndFingerprint(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires network access")
	}

	b, err := New(
		WithHeadless(true),
		WithNoSandbox(),
		WithTimeout(30*time.Second),
		WithStealth(),
		WithRandomFingerprint(),
		WithoutBridge(),
	)
	if err != nil {
		t.Skipf("browser unavailable: %v", err)
	}
	defer func() { _ = b.Close() }()

	t.Run("blank", func(t *testing.T) {
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
		printReport(t, report)
		t.Logf("SUMMARY (stealth+fingerprint): %d/%d detected", report.Detected, report.TotalProbes)
	})

	for _, site := range botCheckSites {
		t.Run(site.Name, func(t *testing.T) {
			page, err := b.NewPage(site.URL)
			if err != nil {
				t.Skipf("navigate: %v", err)
			}
			defer func() { _ = page.Close() }()

			report, err := runProbe(page)
			if err != nil {
				t.Logf("probe failed (site may block): %v", err)
				return
			}
			report.Stealth = true
			printReport(t, report)

			isBot, detail := site.Check(page)
			t.Logf("Site's own verdict: detected=%v detail=%s", isBot, detail)
		})
	}
}

// TestBotProbe_CompareAll is a summary test that runs blank-page probes for all 3 modes
// and prints a comparison table.
func TestBotProbe_CompareAll(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires network access")
	}

	type modeConfig struct {
		Name string
		Opts []Option
	}

	modes := []modeConfig{
		{
			Name: "bare",
			Opts: []Option{WithHeadless(true), WithNoSandbox(), WithTimeout(30 * time.Second), WithoutBridge()},
		},
		{
			Name: "stealth",
			Opts: []Option{WithHeadless(true), WithNoSandbox(), WithTimeout(30 * time.Second), WithStealth(), WithoutBridge()},
		},
		{
			Name: "stealth+fp",
			Opts: []Option{WithHeadless(true), WithNoSandbox(), WithTimeout(30 * time.Second), WithStealth(), WithRandomFingerprint(), WithoutBridge()},
		},
	}

	reports := make([]*ProbeReport, len(modes))

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
		})
	}

	// Print comparison table.
	t.Logf("")
	t.Logf("╔═══════════════════════════════════════════════════════════════╗")
	t.Logf("║              BOT DETECTION PROBE COMPARISON                 ║")
	t.Logf("╠═══════════════════════════════════════════════════════════════╣")
	t.Logf("║  Mode           │ Total │ Detected │ Passed │ Score         ║")
	t.Logf("╠═══════════════════════════════════════════════════════════════╣")
	for i, mode := range modes {
		r := reports[i]
		if r == nil {
			t.Logf("║  %-15s │  ---  │   ---    │  ---   │  skipped      ║", mode.Name)
			continue
		}
		score := float64(r.Passed) / float64(r.TotalProbes) * 100
		t.Logf("║  %-15s │  %3d  │   %3d    │  %3d   │  %5.1f%%        ║", mode.Name, r.TotalProbes, r.Detected, r.Passed, score)
	}
	t.Logf("╚═══════════════════════════════════════════════════════════════╝")

	// Per-probe comparison.
	if reports[0] != nil {
		t.Logf("")
		t.Logf("Per-probe breakdown:")
		t.Logf("  %-12s %-35s %6s %8s %10s", "Category", "Probe", "Bare", "Stealth", "Stealth+FP")
		t.Logf("  %s", "─────────────────────────────────────────────────────────────────────────────")
		for j, r := range reports[0].Results {
			bare := "PASS"
			if r.Detected {
				bare = "FAIL"
			}
			stealth := "---"
			stealthFP := "---"
			if reports[1] != nil && j < len(reports[1].Results) {
				if reports[1].Results[j].Detected {
					stealth = "FAIL"
				} else {
					stealth = "PASS"
				}
			}
			if reports[2] != nil && j < len(reports[2].Results) {
				if reports[2].Results[j].Detected {
					stealthFP = "FAIL"
				} else {
					stealthFP = "PASS"
				}
			}
			t.Logf("  %-12s %-35s %6s %8s %10s", r.Category, truncStr(r.Name, 35), bare, stealth, stealthFP)
		}
	}
}
