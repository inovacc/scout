package mcp

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const maxBodySize = 100 * 1024 // 100KB

// pingResult holds timing breakdown for a single ping.
type pingResult struct {
	Seq     int     `json:"seq"`
	TotalMS float64 `json:"total_ms"`
}

// pingDNS holds DNS resolution results.
type pingDNS struct {
	Resolved   []string `json:"resolved"`
	DurationMS float64  `json:"duration_ms"`
}

// pingTCP holds TCP connection timing.
type pingTCP struct {
	DurationMS float64 `json:"duration_ms"`
}

// pingTLS holds TLS handshake details.
type pingTLS struct {
	Version    string  `json:"version,omitempty"`
	Cipher     string  `json:"cipher,omitempty"`
	CertExpiry string  `json:"cert_expiry,omitempty"`
	DurationMS float64 `json:"duration_ms"`
}

// pingHTTP holds HTTP response timing.
type pingHTTP struct {
	Status     int     `json:"status"`
	DurationMS float64 `json:"duration_ms"`
	TTFBMS     float64 `json:"ttfb_ms"`
}

// pingSummary holds aggregate stats.
type pingSummary struct {
	MinMS float64 `json:"min_ms"`
	MaxMS float64 `json:"max_ms"`
	AvgMS float64 `json:"avg_ms"`
}

// pingResponse is the full ping tool output.
type pingResponse struct {
	URL     string       `json:"url"`
	DNS     *pingDNS     `json:"dns,omitempty"`
	TCP     *pingTCP     `json:"tcp,omitempty"`
	TLS     *pingTLS     `json:"tls,omitempty"`
	HTTP    *pingHTTP    `json:"http,omitempty"`
	TotalMS float64      `json:"total_ms"`
	Pings   []pingResult `json:"pings"`
	Summary *pingSummary `json:"summary"`
	Error   string       `json:"error,omitempty"`
}

// curlRedirect holds info about a redirect hop.
type curlRedirect struct {
	URL    string `json:"url"`
	Status int    `json:"status"`
}

// curlTiming holds request timing breakdown.
type curlTiming struct {
	DNSMS     float64 `json:"dns_ms"`
	ConnectMS float64 `json:"connect_ms"`
	TLSMS     float64 `json:"tls_ms"`
	TTFBMS    float64 `json:"ttfb_ms"`
	TotalMS   float64 `json:"total_ms"`
}

// curlSize holds response size info.
type curlSize struct {
	Headers int `json:"headers"`
	Body    int `json:"body"`
}

// curlResponse is the full curl tool output.
type curlResponse struct {
	Status        int               `json:"status"`
	StatusText    string            `json:"status_text"`
	Headers       map[string]string `json:"headers"`
	Body          string            `json:"body"`
	BodyTruncated bool              `json:"body_truncated"`
	Timing        *curlTiming       `json:"timing"`
	Redirects     []curlRedirect    `json:"redirects,omitempty"`
	TLS           *pingTLS          `json:"tls,omitempty"`
	Size          *curlSize         `json:"size"`
	Error         string            `json:"error,omitempty"`
}

// registerDiagTools adds ping and curl diagnostic tools to the MCP server.
func registerDiagTools(server *mcp.Server, state *mcpState) {
	server.AddTool(&mcp.Tool{
		Name:        "ping",
		Description: "Network diagnostic for a URL: DNS resolution, TCP connect, TLS handshake, and HTTP response timing",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"url":{"type":"string","description":"URL to ping (e.g. https://example.com)"},"count":{"type":"integer","description":"number of pings (default 3)","default":3},"useBrowser":{"type":"boolean","description":"use browser instead of raw HTTP for timing"}},"required":["url"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			URL        string `json:"url"`
			Count      int    `json:"count"`
			UseBrowser bool   `json:"useBrowser"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errResult(err.Error())
		}

		if args.Count <= 0 {
			args.Count = 3
		}

		if args.Count > 20 {
			args.Count = 20
		}

		args.URL = normalizeURL(args.URL)

		if args.UseBrowser {
			return pingViaBrowser(ctx, state, args.URL, args.Count)
		}

		return pingRaw(ctx, args.URL, args.Count)
	})

	server.AddTool(&mcp.Tool{
		Name:        "curl",
		Description: "Full HTTP client: send requests with custom method, headers, body; returns status, headers, body, timing, redirects, and TLS info",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"url":{"type":"string","description":"URL to request"},"method":{"type":"string","description":"HTTP method (default GET)","default":"GET"},"headers":{"type":"object","description":"request headers as key-value pairs","additionalProperties":{"type":"string"}},"body":{"type":"string","description":"request body"},"followRedirects":{"type":"boolean","description":"follow redirects (default true)","default":true},"maxRedirects":{"type":"integer","description":"max redirects to follow (default 10)","default":10},"timeout":{"type":"integer","description":"timeout in seconds (default 30)","default":30},"useBrowser":{"type":"boolean","description":"use browser instead of raw HTTP"}},"required":["url"]}`),
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args struct {
			URL             string            `json:"url"`
			Method          string            `json:"method"`
			Headers         map[string]string `json:"headers"`
			Body            string            `json:"body"`
			FollowRedirects *bool             `json:"followRedirects"`
			MaxRedirects    int               `json:"maxRedirects"`
			Timeout         int               `json:"timeout"`
			UseBrowser      bool              `json:"useBrowser"`
		}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errResult(err.Error())
		}

		if args.Method == "" {
			args.Method = "GET"
		}

		if args.FollowRedirects == nil {
			t := true
			args.FollowRedirects = &t
		}

		if args.MaxRedirects <= 0 {
			args.MaxRedirects = 10
		}

		if args.Timeout <= 0 {
			args.Timeout = 30
		}

		args.URL = normalizeURL(args.URL)

		if args.UseBrowser {
			return curlViaBrowser(ctx, state, args.URL)
		}

		return curlRaw(ctx, args.URL, args.Method, args.Headers, args.Body, *args.FollowRedirects, args.MaxRedirects, args.Timeout)
	})
}

func normalizeURL(u string) string {
	if !strings.Contains(u, "://") {
		u = "https://" + u
	}

	return u
}

func tlsVersionString(v uint16) string {
	switch v {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return fmt.Sprintf("0x%04x", v)
	}
}

const perfTimingJS = `(() => {
	const e = performance.getEntriesByType('navigation')[0];
	if (!e) return null;
	return JSON.stringify({
		dns_ms: e.domainLookupEnd - e.domainLookupStart,
		connect_ms: e.connectEnd - e.connectStart,
		tls_ms: e.secureConnectionStart > 0 ? e.connectEnd - e.secureConnectionStart : 0,
		ttfb_ms: e.responseStart - e.requestStart,
		total_ms: e.responseEnd - e.startTime
	});
})()`

func summarizePings(pings []pingResult) *pingSummary {
	var minVal, maxVal, sum float64
	for i, p := range pings {
		if i == 0 || p.TotalMS < minVal {
			minVal = p.TotalMS
		}

		if p.TotalMS > maxVal {
			maxVal = p.TotalMS
		}

		sum += p.TotalMS
	}

	return &pingSummary{MinMS: minVal, MaxMS: maxVal, AvgMS: sum / float64(len(pings))}
}

func pingRaw(ctx context.Context, rawURL string, count int) (*mcp.CallToolResult, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return errResult(fmt.Sprintf("invalid URL: %s", err))
	}

	resp := pingResponse{URL: rawURL}

	var pings []pingResult

	for i := range count {
		var (
			dnsStart, dnsEnd, connStart, connEnd, tlsStart, tlsEnd, gotFirstByte time.Time
			resolvedAddrs                                                        []string
			tlsState                                                             *tls.ConnectionState
		)

		trace := &httptrace.ClientTrace{
			DNSStart: func(_ httptrace.DNSStartInfo) { dnsStart = time.Now() },
			DNSDone: func(info httptrace.DNSDoneInfo) {
				dnsEnd = time.Now()

				for _, addr := range info.Addrs {
					resolvedAddrs = append(resolvedAddrs, addr.String())
				}
			},
			ConnectStart:      func(_, _ string) { connStart = time.Now() },
			ConnectDone:       func(_, _ string, _ error) { connEnd = time.Now() },
			TLSHandshakeStart: func() { tlsStart = time.Now() },
			TLSHandshakeDone: func(state tls.ConnectionState, _ error) {
				tlsEnd = time.Now()
				tlsState = &state
			},
			GotFirstResponseByte: func() { gotFirstByte = time.Now() },
		}

		reqCtx := httptrace.WithClientTrace(ctx, trace)

		httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodHead, rawURL, nil)
		if err != nil {
			return errResult(fmt.Sprintf("scout-mcp: ping: %s", err))
		}

		start := time.Now()
		client := &http.Client{Timeout: 30 * time.Second}
		httpResp, err := client.Do(httpReq)
		total := time.Since(start)

		if err != nil {
			resp.Error = err.Error()

			pings = append(pings, pingResult{Seq: i + 1, TotalMS: ms(total)})

			continue
		}

		_ = httpResp.Body.Close()

		pings = append(pings, pingResult{Seq: i + 1, TotalMS: ms(total)})

		// Populate details from first successful ping.
		if resp.DNS == nil && !dnsStart.IsZero() {
			resp.DNS = &pingDNS{
				Resolved:   resolvedAddrs,
				DurationMS: ms(dnsEnd.Sub(dnsStart)),
			}
		}

		if resp.TCP == nil && !connStart.IsZero() {
			resp.TCP = &pingTCP{DurationMS: ms(connEnd.Sub(connStart))}
		}

		if resp.TLS == nil && tlsState != nil && parsed.Scheme == "https" {
			t := &pingTLS{
				Version:    tlsVersionString(tlsState.Version),
				Cipher:     tls.CipherSuiteName(tlsState.CipherSuite),
				DurationMS: ms(tlsEnd.Sub(tlsStart)),
			}
			if len(tlsState.PeerCertificates) > 0 {
				t.CertExpiry = tlsState.PeerCertificates[0].NotAfter.Format(time.RFC3339)
			}

			resp.TLS = t
		}

		if resp.HTTP == nil {
			resp.HTTP = &pingHTTP{
				Status:     httpResp.StatusCode,
				DurationMS: ms(total),
			}
			if !gotFirstByte.IsZero() {
				resp.HTTP.TTFBMS = ms(gotFirstByte.Sub(start))
			}
		}
	}

	resp.Pings = pings
	resp.TotalMS = pings[len(pings)-1].TotalMS
	resp.Summary = summarizePings(pings)

	return jsonResult(resp)
}

func pingViaBrowser(ctx context.Context, state *mcpState, rawURL string, count int) (*mcp.CallToolResult, error) {
	page, err := state.ensurePage(ctx)
	if err != nil {
		return errResult(err.Error())
	}

	resp := pingResponse{URL: rawURL}

	var pings []pingResult

	for i := range count {
		start := time.Now()

		if err := page.Navigate(rawURL); err != nil {
			resp.Error = err.Error()

			pings = append(pings, pingResult{Seq: i + 1, TotalMS: ms(time.Since(start))})

			continue
		}

		_ = page.WaitLoad()
		total := time.Since(start)
		pings = append(pings, pingResult{Seq: i + 1, TotalMS: ms(total)})
	}

	if result, err := page.Eval(perfTimingJS); err == nil {
		s := result.String()
		if s != "" && s != "null" {
			var perf curlTiming
			if json.Unmarshal([]byte(s), &perf) == nil {
				resp.DNS = &pingDNS{DurationMS: perf.DNSMS}

				resp.TCP = &pingTCP{DurationMS: perf.ConnectMS}
				if perf.TLSMS > 0 {
					resp.TLS = &pingTLS{DurationMS: perf.TLSMS}
				}

				resp.HTTP = &pingHTTP{
					DurationMS: perf.TotalMS,
					TTFBMS:     perf.TTFBMS,
				}
			}
		}
	}

	resp.Pings = pings
	resp.TotalMS = pings[len(pings)-1].TotalMS
	resp.Summary = summarizePings(pings)

	return jsonResult(resp)
}

func curlRaw(ctx context.Context, rawURL, method string, headers map[string]string, body string, followRedirects bool, maxRedirects, timeout int) (*mcp.CallToolResult, error) {
	var (
		dnsStart, dnsEnd, connStart, connEnd, tlsStart, tlsEnd, gotFirstByte time.Time
		tlsState                                                             *tls.ConnectionState
	)

	trace := &httptrace.ClientTrace{
		DNSStart:          func(_ httptrace.DNSStartInfo) { dnsStart = time.Now() },
		DNSDone:           func(_ httptrace.DNSDoneInfo) { dnsEnd = time.Now() },
		ConnectStart:      func(_, _ string) { connStart = time.Now() },
		ConnectDone:       func(_, _ string, _ error) { connEnd = time.Now() },
		TLSHandshakeStart: func() { tlsStart = time.Now() },
		TLSHandshakeDone: func(state tls.ConnectionState, _ error) {
			tlsEnd = time.Now()
			tlsState = &state
		},
		GotFirstResponseByte: func() { gotFirstByte = time.Now() },
	}

	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	reqCtx := httptrace.WithClientTrace(ctx, trace)

	httpReq, err := http.NewRequestWithContext(reqCtx, method, rawURL, bodyReader)
	if err != nil {
		return errResult(fmt.Sprintf("scout-mcp: curl: %s", err))
	}

	for k, v := range headers {
		httpReq.Header.Set(k, v)
	}

	var redirects []curlRedirect

	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if !followRedirects {
				return http.ErrUseLastResponse
			}

			if len(via) >= maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}

			prev := via[len(via)-1]

			rd := curlRedirect{URL: prev.URL.String()}
			if prev.Response != nil {
				rd.Status = prev.Response.StatusCode
			}

			redirects = append(redirects, rd)

			return nil
		},
	}

	start := time.Now()

	httpResp, err := client.Do(httpReq)
	if err != nil {
		// Return partial result with error.
		cr := curlResponse{Error: err.Error()}
		return jsonResult(cr)
	}

	defer func() { _ = httpResp.Body.Close() }()

	// Read body with limit.
	limited := io.LimitReader(httpResp.Body, maxBodySize+1)

	bodyBytes, err := io.ReadAll(limited)
	if err != nil {
		return errResult(fmt.Sprintf("scout-mcp: curl: read body: %s", err))
	}

	total := time.Since(start)

	truncated := len(bodyBytes) > maxBodySize
	if truncated {
		bodyBytes = bodyBytes[:maxBodySize]
	}

	// Flatten response headers.
	respHeaders := make(map[string]string, len(httpResp.Header))
	headerSize := 0

	for k, vals := range httpResp.Header {
		respHeaders[strings.ToLower(k)] = strings.Join(vals, ", ")
		headerSize += len(k) + len(strings.Join(vals, ", ")) + 4 // ": " + "\r\n"
	}

	cr := curlResponse{
		Status:        httpResp.StatusCode,
		StatusText:    http.StatusText(httpResp.StatusCode),
		Headers:       respHeaders,
		Body:          string(bodyBytes),
		BodyTruncated: truncated,
		Timing: &curlTiming{
			TotalMS: ms(total),
		},
		Redirects: redirects,
		Size: &curlSize{
			Headers: headerSize,
			Body:    len(bodyBytes),
		},
	}

	if !dnsStart.IsZero() && !dnsEnd.IsZero() {
		cr.Timing.DNSMS = ms(dnsEnd.Sub(dnsStart))
	}

	if !connStart.IsZero() && !connEnd.IsZero() {
		cr.Timing.ConnectMS = ms(connEnd.Sub(connStart))
	}

	if !tlsStart.IsZero() && !tlsEnd.IsZero() {
		cr.Timing.TLSMS = ms(tlsEnd.Sub(tlsStart))
	}

	if !gotFirstByte.IsZero() {
		cr.Timing.TTFBMS = ms(gotFirstByte.Sub(start))
	}

	if tlsState != nil {
		cr.TLS = &pingTLS{
			Version:    tlsVersionString(tlsState.Version),
			Cipher:     tls.CipherSuiteName(tlsState.CipherSuite),
			DurationMS: ms(tlsEnd.Sub(tlsStart)),
		}
		if len(tlsState.PeerCertificates) > 0 {
			cr.TLS.CertExpiry = tlsState.PeerCertificates[0].NotAfter.Format(time.RFC3339)
		}
	}

	return jsonResult(cr)
}

func curlViaBrowser(ctx context.Context, state *mcpState, rawURL string) (*mcp.CallToolResult, error) {
	page, err := state.ensurePage(ctx)
	if err != nil {
		return errResult(err.Error())
	}

	start := time.Now()

	if err := page.Navigate(rawURL); err != nil {
		return errResult(err.Error())
	}

	_ = page.WaitLoad()
	total := time.Since(start)

	u, _ := page.URL()
	title, _ := page.Title()

	// Get page content as text.
	bodyText := ""
	if result, err := page.Eval(`document.documentElement.outerHTML`); err == nil {
		bodyText = result.String()
		if len(bodyText) > maxBodySize {
			bodyText = bodyText[:maxBodySize]
		}
	}

	cr := curlResponse{
		Status:        200,
		StatusText:    "OK",
		Headers:       map[string]string{"x-final-url": u, "x-page-title": title},
		Body:          bodyText,
		BodyTruncated: len(bodyText) >= maxBodySize,
		Timing:        &curlTiming{TotalMS: ms(total)},
		Size:          &curlSize{Body: len(bodyText)},
	}

	if result, err := page.Eval(perfTimingJS); err == nil {
		s := result.String()
		if s != "" && s != "null" {
			var perf curlTiming
			if json.Unmarshal([]byte(s), &perf) == nil {
				cr.Timing = &perf
			}
		}
	}

	return jsonResult(cr)
}

func ms(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000.0
}

func jsonResult(v any) (*mcp.CallToolResult, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return errResult(fmt.Sprintf("scout-mcp: marshal: %s", err))
	}

	return textResult(string(data))
}
