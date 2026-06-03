package flow

import (
	"context"
	"fmt"
	"time"

	"github.com/inovacc/scout/pkg/scout"
)

// CaptureOptions configures a capture.
type CaptureOptions struct {
	URL       string        // page to open
	Name      string        // capture name
	URLFilter []string      // glob patterns for which URLs to keep (e.g. "*api*")
	Settle    time.Duration // how long to keep recording after load; 0 → 3s
}

// CaptureFlow opens URL in page, records HTTP traffic via the session hijacker,
// and returns a normalized Capture. Body capture is enabled so responses are
// preserved for analysis. Requests/responses are correlated by RequestID and
// emitted in first-seen order.
func CaptureFlow(ctx context.Context, page *scout.Page, opts CaptureOptions) (*Capture, error) {
	settle := opts.Settle
	if settle == 0 {
		settle = 3 * time.Second
	}

	hj, err := page.NewSessionHijacker(hijackOpts(opts.URLFilter)...)
	if err != nil {
		return nil, fmt.Errorf("scout: flow: capture: start hijack: %w", err)
	}
	defer hj.Stop()

	pending := map[string]*CaptureEntry{}
	var order []string

	done := make(chan struct{})
	go func() {
		for ev := range hj.Events() {
			switch {
			case ev.Request != nil:
				ce := &CaptureEntry{
					Method:     ev.Request.Method,
					URL:        ev.Request.URL,
					ReqHeaders: ev.Request.Headers,
					ReqBody:    ev.Request.Body,
				}
				if _, ok := pending[ev.Request.RequestID]; !ok {
					order = append(order, ev.Request.RequestID)
				}
				pending[ev.Request.RequestID] = ce
			case ev.Response != nil:
				if ce, ok := pending[ev.Response.RequestID]; ok {
					ce.Status = ev.Response.Status
					ce.RespHeaders = ev.Response.Headers
					ce.RespBody = ev.Response.Body
					ce.MimeType = ev.Response.MimeType
				}
			}
		}
		close(done)
	}()

	if err := page.Navigate(opts.URL); err != nil {
		return nil, fmt.Errorf("scout: flow: capture: navigate: %w", err)
	}
	_ = page.WaitLoad()
	select {
	case <-time.After(settle):
	case <-ctx.Done():
	}
	hj.Stop()
	<-done

	out := &Capture{Version: "1", Name: opts.Name}
	out.Entries = make([]CaptureEntry, 0, len(order))
	for _, id := range order {
		if ce := pending[id]; ce != nil {
			out.Entries = append(out.Entries, *ce)
		}
	}
	return out, nil
}

// hijackOpts builds session-hijacker options: always capture bodies (responses are
// needed for analysis) and, when filters are given, restrict capture to those globs.
func hijackOpts(filters []string) []scout.HijackOption {
	opts := []scout.HijackOption{scout.WithHijackBodyCapture()}
	if len(filters) > 0 {
		opts = append(opts, scout.WithHijackURLFilter(filters...))
	}
	return opts
}
