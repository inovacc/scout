package engine

import (
	"log/slog"
	"strings"

	proto2 "github.com/inovacc/scout/internal/engine/lib/proto"
)

// installBlockRules registers a HijackRequests router that aborts every
// request matching one of the configured rules. Method filtering happens
// inside the handler (CDP Fetch.enable doesn't filter by method itself).
//
// Aborted requests still surface in any concurrent HAR / hijack capture
// — the recorder sees the requestPaused event with method, URL, headers,
// and body before the fail callback fires. That's the whole point: capture
// the intended payload without the server seeing the request.
func (b *Browser) installBlockRules(rules []BlockRule) {
	if b == nil || b.browser == nil || len(rules) == 0 {
		return
	}

	router := b.browser.HijackRequests()

	for i := range rules {
		rule := rules[i]
		err := router.Add(rule.Pattern, "", func(h *Hijack) {
			if rule.Method != "" && !strings.EqualFold(h.Request.Method(), rule.Method) {
				h.ContinueRequest(&proto2.FetchContinueRequest{})
				return
			}

			slog.Info("scout: blocked request",
				"pattern", rule.Pattern,
				"method", h.Request.Method(),
				"url", h.Request.URL().String(),
			)

			h.Response.Fail(proto2.NetworkErrorReasonBlockedByClient)
		})
		if err != nil {
			slog.Warn("scout: install block rule failed",
				"pattern", rule.Pattern,
				"err", err,
			)
		}
	}

	go router.Run()
	b.blockRouter = router
}

// InstallRequestFilter enforces an egress/SSRF policy on EVERY outbound request,
// not just the top-level navigation. The filter is consulted with each request's
// URL — including HTTP redirects and in-page fetch()/XHR issued by evaluated
// JavaScript — and any request it rejects is aborted at the network layer
// (Fetch.failRequest). This closes the gap where a navigate-time URL check is
// bypassed by a 30x redirect to an internal host or by the eval tool.
//
// Intended for untrusted-caller front ends (the agent REST API and the MCP
// server). Call once, right after New(), before navigation.
//
// Note: the browser resolves DNS independently of the filter, so this reduces
// but does not fully eliminate DNS rebinding (TOCTOU); pinning the resolved IP
// additionally requires Chrome --host-resolver-rules.
func (b *Browser) InstallRequestFilter(filter func(rawURL string) bool) {
	if b == nil || b.browser == nil || filter == nil {
		return
	}

	router := b.blockRouter
	fresh := router == nil
	if fresh {
		router = b.browser.HijackRequests()
	}

	err := router.Add("*", "", func(h *Hijack) {
		u := h.Request.URL().String()
		if filter(u) {
			h.ContinueRequest(&proto2.FetchContinueRequest{})
			return
		}

		slog.Warn("scout: SSRF policy blocked request", "url", u)
		h.Response.Fail(proto2.NetworkErrorReasonBlockedByClient)
	})
	if err != nil {
		slog.Warn("scout: install request filter failed", "err", err)
		return
	}

	if fresh {
		go router.Run()
		b.blockRouter = router
	}
}
