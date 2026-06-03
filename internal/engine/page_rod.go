package engine

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/inovacc/scout/internal/engine/lib/cdp"
	devices2 "github.com/inovacc/scout/internal/engine/lib/devices"
	proto2 "github.com/inovacc/scout/internal/engine/lib/proto"
	utils2 "github.com/inovacc/scout/internal/engine/lib/utils"
	"github.com/ysmood/goob"
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

// Activate (focuses) the page.
func (p *rodPage) Activate() (*rodPage, error) {
	err := proto2.TargetActivateTarget{TargetID: p.TargetID}.Call(p.browser.Context(p.ctx))
	return p, err
}

// SetDocumentContent sets the page document html content.
func (p *rodPage) SetDocumentContent(html string) error {
	return proto2.PageSetDocumentContent{
		FrameID: p.FrameID,
		HTML:    html,
	}.Call(p)
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
			// upstream limitation: chromium CDP rejects PageClose while navigation is in progress;
			// retry until navigation completes.
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

// Release the remote object. Usually, you don't need to call it.
// When a page is closed or reloaded, all remote objects will be released automatically.
// It's useful if the page never closes or reloads.
func (p *rodPage) Release(obj *proto2.RuntimeRemoteObject) error {
	err := proto2.RuntimeReleaseObject{ObjectID: obj.ObjectID}.Call(p)
	return err
}
