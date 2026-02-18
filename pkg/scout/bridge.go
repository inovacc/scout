package scout

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/inovacc/scout/pkg/rod/lib/proto"
)

// BridgeHandler processes an event received from the browser.
type BridgeHandler func(data json.RawMessage)

// BridgeEvent represents a message sent between Go and the browser.
type BridgeEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
	TS   int64           `json:"ts"`
}

// MutationEvent represents a DOM mutation observed by the bridge.
type MutationEvent struct {
	Type          string `json:"type"`
	Target        string `json:"target"`
	AddedNodes    int    `json:"addedNodes"`
	RemovedNodes  int    `json:"removedNodes"`
	AttributeName string `json:"attributeName,omitempty"`
	OldValue      string `json:"oldValue,omitempty"`
}

// BridgeOption configures bridge behavior.
type BridgeOption func(*bridgeOptions)

type bridgeOptions struct {
	queryTimeout time.Duration
}

func bridgeDefaults() *bridgeOptions {
	return &bridgeOptions{
		queryTimeout: 10 * time.Second,
	}
}

// WithQueryTimeout sets the timeout for Bridge.Query() calls.
func WithQueryTimeout(d time.Duration) BridgeOption {
	return func(o *bridgeOptions) { o.queryTimeout = d }
}

// Bridge provides bidirectional communication between Go and the browser
// runtime via CDP bindings. It is initialized lazily via Page.Bridge().
type Bridge struct {
	page      *Page
	handlers  map[string][]BridgeHandler
	queries   map[string]chan json.RawMessage
	mu        sync.RWMutex
	opts      *bridgeOptions
	ready     bool
	available bool
}

// Bridge returns the bridge for this page, initializing it on first call.
// The bridge sets up a CDP binding (__scoutSend) and a command dispatcher
// so Go and the content script can exchange messages.
func (p *Page) Bridge(opts ...BridgeOption) (*Bridge, error) {
	o := bridgeDefaults()
	for _, fn := range opts {
		fn(o)
	}

	b := &Bridge{
		page:     p,
		handlers: make(map[string][]BridgeHandler),
		queries:  make(map[string]chan json.RawMessage),
		opts:     o,
	}

	if err := b.init(); err != nil {
		return nil, fmt.Errorf("scout: bridge init: %w", err)
	}

	return b, nil
}

func (b *Bridge) init() error {
	rodPage := b.page.RodPage()

	// Register the CDP binding so content script can call __scoutSend().
	if err := (proto.RuntimeAddBinding{Name: "__scoutSend"}).Call(rodPage); err != nil {
		return fmt.Errorf("add binding: %w", err)
	}

	// Listen for binding calls and route to handlers.
	go rodPage.EachEvent(func(e *proto.RuntimeBindingCalled) {
		if e.Name != "__scoutSend" {
			return
		}

		var evt BridgeEvent
		if err := json.Unmarshal([]byte(e.Payload), &evt); err != nil {
			return
		}

		// Handle internal query responses.
		if evt.Type == "__query_response" {
			b.handleQueryResponse(evt.Data)
			return
		}

		// Mark bridge as available when content script reports ready.
		if evt.Type == "__bridge_ready" {
			b.mu.Lock()
			b.ready = true
			b.available = true
			b.mu.Unlock()
		}

		b.mu.RLock()
		fns := b.handlers[evt.Type]
		b.mu.RUnlock()

		for _, fn := range fns {
			fn(evt.Data)
		}
	})()

	// Inject the command dispatcher JS on every new document so Go→browser
	// commands work even after navigation.
	_, err := b.page.EvalOnNewDocument(commandDispatcherJS)
	if err != nil {
		return fmt.Errorf("inject command dispatcher: %w", err)
	}

	// Also inject into the current page immediately.
	if _, err := b.page.Eval(commandDispatcherExpr); err != nil {
		// Non-fatal: page may not be ready yet.
		_ = err
	}

	return nil
}

// commandDispatcherJS is injected into every page so Go can dispatch
// commands to the content script via CustomEvent.
// NOTE: This is used with EvalOnNewDocument (raw injection), NOT page.Eval()
// (which wraps in a function). For page.Eval() use commandDispatcherExpr.
const commandDispatcherJS = `
(function() {
  if (window.__scoutDispatch) return;
  window.__scoutDispatch = function(type, data, id) {
    window.dispatchEvent(new CustomEvent('__scoutCommand', {
      detail: { type: type, data: data, id: id || null }
    }));
  };
})();
`

// commandDispatcherExpr is an expression-safe version for use with page.Eval().
// rod wraps Eval JS as: function() { return (JS).apply(this, arguments) }
// So the JS must be a function expression or a value expression.
const commandDispatcherExpr = `function() {
  if (!window.__scoutDispatch) {
    window.__scoutDispatch = function(type, data, id) {
      window.dispatchEvent(new CustomEvent('__scoutCommand', {
        detail: { type: type, data: data, id: id || null }
      }));
    };
  }
}`

// Available returns true if the bridge extension's content script has loaded
// and signaled readiness.
func (b *Bridge) Available() bool {
	if b == nil {
		return false
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	return b.available
}

// Send dispatches a command from Go to the browser content script.
func (b *Bridge) Send(eventType string, data any) error {
	if b == nil {
		return fmt.Errorf("scout: bridge is nil")
	}

	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("scout: bridge marshal: %w", err)
	}

	js := fmt.Sprintf(`function() { if (window.__scoutDispatch) window.__scoutDispatch(%q, %s) }`, eventType, string(payload))
	if _, err := b.page.Eval(js); err != nil {
		return fmt.Errorf("scout: bridge send: %w", err)
	}

	return nil
}

// On registers a handler for events of the given type from the browser.
func (b *Bridge) On(eventType string, handler BridgeHandler) {
	if b == nil {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b.handlers[eventType] = append(b.handlers[eventType], handler)
}

// Off removes all handlers for the given event type.
func (b *Bridge) Off(eventType string) {
	if b == nil {
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	delete(b.handlers, eventType)
}

// OnMutation registers a handler for DOM mutation events.
func (b *Bridge) OnMutation(handler func([]MutationEvent)) {
	b.On("mutation", func(data json.RawMessage) {
		var mutations []MutationEvent
		if err := json.Unmarshal(data, &mutations); err != nil {
			return
		}

		handler(mutations)
	})
}

// ObserveMutations starts the DOM MutationObserver in the browser for the given selector.
// If selector is empty, observes document.body.
func (b *Bridge) ObserveMutations(selector string) error {
	if selector == "" {
		return b.Send("__observe_mutations", nil)
	}

	return b.Send("__observe_mutations", map[string]string{"selector": selector})
}

// Query sends a request to the browser and waits for a response with a timeout.
func (b *Bridge) Query(method string, params any) (json.RawMessage, error) {
	if b == nil {
		return nil, fmt.Errorf("scout: bridge is nil")
	}

	id := fmt.Sprintf("q_%d_%d", time.Now().UnixNano(), rand.IntN(10000)) //nolint:gosec

	ch := make(chan json.RawMessage, 1)
	b.mu.Lock()
	b.queries[id] = ch
	b.mu.Unlock()

	defer func() {
		b.mu.Lock()
		delete(b.queries, id)
		b.mu.Unlock()
	}()

	payload, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("scout: bridge query marshal: %w", err)
	}

	js := fmt.Sprintf(`function() { if (window.__scoutDispatch) window.__scoutDispatch(%q, %s, %q) }`, method, string(payload), id)
	if _, err := b.page.Eval(js); err != nil {
		return nil, fmt.Errorf("scout: bridge query send: %w", err)
	}

	select {
	case result := <-ch:
		return result, nil
	case <-time.After(b.opts.queryTimeout):
		return nil, fmt.Errorf("scout: bridge query %q: timeout after %s", method, b.opts.queryTimeout)
	}
}

func (b *Bridge) handleQueryResponse(data json.RawMessage) {
	var resp struct {
		ID     string          `json:"id"`
		Result json.RawMessage `json:"result"`
		Error  *string         `json:"error"`
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return
	}

	b.mu.RLock()
	ch, ok := b.queries[resp.ID]
	b.mu.RUnlock()

	if ok {
		ch <- resp.Result
	}
}

// writeBridgeExtension writes the embedded bridge extension to a temp directory
// and returns its path. The caller should ensure cleanup.
func writeBridgeExtension() (string, error) {
	dir, err := os.MkdirTemp("", "scout-bridge-*")
	if err != nil {
		return "", fmt.Errorf("scout: create bridge temp dir: %w", err)
	}

	files := map[string]string{
		"manifest.json": bridgeManifestJSON,
		"content.js":    bridgeContentJS,
		"background.js": bridgeBackgroundJS,
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			_ = os.RemoveAll(dir)
			return "", fmt.Errorf("scout: write bridge %s: %w", name, err)
		}
	}

	return dir, nil
}
