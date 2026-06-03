package engine

import (
	devices2 "github.com/inovacc/scout/internal/engine/lib/devices"
	proto2 "github.com/inovacc/scout/internal/engine/lib/proto"
)

// Navigate to the url. If the url is empty, "about:blank" will be used.
// It will return immediately after the server responds the http header.
func (p *rodPage) Navigate(url string) error {
	if url == "" {
		url = "about:blank"
	}

	// try to stop loading
	_ = p.StopLoading()

	res, err := proto2.PageNavigate{URL: url}.Call(p)
	if err != nil {
		return err
	}

	if res.ErrorText != "" {
		return &NavigationError{res.ErrorText}
	}

	p.root.unsetJSCtxID()

	return nil
}

// NavigateBack history.
func (p *rodPage) NavigateBack() error {
	// Not using cdp API because it doesn't work for iframe
	_, err := p.Evaluate(Eval(`() => history.back()`).ByUser())
	return err
}

// ResetNavigationHistory reset history.
func (p *rodPage) ResetNavigationHistory() error {
	err := proto2.PageResetNavigationHistory{}.Call(p)
	return err
}

// GetNavigationHistory get navigation history.
func (p *rodPage) GetNavigationHistory() (*proto2.PageGetNavigationHistoryResult, error) {
	return proto2.PageGetNavigationHistory{}.Call(p)
}

// NavigateForward history.
func (p *rodPage) NavigateForward() error {
	// Not using cdp API because it doesn't work for iframe
	_, err := p.Evaluate(Eval(`() => history.forward()`).ByUser())
	return err
}

// Reload page.
func (p *rodPage) Reload() error {
	p, cancel := p.WithCancel()
	defer cancel()

	wait := p.EachEvent(func(e *proto2.PageFrameNavigated) bool {
		return e.Frame.ID == p.FrameID
	})

	// Not using cdp API because it doesn't work for iframe
	_, err := p.Evaluate(Eval(`() => location.reload()`).ByUser())
	if err != nil {
		return err
	}

	wait()

	p.unsetJSCtxID()

	return nil
}

func (p *rodPage) getWindowID() (proto2.BrowserWindowID, error) {
	res, err := proto2.BrowserGetWindowForTarget{TargetID: p.TargetID}.Call(p)
	if err != nil {
		return 0, err
	}

	return res.WindowID, err
}

// GetWindow position and size info.
func (p *rodPage) GetWindow() (*proto2.BrowserBounds, error) {
	id, err := p.getWindowID()
	if err != nil {
		return nil, err
	}

	res, err := proto2.BrowserGetWindowBounds{WindowID: id}.Call(p)
	if err != nil {
		return nil, err
	}

	return res.Bounds, nil
}

// SetWindow location and size.
func (p *rodPage) SetWindow(bounds *proto2.BrowserBounds) error {
	id, err := p.getWindowID()
	if err != nil {
		return err
	}

	err = proto2.BrowserSetWindowBounds{WindowID: id, Bounds: bounds}.Call(p)

	return err
}

// SetViewport overrides the values of device screen dimensions.
func (p *rodPage) SetViewport(params *proto2.EmulationSetDeviceMetricsOverride) error {
	if params == nil {
		return proto2.EmulationClearDeviceMetricsOverride{}.Call(p)
	}

	return params.Call(p)
}

// Emulate the device, such as iPhone9. If device is devices.Clear, it will clear the override.
func (p *rodPage) Emulate(device devices2.Device) error {
	err := p.SetViewport(device.MetricsEmulation())
	if err != nil {
		return err
	}

	err = device.TouchEmulation().Call(p)
	if err != nil {
		return err
	}

	return p.SetUserAgent(device.UserAgentEmulation())
}
