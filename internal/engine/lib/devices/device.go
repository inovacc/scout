// Package devices ...
package devices

import (
	proto2 "github.com/inovacc/scout/internal/engine/lib/proto"
	"github.com/ysmood/gson"
)

// Device represents a emulated device.
type Device struct {
	Capabilities   []string
	UserAgent      string
	AcceptLanguage string
	Screen         Screen
	Title          string

	landscape bool
	clear     bool
}

// Screen represents the screen of a device.
type Screen struct {
	DevicePixelRatio float64
	Horizontal       ScreenSize
	Vertical         ScreenSize
}

// ScreenSize represents the size of the screen.
type ScreenSize struct {
	Width  int
	Height int
}

// Landscape clones the device and set it to landscape mode.
func (device Device) Landscape() Device {
	d := device
	d.landscape = true

	return d
}

// MetricsEmulation config.
func (device Device) MetricsEmulation() *proto2.EmulationSetDeviceMetricsOverride {
	if device.IsClear() {
		return nil
	}

	var (
		screen      ScreenSize
		orientation *proto2.EmulationScreenOrientation
	)

	if device.landscape {
		screen = device.Screen.Horizontal
		orientation = &proto2.EmulationScreenOrientation{
			Angle: 90,
			Type:  proto2.EmulationScreenOrientationTypeLandscapePrimary,
		}
	} else {
		screen = device.Screen.Vertical
		orientation = &proto2.EmulationScreenOrientation{
			Angle: 0,
			Type:  proto2.EmulationScreenOrientationTypePortraitPrimary,
		}
	}

	return &proto2.EmulationSetDeviceMetricsOverride{
		Width:             screen.Width,
		Height:            screen.Height,
		DeviceScaleFactor: device.Screen.DevicePixelRatio,
		ScreenOrientation: orientation,
		Mobile:            has(device.Capabilities, "mobile"),
	}
}

// TouchEmulation config.
func (device Device) TouchEmulation() *proto2.EmulationSetTouchEmulationEnabled {
	if device.IsClear() {
		return &proto2.EmulationSetTouchEmulationEnabled{
			Enabled: false,
		}
	}

	return &proto2.EmulationSetTouchEmulationEnabled{
		Enabled:        has(device.Capabilities, "touch"),
		MaxTouchPoints: gson.Int(5),
	}
}

// UserAgentEmulation config.
func (device Device) UserAgentEmulation() *proto2.NetworkSetUserAgentOverride {
	if device.IsClear() {
		return nil
	}

	return &proto2.NetworkSetUserAgentOverride{
		UserAgent:      device.UserAgent,
		AcceptLanguage: device.AcceptLanguage,
	}
}

// IsClear type.
func (device Device) IsClear() bool {
	return device.clear
}
