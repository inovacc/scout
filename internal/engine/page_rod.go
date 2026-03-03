package engine

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/inovacc/scout/internal/engine/lib/cdp"
	devices2 "github.com/inovacc/scout/internal/engine/lib/devices"
	"github.com/inovacc/scout/internal/engine/lib/js"
	proto2 "github.com/inovacc/scout/internal/engine/lib/proto"
	utils2 "github.com/inovacc/scout/internal/engine/lib/utils"
	"github.com/ysmood/goob"
	"github.com/ysmood/got/lib/lcs"
	"github.com/ysmood/gson"
)

// Page implements these interfaces.
var (
	_ proto2.Client      = &rodPage{}
	_ proto2.Contextable = &rodPage{}
	_ proto2.Sessionable = &rodPage{}
)

// Page represents the webpage.
// We try to hold as less states as possible.
// When a page is closed by Rod or not all the ongoing operations an events on it will abort.
type rodPage struct {
	// TargetID is a unique ID for a remote page.
	// It's usually used in events sent from the browser to tell which page an event belongs to.
	TargetID proto2.TargetTargetID

	// FrameID is a unique ID for a browsing context.
	// Usually, different FrameID means different javascript execution context.
	// Such as an iframe and the page it belongs to will have the same TargetID but different FrameIDs.
	FrameID proto2.PageFrameID

	// SessionID is a unique ID for a page attachment to a controller.
	// It's usually used in transport layer to tell which page to send the control signal.
	// A page can attached to multiple controllers, the browser uses it distinguish controllers.
	SessionID proto2.TargetSessionID

	e eFunc

	ctx context.Context //nolint:containedctx // internalized rod pattern

	// Used to abort all ongoing actions when a page closes.
	sessionCancel func()

	root *rodPage

	sleeper func() utils2.Sleeper

	browser *rodBrowser
	event   *goob.Observable

	// devices
	Mouse    *Mouse
	Keyboard *Keyboard
	Touch    *Touch

	element *rodElement // iframe only

	jsCtxLock   *sync.Mutex
	jsCtxID     *proto2.RuntimeRemoteObjectID // use pointer so that page clones can share the change
	helpersLock *sync.Mutex
	helpers     map[proto2.RuntimeRemoteObjectID]map[string]proto2.RuntimeRemoteObjectID
}

// String interface.
func (p *rodPage) String() string {
	id := p.TargetID
	if len(id) > 8 {
		id = id[:8]
	}

	return fmt.Sprintf("<page:%s>", id)
}

// IsIframe tells if it's iframe.
func (p *rodPage) IsIframe() bool {
	return p.element != nil
}

// GetSessionID interface.
func (p *rodPage) GetSessionID() proto2.TargetSessionID {
	return p.SessionID
}

// Browser of the page.
func (p *rodPage) Browser() *rodBrowser {
	return p.browser
}

// Info of the page, such as the URL or title of the page.
func (p *rodPage) Info() (*proto2.TargetTargetInfo, error) {
	return p.browser.Context(p.ctx).pageInfo(p.TargetID)
}

// HTML of the page.
func (p *rodPage) HTML() (string, error) {
	el, err := p.Element("html")
	if err != nil {
		return "", err
	}

	return el.HTML()
}

// Cookies returns the page cookies. By default it will return the cookies for current page.
// The urls is the list of URLs for which applicable cookies will be fetched.
func (p *rodPage) Cookies(urls []string) ([]*proto2.NetworkCookie, error) {
	if len(urls) == 0 {
		info, err := p.Info()
		if err != nil {
			return nil, err
		}

		urls = []string{info.URL}
	}

	res, err := proto2.NetworkGetCookies{Urls: urls}.Call(p)
	if err != nil {
		return nil, err
	}

	return res.Cookies, nil
}

// SetCookies is similar to Browser.SetCookies .
func (p *rodPage) SetCookies(cookies []*proto2.NetworkCookieParam) error {
	if cookies == nil {
		return proto2.NetworkClearBrowserCookies{}.Call(p)
	}

	return proto2.NetworkSetCookies{Cookies: cookies}.Call(p)
}

// SetExtraHeaders whether to always send extra HTTP headers with the requests from this page.
func (p *rodPage) SetExtraHeaders(dict []string) (func(), error) {
	headers := proto2.NetworkHeaders{}

	for i := 0; i < len(dict); i += 2 {
		headers[dict[i]] = gson.New(dict[i+1])
	}

	return p.EnableDomain(&proto2.NetworkEnable{}), proto2.NetworkSetExtraHTTPHeaders{Headers: headers}.Call(p)
}

// SetUserAgent (browser brand, accept-language, etc) of the page.
// If req is nil, a default user agent will be used, a typical mac chrome.
func (p *rodPage) SetUserAgent(req *proto2.NetworkSetUserAgentOverride) error {
	if req == nil {
		req = devices2.LaptopWithMDPIScreen.UserAgentEmulation()
	}

	return req.Call(p)
}

// SetBlockedURLs For some requests that do not want to be triggered,
// such as some dangerous operations, delete, quit logout, etc.
// Wildcards ('*') are allowed, such as ["*/api/logout/*","delete"].
// NOTE: if you set empty pattern "", it will block all requests.
func (p *rodPage) SetBlockedURLs(urls []string) error {
	if len(urls) == 0 {
		return nil
	}

	return proto2.NetworkSetBlockedURLs{Urls: urls}.Call(p)
}

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

// Activate (focuses) the page.
func (p *rodPage) Activate() (*rodPage, error) {
	err := proto2.TargetActivateTarget{TargetID: p.TargetID}.Call(p.browser.Context(p.ctx))
	return p, err
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

// SetDocumentContent sets the page document html content.
func (p *rodPage) SetDocumentContent(html string) error {
	return proto2.PageSetDocumentContent{
		FrameID: p.FrameID,
		HTML:    html,
	}.Call(p)
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

// StopLoading forces the page stop navigation and pending resource fetches.
func (p *rodPage) StopLoading() error {
	return proto2.PageStopLoading{}.Call(p)
}

// Close tries to close page, running its beforeunload hooks, if has any.
func (p *rodPage) Close() error {
	p.browser.targetsLock.Lock()
	defer p.browser.targetsLock.Unlock()

	success := true

	ctx, cancel := context.WithCancel(p.ctx)
	defer cancel()

	messages := p.browser.Context(ctx).Event()

	for {
		err := proto2.PageClose{}.Call(p)
		if errors.Is(err, cdp.ErrNotAttachedToActivePage) {
			// TODO: I don't know why chromium doesn't allow us to close a page while it's navigating.
			// Looks like a bug in chromium.
			utils2.Sleep(0.1)
			continue
		} else if err != nil {
			return err
		}

		break
	}

	for msg := range messages {
		stop := false

		destroyed := proto2.TargetTargetDestroyed{}

		closed := proto2.PageJavascriptDialogClosed{}
		if msg.Load(&destroyed) {
			stop = destroyed.TargetID == p.TargetID
		} else if msg.SessionID == p.SessionID && msg.Load(&closed) {
			success = closed.Result
			stop = !success
		}

		if stop {
			break
		}
	}

	if success {
		p.cleanupStates()
	} else {
		return &PageCloseCanceledError{}
	}

	return nil
}

// TriggerFavicon supports when browser in headless mode
// to trigger favicon's request. Pay attention to this
// function only supported when browser in headless mode,
// if you call it in no-headless mode, it will raise an error
// with the message "browser is no-headless".
func (p *rodPage) TriggerFavicon() error {
	// check if browser whether in headless mode
	// if not in headless mode then raise error
	if !p.browser.Context(p.ctx).isHeadless() {
		return errors.New("browser is no-headless")
	}

	_, err := p.Evaluate(evalHelper(js.TriggerFavicon).ByPromise())
	if err != nil {
		return err
	}

	return nil
}

// HandleDialog accepts or dismisses next JavaScript initiated dialog (alert, confirm, prompt, or onbeforeunload).
// Because modal dialog will block js, usually you have to trigger the dialog in another goroutine.
// For example:
//
//	wait, handle := page.MustHandleDialog()
//	go page.MustElement("button").MustClick()
//	wait()
//	handle(true, "")
func (p *rodPage) HandleDialog() (
	wait func() *proto2.PageJavascriptDialogOpening,
	handle func(*proto2.PageHandleJavaScriptDialog) error,
) {
	restore := p.EnableDomain(&proto2.PageEnable{})

	var e proto2.PageJavascriptDialogOpening

	w := p.WaitEvent(&e)

	return func() *proto2.PageJavascriptDialogOpening {
			w()
			return &e
		}, func(h *proto2.PageHandleJavaScriptDialog) error {
			defer restore()
			return h.Call(p)
		}
}

// HandleFileDialog return a functions that waits for the next file chooser dialog pops up and returns the element
// for the event.
func (p *rodPage) HandleFileDialog() (func([]string) error, error) {
	err := proto2.PageSetInterceptFileChooserDialog{Enabled: true}.Call(p)
	if err != nil {
		return nil, err
	}

	var e proto2.PageFileChooserOpened

	w := p.WaitEvent(&e)

	return func(paths []string) error {
		w()

		err := proto2.PageSetInterceptFileChooserDialog{Enabled: false}.Call(p)
		if err != nil {
			return err
		}

		return proto2.DOMSetFileInputFiles{
			Files:         utils2.AbsolutePaths(paths),
			BackendNodeID: e.BackendNodeID,
		}.Call(p)
	}, nil
}

// Screenshot captures the screenshot of current page.
func (p *rodPage) Screenshot(fullPage bool, req *proto2.PageCaptureScreenshot) ([]byte, error) {
	if req == nil {
		req = &proto2.PageCaptureScreenshot{}
	}

	if fullPage {
		metrics, err := proto2.PageGetLayoutMetrics{}.Call(p)
		if err != nil {
			return nil, err
		}

		if metrics.CSSContentSize == nil {
			return nil, errors.New("failed to get css content size")
		}

		oldView := proto2.EmulationSetDeviceMetricsOverride{}
		set := p.LoadState(&oldView)
		view := oldView
		view.Width = int(metrics.CSSContentSize.Width)
		view.Height = int(metrics.CSSContentSize.Height)

		err = p.SetViewport(&view)
		if err != nil {
			return nil, err
		}

		defer func() { // try to recover the viewport
			if !set {
				_ = proto2.EmulationClearDeviceMetricsOverride{}.Call(p)
				return
			}

			_ = p.SetViewport(&oldView)
		}()
	}

	shot, err := req.Call(p)
	if err != nil {
		return nil, err
	}

	return shot.Data, nil
}

// ScrollScreenshotOptions is the options for the ScrollScreenshot.
type ScrollScreenshotOptions struct {
	// Format (optional) Image compression format (defaults to png).
	Format proto2.PageCaptureScreenshotFormat `json:"format,omitempty"`

	// Quality (optional) Compression quality from range [0..100] (jpeg only).
	Quality *int `json:"quality,omitempty"`

	// FixedTop (optional) The number of pixels to skip from the top.
	// It is suitable for optimizing the screenshot effect when there is a fixed
	// positioning element at the top of the page.
	FixedTop float64

	// FixedBottom (optional) The number of pixels to skip from the bottom.
	FixedBottom float64

	// WaitPerScroll until no animation (default is 300ms)
	WaitPerScroll time.Duration
}

// ScrollScreenshot Scroll screenshot does not adjust the size of the viewport,
// but achieves it by scrolling and capturing screenshots in a loop, and then stitching them together.
// Note that this method also has a flaw: when there are elements with fixed
// positioning on the page (usually header navigation components),
// these elements will appear repeatedly, you can set the FixedTop parameter to optimize it.
//
// Only support png and jpeg format yet, webP is not supported because no suitable processing
// library was found in golang.
func (p *rodPage) ScrollScreenshot(opt *ScrollScreenshotOptions) ([]byte, error) {
	if opt == nil {
		opt = &ScrollScreenshotOptions{}
	}

	if opt.WaitPerScroll == 0 {
		opt.WaitPerScroll = time.Millisecond * 300
	}

	metrics, err := proto2.PageGetLayoutMetrics{}.Call(p)
	if err != nil {
		return nil, err
	}

	if metrics.CSSContentSize == nil || metrics.CSSVisualViewport == nil {
		return nil, errors.New("failed to get css content size")
	}

	viewpointHeight := metrics.CSSVisualViewport.ClientHeight
	contentHeight := metrics.CSSContentSize.Height

	var (
		scrollTop float64
		images    []utils2.ImgWithBox
	)

	for {
		clip := &proto2.PageViewport{
			X:     0,
			Y:     scrollTop,
			Width: metrics.CSSVisualViewport.ClientWidth,
			Scale: 1,
		}

		scrollY := viewpointHeight - (opt.FixedTop + opt.FixedBottom)
		if scrollTop+viewpointHeight > contentHeight {
			clip.Height = contentHeight - scrollTop
		} else {
			clip.Height = scrollY
			if scrollTop != 0 {
				clip.Y += opt.FixedTop
			}
		}

		req := &proto2.PageCaptureScreenshot{
			Format:                opt.Format,
			Quality:               opt.Quality,
			Clip:                  clip,
			FromSurface:           false,
			CaptureBeyondViewport: false,
			OptimizeForSpeed:      false,
		}

		shot, err := req.Call(p)
		if err != nil {
			return nil, err
		}

		images = append(images, utils2.ImgWithBox{Img: shot.Data})

		scrollTop += scrollY
		if scrollTop >= contentHeight {
			break
		}

		err = p.Mouse.Scroll(0, scrollY, 1)
		if err != nil {
			return nil, fmt.Errorf("scroll error: %w", err)
		}

		err = p.WaitDOMStable(opt.WaitPerScroll, 0)
		if err != nil {
			return nil, fmt.Errorf("WaitDOMStable error: %w", err)
		}
	}

	var imgOption *utils2.ImgOption
	if opt.Quality != nil {
		imgOption = &utils2.ImgOption{
			Quality: *opt.Quality,
		}
	}

	bs, err := utils2.SplicePngVertical(images, opt.Format, imgOption)
	if err != nil {
		return nil, err
	}

	return bs, nil
}

// CaptureDOMSnapshot Returns a document snapshot, including the full DOM tree of the root node
// (including iframes, template contents, and imported documents) in a flattened array,
// as well as layout and white-listed computed style information for the nodes.
// Shadow DOM in the returned DOM tree is flattened.
// `Documents` The nodes in the DOM tree. The DOMNode at index 0 corresponds to the root document.
// `Strings` Shared string table that all string properties refer to with indexes.
// Normally use `Strings` is enough.
func (p *rodPage) CaptureDOMSnapshot() (domSnapshot *proto2.DOMSnapshotCaptureSnapshotResult, err error) {
	_ = proto2.DOMSnapshotEnable{}.Call(p)

	snapshot, err := proto2.DOMSnapshotCaptureSnapshot{
		ComputedStyles:                 []string{},
		IncludePaintOrder:              true,
		IncludeDOMRects:                true,
		IncludeBlendedBackgroundColors: true,
		IncludeTextColorOpacities:      true,
	}.Call(p)
	if err != nil {
		return nil, err
	}

	return snapshot, nil
}

// PDF prints page as PDF.
func (p *rodPage) PDF(req *proto2.PagePrintToPDF) (*StreamReader, error) {
	req.TransferMode = proto2.PagePrintToPDFTransferModeReturnAsStream

	res, err := req.Call(p)
	if err != nil {
		return nil, err
	}

	return NewStreamReader(p, res.Stream), nil
}

// GetResource content by the url. Such as image, css, html, etc.
// Use the [proto.PageGetResourceTree] to list all the resources.
func (p *rodPage) GetResource(url string) ([]byte, error) {
	res, err := proto2.PageGetResourceContent{
		FrameID: p.FrameID,
		URL:     url,
	}.Call(p)
	if err != nil {
		return nil, err
	}

	data := res.Content

	var bin []byte
	if res.Base64Encoded {
		bin, err = base64.StdEncoding.DecodeString(data)
		utils2.E(err)
	} else {
		bin = []byte(data)
	}

	return bin, nil
}

// WaitOpen waits for the next new page opened by the current one.
func (p *rodPage) WaitOpen() func() (*rodPage, error) {
	var targetID proto2.TargetTargetID

	b := p.browser.Context(p.ctx)
	wait := b.EachEvent(func(e *proto2.TargetTargetCreated) bool {
		targetID = e.TargetInfo.TargetID
		return e.TargetInfo.OpenerID == p.TargetID
	})

	return func() (*rodPage, error) {
		defer p.tryTrace(TraceTypeWait, "wait open")()

		wait()

		return b.PageFromTarget(targetID)
	}
}

// EachEvent of the specified event types, if any callback returns true the wait function will resolve,
// The type of each callback is (? means optional):
//
//	func(proto.Event, proto.TargetSessionID?) bool?
//
// You can listen to multiple event types at the same time like:
//
//	browser.EachEvent(func(a *proto.A) {}, func(b *proto.B) {})
//
// Such as subscribe the events to know when the navigation is complete or when the page is rendered.
// Here's an example to dismiss all dialogs/alerts on the page:
//
//	go page.EachEvent(func(e *proto.PageJavascriptDialogOpening) {
//	    _ = proto.PageHandleJavaScriptDialog{ Accept: false, PromptText: ""}.Call(page)
//	})()
func (p *rodPage) EachEvent(callbacks ...any) (wait func()) {
	return p.browser.Context(p.ctx).eachEvent(p.SessionID, callbacks...)
}

// WaitEvent waits for the next event for one time. It will also load the data into the event object.
func (p *rodPage) WaitEvent(e proto2.Event) (wait func()) {
	defer p.tryTrace(TraceTypeWait, "event", e.ProtoEvent())()
	return p.browser.Context(p.ctx).waitEvent(p.SessionID, e)
}

// WaitNavigation wait for a page lifecycle event when navigating.
// Usually you will wait for [proto.PageLifecycleEventNameNetworkAlmostIdle].
func (p *rodPage) WaitNavigation(name proto2.PageLifecycleEventName) func() {
	_ = proto2.PageSetLifecycleEventsEnabled{Enabled: true}.Call(p)

	wait := p.EachEvent(func(e *proto2.PageLifecycleEvent) bool {
		return e.Name == name
	})

	return func() {
		defer p.tryTrace(TraceTypeWait, "navigation", name)()

		wait()

		_ = proto2.PageSetLifecycleEventsEnabled{Enabled: false}.Call(p)
	}
}

// WaitRequestIdle returns a wait function that waits until no request for d duration.
// Be careful, d is not the max wait timeout, it's the least idle time.
// If you want to set a timeout you can use the [Page.Timeout] function.
// Use the includes and excludes regexp list to filter the requests by their url.
func (p *rodPage) WaitRequestIdle(
	d time.Duration,
	includes, excludes []string,
	excludeTypes []proto2.NetworkResourceType,
) func() {
	defer p.tryTrace(TraceTypeWait, "request-idle")()

	if excludeTypes == nil {
		excludeTypes = []proto2.NetworkResourceType{
			proto2.NetworkResourceTypeWebSocket,
			proto2.NetworkResourceTypeEventSource,
			proto2.NetworkResourceTypeMedia,
			proto2.NetworkResourceTypeImage,
			proto2.NetworkResourceTypeFont,
		}
	}

	if len(includes) == 0 {
		includes = []string{""}
	}

	p, cancel := p.WithCancel()
	match := genRegMatcher(includes, excludes)
	waitList := map[proto2.NetworkRequestID]string{}
	idleCounter := utils2.NewIdleCounter(d)
	update := p.tryTraceReq(includes, excludes)
	update(nil)

	checkDone := func(id proto2.NetworkRequestID) {
		if _, has := waitList[id]; has {
			delete(waitList, id)
			update(waitList)
			idleCounter.Done()
		}
	}

	wait := p.EachEvent(func(sent *proto2.NetworkRequestWillBeSent) {
		if slices.Contains(excludeTypes, sent.Type) {
			return
		}

		if match(sent.Request.URL) {
			// Redirect will send multiple NetworkRequestWillBeSent events with the same RequestID,
			// we should filter them out.
			if _, has := waitList[sent.RequestID]; !has {
				waitList[sent.RequestID] = sent.Request.URL
				update(waitList)
				idleCounter.Add()
			}
		}
	}, func(e *proto2.NetworkLoadingFinished) {
		checkDone(e.RequestID)
	}, func(e *proto2.NetworkLoadingFailed) {
		checkDone(e.RequestID)
	})

	return func() {
		go func() {
			idleCounter.Wait(p.ctx)
			cancel()
		}()

		wait()
	}
}

// WaitDOMStable waits until the change of the DOM tree is less or equal than diff percent for d duration.
// Be careful, d is not the max wait timeout, it's the least stable time.
// If you want to set a timeout you can use the [Page.Timeout] function.
func (p *rodPage) WaitDOMStable(d time.Duration, diff float64) error {
	defer p.tryTrace(TraceTypeWait, "dom-stable")()

	domSnapshot, err := p.CaptureDOMSnapshot()
	if err != nil {
		return err
	}

	t := time.NewTicker(d)
	defer t.Stop()

	for {
		select {
		case <-t.C:
		case <-p.ctx.Done():
			return p.ctx.Err()
		}

		currentDomSnapshot, err := p.CaptureDOMSnapshot()
		if err != nil {
			return err
		}

		xs := lcs.NewWords(domSnapshot.Strings)
		ys := lcs.NewWords(currentDomSnapshot.Strings)
		lcs := xs.YadLCS(p.ctx, ys)

		df := 1 - float64(len(lcs))/float64(len(ys))
		if df <= diff {
			break
		}

		domSnapshot = currentDomSnapshot
	}

	return nil
}

// WaitStable waits until the page is stable for d duration.
func (p *rodPage) WaitStable(d time.Duration) error {
	defer p.tryTrace(TraceTypeWait, "stable")()

	var err error

	setErr := sync.Once{}

	utils2.All(func() {
		e := p.WaitLoad()

		setErr.Do(func() { err = e })
	}, func() {
		p.WaitRequestIdle(d, nil, nil, nil)()
	}, func() {
		e := p.WaitDOMStable(d, 0)

		setErr.Do(func() { err = e })
	})()

	return err
}

// WaitIdle waits until the next window.requestIdleCallback is called.
func (p *rodPage) WaitIdle(timeout time.Duration) (err error) {
	_, err = p.Evaluate(evalHelper(js.WaitIdle, timeout.Milliseconds()).ByPromise())
	return err
}

// WaitRepaint waits until the next repaint.
// Doc: https://developer.mozilla.org/en-US/docs/Web/API/window/requestAnimationFrame
func (p *rodPage) WaitRepaint() error {
	// we use root here because iframe doesn't trigger requestAnimationFrame
	_, err := p.root.Eval(`() => new Promise(r => requestAnimationFrame(r))`)
	return err
}

// WaitLoad waits for the `window.onload` event, it returns immediately if the event is already fired.
func (p *rodPage) WaitLoad() error {
	defer p.tryTrace(TraceTypeWait, "load")()

	_, err := p.Evaluate(evalHelper(js.WaitLoad).ByPromise())

	return err
}

// AddScriptTag to page. If url is empty, content will be used.
func (p *rodPage) AddScriptTag(url, content string) error {
	hash := md5.Sum([]byte(url + content))
	id := hex.EncodeToString(hash[:])
	_, err := p.Evaluate(evalHelper(js.AddScriptTag, id, url, content).ByPromise())

	return err
}

// AddStyleTag to page. If url is empty, content will be used.
func (p *rodPage) AddStyleTag(url, content string) error {
	hash := md5.Sum([]byte(url + content))
	id := hex.EncodeToString(hash[:])
	_, err := p.Evaluate(evalHelper(js.AddStyleTag, id, url, content).ByPromise())

	return err
}

// EvalOnNewDocument Evaluates given script in every frame upon creation (before loading frame's scripts).
func (p *rodPage) EvalOnNewDocument(js string) (remove func() error, err error) {
	res, err := proto2.PageAddScriptToEvaluateOnNewDocument{Source: js}.Call(p)
	if err != nil {
		return
	}

	remove = func() error {
		return proto2.PageRemoveScriptToEvaluateOnNewDocument{
			Identifier: res.Identifier,
		}.Call(p)
	}

	return
}

// Wait until the js returns true.
func (p *rodPage) Wait(opts *EvalOptions) error {
	return utils2.Retry(p.ctx, p.sleeper(), func() (bool, error) {
		res, err := p.Evaluate(opts)
		if err != nil {
			return true, err
		}

		return res.Value.Bool(), nil
	})
}

// WaitElementsMoreThan waits until there are more than num elements that match the selector.
func (p *rodPage) WaitElementsMoreThan(selector string, num int) error {
	return p.Wait(Eval(`(s, n) => document.querySelectorAll(s).length > n`, selector, num))
}

// ObjectToJSON by object id.
func (p *rodPage) ObjectToJSON(obj *proto2.RuntimeRemoteObject) (gson.JSON, error) {
	if obj.ObjectID == "" {
		return obj.Value, nil
	}

	res, err := proto2.RuntimeCallFunctionOn{
		ObjectID:            obj.ObjectID,
		FunctionDeclaration: `function() { return this }`,
		ReturnByValue:       true,
	}.Call(p)
	if err != nil {
		return gson.New(nil), err
	}

	return res.Result.Value, nil
}

// ElementFromObject creates an Element from the remote object id.
func (p *rodPage) ElementFromObject(obj *proto2.RuntimeRemoteObject) (*rodElement, error) {
	// If the element is in an iframe, we need the jsCtxID to inject helper.js to the correct context.
	id, err := p.jsCtxIDByObjectID(obj.ObjectID)
	if err != nil {
		return nil, err
	}

	pid, err := p.getJSCtxID()
	if err != nil {
		return nil, err
	}

	if id != pid {
		clone := *p
		clone.jsCtxID = &id
		p = &clone
	}

	return &rodElement{
		e:       p.e,
		ctx:     p.ctx,
		sleeper: p.sleeper,
		page:    p,
		Object:  obj,
	}, nil
}

// ElementFromNode creates an Element from the node, [proto.DOMNodeID] or [proto.DOMBackendNodeID] must be specified.
func (p *rodPage) ElementFromNode(node *proto2.DOMNode) (*rodElement, error) {
	res, err := proto2.DOMResolveNode{
		NodeID:        node.NodeID,
		BackendNodeID: node.BackendNodeID,
	}.Call(p)
	if err != nil {
		return nil, err
	}

	el, err := p.ElementFromObject(res.Object)
	if err != nil {
		return nil, err
	}

	// make sure always return an element node
	desc, err := el.Describe(0, false)
	if err != nil {
		return nil, err
	}

	if desc.NodeName == "#text" {
		el, err = el.Parent()
		if err != nil {
			return nil, err
		}
	}

	return el, nil
}

// ElementFromPoint creates an Element from the absolute point on the page.
// The point should include the window scroll offset.
func (p *rodPage) ElementFromPoint(x, y int) (*rodElement, error) {
	node, err := proto2.DOMGetNodeForLocation{X: x, Y: y}.Call(p)
	if err != nil {
		return nil, err
	}

	return p.ElementFromNode(&proto2.DOMNode{
		BackendNodeID: node.BackendNodeID,
	})
}

// Release the remote object. Usually, you don't need to call it.
// When a page is closed or reloaded, all remote objects will be released automatically.
// It's useful if the page never closes or reloads.
func (p *rodPage) Release(obj *proto2.RuntimeRemoteObject) error {
	err := proto2.RuntimeReleaseObject{ObjectID: obj.ObjectID}.Call(p)
	return err
}

// Call implements the [proto.Client].
func (p *rodPage) Call(ctx context.Context, sessionID, methodName string, params any) (res []byte, err error) {
	return p.browser.Call(ctx, sessionID, methodName, params)
}

// Event of the page.
func (p *rodPage) Event() <-chan *Message {
	dst := make(chan *Message)
	s := p.event.Subscribe(p.ctx)

	go func() {
		defer close(dst)

		for {
			select {
			case <-p.ctx.Done():
				return
			case msg, ok := <-s:
				if !ok {
					return
				}

				select {
				case <-p.ctx.Done():
					return
				case dst <- msg.(*Message): //nolint: forcetypeassert
				}
			}
		}
	}()

	return dst
}

func (p *rodPage) initEvents() {
	p.event = goob.New(p.ctx)
	event := p.browser.Context(p.ctx).Event()

	go func() {
		for msg := range event {
			detached := proto2.TargetDetachedFromTarget{}
			destroyed := proto2.TargetTargetDestroyed{}

			if (msg.Load(&detached) && detached.SessionID == p.SessionID) ||
				(msg.Load(destroyed) && destroyed.TargetID == p.TargetID) {
				p.sessionCancel()
				return
			}

			if msg.SessionID != p.SessionID {
				continue
			}

			p.event.Publish(msg)
		}
	}()
}
