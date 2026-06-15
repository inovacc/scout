package engine

import (
	"fmt"

	proto2 "github.com/inovacc/scout/internal/engine/lib/proto"
)

// GeolocationCfg is a geolocation override (degrees + accuracy in metres).
type GeolocationCfg struct {
	Latitude  float64
	Longitude float64
	Accuracy  float64
}

// NetworkCfg overrides network conditions — offline mode or a throughput throttle.
type NetworkCfg struct {
	Offline            bool
	LatencyMs          float64
	DownloadThroughput float64 // bytes/sec; 0 = unlimited
	UploadThroughput   float64 // bytes/sec; 0 = unlimited
}

// ---- launch-time options (auto-exported to the facade) ----

// WithDevice applies a device preset (see Devices / DeviceNames): viewport,
// user-agent, device-scale-factor, mobile flag, and touch. Unknown names no-op.
func WithDevice(name string) Option {
	return func(o *options) {
		d, ok := DeviceByName(name)
		if !ok {
			return
		}
		o.windowW, o.windowH = d.Width, d.Height
		o.userAgent = d.UserAgent
		o.deviceScaleFactor = d.DeviceScaleFactor
		o.isMobile = d.IsMobile
		o.touchEmulation = d.HasTouch
	}
}

// WithDeviceScaleFactor sets the device pixel ratio (e.g. 2.0 for retina).
func WithDeviceScaleFactor(dpr float64) Option {
	return func(o *options) { o.deviceScaleFactor = dpr }
}

// WithMobileEmulation toggles mobile viewport/UA emulation (also enables touch).
func WithMobileEmulation(enabled bool) Option {
	return func(o *options) {
		o.isMobile = enabled
		if enabled {
			o.touchEmulation = true
		}
	}
}

// WithLocale overrides the browser locale, e.g. "pt-BR".
func WithLocale(locale string) Option { return func(o *options) { o.locale = locale } }

// WithTimezone overrides the browser timezone, e.g. "America/Sao_Paulo".
func WithTimezone(tz string) Option { return func(o *options) { o.timezone = tz } }

// WithColorScheme emulates prefers-color-scheme ("light"|"dark"|"no-preference").
func WithColorScheme(scheme string) Option { return func(o *options) { o.colorScheme = scheme } }

// WithGeolocation sets a geolocation override and auto-grants the permission.
func WithGeolocation(lat, lng, accuracy float64) Option {
	return func(o *options) {
		o.geolocation = &GeolocationCfg{Latitude: lat, Longitude: lng, Accuracy: accuracy}
		o.grantGeolocation = true
	}
}

// WithOffline starts the page in offline mode.
func WithOffline(offline bool) Option {
	return func(o *options) {
		if o.networkConditions == nil {
			o.networkConditions = &NetworkCfg{}
		}
		o.networkConditions.Offline = offline
	}
}

// ---- runtime Page methods ----

// SetGeolocation overrides the page geolocation. Grant the "geolocation"
// permission first (or use WithGeolocation, which grants automatically).
func (p *Page) SetGeolocation(lat, lng, accuracy float64) error {
	if err := (proto2.EmulationSetGeolocationOverride{Latitude: &lat, Longitude: &lng, Accuracy: &accuracy}).Call(p.page); err != nil {
		return fmt.Errorf("scout: set geolocation: %w", err)
	}
	return nil
}

// ClearGeolocation removes any geolocation override.
func (p *Page) ClearGeolocation() error {
	if err := (proto2.EmulationSetGeolocationOverride{}).Call(p.page); err != nil {
		return fmt.Errorf("scout: clear geolocation: %w", err)
	}
	return nil
}

// SetTimezone overrides the page timezone, e.g. "Europe/London".
func (p *Page) SetTimezone(tz string) error {
	if err := (proto2.EmulationSetTimezoneOverride{TimezoneID: tz}).Call(p.page); err != nil {
		return fmt.Errorf("scout: set timezone: %w", err)
	}
	return nil
}

// SetLocale overrides the page locale, e.g. "pt-BR".
func (p *Page) SetLocale(locale string) error {
	if err := (proto2.EmulationSetLocaleOverride{Locale: locale}).Call(p.page); err != nil {
		return fmt.Errorf("scout: set locale: %w", err)
	}
	return nil
}

// SetColorScheme emulates prefers-color-scheme ("light"|"dark"|"no-preference").
func (p *Page) SetColorScheme(scheme string) error {
	if err := (proto2.EmulationSetEmulatedMedia{
		Features: []*proto2.EmulationMediaFeature{{Name: "prefers-color-scheme", Value: scheme}},
	}).Call(p.page); err != nil {
		return fmt.Errorf("scout: set color-scheme: %w", err)
	}
	return nil
}

// GrantPermissions grants browser permissions (e.g. "geolocation",
// "notifications", "camera", "clipboard-read") for an origin (empty = all).
func (p *Page) GrantPermissions(perms []string, origin string) error {
	pp := make([]proto2.BrowserPermissionType, 0, len(perms))
	for _, s := range perms {
		pp = append(pp, proto2.BrowserPermissionType(s))
	}
	cmd := proto2.BrowserGrantPermissions{Permissions: pp}
	if origin != "" {
		cmd.Origin = origin
	}
	if err := cmd.Call(p.page); err != nil {
		return fmt.Errorf("scout: grant permissions: %w", err)
	}
	return nil
}

// SetOffline toggles offline mode for the page.
func (p *Page) SetOffline(offline bool) error {
	if err := (proto2.NetworkEmulateNetworkConditions{Offline: offline, DownloadThroughput: -1, UploadThroughput: -1}).Call(p.page); err != nil {
		return fmt.Errorf("scout: set offline: %w", err)
	}
	return nil
}

// EmulateDevice applies a device preset (viewport + DPR + mobile + touch) at
// runtime. The user-agent is best set at launch via WithDevice.
func (p *Page) EmulateDevice(name string) error {
	d, ok := DeviceByName(name)
	if !ok {
		return fmt.Errorf("scout: emulate device: unknown device %q", name)
	}
	dpr := d.DeviceScaleFactor
	if dpr == 0 {
		dpr = 1
	}
	if err := (proto2.EmulationSetDeviceMetricsOverride{Width: d.Width, Height: d.Height, DeviceScaleFactor: dpr, Mobile: d.IsMobile}).Call(p.page); err != nil {
		return fmt.Errorf("scout: emulate device: metrics: %w", err)
	}
	if d.HasTouch {
		maxTP := 5
		if err := (proto2.EmulationSetTouchEmulationEnabled{Enabled: true, MaxTouchPoints: &maxTP}).Call(p.page); err != nil {
			return fmt.Errorf("scout: emulate device: touch: %w", err)
		}
	}
	return nil
}

// applyEmulationOptions applies any option-level emulation overrides; called by
// NewPage after the page is created.
func (p *Page) applyEmulationOptions(o *options) error {
	if o.isMobile || o.deviceScaleFactor > 0 {
		dpr := o.deviceScaleFactor
		if dpr == 0 {
			dpr = 1
		}
		if err := (proto2.EmulationSetDeviceMetricsOverride{Width: o.windowW, Height: o.windowH, DeviceScaleFactor: dpr, Mobile: o.isMobile}).Call(p.page); err != nil {
			return fmt.Errorf("scout: emulate metrics: %w", err)
		}
	}
	if o.locale != "" {
		if err := p.SetLocale(o.locale); err != nil {
			return err
		}
	}
	if o.timezone != "" {
		if err := p.SetTimezone(o.timezone); err != nil {
			return err
		}
	}
	if o.colorScheme != "" {
		if err := p.SetColorScheme(o.colorScheme); err != nil {
			return err
		}
	}
	if o.geolocation != nil {
		if o.grantGeolocation {
			_ = p.GrantPermissions([]string{"geolocation"}, "")
		}
		if err := p.SetGeolocation(o.geolocation.Latitude, o.geolocation.Longitude, o.geolocation.Accuracy); err != nil {
			return err
		}
	}
	if o.networkConditions != nil {
		dl, ul := -1.0, -1.0
		if o.networkConditions.DownloadThroughput > 0 {
			dl = o.networkConditions.DownloadThroughput
		}
		if o.networkConditions.UploadThroughput > 0 {
			ul = o.networkConditions.UploadThroughput
		}
		if err := (proto2.NetworkEmulateNetworkConditions{Offline: o.networkConditions.Offline, Latency: o.networkConditions.LatencyMs, DownloadThroughput: dl, UploadThroughput: ul}).Call(p.page); err != nil {
			return fmt.Errorf("scout: emulate network: %w", err)
		}
	}
	return nil
}
