package engine

import (
	"fmt"
	"time"

	proto2 "github.com/inovacc/scout/internal/engine/lib/proto"
)

// MobileConfig holds configuration for mobile browser automation.
type MobileConfig struct {
	DeviceID    string // ADB device serial (from "adb devices")
	ADBPath     string // path to adb binary (default: "adb")
	PackageName string // Android Chrome package (default: "com.android.chrome")
	CDPPort     int    // local port for CDP forwarding (default: 9222)
}

// WithMobile configures the browser for mobile device automation via ADB.
// It connects to an Android device's Chrome browser via CDP port forwarding.
func WithMobile(cfg MobileConfig) Option {
	return func(o *options) {
		o.mobile = &cfg
	}
}

// WithTouchEmulation enables touch event simulation without a physical device.
// Use with device emulation (WithDevice) for mobile testing on desktop.
func WithTouchEmulation() Option {
	return func(o *options) { o.touchEmulation = true }
}

// TouchPoint represents a single touch contact point.
type TouchPoint struct {
	X, Y  float64
	Force float64 // pressure 0.0-1.0
	ID    int
}

// Touch performs a tap gesture at the given coordinates.
func (p *Page) Touch(x, y float64) error {
	return p.TouchWithPoints([]TouchPoint{{X: x, Y: y, Force: 1.0}})
}

// TouchWithPoints dispatches a complete touch sequence (start -> end).
func (p *Page) TouchWithPoints(points []TouchPoint) error {
	touchPoints := make([]*proto2.InputTouchPoint, len(points))
	for i, pt := range points {
		force := pt.Force
		id := float64(pt.ID)
		touchPoints[i] = &proto2.InputTouchPoint{
			X:     pt.X,
			Y:     pt.Y,
			Force: &force,
			ID:    &id,
		}
	}

	// Touch start
	if err := (proto2.InputDispatchTouchEvent{
		Type:        proto2.InputDispatchTouchEventTypeTouchStart,
		TouchPoints: touchPoints,
	}).Call(p.page); err != nil {
		return fmt.Errorf("scout: touch start: %w", err)
	}

	// Touch end
	if err := (proto2.InputDispatchTouchEvent{
		Type:        proto2.InputDispatchTouchEventTypeTouchEnd,
		TouchPoints: []*proto2.InputTouchPoint{},
	}).Call(p.page); err != nil {
		return fmt.Errorf("scout: touch end: %w", err)
	}

	return nil
}

// Swipe performs a swipe gesture from (startX, startY) to (endX, endY).
func (p *Page) Swipe(startX, startY, endX, endY float64, duration time.Duration) error {
	steps := 10
	if duration < 100*time.Millisecond {
		steps = 5
	}

	id := float64(0)
	force := 1.0

	// Touch start at origin
	if err := (proto2.InputDispatchTouchEvent{
		Type: proto2.InputDispatchTouchEventTypeTouchStart,
		TouchPoints: []*proto2.InputTouchPoint{{
			X: startX, Y: startY, Force: &force, ID: &id,
		}},
	}).Call(p.page); err != nil {
		return fmt.Errorf("scout: swipe start: %w", err)
	}

	// Interpolate move events
	stepDelay := duration / time.Duration(steps)
	for i := 1; i <= steps; i++ {
		t := float64(i) / float64(steps)
		x := startX + (endX-startX)*t
		y := startY + (endY-startY)*t

		if err := (proto2.InputDispatchTouchEvent{
			Type: proto2.InputDispatchTouchEventTypeTouchMove,
			TouchPoints: []*proto2.InputTouchPoint{{
				X: x, Y: y, Force: &force, ID: &id,
			}},
		}).Call(p.page); err != nil {
			return fmt.Errorf("scout: swipe move: %w", err)
		}

		time.Sleep(stepDelay)
	}

	// Touch end
	if err := (proto2.InputDispatchTouchEvent{
		Type:        proto2.InputDispatchTouchEventTypeTouchEnd,
		TouchPoints: []*proto2.InputTouchPoint{},
	}).Call(p.page); err != nil {
		return fmt.Errorf("scout: swipe end: %w", err)
	}

	return nil
}

// PinchZoom performs a pinch zoom gesture centered at (cx, cy).
// scale > 1.0 zooms in, scale < 1.0 zooms out.
func (p *Page) PinchZoom(cx, cy, scale float64) error {
	// Calculate finger positions: two fingers moving apart (zoom in) or together (zoom out)
	startDist := 50.0 // starting distance from center
	endDist := startDist * scale

	id0 := float64(0)
	id1 := float64(1)
	force := 1.0
	steps := 10

	// Start with two fingers
	if err := (proto2.InputDispatchTouchEvent{
		Type: proto2.InputDispatchTouchEventTypeTouchStart,
		TouchPoints: []*proto2.InputTouchPoint{
			{X: cx - startDist, Y: cy, Force: &force, ID: &id0},
			{X: cx + startDist, Y: cy, Force: &force, ID: &id1},
		},
	}).Call(p.page); err != nil {
		return fmt.Errorf("scout: pinch start: %w", err)
	}

	// Move fingers
	for i := 1; i <= steps; i++ {
		t := float64(i) / float64(steps)
		dist := startDist + (endDist-startDist)*t

		if err := (proto2.InputDispatchTouchEvent{
			Type: proto2.InputDispatchTouchEventTypeTouchMove,
			TouchPoints: []*proto2.InputTouchPoint{
				{X: cx - dist, Y: cy, Force: &force, ID: &id0},
				{X: cx + dist, Y: cy, Force: &force, ID: &id1},
			},
		}).Call(p.page); err != nil {
			return fmt.Errorf("scout: pinch move: %w", err)
		}

		time.Sleep(16 * time.Millisecond) // ~60fps
	}

	// End
	if err := (proto2.InputDispatchTouchEvent{
		Type:        proto2.InputDispatchTouchEventTypeTouchEnd,
		TouchPoints: []*proto2.InputTouchPoint{},
	}).Call(p.page); err != nil {
		return fmt.Errorf("scout: pinch end: %w", err)
	}

	return nil
}
