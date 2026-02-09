package scout

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// Cookie represents an HTTP cookie.
type Cookie struct {
	Name     string    `json:"name"`
	Value    string    `json:"value"`
	URL      string    `json:"url,omitempty"`
	Domain   string    `json:"domain,omitempty"`
	Path     string    `json:"path,omitempty"`
	Expires  time.Time `json:"expires,omitzero"`
	Secure   bool      `json:"secure,omitempty"`
	HTTPOnly bool      `json:"http_only,omitempty"`
	SameSite string    `json:"same_site,omitempty"`
}

// HijackHandler is a function called for each intercepted request.
type HijackHandler func(*HijackContext)

// HijackRouter manages request interception. Call Run() in a goroutine,
// then Stop() when done.
type HijackRouter struct {
	router *rod.HijackRouter
}

// Run starts the hijack router. This method blocks, so call it in a goroutine.
func (r *HijackRouter) Run() {
	r.router.Run()
}

// Stop stops the hijack router and disables request interception.
func (r *HijackRouter) Stop() error {
	if err := r.router.Stop(); err != nil {
		return fmt.Errorf("scout: stop hijack router: %w", err)
	}

	return nil
}

// HijackContext provides access to the intercepted request and response.
type HijackContext struct {
	hijack *rod.Hijack
}

// HijackRequest provides read access to the intercepted request.
type HijackRequest struct {
	req *rod.HijackRequest
}

// HijackResponse provides write access to the response.
type HijackResponse struct {
	resp *rod.HijackResponse
}

// Request returns the intercepted request.
func (c *HijackContext) Request() *HijackRequest {
	return &HijackRequest{req: c.hijack.Request}
}

// Response returns the response that will be sent to the browser.
func (c *HijackContext) Response() *HijackResponse {
	return &HijackResponse{resp: c.hijack.Response}
}

// ContinueRequest forwards the request to the server without modification.
func (c *HijackContext) ContinueRequest() {
	c.hijack.ContinueRequest(&proto.FetchContinueRequest{})
}

// LoadResponse sends the request to the server and loads the response.
func (c *HijackContext) LoadResponse(loadBody bool) error {
	if err := c.hijack.LoadResponse(http.DefaultClient, loadBody); err != nil {
		return fmt.Errorf("scout: load response: %w", err)
	}

	return nil
}

// Skip marks this handler to skip to the next matching handler.
func (c *HijackContext) Skip() {
	c.hijack.Skip = true
}

// Method returns the HTTP method of the request.
func (r *HijackRequest) Method() string {
	return r.req.Method()
}

// URL returns the URL of the request.
func (r *HijackRequest) URL() *url.URL {
	return r.req.URL()
}

// Header returns the value of the given header.
func (r *HijackRequest) Header(key string) string {
	return r.req.Header(key)
}

// Body returns the request body as a string.
func (r *HijackRequest) Body() string {
	return r.req.Body()
}

// SetBody sets the response body. Accepts string, []byte, or any JSON-serializable value.
func (r *HijackResponse) SetBody(body any) {
	r.resp.SetBody(body)
}

// SetHeader sets a response header.
func (r *HijackResponse) SetHeader(pairs ...string) {
	r.resp.SetHeader(pairs...)
}

// Fail sends an error response.
func (r *HijackResponse) Fail(reason proto.NetworkErrorReason) {
	r.resp.Fail(reason)
}

// SetHeaders sets extra HTTP headers for all requests from this page.
// Returns a cleanup function that removes the headers.
func (p *Page) SetHeaders(headers map[string]string) (cleanup func(), err error) {
	dict := make([]string, 0, len(headers)*2)
	for k, v := range headers {
		dict = append(dict, k, v)
	}

	cleanup, err = p.page.SetExtraHeaders(dict)
	if err != nil {
		return nil, fmt.Errorf("scout: set headers: %w", err)
	}

	return cleanup, nil
}

// SetUserAgent overrides the User-Agent for this page.
func (p *Page) SetUserAgent(ua string) error {
	if err := p.page.SetUserAgent(&proto.NetworkSetUserAgentOverride{
		UserAgent: ua,
	}); err != nil {
		return fmt.Errorf("scout: set user agent: %w", err)
	}

	return nil
}

// SetCookies sets cookies on the page.
func (p *Page) SetCookies(cookies ...Cookie) error {
	params := make([]*proto.NetworkCookieParam, len(cookies))
	for i, c := range cookies {
		param := &proto.NetworkCookieParam{
			Name:     c.Name,
			Value:    c.Value,
			URL:      c.URL,
			Domain:   c.Domain,
			Path:     c.Path,
			Secure:   c.Secure,
			HTTPOnly: c.HTTPOnly,
		}
		if !c.Expires.IsZero() {
			param.Expires = proto.TimeSinceEpoch(c.Expires.Unix())
		}

		params[i] = param
	}

	if err := p.page.SetCookies(params); err != nil {
		return fmt.Errorf("scout: set cookies: %w", err)
	}

	return nil
}

// GetCookies returns cookies for the current page or the given URLs.
func (p *Page) GetCookies(urls ...string) ([]Cookie, error) {
	rodCookies, err := p.page.Cookies(urls)
	if err != nil {
		return nil, fmt.Errorf("scout: get cookies: %w", err)
	}

	cookies := make([]Cookie, len(rodCookies))
	for i, c := range rodCookies {
		cookies[i] = Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  time.Unix(int64(c.Expires), 0),
			Secure:   c.Secure,
			HTTPOnly: c.HTTPOnly,
			SameSite: string(c.SameSite),
		}
	}

	return cookies, nil
}

// ClearCookies removes all cookies from the page.
func (p *Page) ClearCookies() error {
	if err := p.page.SetCookies(nil); err != nil {
		return fmt.Errorf("scout: clear cookies: %w", err)
	}

	return nil
}

// HandleAuth sets up HTTP basic authentication for the browser.
// Returns a function that waits for and handles the next auth challenge.
func (b *Browser) HandleAuth(username, password string) func() error {
	return b.browser.HandleAuth(username, password)
}

// Hijack creates a request hijack router for the page.
// The pattern uses glob-style matching (e.g. "*api*", "*.js").
// Call Run() on the returned router in a goroutine, and Stop() when done.
func (p *Page) Hijack(pattern string, handler HijackHandler) (*HijackRouter, error) {
	router := p.page.HijackRequests()

	err := router.Add(pattern, "", func(h *rod.Hijack) {
		handler(&HijackContext{hijack: h})
	})
	if err != nil {
		return nil, fmt.Errorf("scout: hijack %q: %w", pattern, err)
	}

	return &HijackRouter{router: router}, nil
}

// SetBlockedURLs blocks requests matching the given URL patterns.
// Wildcards (*) are supported (e.g. "*.css", "*analytics*").
func (p *Page) SetBlockedURLs(urls ...string) error {
	if err := p.page.SetBlockedURLs(urls); err != nil {
		return fmt.Errorf("scout: set blocked urls: %w", err)
	}

	return nil
}
