// scout-diag is a Scout plugin providing ping and curl MCP tools.
//
// Install: scout plugin install ./plugins/scout-diag
// Or build: go build -o scout-diag ./plugins/scout-diag
package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strings"
	"time"

	"github.com/inovacc/scout/pkg/scout/plugin/sdk"
)

const maxBodySize = 100 * 1024 // 100KB

func main() {
	srv := sdk.NewServer()
	srv.RegisterTool("ping", sdk.ToolHandlerFunc(handlePing))
	srv.RegisterTool("curl", sdk.ToolHandlerFunc(handleCurl))

	if err := srv.Run(); err != nil {
		log.Fatal(err)
	}
}

func handlePing(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
	rawURL, _ := args["url"].(string)
	if rawURL == "" {
		return sdk.ErrorResult("url is required"), nil
	}

	rawURL = normalizeURL(rawURL)

	count := 3
	if c, ok := args["count"].(float64); ok && c > 0 {
		count = int(c)
	}

	if count > 20 {
		count = 20
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return sdk.ErrorResult(fmt.Sprintf("invalid URL: %s", err)), nil
	}

	type response struct {
		URL     string              `json:"url"`
		DNS     any                 `json:"dns,omitempty"`
		TCP     any                 `json:"tcp,omitempty"`
		TLS     any                 `json:"tls,omitempty"`
		HTTP    any                 `json:"http,omitempty"`
		TotalMS float64             `json:"total_ms"`
		Pings   []pingResultEntry   `json:"pings"`
		Summary map[string]float64  `json:"summary"`
		Error   string              `json:"error,omitempty"`
	}

	resp := response{URL: rawURL}

	var pings []pingResultEntry

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

		reqCtx := httptrace.WithClientTrace(context.Background(), trace)

		httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodHead, rawURL, nil)
		if err != nil {
			return sdk.ErrorResult(err.Error()), nil
		}

		start := time.Now()
		client := &http.Client{Timeout: 30 * time.Second}
		httpResp, err := client.Do(httpReq)
		total := time.Since(start)

		if err != nil {
			resp.Error = err.Error()
			pings = append(pings, pingResultEntry{Seq: i + 1, TotalMS: ms(total)})

			continue
		}

		_ = httpResp.Body.Close()
		pings = append(pings, pingResultEntry{Seq: i + 1, TotalMS: ms(total)})

		if resp.DNS == nil && !dnsStart.IsZero() {
			resp.DNS = map[string]any{"resolved": resolvedAddrs, "duration_ms": ms(dnsEnd.Sub(dnsStart))}
		}

		if resp.TCP == nil && !connStart.IsZero() {
			resp.TCP = map[string]any{"duration_ms": ms(connEnd.Sub(connStart))}
		}

		if resp.TLS == nil && tlsState != nil && parsed.Scheme == "https" {
			t := map[string]any{
				"version":     tlsVersionString(tlsState.Version),
				"cipher":      tls.CipherSuiteName(tlsState.CipherSuite),
				"duration_ms": ms(tlsEnd.Sub(tlsStart)),
			}
			if len(tlsState.PeerCertificates) > 0 {
				t["cert_expiry"] = tlsState.PeerCertificates[0].NotAfter.Format(time.RFC3339)
			}

			resp.TLS = t
		}

		if resp.HTTP == nil {
			h := map[string]any{"status": httpResp.StatusCode, "duration_ms": ms(total)}
			if !gotFirstByte.IsZero() {
				h["ttfb_ms"] = ms(gotFirstByte.Sub(start))
			}

			resp.HTTP = h
		}
	}

	resp.Pings = pings
	if len(pings) > 0 {
		resp.TotalMS = pings[len(pings)-1].TotalMS
		resp.Summary = summarizePings(pings)
	}

	return jsonResult(resp)
}

func handleCurl(ctx context.Context, args map[string]any) (*sdk.ToolResult, error) {
	rawURL, _ := args["url"].(string)
	if rawURL == "" {
		return sdk.ErrorResult("url is required"), nil
	}

	rawURL = normalizeURL(rawURL)

	method, _ := args["method"].(string)
	if method == "" {
		method = "GET"
	}

	headers := make(map[string]string)
	if h, ok := args["headers"].(map[string]any); ok {
		for k, v := range h {
			if s, ok := v.(string); ok {
				headers[k] = s
			}
		}
	}

	body, _ := args["body"].(string)

	followRedirects := true
	if f, ok := args["followRedirects"].(bool); ok {
		followRedirects = f
	}

	maxRedirects := 10
	if m, ok := args["maxRedirects"].(float64); ok && m > 0 {
		maxRedirects = int(m)
	}

	timeout := 30
	if t, ok := args["timeout"].(float64); ok && t > 0 {
		timeout = int(t)
	}

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
		return sdk.ErrorResult(err.Error()), nil
	}

	for k, v := range headers {
		httpReq.Header.Set(k, v)
	}

	type redirectEntry struct {
		URL    string `json:"url"`
		Status int    `json:"status"`
	}

	var redirects []redirectEntry

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
			rd := redirectEntry{URL: prev.URL.String()}
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
		return jsonResult(map[string]any{"error": err.Error()})
	}

	defer func() { _ = httpResp.Body.Close() }()

	limited := io.LimitReader(httpResp.Body, maxBodySize+1)

	bodyBytes, err := io.ReadAll(limited)
	if err != nil {
		return sdk.ErrorResult(fmt.Sprintf("read body: %s", err)), nil
	}

	total := time.Since(start)
	truncated := len(bodyBytes) > maxBodySize

	if truncated {
		bodyBytes = bodyBytes[:maxBodySize]
	}

	respHeaders := make(map[string]string, len(httpResp.Header))
	headerSize := 0

	for k, vals := range httpResp.Header {
		respHeaders[strings.ToLower(k)] = strings.Join(vals, ", ")
		headerSize += len(k) + len(strings.Join(vals, ", ")) + 4
	}

	timing := map[string]any{"total_ms": ms(total)}

	if !dnsStart.IsZero() && !dnsEnd.IsZero() {
		timing["dns_ms"] = ms(dnsEnd.Sub(dnsStart))
	}

	if !connStart.IsZero() && !connEnd.IsZero() {
		timing["connect_ms"] = ms(connEnd.Sub(connStart))
	}

	if !tlsStart.IsZero() && !tlsEnd.IsZero() {
		timing["tls_ms"] = ms(tlsEnd.Sub(tlsStart))
	}

	if !gotFirstByte.IsZero() {
		timing["ttfb_ms"] = ms(gotFirstByte.Sub(start))
	}

	cr := map[string]any{
		"status":         httpResp.StatusCode,
		"status_text":    http.StatusText(httpResp.StatusCode),
		"headers":        respHeaders,
		"body":           string(bodyBytes),
		"body_truncated": truncated,
		"timing":         timing,
		"size":           map[string]int{"headers": headerSize, "body": len(bodyBytes)},
	}

	if len(redirects) > 0 {
		cr["redirects"] = redirects
	}

	if tlsState != nil {
		t := map[string]any{
			"version":     tlsVersionString(tlsState.Version),
			"cipher":      tls.CipherSuiteName(tlsState.CipherSuite),
			"duration_ms": ms(tlsEnd.Sub(tlsStart)),
		}
		if len(tlsState.PeerCertificates) > 0 {
			t["cert_expiry"] = tlsState.PeerCertificates[0].NotAfter.Format(time.RFC3339)
		}

		cr["tls"] = t
	}

	return jsonResult(cr)
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

func ms(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000.0
}

type pingResultEntry struct {
	Seq     int     `json:"seq"`
	TotalMS float64 `json:"total_ms"`
}

func summarizePings(pings []pingResultEntry) map[string]float64 {
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

	return map[string]float64{"min_ms": minVal, "max_ms": maxVal, "avg_ms": sum / float64(len(pings))}
}

func jsonResult(data any) (*sdk.ToolResult, error) {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return sdk.ErrorResult(err.Error()), nil
	}

	return sdk.TextResult(string(b)), nil
}
