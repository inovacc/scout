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
