package sdk

import "context"

// MiddlewareHandler handles browser lifecycle hooks from Scout.
type MiddlewareHandler interface {
	// BeforeNavigate is called before page navigation. Can modify URL or block.
	BeforeNavigate(ctx context.Context, hookCtx MiddlewareContext) (*MiddlewareResult, error)

	// AfterLoad is called after page load completes. Can inject JS or modify DOM.
	AfterLoad(ctx context.Context, hookCtx MiddlewareContext) (*MiddlewareResult, error)

	// BeforeExtract is called before data extraction. Can transform selectors.
	BeforeExtract(ctx context.Context, hookCtx MiddlewareContext) (*MiddlewareResult, error)

	// OnError is called on page errors. Can retry or skip.
	OnError(ctx context.Context, hookCtx MiddlewareContext) (*MiddlewareResult, error)
}

// MiddlewareContext is the context passed to middleware hooks.
type MiddlewareContext struct {
	Hook      string         `json:"hook"`
	URL       string         `json:"url"`
	PageState map[string]any `json:"page_state,omitempty"`
	Error     string         `json:"error,omitempty"`
}

// MiddlewareResult is the response from a middleware hook.
type MiddlewareResult struct {
	Action      string         `json:"action"` // allow, block, modify, retry
	ModifiedURL string         `json:"modified_url,omitempty"`
	InjectJS    string         `json:"inject_js,omitempty"`
	Data        map[string]any `json:"data,omitempty"`
}

// AllowResult returns a result that allows the operation to proceed.
func AllowResult() *MiddlewareResult {
	return &MiddlewareResult{Action: "allow"}
}

// BlockResult returns a result that blocks the operation.
func BlockResult() *MiddlewareResult {
	return &MiddlewareResult{Action: "block"}
}

// ModifyURLResult returns a result that modifies the navigation URL.
func ModifyURLResult(url string) *MiddlewareResult {
	return &MiddlewareResult{Action: "modify", ModifiedURL: url}
}

// RetryResult returns a result that requests a retry.
func RetryResult() *MiddlewareResult {
	return &MiddlewareResult{Action: "retry"}
}
