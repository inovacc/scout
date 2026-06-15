package engine

import (
	"sort"
	"strings"
)

// Device is a viewport + user-agent + scale-factor preset for emulating a real
// device, mirroring Playwright's well-known device descriptors. Apply at launch
// with WithDevice(name) or at runtime with Page.EmulateDevice(name).
type Device struct {
	UserAgent         string
	Width             int
	Height            int
	DeviceScaleFactor float64
	IsMobile          bool
	HasTouch          bool
}

// Devices maps a human-readable device name to its emulation profile. Values
// follow the well-known Playwright/Chromium device descriptors (UA strings are
// representative — pin exact strings per target site if a server is UA-strict).
//
//nolint:gochecknoglobals // a read-only descriptor table, by design
var Devices = map[string]Device{
	"Desktop Chrome": {
		UserAgent:         "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
		Width:             1280, Height: 720, DeviceScaleFactor: 1,
	},
	"Desktop Safari": {
		UserAgent:         "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
		Width:             1280, Height: 720, DeviceScaleFactor: 2,
	},
	"iPhone 13": {
		UserAgent:         "Mozilla/5.0 (iPhone; CPU iPhone OS 15_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.0 Mobile/15E148 Safari/604.1",
		Width:             390, Height: 664, DeviceScaleFactor: 3, IsMobile: true, HasTouch: true,
	},
	"iPhone SE": {
		UserAgent:         "Mozilla/5.0 (iPhone; CPU iPhone OS 14_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0 Mobile/15E148 Safari/604.1",
		Width:             375, Height: 667, DeviceScaleFactor: 2, IsMobile: true, HasTouch: true,
	},
	"Pixel 5": {
		UserAgent:         "Mozilla/5.0 (Linux; Android 11; Pixel 5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Mobile Safari/537.36",
		Width:             393, Height: 727, DeviceScaleFactor: 2.75, IsMobile: true, HasTouch: true,
	},
	"Galaxy S9+": {
		UserAgent:         "Mozilla/5.0 (Linux; Android 8.0.0; SM-G965F) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Mobile Safari/537.36",
		Width:             320, Height: 658, DeviceScaleFactor: 4.5, IsMobile: true, HasTouch: true,
	},
	"iPad (gen 7)": {
		UserAgent:         "Mozilla/5.0 (iPad; CPU OS 14_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0 Mobile/15E148 Safari/604.1",
		Width:             810, Height: 1080, DeviceScaleFactor: 2, IsMobile: true, HasTouch: true,
	},
}

// DeviceByName returns a device preset by name (case-insensitive) and whether it
// was found.
func DeviceByName(name string) (Device, bool) {
	if d, ok := Devices[name]; ok {
		return d, true
	}
	for k, d := range Devices {
		if strings.EqualFold(k, name) {
			return d, true
		}
	}
	return Device{}, false
}

// DeviceNames returns the available device preset names, sorted.
func DeviceNames() []string {
	names := make([]string, 0, len(Devices))
	for k := range Devices {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
