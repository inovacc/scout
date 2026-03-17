package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
)

// HookPoint identifies a browser lifecycle hook.
type HookPoint string

const (
	HookBeforeNavigate HookPoint = "before_navigate"
	HookAfterLoad      HookPoint = "after_load"
	HookBeforeExtract  HookPoint = "before_extract"
	HookOnError        HookPoint = "on_error"
)

// HookAction is the action returned by a middleware plugin.
type HookAction string

const (
	ActionAllow  HookAction = "allow"
	ActionBlock  HookAction = "block"
	ActionModify HookAction = "modify"
	ActionRetry  HookAction = "retry"
)

// HookContext carries data through middleware hooks.
type HookContext struct {
	Hook      HookPoint      `json:"hook"`
	URL       string         `json:"url"`
	PageState map[string]any `json:"page_state,omitempty"`
	Error     string         `json:"error,omitempty"`
}

// HookResult is the response from a middleware plugin.
type HookResult struct {
	Action     HookAction     `json:"action"`
	ModifiedURL string        `json:"modified_url,omitempty"`
	InjectJS   string         `json:"inject_js,omitempty"`
	Data       map[string]any `json:"data,omitempty"`
}

// MiddlewareProxy bridges a plugin's browser_middleware capability.
type MiddlewareProxy struct {
	client   *Client
	hooks    []HookPoint
	priority int
}

// NewMiddlewareProxy creates a MiddlewareProxy for the given plugin client.
func NewMiddlewareProxy(client *Client, hooks []HookPoint, priority int) *MiddlewareProxy {
	return &MiddlewareProxy{client: client, hooks: hooks, priority: priority}
}

// Priority returns the middleware priority (0=first, 100=last).
func (p *MiddlewareProxy) Priority() int { return p.priority }

// Hooks returns the hooks this middleware subscribes to.
func (p *MiddlewareProxy) Hooks() []HookPoint { return p.hooks }

// Execute calls the middleware for a specific hook.
func (p *MiddlewareProxy) Execute(ctx context.Context, hookCtx *HookContext) (*HookResult, error) {
	raw, err := p.client.Call(ctx, "middleware/"+string(hookCtx.Hook), hookCtx)
	if err != nil {
		return nil, fmt.Errorf("plugin middleware: %s: %w", hookCtx.Hook, err)
	}

	var result HookResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("plugin middleware: unmarshal: %w", err)
	}

	return &result, nil
}

// MiddlewareChain manages an ordered set of middleware plugins.
type MiddlewareChain struct {
	middlewares []*MiddlewareProxy
}

// NewMiddlewareChain creates an empty middleware chain.
func NewMiddlewareChain() *MiddlewareChain {
	return &MiddlewareChain{}
}

// Register adds a middleware to the chain, maintaining priority order.
func (c *MiddlewareChain) Register(m *MiddlewareProxy) {
	c.middlewares = append(c.middlewares, m)
	sort.Slice(c.middlewares, func(i, j int) bool {
		return c.middlewares[i].priority < c.middlewares[j].priority
	})
}

// Execute runs all middlewares for the given hook in priority order.
// Stops early if any middleware returns Block.
func (c *MiddlewareChain) Execute(ctx context.Context, hookCtx *HookContext) (*HookResult, error) {
	final := &HookResult{Action: ActionAllow}

	for _, m := range c.middlewares {
		if !hasHook(m.hooks, hookCtx.Hook) {
			continue
		}

		result, err := m.Execute(ctx, hookCtx)
		if err != nil {
			continue // Non-fatal: skip failing middleware.
		}

		switch result.Action {
		case ActionBlock:
			return result, nil
		case ActionModify:
			if result.ModifiedURL != "" {
				hookCtx.URL = result.ModifiedURL
			}

			final = result
		case ActionRetry:
			return result, nil
		}
	}

	return final, nil
}

// Len returns the number of registered middlewares.
func (c *MiddlewareChain) Len() int {
	return len(c.middlewares)
}

func hasHook(hooks []HookPoint, target HookPoint) bool {
	for _, h := range hooks {
		if h == target {
			return true
		}
	}

	return false
}
