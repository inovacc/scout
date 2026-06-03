package engine

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"slices"
	"sync"
	"time"

	"github.com/inovacc/scout/internal/engine/lib/js"
	proto2 "github.com/inovacc/scout/internal/engine/lib/proto"
	utils2 "github.com/inovacc/scout/internal/engine/lib/utils"
	"github.com/ysmood/goob"
	"github.com/ysmood/got/lib/lcs"
	"github.com/ysmood/gson"
)

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
