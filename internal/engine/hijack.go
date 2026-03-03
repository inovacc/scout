package engine

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	proto2 "github.com/inovacc/scout/internal/engine/lib/proto"
	"github.com/inovacc/scout/internal/engine/lib/utils"
	"github.com/ysmood/gson"
)

// HijackRequests same as Page.HijackRequests, but can intercept requests of the entire browser.
func (b *rodBrowser) HijackRequests() *rodHijackRouter {
	return newHijackRouter(b, b).initEvents()
}

// HijackRequests creates a new router instance for requests hijacking.
// When use Fetch domain outside the router should be stopped. Enabling hijacking disables page caching,
// but such as 304 Not Modified will still work as expected.
// The entire process of hijacking one request:
//
//	browser --req-> rod ---> server ---> rod --res-> browser
//
// The --req-> and --res-> are the parts that can be modified.
func (p *rodPage) HijackRequests() *rodHijackRouter {
	return newHijackRouter(p.browser, p).initEvents()
}

// HijackRouter context.
type rodHijackRouter struct {
	run      func()
	stop     func()
	handlers []*hijackHandler
	enable   *proto2.FetchEnable
	client   proto2.Client
	browser  *rodBrowser
}

func newHijackRouter(browser *rodBrowser, client proto2.Client) *rodHijackRouter {
	return &rodHijackRouter{
		enable:   &proto2.FetchEnable{},
		browser:  browser,
		client:   client,
		handlers: []*hijackHandler{},
	}
}

func (r *rodHijackRouter) initEvents() *rodHijackRouter { //nolint: gocognit
	ctx := r.browser.ctx
	if cta, ok := r.client.(proto2.Contextable); ok {
		ctx = cta.GetContext()
	}

	var sessionID proto2.TargetSessionID
	if tsa, ok := r.client.(proto2.Sessionable); ok {
		sessionID = tsa.GetSessionID()
	}

	eventCtx, cancel := context.WithCancel(ctx)
	r.stop = cancel

	_ = r.enable.Call(r.client)

	r.run = r.browser.Context(eventCtx).eachEvent(sessionID, func(e *proto2.FetchRequestPaused) bool {
		go func() {
			ctx := r.new(eventCtx, e)
			for _, h := range r.handlers {
				if !h.regexp.MatchString(e.Request.URL) {
					continue
				}

				h.handler(ctx)

				if ctx.continueRequest != nil {
					ctx.continueRequest.RequestID = e.RequestID

					err := ctx.continueRequest.Call(r.client) //nolint:contextcheck // internalized rod pattern
					if err != nil {
						ctx.OnError(err)
					}

					return
				}

				if ctx.Skip {
					continue
				}

				if ctx.Response.fail.ErrorReason != "" {
					err := ctx.Response.fail.Call(r.client) //nolint:contextcheck // internalized rod pattern
					if err != nil {
						ctx.OnError(err)
					}

					return
				}

				err := ctx.Response.payload.Call(r.client) //nolint:contextcheck // internalized rod pattern
				if err != nil {
					ctx.OnError(err)
					return
				}
			}
		}()

		return false
	})

	return r
}

// Add a hijack handler to router, the doc of the pattern is the same as "proto.FetchRequestPattern.URLPattern".
func (r *rodHijackRouter) Add(pattern string, resourceType proto2.NetworkResourceType, handler func(*Hijack)) error {
	r.enable.Patterns = append(r.enable.Patterns, &proto2.FetchRequestPattern{
		URLPattern:   pattern,
		ResourceType: resourceType,
	})

	reg := regexp.MustCompile(proto2.PatternToReg(pattern))

	r.handlers = append(r.handlers, &hijackHandler{
		pattern: pattern,
		regexp:  reg,
		handler: handler,
	})

	return r.enable.Call(r.client)
}

// Remove handler via the pattern.
func (r *rodHijackRouter) Remove(pattern string) error {
	patterns := []*proto2.FetchRequestPattern{}

	handlers := []*hijackHandler{}
	for _, h := range r.handlers {
		if h.pattern != pattern {
			patterns = append(patterns, &proto2.FetchRequestPattern{URLPattern: h.pattern})
			handlers = append(handlers, h)
		}
	}

	r.enable.Patterns = patterns
	r.handlers = handlers

	return r.enable.Call(r.client)
}

// new context.
func (r *rodHijackRouter) new(ctx context.Context, e *proto2.FetchRequestPaused) *Hijack {
	headers := http.Header{}
	for k, v := range e.Request.Headers {
		headers[k] = []string{v.String()}
	}

	u, _ := url.Parse(e.Request.URL)

	req := &http.Request{
		Method: e.Request.Method,
		URL:    u,
		Body:   io.NopCloser(strings.NewReader(e.Request.PostData)),
		Header: headers,
	}

	return &Hijack{
		Request: &rodHijackRequest{
			event: e,
			req:   req.WithContext(ctx),
		},
		Response: &rodHijackResponse{
			payload: &proto2.FetchFulfillRequest{
				ResponseCode: 200,
				RequestID:    e.RequestID,
			},
			fail: &proto2.FetchFailRequest{
				RequestID: e.RequestID,
			},
		},
		OnError: func(_ error) {},

		browser: r.browser,
	}
}

// Run the router, after you call it, you shouldn't add new handler to it.
func (r *rodHijackRouter) Run() {
	r.run()
}

// Stop the router.
func (r *rodHijackRouter) Stop() error {
	r.stop()
	return proto2.FetchDisable{}.Call(r.client)
}

// hijackHandler to handle each request that match the regexp.
type hijackHandler struct {
	pattern string
	regexp  *regexp.Regexp
	handler func(*Hijack)
}

// Hijack context.
type Hijack struct {
	Request  *rodHijackRequest
	Response *rodHijackResponse
	OnError  func(error)

	// Skip to next handler
	Skip bool

	continueRequest *proto2.FetchContinueRequest

	// CustomState is used to store things for this context
	CustomState any

	browser *rodBrowser
}

// ContinueRequest without hijacking. The RequestID will be set by the router, you don't have to set it.
func (h *Hijack) ContinueRequest(cq *proto2.FetchContinueRequest) {
	h.continueRequest = cq
}

// LoadResponse will send request to the real destination and load the response as default response to override.
func (h *Hijack) LoadResponse(client *http.Client, loadBody bool) error {
	res, err := client.Do(h.Request.req)
	if err != nil {
		return err
	}

	defer func() { _ = res.Body.Close() }()

	h.Response.payload.ResponseCode = res.StatusCode
	h.Response.RawResponse = res

	for k, vs := range res.Header {
		for _, v := range vs {
			h.Response.SetHeader(k, v)
		}
	}

	if loadBody {
		b, err := io.ReadAll(res.Body)
		if err != nil {
			return err
		}

		h.Response.payload.Body = b
	}

	return nil
}

// HijackRequest context.
type rodHijackRequest struct {
	event *proto2.FetchRequestPaused
	req   *http.Request
}

// Type of the resource.
func (ctx *rodHijackRequest) Type() proto2.NetworkResourceType {
	return ctx.event.ResourceType
}

// Method of the request.
func (ctx *rodHijackRequest) Method() string {
	return ctx.event.Request.Method
}

// URL of the request.
func (ctx *rodHijackRequest) URL() *url.URL {
	u, _ := url.Parse(ctx.event.Request.URL)
	return u
}

// Header via a key.
func (ctx *rodHijackRequest) Header(key string) string {
	return ctx.event.Request.Headers[key].String()
}

// Headers of request.
func (ctx *rodHijackRequest) Headers() proto2.NetworkHeaders {
	return ctx.event.Request.Headers
}

// Body of the request, devtools API doesn't support binary data yet, only string can be captured.
func (ctx *rodHijackRequest) Body() string {
	return ctx.event.Request.PostData
}

// JSONBody of the request.
func (ctx *rodHijackRequest) JSONBody() gson.JSON {
	return gson.NewFrom(ctx.Body())
}

// Req returns the underlying http.Request instance that will be used to send the request.
func (ctx *rodHijackRequest) Req() *http.Request {
	return ctx.req
}

// SetContext of the underlying http.Request instance.
func (ctx *rodHijackRequest) SetContext(c context.Context) *rodHijackRequest {
	ctx.req = ctx.req.WithContext(c)
	return ctx
}

// SetBody of the request, if obj is []byte or string, raw body will be used, else it will be encoded as json.
func (ctx *rodHijackRequest) SetBody(obj any) *rodHijackRequest {
	var b []byte

	switch body := obj.(type) {
	case []byte:
		b = body
	case string:
		b = []byte(body)
	default:
		b = utils.MustToJSONBytes(body)
	}

	ctx.req.Body = io.NopCloser(bytes.NewBuffer(b))

	return ctx
}

// IsNavigation determines whether the request is a navigation request.
func (ctx *rodHijackRequest) IsNavigation() bool {
	return ctx.Type() == proto2.NetworkResourceTypeDocument
}

// HijackResponse context.
type rodHijackResponse struct {
	payload     *proto2.FetchFulfillRequest
	RawResponse *http.Response
	fail        *proto2.FetchFailRequest
}

// Payload to respond the request from the browser.
func (ctx *rodHijackResponse) Payload() *proto2.FetchFulfillRequest {
	return ctx.payload
}

// Body of the payload.
func (ctx *rodHijackResponse) Body() string {
	return string(ctx.payload.Body)
}

// Headers returns the clone of response headers.
// If you want to modify the response headers use HijackResponse.SetHeader .
func (ctx *rodHijackResponse) Headers() http.Header {
	header := http.Header{}

	for _, h := range ctx.payload.ResponseHeaders {
		header.Add(h.Name, h.Value)
	}

	return header
}

// SetHeader of the payload via key-value pairs.
func (ctx *rodHijackResponse) SetHeader(pairs ...string) *rodHijackResponse {
	headerIndex := make(map[string]int, len(ctx.payload.ResponseHeaders))
	for i, header := range ctx.payload.ResponseHeaders {
		headerIndex[header.Name] = i
	}

	for i := 0; i < len(pairs); i += 2 {
		name := pairs[i]
		value := pairs[i+1]

		if idx, exists := headerIndex[name]; exists {
			ctx.payload.ResponseHeaders[idx].Value = value
		} else {
			ctx.payload.ResponseHeaders = append(ctx.payload.ResponseHeaders, &proto2.FetchHeaderEntry{
				Name:  name,
				Value: value,
			})
			headerIndex[name] = len(ctx.payload.ResponseHeaders) - 1
		}
	}

	return ctx
}

// AddHeader appends key-value pairs to the end of the response headers.
// Duplicate keys will be preserved.
func (ctx *rodHijackResponse) AddHeader(pairs ...string) *rodHijackResponse {
	for i := 0; i < len(pairs); i += 2 {
		ctx.payload.ResponseHeaders = append(ctx.payload.ResponseHeaders, &proto2.FetchHeaderEntry{
			Name:  pairs[i],
			Value: pairs[i+1],
		})
	}

	return ctx
}

// SetBody of the payload, if obj is []byte or string, raw body will be used, else it will be encoded as json.
func (ctx *rodHijackResponse) SetBody(obj any) *rodHijackResponse {
	switch body := obj.(type) {
	case []byte:
		ctx.payload.Body = body
	case string:
		ctx.payload.Body = []byte(body)
	default:
		ctx.payload.Body = utils.MustToJSONBytes(body)
	}

	return ctx
}

// Fail request.
func (ctx *rodHijackResponse) Fail(reason proto2.NetworkErrorReason) *rodHijackResponse {
	ctx.fail.ErrorReason = reason
	return ctx
}

// HandleAuth for the next basic HTTP authentication.
// It will prevent the popup that requires user to input user name and password.
// Ref: https://developer.mozilla.org/en-US/docs/Web/HTTP/Authentication
func (b *rodBrowser) HandleAuth(username, password string) func() error {
	enable := b.DisableDomain("", &proto2.FetchEnable{})
	disable := b.EnableDomain("", &proto2.FetchEnable{
		HandleAuthRequests: true,
	})

	paused := &proto2.FetchRequestPaused{}
	auth := &proto2.FetchAuthRequired{}

	ctx, cancel := context.WithCancel(b.ctx)
	waitPaused := b.Context(ctx).WaitEvent(paused)
	waitAuth := b.Context(ctx).WaitEvent(auth)

	return func() (err error) {
		defer enable()
		defer disable()
		defer cancel()

		waitPaused()

		err = proto2.FetchContinueRequest{
			RequestID: paused.RequestID,
		}.Call(b)
		if err != nil {
			return
		}

		waitAuth()

		err = proto2.FetchContinueWithAuth{
			RequestID: auth.RequestID,
			AuthChallengeResponse: &proto2.FetchAuthChallengeResponse{
				Response: proto2.FetchAuthChallengeResponseResponseProvideCredentials,
				Username: username,
				Password: password,
			},
		}.Call(b)

		return
	}
}
