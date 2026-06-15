// Hand-written Playwright-style DX re-exports.
// pkg/scout/scout.go is generated (DO NOT EDIT). Locators, web-first
// assertions, device presets, and emulation are surfaced here so the generated
// facade stays untouched. Every symbol is verified ABSENT from scout.go.
//
// Page methods (GetByRole/GetByText/.../SetTimezone/EmulateDevice/...) are
// available automatically via `type Page = engine.Page`; only the new types,
// constructors, and With* options need re-exporting.
package scout

import (
	"time"

	"github.com/inovacc/scout/internal/engine"
)

type (
	// Locator is a lazy, auto-waiting handle to page element(s).
	Locator = engine.Locator
	// LocatorOption tunes locator matching (WithExact / WithName).
	LocatorOption = engine.LocatorOption
	// LocatorAssertions are web-first retrying assertions on a Locator.
	LocatorAssertions = engine.LocatorAssertions
	// PageAssertions are web-first retrying assertions on a Page.
	PageAssertions = engine.PageAssertions
	// ExpectOption configures a web-first assertion (WithExpectTimeout).
	ExpectOption = engine.ExpectOption
	// Device is a viewport + UA + scale-factor emulation preset.
	Device = engine.Device
	// GeolocationCfg is a geolocation override.
	GeolocationCfg = engine.GeolocationCfg
	// NetworkCfg overrides network conditions (offline / throttle).
	NetworkCfg = engine.NetworkCfg
)

// Devices is the registry of device presets.
//
//nolint:gochecknoglobals // re-export of a read-only table
var Devices = engine.Devices

// DeviceByName returns a device preset (case-insensitive) and whether found.
func DeviceByName(name string) (Device, bool) { return engine.DeviceByName(name) }

// DeviceNames returns the available device preset names, sorted.
func DeviceNames() []string { return engine.DeviceNames() }

// ---- emulation launch options ----

func WithDevice(name string) Option                 { return engine.WithDevice(name) }
func WithDeviceScaleFactor(dpr float64) Option      { return engine.WithDeviceScaleFactor(dpr) }
func WithMobileEmulation(enabled bool) Option       { return engine.WithMobileEmulation(enabled) }
func WithLocale(locale string) Option               { return engine.WithLocale(locale) }
func WithTimezone(tz string) Option                 { return engine.WithTimezone(tz) }
func WithColorScheme(scheme string) Option          { return engine.WithColorScheme(scheme) }
func WithGeolocation(lat, lng, acc float64) Option  { return engine.WithGeolocation(lat, lng, acc) }
func WithOffline(offline bool) Option               { return engine.WithOffline(offline) }

// ---- locator + expect options/constructors ----

func WithExact(exact bool) LocatorOption            { return engine.WithExact(exact) }
func WithName(name string) LocatorOption            { return engine.WithName(name) }
func WithExpectTimeout(d time.Duration) ExpectOption { return engine.WithExpectTimeout(d) }

// Expect begins a web-first assertion chain on a locator.
func Expect(loc *Locator, opts ...ExpectOption) *LocatorAssertions { return engine.Expect(loc, opts...) }

// ExpectPage begins a web-first assertion chain on a page.
func ExpectPage(p *Page, opts ...ExpectOption) *PageAssertions { return engine.ExpectPage(p, opts...) }
