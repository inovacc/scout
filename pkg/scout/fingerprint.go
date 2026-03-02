package scout

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/big"
	mrand "math/rand"
	"strings"
	"time"
)

// Fingerprint represents a complete browser fingerprint for anti-detection.
// All fields are chosen to be internally consistent (e.g. a Mac user agent
// will have Apple WebGL and Mac-like screen resolutions).
type Fingerprint struct {
	UserAgent           string   `json:"user_agent"`
	Platform            string   `json:"platform"`
	Vendor              string   `json:"vendor"`
	Languages           []string `json:"languages"`
	Timezone            string   `json:"timezone"`
	ScreenWidth         int      `json:"screen_width"`
	ScreenHeight        int      `json:"screen_height"`
	ColorDepth          int      `json:"color_depth"`
	PixelRatio          float64  `json:"pixel_ratio"`
	WebGLVendor         string   `json:"webgl_vendor"`
	WebGLRenderer       string   `json:"webgl_renderer"`
	HardwareConcurrency int      `json:"hardware_concurrency"`
	DeviceMemory        int      `json:"device_memory"`
	MaxTouchPoints      int      `json:"max_touch_points"`
	DoNotTrack          string   `json:"do_not_track"`
}

// FingerprintOption configures fingerprint generation.
type FingerprintOption func(*fingerprintConfig)

type fingerprintConfig struct {
	os     string // "windows", "mac", "linux"
	mobile bool
	locale string // e.g. "en-US", "pt-BR"
}

// WithFingerprintOS restricts the generated fingerprint to a specific OS.
// Supported values: "windows", "mac", "linux". Empty means random.
func WithFingerprintOS(os string) FingerprintOption {
	return func(c *fingerprintConfig) { c.os = strings.ToLower(os) }
}

// WithFingerprintMobile generates a mobile fingerprint when true.
func WithFingerprintMobile(mobile bool) FingerprintOption {
	return func(c *fingerprintConfig) { c.mobile = mobile }
}

// WithFingerprintLocale sets the locale/language for the fingerprint.
// This also determines the timezone. E.g. "en-US", "pt-BR", "de-DE".
func WithFingerprintLocale(locale string) FingerprintOption {
	return func(c *fingerprintConfig) { c.locale = locale }
}

// GenerateFingerprint creates a realistic, randomized browser fingerprint.
// Options control OS, mobile mode, and locale. Without options, all values
// are chosen randomly from curated pools of real browser data.
func GenerateFingerprint(opts ...FingerprintOption) *Fingerprint {
	cfg := &fingerprintConfig{}
	for _, fn := range opts {
		fn(cfg)
	}

	rng := newCryptoSeededRand()

	fp := &Fingerprint{}

	if cfg.mobile {
		generateMobile(fp, rng)
	} else {
		generateDesktop(fp, cfg, rng)
	}

	// Timezone and languages.
	tzl := pickTimezoneLocale(cfg, rng)
	fp.Timezone = tzl.Timezone
	fp.Languages = tzl.Langs

	// Hardware.
	fp.HardwareConcurrency = pick(rng, hardwareConcurrencies)
	fp.DeviceMemory = pick(rng, deviceMemories)

	if cfg.mobile {
		fp.MaxTouchPoints = 5
		fp.DoNotTrack = ""
	} else {
		fp.MaxTouchPoints = 0
		// ~30% of users set DNT.
		if rng.Intn(10) < 3 {
			fp.DoNotTrack = "1"
		}
	}

	return fp
}

func generateDesktop(fp *Fingerprint, cfg *fingerprintConfig, rng *mrand.Rand) {
	os := cfg.os
	if os == "" {
		os = pick(rng, []string{"windows", "mac", "linux"})
	}

	switch os {
	case "mac":
		fp.UserAgent = pick(rng, userAgentsMac)
		fp.Platform = "MacIntel"
		fp.Vendor = "Google Inc."
		res := pick(rng, screenResolutionsMac)
		fp.ScreenWidth = res.Width
		fp.ScreenHeight = res.Height
		wgl := pick(rng, webglProfilesMac)
		fp.WebGLVendor = wgl.Vendor
		fp.WebGLRenderer = wgl.Renderer
	case "linux":
		fp.UserAgent = pick(rng, userAgentsLinux)
		fp.Platform = "Linux x86_64"
		fp.Vendor = "Google Inc."
		res := pick(rng, screenResolutionsLinux)
		fp.ScreenWidth = res.Width
		fp.ScreenHeight = res.Height
		wgl := pick(rng, webglProfilesLinux)
		fp.WebGLVendor = wgl.Vendor
		fp.WebGLRenderer = wgl.Renderer
	default: // windows
		fp.UserAgent = pick(rng, userAgentsWindows)
		fp.Platform = "Win32"
		fp.Vendor = "Google Inc."
		res := pick(rng, screenResolutionsWindows)
		fp.ScreenWidth = res.Width
		fp.ScreenHeight = res.Height
		wgl := pick(rng, webglProfilesWindows)
		fp.WebGLVendor = wgl.Vendor
		fp.WebGLRenderer = wgl.Renderer
	}

	fp.ColorDepth = pick(rng, colorDepths)
	fp.PixelRatio = pick(rng, pixelRatiosDesktop)
}

func generateMobile(fp *Fingerprint, rng *mrand.Rand) {
	fp.UserAgent = pick(rng, userAgentsMobile)
	if strings.Contains(fp.UserAgent, "iPhone") {
		fp.Platform = "iPhone"
		fp.Vendor = "Apple Computer, Inc."
		wgl := webglProfilesMobile[len(webglProfilesMobile)-1] // Apple GPU
		fp.WebGLVendor = wgl.Vendor
		fp.WebGLRenderer = wgl.Renderer
	} else {
		fp.Platform = "Linux armv81"
		fp.Vendor = "Google Inc."
		wgl := pick(rng, webglProfilesMobile[:len(webglProfilesMobile)-1])
		fp.WebGLVendor = wgl.Vendor
		fp.WebGLRenderer = wgl.Renderer
	}

	res := pick(rng, screenResolutionsMobile)
	fp.ScreenWidth = res.Width
	fp.ScreenHeight = res.Height
	fp.ColorDepth = 24
	fp.PixelRatio = pick(rng, pixelRatiosMobile)
}

func pickTimezoneLocale(cfg *fingerprintConfig, rng *mrand.Rand) timezoneLocale {
	if cfg.locale != "" {
		for _, tz := range timezoneLocales {
			if strings.EqualFold(tz.Locale, cfg.locale) {
				return tz
			}
		}
		// Locale not found in our list, fall back to en-US with the locale as language.
		return timezoneLocale{
			Timezone: "America/New_York",
			Locale:   cfg.locale,
			Langs:    []string{cfg.locale, "en"},
		}
	}

	return pick(rng, timezoneLocales)
}

// ToJS generates JavaScript that spoofs all navigator, screen, and WebGL
// properties to match this fingerprint. Designed to be injected via
// EvalOnNewDocument before page scripts run.
func (fp *Fingerprint) ToJS() string {
	langsJSON, _ := json.Marshal(fp.Languages)

	return fmt.Sprintf(`(function() {
  // Navigator overrides
  const fpNav = {
    userAgent: %q,
    platform: %q,
    vendor: %q,
    languages: %s,
    language: %s,
    hardwareConcurrency: %d,
    deviceMemory: %d,
    maxTouchPoints: %d,
    doNotTrack: %s,
  };
  for (const [k, v] of Object.entries(fpNav)) {
    try {
      Object.defineProperty(navigator, k, { get: () => v, configurable: true });
    } catch(e) {}
  }

  // Screen overrides
  const fpScreen = {
    width: %d, height: %d, availWidth: %d, availHeight: %d,
    colorDepth: %d, pixelDepth: %d,
  };
  for (const [k, v] of Object.entries(fpScreen)) {
    try {
      Object.defineProperty(screen, k, { get: () => v, configurable: true });
    } catch(e) {}
  }

  // Device pixel ratio
  try {
    Object.defineProperty(window, 'devicePixelRatio', { get: () => %g, configurable: true });
  } catch(e) {}

  // Timezone via Intl.DateTimeFormat
  const origDTF = Intl.DateTimeFormat;
  const fpTZ = %q;
  Intl.DateTimeFormat = function(loc, opts) {
    opts = Object.assign({}, opts, { timeZone: fpTZ });
    return new origDTF(loc, opts);
  };
  Intl.DateTimeFormat.prototype = origDTF.prototype;
  Intl.DateTimeFormat.supportedLocalesOf = origDTF.supportedLocalesOf;

  // WebGL vendor/renderer override
  const fpWebGL = { vendor: %q, renderer: %q };
  function patchGL(proto) {
    const orig = proto.getParameter;
    proto.getParameter = function(param) {
      const ext = this.getExtension('WEBGL_debug_renderer_info');
      if (ext) {
        if (param === ext.UNMASKED_VENDOR_WEBGL) return fpWebGL.vendor;
        if (param === ext.UNMASKED_RENDERER_WEBGL) return fpWebGL.renderer;
      }
      return orig.call(this, param);
    };
  }
  patchGL(WebGLRenderingContext.prototype);
  if (typeof WebGL2RenderingContext !== 'undefined') {
    patchGL(WebGL2RenderingContext.prototype);
  }
})();`,
		fp.UserAgent, fp.Platform, fp.Vendor,
		string(langsJSON),
		quotedOrNull(firstOrEmpty(fp.Languages)),
		fp.HardwareConcurrency, fp.DeviceMemory, fp.MaxTouchPoints,
		quotedOrNull(fp.DoNotTrack),
		fp.ScreenWidth, fp.ScreenHeight, fp.ScreenWidth, fp.ScreenHeight,
		fp.ColorDepth, fp.ColorDepth,
		fp.PixelRatio,
		fp.Timezone,
		fp.WebGLVendor, fp.WebGLRenderer,
	)
}

// ToProfile converts the fingerprint to a UserProfile for persistence.
func (fp *Fingerprint) ToProfile() *UserProfile {
	now := time.Now()

	lang := ""
	if len(fp.Languages) > 0 {
		lang = fp.Languages[0]
	}

	return &UserProfile{
		Version:   1,
		Name:      "fingerprint-" + now.Format("20060102-150405"),
		CreatedAt: now,
		UpdatedAt: now,
		Browser: ProfileBrowser{
			WindowW: fp.ScreenWidth,
			WindowH: fp.ScreenHeight,
		},
		Identity: ProfileIdentity{
			UserAgent: fp.UserAgent,
			Language:  lang,
			Timezone:  fp.Timezone,
		},
	}
}

// JSON returns the fingerprint as a JSON string.
func (fp *Fingerprint) JSON() (string, error) {
	data, err := json.MarshalIndent(fp, "", "  ")
	if err != nil {
		return "", fmt.Errorf("scout: fingerprint: marshal: %w", err)
	}

	return string(data), nil
}

// newCryptoSeededRand creates a math/rand.Rand seeded from crypto/rand.
func newCryptoSeededRand() *mrand.Rand {
	var seed int64
	if b, err := rand.Int(rand.Reader, big.NewInt(1<<62)); err == nil {
		seed = b.Int64()
	} else {
		// Fallback: read 8 bytes directly.
		var buf [8]byte

		_, _ = rand.Read(buf[:])
		seed = int64(binary.LittleEndian.Uint64(buf[:]))
	}

	return mrand.New(mrand.NewSource(seed)) //nolint:gosec
}

// pick selects a random element from a slice.
func pick[T any](rng *mrand.Rand, items []T) T {
	return items[rng.Intn(len(items))]
}

func quotedOrNull(s string) string {
	if s == "" {
		return "null"
	}

	return fmt.Sprintf("%q", s)
}

func firstOrEmpty(ss []string) string {
	if len(ss) == 0 {
		return ""
	}

	return ss[0]
}
