package engine

import (
	"fmt"

	proto2 "github.com/inovacc/scout/internal/engine/lib/proto"
)

// WindowState represents the state of a browser window.
type WindowState string

const (
	WindowStateNormal     WindowState = "normal"
	WindowStateMinimized  WindowState = "minimized"
	WindowStateMaximized  WindowState = "maximized"
	WindowStateFullscreen WindowState = "fullscreen"
)

// WindowBounds holds the position, size, and state of a browser window.
type WindowBounds struct {
	Left   int
	Top    int
	Width  int
	Height int
	State  WindowState
}

// GetWindow returns the current browser window bounds and state.
func (p *Page) GetWindow() (*WindowBounds, error) {
	if p == nil || p.page == nil {
		return nil, fmt.Errorf("scout: page is nil")
	}

	bounds, err := p.page.GetWindow()
	if err != nil {
		return nil, fmt.Errorf("scout: get window: %w", err)
	}

	wb := &WindowBounds{
		State: WindowState(bounds.WindowState),
	}

	if bounds.Left != nil {
		wb.Left = *bounds.Left
	}

	if bounds.Top != nil {
		wb.Top = *bounds.Top
	}

	if bounds.Width != nil {
		wb.Width = *bounds.Width
	}

	if bounds.Height != nil {
		wb.Height = *bounds.Height
	}

	return wb, nil
}

// Minimize minimizes the browser window.
func (p *Page) Minimize() error {
	return p.setWindowState(WindowStateMinimized)
}

// Maximize maximizes the browser window.
func (p *Page) Maximize() error {
	return p.setWindowState(WindowStateMaximized)
}

// Fullscreen puts the browser window into fullscreen mode.
func (p *Page) Fullscreen() error {
	return p.setWindowState(WindowStateFullscreen)
}

// RestoreWindow restores the browser window to its normal state.
func (p *Page) RestoreWindow() error {
	return p.setWindowState(WindowStateNormal)
}

func (p *Page) setWindowState(state WindowState) error {
	if p == nil || p.page == nil {
		return fmt.Errorf("scout: page is nil")
	}

	bounds := &proto2.BrowserBounds{
		WindowState: proto2.BrowserWindowState(state),
	}

	// Chrome requires restoring to normal before changing to another non-normal state.
	// If transitioning between non-normal states, restore first.
	if state != WindowStateNormal {
		current, err := p.page.GetWindow()
		if err == nil && current.WindowState != "" && current.WindowState != proto2.BrowserWindowStateNormal {
			if err := p.page.SetWindow(&proto2.BrowserBounds{
				WindowState: proto2.BrowserWindowStateNormal,
			}); err != nil {
				return fmt.Errorf("scout: restore window before %s: %w", state, err)
			}
		}
	}

	if err := p.page.SetWindow(bounds); err != nil {
		return fmt.Errorf("scout: set window state %s: %w", state, err)
	}

	// After maximize/fullscreen, clear the viewport override so Chrome uses the
	// actual window dimensions. Without this, the initial SetViewport (e.g. 1920x1080)
	// pins the viewport and causes blank/white space in the rendered page.
	if state == WindowStateMaximized || state == WindowStateFullscreen {
		_ = proto2.EmulationClearDeviceMetricsOverride{}.Call(p.page)
	}

	return nil
}
