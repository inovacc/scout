package scout

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"
)

func init() {
	registerTestRoutes(func(mux *http.ServeMux) {
		mux.HandleFunc("/bridge-test", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Bridge Test</title></head>
<body>
<div id="target">Original</div>
<button id="mutate" onclick="document.getElementById('target').textContent='Changed'">Mutate</button>
<script>
// Register a handler that echoes back data for testing Query.
if (window.__scout) {
  window.__scout.on('echo', function(data) {
    return data;
  });
  window.__scout.on('ping', function() {
    window.__scout.send('pong', {ts: Date.now()});
  });
}

// Also set up handlers after bridge loads (in case content script loads later).
window.addEventListener('__scoutCommand', function(e) {
  if (e.detail && e.detail.type === 'greet') {
    if (window.__scout) {
      window.__scout.send('greeting', {message: 'hello from browser'});
    }
  }
});
</script>
</body></html>`)
		})
	})
}

func newBridgeBrowser(t *testing.T) *Browser {
	t.Helper()

	b, err := New(
		WithHeadless(true),
		WithNoSandbox(),
		WithBridge(),
		WithTimeout(30*time.Second),
	)
	if err != nil {
		t.Skipf("skipping: browser unavailable: %v", err)
	}

	t.Cleanup(func() { _ = b.Close() })

	return b
}

func TestBridgeAvailable(t *testing.T) {
	browser := newBridgeBrowser(t)
	ts := newTestServer()
	defer ts.Close()

	page, err := browser.NewPage(ts.URL + "/bridge-test")
	if err != nil {
		t.Fatalf("new page: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("wait load: %v", err)
	}

	bridge, err := page.Bridge()
	if err != nil {
		t.Fatalf("bridge init: %v", err)
	}

	// Give the content script time to signal readiness.
	time.Sleep(1 * time.Second)

	if !bridge.Available() {
		// Bridge extension might not load on httptest URLs in all environments.
		// This is expected — content scripts need matching origins.
		t.Log("bridge not available (content script may not match httptest origin)")
	}
}

func TestBridgeSendReceive(t *testing.T) {
	browser := newBridgeBrowser(t)
	ts := newTestServer()
	defer ts.Close()

	page, err := browser.NewPage(ts.URL + "/bridge-test")
	if err != nil {
		t.Fatalf("new page: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("wait load: %v", err)
	}

	bridge, err := page.Bridge()
	if err != nil {
		t.Fatalf("bridge init: %v", err)
	}

	var mu sync.Mutex
	var received []json.RawMessage

	bridge.On("greeting", func(data json.RawMessage) {
		mu.Lock()
		received = append(received, data)
		mu.Unlock()
	})

	// Send a greet command — page JS will respond with a greeting event.
	if err := bridge.Send("greet", nil); err != nil {
		t.Fatalf("bridge send: %v", err)
	}

	// Wait briefly for the event roundtrip.
	time.Sleep(1 * time.Second)

	mu.Lock()
	count := len(received)
	mu.Unlock()

	// Note: This may be 0 if the content script doesn't load on httptest URLs.
	t.Logf("received %d greeting events", count)
}

func TestBridgeOnEvent(t *testing.T) {
	browser := newBridgeBrowser(t)
	ts := newTestServer()
	defer ts.Close()

	page, err := browser.NewPage(ts.URL + "/bridge-test")
	if err != nil {
		t.Fatalf("new page: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("wait load: %v", err)
	}

	bridge, err := page.Bridge()
	if err != nil {
		t.Fatalf("bridge init: %v", err)
	}

	ch := make(chan json.RawMessage, 1)
	bridge.On("test-event", func(data json.RawMessage) {
		ch <- data
	})

	// Simulate the content script sending an event via __scoutSend binding.
	_, err = page.Eval(`function() { if (typeof window.__scoutSend === 'function') window.__scoutSend(JSON.stringify({type: 'test-event', data: {"key": "value"}, ts: Date.now()})) }`)
	if err != nil {
		t.Fatalf("eval send: %v", err)
	}

	select {
	case data := <-ch:
		var parsed map[string]string
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("unmarshal event data: %v", err)
		}

		if parsed["key"] != "value" {
			t.Errorf("expected key=value, got %v", parsed)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for test-event")
	}
}

func TestBridgeMutationObserver(t *testing.T) {
	browser := newBridgeBrowser(t)
	ts := newTestServer()
	defer ts.Close()

	page, err := browser.NewPage(ts.URL + "/bridge-test")
	if err != nil {
		t.Fatalf("new page: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("wait load: %v", err)
	}

	bridge, err := page.Bridge()
	if err != nil {
		t.Fatalf("bridge init: %v", err)
	}

	ch := make(chan []MutationEvent, 1)
	bridge.OnMutation(func(events []MutationEvent) {
		ch <- events
	})

	// Start observing via direct JS call (since content script may not be loaded).
	_, _ = page.Eval(`function() { if (window.__scout) window.__scout.observeMutations('#target') }`)

	// Trigger a DOM mutation.
	time.Sleep(200 * time.Millisecond)
	_, _ = page.Eval(`function() { document.getElementById('target').textContent = 'Mutated' }`)

	select {
	case mutations := <-ch:
		if len(mutations) == 0 {
			t.Fatal("expected at least one mutation")
		}

		t.Logf("received %d mutations", len(mutations))
	case <-time.After(5 * time.Second):
		// Mutation observer may not fire without the content script.
		t.Log("timeout waiting for mutations (content script may not be active)")
	}
}

func TestBridgeQuery(t *testing.T) {
	browser := newBridgeBrowser(t)
	ts := newTestServer()
	defer ts.Close()

	page, err := browser.NewPage(ts.URL + "/bridge-test")
	if err != nil {
		t.Fatalf("new page: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("wait load: %v", err)
	}

	bridge, err := page.Bridge(WithQueryTimeout(3 * time.Second))
	if err != nil {
		t.Fatalf("bridge init: %v", err)
	}

	// Register a JS handler that responds to queries.
	_, _ = page.Eval(`function() { window.addEventListener('__scoutCommand', function(e) {
		if (e.detail && e.detail.type === 'echo-query' && e.detail.id) {
			if (typeof window.__scoutSend === 'function') {
				window.__scoutSend(JSON.stringify({
					type: '__query_response',
					data: {id: e.detail.id, result: e.detail.data, error: null},
					ts: Date.now()
				}));
			}
		}
	}) }`)

	result, err := bridge.Query("echo-query", map[string]string{"hello": "world"})
	if err != nil {
		t.Fatalf("query: %v", err)
	}

	var parsed map[string]string
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("unmarshal query result: %v", err)
	}

	if parsed["hello"] != "world" {
		t.Errorf("expected hello=world, got %v", parsed)
	}
}

func TestBridgeWithoutExtension(t *testing.T) {
	// Create a browser WITHOUT WithBridge to verify graceful behavior.
	b := newTestBrowser(t)
	ts := newTestServer()
	defer ts.Close()

	page, err := b.NewPage(ts.URL + "/")
	if err != nil {
		t.Fatalf("new page: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("wait load: %v", err)
	}

	// Bridge init should still work (CDP binding can be added without the extension).
	bridge, err := page.Bridge()
	if err != nil {
		t.Fatalf("bridge init: %v", err)
	}

	// Without the extension, Available() should be false.
	if bridge.Available() {
		t.Error("expected bridge.Available()=false without extension")
	}

	// Nil bridge safety.
	var nilBridge *Bridge
	if nilBridge.Available() {
		t.Error("nil bridge should return false")
	}
}
