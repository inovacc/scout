package engine

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/inovacc/scout/internal/engine/lib/cdp"
	rodDefaults "github.com/inovacc/scout/internal/engine/lib/defaults"
	devices2 "github.com/inovacc/scout/internal/engine/lib/devices"
	launcher2 "github.com/inovacc/scout/internal/engine/lib/launcher"
	proto2 "github.com/inovacc/scout/internal/engine/lib/proto"
	utils2 "github.com/inovacc/scout/internal/engine/lib/utils"
	"github.com/ysmood/goob"
)

// Browser implements these interfaces.
var (
	_ proto2.Client      = &rodBrowser{}
	_ proto2.Contextable = &rodBrowser{}
)

// Browser represents the browser.
// It doesn't depends on file system, it should work with remote browser seamlessly.
// To check the env var you can use to quickly enable options from CLI, check here:
// https://pkg.go.dev/github.com/inovacc/scout/pkg/rod/lib/defaults
type rodBrowser struct {
	// BrowserContextID is the id for incognito window
	BrowserContextID proto2.BrowserBrowserContextID

	e eFunc

	ctx context.Context //nolint:containedctx // internalized rod pattern

	sleeper func() utils2.Sleeper

	logger utils2.Logger

	slowMotion time.Duration // see rodDefaults.slow
	trace      bool          // see rodDefaults.Trace
	monitor    string

	defaultDevice devices2.Device

	controlURL  string
	client      CDPClient
	event       *goob.Observable // all the browser events from cdp client
	targetsLock *sync.Mutex

	// stores all the previous cdp call of same type. Browser doesn't have enough API
	// for us to retrieve all its internal states. This is an workaround to map them to local.
	// For example you can't use cdp API to get the current position of mouse.
	states *sync.Map
}

// New creates a controller.
// DefaultDevice to emulate is set to [devices.LaptopWithMDPIScreen].Landscape(), it will change the default
// user-agent and can make the actual view area smaller than the browser window on headful mode,
// you can use [Browser.NoDefaultDevice] to disable it.
func newRodBrowser() *rodBrowser {
	return (&rodBrowser{
		ctx:           context.Background(),
		sleeper:       DefaultSleeper,
		controlURL:    rodDefaults.URL,
		slowMotion:    rodDefaults.Slow,
		trace:         rodDefaults.Trace,
		monitor:       rodDefaults.Monitor,
		logger:        DefaultLogger,
		defaultDevice: devices2.LaptopWithMDPIScreen.Landscape(),
		targetsLock:   &sync.Mutex{},
		states:        &sync.Map{},
	}).WithPanic(utils2.Panic)
}

// Incognito creates a new incognito browser.
func (b *rodBrowser) Incognito() (*rodBrowser, error) {
	res, err := proto2.TargetCreateBrowserContext{}.Call(b)
	if err != nil {
		return nil, err
	}

	incognito := *b
	incognito.BrowserContextID = res.BrowserContextID

	return &incognito, nil
}

// ControlURL set the url to remote control browser.
func (b *rodBrowser) ControlURL(url string) *rodBrowser {
	b.controlURL = url
	return b
}

// SlowMotion set the delay for each control action, such as the simulation of the human inputs.
func (b *rodBrowser) SlowMotion(delay time.Duration) *rodBrowser {
	b.slowMotion = delay
	return b
}

// Trace enables/disables the visual tracing of the input actions on the page.
func (b *rodBrowser) Trace(enable bool) *rodBrowser {
	b.trace = enable
	return b
}

// Monitor address to listen if not empty. Shortcut for [Browser.ServeMonitor].
func (b *rodBrowser) Monitor(url string) *rodBrowser {
	b.monitor = url
	return b
}

// Logger overrides the default log functions for tracing.
func (b *rodBrowser) Logger(l utils2.Logger) *rodBrowser {
	b.logger = l
	return b
}

// Client set the cdp client.
func (b *rodBrowser) Client(c CDPClient) *rodBrowser {
	b.client = c
	return b
}

// DefaultDevice sets the default device for new page to emulate in the future.
// Default is [devices.LaptopWithMDPIScreen].
// Set it to [devices.Clear] to disable it.
func (b *rodBrowser) DefaultDevice(d devices2.Device) *rodBrowser {
	b.defaultDevice = d
	return b
}

// NoDefaultDevice is the same as [Browser.DefaultDevice](devices.Clear).
func (b *rodBrowser) NoDefaultDevice() *rodBrowser {
	return b.DefaultDevice(devices2.Clear)
}

// Connect to the browser and start to control it.
// If fails to connect, try to launch a local browser, if local browser not found try to download one.
func (b *rodBrowser) Connect() error {
	if b.client == nil {
		u := b.controlURL
		if u == "" {
			var err error

			u, err = launcher2.New().Context(b.ctx).Launch()
			if err != nil {
				return err
			}
		}

		c, err := cdp.StartWithURL(b.ctx, u, nil)
		if err != nil {
			return err
		}

		b.client = c
	} else if b.controlURL != "" {
		panic("Browser.Client and Browser.ControlURL can't be set at the same time")
	}

	b.initEvents()

	if b.monitor != "" {
		launcher2.Open(b.ServeMonitor(b.monitor))
	}

	return proto2.TargetSetDiscoverTargets{Discover: true}.Call(b)
}

// Close the browser.
func (b *rodBrowser) Close() error {
	if b.BrowserContextID == "" {
		return proto2.BrowserClose{}.Call(b)
	}

	return proto2.TargetDisposeBrowserContext{BrowserContextID: b.BrowserContextID}.Call(b)
}

// Page creates a new browser tab. If opts.URL is empty, the default target will be "about:blank".
func (b *rodBrowser) Page(opts proto2.TargetCreateTarget) (p *rodPage, err error) {
	req := opts
	req.BrowserContextID = b.BrowserContextID
	req.URL = "about:blank"

	target, err := req.Call(b)
	if err != nil {
		return nil, err
	}

	defer func() {
		// If Navigate or PageFromTarget fails we should close the target to prevent leak
		if err != nil {
			_, _ = proto2.TargetCloseTarget{TargetID: target.TargetID}.Call(b)
		}
	}()

	p, err = b.PageFromTarget(target.TargetID)
	if err != nil {
		return
	}

	if opts.URL == "" {
		return
	}

	err = p.Navigate(opts.URL)

	return
}

// Pages retrieves all visible pages.
func (b *rodBrowser) Pages() (Pages, error) {
	list, err := proto2.TargetGetTargets{}.Call(b)
	if err != nil {
		return nil, err
	}

	pageList := Pages{}

	for _, target := range list.TargetInfos {
		if target.Type != proto2.TargetTargetInfoTypePage {
			continue
		}

		page, err := b.PageFromTarget(target.TargetID)
		if err != nil {
			return nil, err
		}

		pageList = append(pageList, page)
	}

	return pageList, nil
}

// Call implements the [proto.Client] to call raw cdp interface directly.
func (b *rodBrowser) Call(ctx context.Context, sessionID, methodName string, params any) (res []byte, err error) {
	res, err = b.client.Call(ctx, sessionID, methodName, params)
	if err != nil {
		return nil, err
	}

	b.set(proto2.TargetSessionID(sessionID), methodName, params)

	return
}

// PageFromSession is used for low-level debugging.
func (b *rodBrowser) PageFromSession(sessionID proto2.TargetSessionID) *rodPage {
	sessionCtx, cancel := context.WithCancel(b.ctx)

	return &rodPage{
		e:             b.e,
		ctx:           sessionCtx,
		sessionCancel: cancel,
		sleeper:       b.sleeper,
		browser:       b,
		SessionID:     sessionID,
	}
}

// PageFromTarget gets or creates a Page instance.
func (b *rodBrowser) PageFromTarget(targetID proto2.TargetTargetID) (*rodPage, error) {
	b.targetsLock.Lock()
	defer b.targetsLock.Unlock()

	page := b.loadCachedPage(targetID)
	if page != nil {
		return page, nil
	}

	session, err := proto2.TargetAttachToTarget{
		TargetID: targetID,
		Flatten:  true, // if it's not set no response will return
	}.Call(b)
	if err != nil {
		return nil, err
	}

	sessionCtx, cancel := context.WithCancel(b.ctx)

	page = &rodPage{
		e:             b.e,
		ctx:           sessionCtx,
		sessionCancel: cancel,
		sleeper:       b.sleeper,
		browser:       b,
		TargetID:      targetID,
		SessionID:     session.SessionID,
		FrameID:       proto2.PageFrameID(targetID),
		jsCtxLock:     &sync.Mutex{},
		jsCtxID:       new(proto2.RuntimeRemoteObjectID),
		helpersLock:   &sync.Mutex{},
	}

	page.root = page
	page.newKeyboard().newMouse().newTouch()

	if !b.defaultDevice.IsClear() {
		err = page.Emulate(b.defaultDevice)
		if err != nil {
			return nil, err
		}
	}

	b.cachePage(page)

	page.initEvents()

	// If we don't enable it, it will cause a lot of unexpected browser behavior.
	// Such as proto.PageAddScriptToEvaluateOnNewDocument won't work.
	page.EnableDomain(&proto2.PageEnable{})

	return page, nil
}

// EachEvent is similar to [Page.EachEvent], but catches events of the entire browser.
func (b *rodBrowser) EachEvent(callbacks ...any) (wait func()) {
	return b.eachEvent("", callbacks...)
}

// WaitEvent waits for the next event for one time. It will also load the data into the event object.
func (b *rodBrowser) WaitEvent(e proto2.Event) (wait func()) {
	return b.waitEvent("", e)
}

// waits for the next event for one time. It will also load the data into the event object.
func (b *rodBrowser) waitEvent(sessionID proto2.TargetSessionID, e proto2.Event) (wait func()) {
	valE := reflect.ValueOf(e)
	valTrue := reflect.ValueOf(true)

	if valE.Kind() != reflect.Ptr {
		valE = reflect.New(valE.Type())
	}

	// dynamically creates a function on runtime:
	//
	// func(ee proto.Event) bool {
	//   *e = *ee
	//   return true
	// }
	fnType := reflect.FuncOf([]reflect.Type{valE.Type()}, []reflect.Type{valTrue.Type()}, false)
	fnVal := reflect.MakeFunc(fnType, func(args []reflect.Value) []reflect.Value {
		valE.Elem().Set(args[0].Elem())
		return []reflect.Value{valTrue}
	})

	return b.eachEvent(sessionID, fnVal.Interface())
}

// If the any callback returns true the event loop will stop.
// It will enable the related domains if not enabled, and restore them after wait ends.
func (b *rodBrowser) eachEvent(sessionID proto2.TargetSessionID, callbacks ...any) (wait func()) {
	cbMap := map[string]reflect.Value{}
	restores := []func(){}

	for _, cb := range callbacks {
		cbVal := reflect.ValueOf(cb)
		eType := cbVal.Type().In(0)
		name := reflect.New(eType.Elem()).Interface().(proto2.Event).ProtoEvent() //nolint: forcetypeassert
		cbMap[name] = cbVal

		// Only enabled domains will emit events to cdp client.
		// We enable the domains for the event types if it's not enabled.
		// We restore the domains to their previous states after the wait ends.
		domain, _ := proto2.ParseMethodName(name)
		if req := proto2.GetType(domain + ".enable"); req != nil {
			enable := reflect.New(req).Interface().(proto2.Request) //nolint: forcetypeassert
			restores = append(restores, b.EnableDomain(sessionID, enable))
		}
	}

	b, cancel := b.WithCancel()
	messages := b.Event()

	return func() {
		if messages == nil {
			panic("can't use wait function twice")
		}

		defer func() {
			cancel()

			messages = nil

			for _, restore := range restores {
				restore()
			}
		}()

		for msg := range messages {
			if sessionID != "" && msg.SessionID != sessionID {
				continue
			}

			if cbVal, has := cbMap[msg.Method]; has {
				e := reflect.New(proto2.GetType(msg.Method))
				msg.Load(e.Interface().(proto2.Event)) //nolint: forcetypeassert

				args := []reflect.Value{e}
				if cbVal.Type().NumIn() == 2 {
					args = append(args, reflect.ValueOf(msg.SessionID))
				}

				res := cbVal.Call(args)
				if len(res) > 0 {
					if res[0].Bool() {
						return
					}
				}
			}
		}
	}
}

// Event of the browser.
func (b *rodBrowser) Event() <-chan *Message {
	src := b.event.Subscribe(b.ctx)
	dst := make(chan *Message)

	go func() {
		defer close(dst)

		for {
			select {
			case <-b.ctx.Done():
				return
			case e, ok := <-src:
				if !ok {
					return
				}

				select {
				case <-b.ctx.Done():
					return
				case dst <- e.(*Message): //nolint: forcetypeassert
				}
			}
		}
	}()

	return dst
}

func (b *rodBrowser) initEvents() {
	ctx, cancel := context.WithCancel(b.ctx)
	b.event = goob.New(ctx)
	event := b.client.Event()

	go func() {
		defer cancel()

		for e := range event {
			b.event.Publish(&Message{
				SessionID: proto2.TargetSessionID(e.SessionID),
				Method:    e.Method,
				lock:      &sync.Mutex{},
				data:      e.Params,
			})
		}
	}()
}

func (b *rodBrowser) pageInfo(id proto2.TargetTargetID) (*proto2.TargetTargetInfo, error) {
	res, err := proto2.TargetGetTargetInfo{TargetID: id}.Call(b)
	if err != nil {
		return nil, err
	}

	return res.TargetInfo, nil
}

func (b *rodBrowser) isHeadless() (enabled bool) {
	res, _ := proto2.BrowserGetBrowserCommandLine{}.Call(b)
	for _, v := range res.Arguments {
		if strings.Contains(v, "headless") {
			return true
		}
	}

	return false
}

// IgnoreCertErrors switch. If enabled, all certificate errors will be ignored.
func (b *rodBrowser) IgnoreCertErrors(enable bool) error {
	return proto2.SecuritySetIgnoreCertificateErrors{Ignore: enable}.Call(b)
}

// GetCookies from the browser.
func (b *rodBrowser) GetCookies() ([]*proto2.NetworkCookie, error) {
	res, err := proto2.StorageGetCookies{BrowserContextID: b.BrowserContextID}.Call(b)
	if err != nil {
		return nil, err
	}

	return res.Cookies, nil
}

// SetCookies to the browser. If the cookies is nil it will clear all the cookies.
func (b *rodBrowser) SetCookies(cookies []*proto2.NetworkCookieParam) error {
	if cookies == nil {
		return proto2.StorageClearCookies{BrowserContextID: b.BrowserContextID}.Call(b)
	}

	return proto2.StorageSetCookies{
		Cookies:          cookies,
		BrowserContextID: b.BrowserContextID,
	}.Call(b)
}

// WaitDownload returns a helper to get the next download file.
// The file path will be:
//
//	filepath.Join(dir, info.GUID)
func (b *rodBrowser) WaitDownload(dir string) func() (info *proto2.PageDownloadWillBegin) { //nolint:staticcheck // internalized rod API uses deprecated CDP types
	var oldDownloadBehavior proto2.BrowserSetDownloadBehavior

	has := b.LoadState("", &oldDownloadBehavior)

	_ = proto2.BrowserSetDownloadBehavior{
		Behavior:         proto2.BrowserSetDownloadBehaviorBehaviorAllowAndName,
		BrowserContextID: b.BrowserContextID,
		DownloadPath:     dir,
	}.Call(b)

	var start *proto2.PageDownloadWillBegin //nolint:staticcheck // internalized rod API

	waitProgress := b.EachEvent(func(e *proto2.PageDownloadWillBegin) { //nolint:staticcheck // internalized rod API
		start = e
	}, func(e *proto2.PageDownloadProgress) bool { //nolint:staticcheck // internalized rod API
		return start != nil && start.GUID == e.GUID && e.State == proto2.PageDownloadProgressStateCompleted
	})

	return func() *proto2.PageDownloadWillBegin { //nolint:staticcheck // internalized rod API
		defer func() {
			if has {
				_ = oldDownloadBehavior.Call(b)
			} else {
				_ = proto2.BrowserSetDownloadBehavior{
					Behavior:         proto2.BrowserSetDownloadBehaviorBehaviorDefault,
					BrowserContextID: b.BrowserContextID,
				}.Call(b)
			}
		}()

		waitProgress()

		return start
	}
}

// Version info of the browser.
func (b *rodBrowser) Version() (*proto2.BrowserGetVersionResult, error) {
	return proto2.BrowserGetVersion{}.Call(b)
}
