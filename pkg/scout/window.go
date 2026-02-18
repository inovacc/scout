package scout

import (
	"fmt"

	"github.com/inovacc/scout/pkg/rod/lib/proto"
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

	bounds := &proto.BrowserBounds{
		WindowState: proto.BrowserWindowState(state),
	}

	// Chrome requires restoring to normal before changing to another non-normal state.
	// If transitioning between non-normal states, restore first.
	if state != WindowStateNormal {
		current, err := p.page.GetWindow()
		if err == nil && current.WindowState != "" && current.WindowState != proto.BrowserWindowStateNormal {
			if err := p.page.SetWindow(&proto.BrowserBounds{
				WindowState: proto.BrowserWindowStateNormal,
			}); err != nil {
				return fmt.Errorf("scout: restore window before %s: %w", state, err)
			}
		}
	}

	if err := p.page.SetWindow(bounds); err != nil {
		return fmt.Errorf("scout: set window state %s: %w", state, err)
	}

	return nil
}
