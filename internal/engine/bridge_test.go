package engine

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
</body></html>`)
		})

		mux.HandleFunc("/bridge-dom", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>DOM Test</title></head>
<body>
<main>
<h1>Hello World</h1>
<p>This is a <strong>test</strong> paragraph with a <a href="https://example.com">link</a>.</p>
<ul>
<li>Item one</li>
<li>Item two</li>
</ul>
<div id="scoped"><h2>Scoped Section</h2><p>Inner content.</p></div>
</main>
<script>var x = 1;</script>
</body></html>`)
		})
	})
}

// injectScoutAPI injects the bridge content script API into the page via Eval,
// since the Chrome extension content script does not load on httptest origins.
// This simulates what the real content script does in production.
func injectScoutAPI(p *Page) {
	_, _ = p.Eval(`function() {
		if (window.__scout) return;
		var handlers = {};
		var mutationObserver = null;
		window.__scout = {
			send: function(type, data) {
				if (typeof window.__scoutSend === 'function') {
					window.__scoutSend(JSON.stringify({type: type, data: data !== undefined ? data : null, ts: Date.now()}));
				}
			},
			on: function(type, handler) {
				if (!handlers[type]) handlers[type] = [];
				handlers[type].push(handler);
			},
			off: function(type) { delete handlers[type]; },
			observeMutations: function(selector) {
				if (mutationObserver) mutationObserver.disconnect();
				var target = selector ? document.querySelector(selector) : document.body;
				if (!target) return;
				mutationObserver = new MutationObserver(function(mutations) {
					var summary = mutations.map(function(m) {
						return {type: m.type, target: m.target.nodeName, addedNodes: m.addedNodes.length, removedNodes: m.removedNodes.length, attributeName: m.attributeName || null, oldValue: m.oldValue || null};
					});
					window.__scout.send('mutation', summary);
				});
				mutationObserver.observe(target, {childList: true, attributes: true, characterData: true, subtree: true, attributeOldValue: true});
			},
			stopMutations: function() { if (mutationObserver) { mutationObserver.disconnect(); mutationObserver = null; } },
			available: function() { return typeof window.__scoutSend === 'function'; }
		};
		window.addEventListener('__scoutCommand', function(e) {
			var detail = e.detail;
			if (!detail || !detail.type) return;
			var fns = handlers[detail.type];
			if (fns) {
				for (var i = 0; i < fns.length; i++) {
					try {
						var result = fns[i](detail.data);
						if (detail.id) {
							window.__scout.send('__query_response', {id: detail.id, result: result !== undefined ? result : null, error: null});
						}
					} catch (err) {
						if (detail.id) {
							window.__scout.send('__query_response', {id: detail.id, result: null, error: err.message || String(err)});
						}
					}
				}
			}
		});
		// Built-in handler: dom_json
		window.__scout.on('dom_json', function(params) {
			params = params || {};
			var selector = params.selector || null;
			var maxDepth = params.depth || 50;
			var skipTags = {SCRIPT:1, STYLE:1, NOSCRIPT:1};
			function walk(node, depth) {
				if (depth > maxDepth) return null;
				if (node.nodeType === Node.TEXT_NODE) {
					var text = node.textContent.trim();
					if (!text) return null;
					return {tag:'#text', text: text};
				}
				if (node.nodeType !== Node.ELEMENT_NODE) return null;
				var tag = node.tagName;
				if (skipTags[tag]) return null;
				var obj = {tag: tag.toLowerCase()};
				if (node.attributes && node.attributes.length > 0) {
					obj.attributes = {};
					for (var i = 0; i < node.attributes.length; i++) {
						obj.attributes[node.attributes[i].name] = node.attributes[i].value;
					}
				}
				var children = [];
				for (var c = node.firstChild; c; c = c.nextSibling) {
					var child = walk(c, depth + 1);
					if (child) children.push(child);
				}
				if (children.length > 0) obj.children = children;
				return obj;
			}
			var root = selector ? document.querySelector(selector) : document.documentElement;
			if (!root) return {error: 'selector not found: ' + selector};
			return walk(root, 0);
		});
		// Built-in handler: dom_markdown
		window.__scout.on('dom_markdown', function(params) {
			params = params || {};
			var selector = params.selector || null;
			var mainOnly = params.mainOnly || false;
			function findMain(doc) {
				var c = [doc.querySelector('main'), doc.querySelector('article'), doc.querySelector('[role="main"]'), doc.querySelector('#content'), doc.querySelector('.content')];
				for (var i = 0; i < c.length; i++) { if (c[i]) return c[i]; }
				return doc.body || doc.documentElement;
			}
			var root;
			if (selector) { root = document.querySelector(selector); if (!root) return '<!-- selector not found -->'; }
			else if (mainOnly) { root = findMain(document); }
			else { root = document.body || document.documentElement; }
			var skipTags = {SCRIPT:1, STYLE:1, NOSCRIPT:1, SVG:1};
			var blockTags = {P:1, DIV:1, SECTION:1, ARTICLE:1, ASIDE:1, HEADER:1, FOOTER:1, NAV:1, MAIN:1, BLOCKQUOTE:1};
			function convert(node) {
				if (node.nodeType === Node.TEXT_NODE) return node.textContent;
				if (node.nodeType !== Node.ELEMENT_NODE) return '';
				var tag = node.tagName;
				if (skipTags[tag]) return '';
				var inner = '';
				for (var c = node.firstChild; c; c = c.nextSibling) inner += convert(c);
				inner = inner.replace(/\n{3,}/g, '\n\n');
				switch(tag) {
					case 'H1': return '\n\n# ' + inner.trim() + '\n\n';
					case 'H2': return '\n\n## ' + inner.trim() + '\n\n';
					case 'H3': return '\n\n### ' + inner.trim() + '\n\n';
					case 'P': return '\n\n' + inner.trim() + '\n\n';
					case 'STRONG': case 'B': return '**' + inner.trim() + '**';
					case 'EM': case 'I': return '*' + inner.trim() + '*';
					case 'A': return '[' + inner.trim() + '](' + (node.getAttribute('href')||'') + ')';
					case 'UL': return '\n\n' + inner + '\n';
					case 'LI': return '- ' + inner.trim() + '\n';
					case 'CODE': var bt = String.fromCharCode(96); return bt + inner.trim() + bt;
					case 'PRE': var bt3 = String.fromCharCode(96).repeat(3); return '\n\n' + bt3 + '\n' + inner.trim() + '\n' + bt3 + '\n\n';
					default:
						if (blockTags[tag]) return '\n\n' + inner.trim() + '\n\n';
						return inner;
				}
			}
			var result = convert(root);
			return result.replace(/\n{3,}/g, '\n\n').trim();
		});
		window.__scout.send('__bridge_ready', {url: window.location.href});
	}`)
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

	// Inject the __scout API (simulates the content script for httptest origins).
	injectScoutAPI(page)

	// Give the __bridge_ready event time to arrive.
	time.Sleep(500 * time.Millisecond)

	if !bridge.Available() {
		t.Error("expected bridge.Available()=true after injecting scout API")
	} else {
		t.Log("bridge available: true")
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

	// Inject the content script API so Go→Browser→Go works.
	injectScoutAPI(page)
	time.Sleep(200 * time.Millisecond)

	// Register a handler in the browser that responds to 'greet' with a 'greeting' event.
	_, _ = page.Eval(`function() {
		window.__scout.on('greet', function() {
			window.__scout.send('greeting', {message: 'hello from browser'});
		});
	}`)

	var (
		mu       sync.Mutex
		received []json.RawMessage
	)

	bridge.On("greeting", func(data json.RawMessage) {
		mu.Lock()

		received = append(received, data)
		mu.Unlock()
	})

	// Send a greet command — page JS handler will respond with a greeting event.
	if err := bridge.Send("greet", nil); err != nil {
		t.Fatalf("bridge send: %v", err)
	}

	// Wait for the event roundtrip.
	time.Sleep(1 * time.Second)

	mu.Lock()
	count := len(received)
	mu.Unlock()

	if count == 0 {
		t.Fatal("expected at least 1 greeting event, got 0")
	}

	var msg struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(received[0], &msg); err != nil {
		t.Fatalf("unmarshal greeting: %v", err)
	}

	if msg.Message != "hello from browser" {
		t.Errorf("expected 'hello from browser', got %q", msg.Message)
	}

	t.Logf("received %d greeting events, message=%q", count, msg.Message)
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

	// Inject the content script API.
	injectScoutAPI(page)
	time.Sleep(200 * time.Millisecond)

	ch := make(chan []MutationEvent, 1)

	bridge.OnMutation(func(events []MutationEvent) {
		select {
		case ch <- events:
		default:
		}
	})

	// Start observing via the injected __scout API.
	_, _ = page.Eval(`function() { window.__scout.observeMutations('#target') }`)

	// Trigger a DOM mutation.
	time.Sleep(200 * time.Millisecond)

	_, _ = page.Eval(`function() { document.getElementById('target').textContent = 'Mutated' }`)

	select {
	case mutations := <-ch:
		if len(mutations) == 0 {
			t.Fatal("expected at least one mutation")
		}

		t.Logf("received %d mutations, first type=%q target=%q", len(mutations), mutations[0].Type, mutations[0].Target)
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for mutations")
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

func TestBridgeFullRoundtrip(t *testing.T) {
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

	bridge, err := page.Bridge(WithQueryTimeout(5 * time.Second))
	if err != nil {
		t.Fatalf("bridge init: %v", err)
	}

	// Inject the content script API.
	injectScoutAPI(page)
	time.Sleep(300 * time.Millisecond)

	t.Run("available", func(t *testing.T) {
		if !bridge.Available() {
			t.Fatal("bridge should be available after injecting scout API")
		}
	})

	t.Run("go_to_browser_to_go", func(t *testing.T) {
		// Register browser-side handler: on 'compute', return sum.
		_, _ = page.Eval(`function() {
			window.__scout.on('compute', function(data) {
				window.__scout.send('result', {sum: data.a + data.b});
			});
		}`)

		ch := make(chan json.RawMessage, 1)

		bridge.On("result", func(data json.RawMessage) {
			ch <- data
		})

		if err := bridge.Send("compute", map[string]int{"a": 17, "b": 25}); err != nil {
			t.Fatalf("send compute: %v", err)
		}

		select {
		case data := <-ch:
			var result struct {
				Sum int `json:"sum"`
			}
			if err := json.Unmarshal(data, &result); err != nil {
				t.Fatalf("unmarshal result: %v", err)
			}

			if result.Sum != 42 {
				t.Errorf("expected sum=42, got %d", result.Sum)
			}

			t.Logf("Go→Browser→Go: sent compute(17+25), got result=%d", result.Sum)
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for compute result")
		}

		bridge.Off("result")
	})

	t.Run("query_roundtrip", func(t *testing.T) {
		// Register a query handler in the browser.
		_, _ = page.Eval(`function() {
			window.__scout.on('reverse', function(data) {
				return data.text.split('').reverse().join('');
			});
		}`)

		result, err := bridge.Query("reverse", map[string]string{"text": "scout"})
		if err != nil {
			t.Fatalf("query reverse: %v", err)
		}

		var reversed string
		if err := json.Unmarshal(result, &reversed); err != nil {
			t.Fatalf("unmarshal reverse result: %v", err)
		}

		if reversed != "tuocs" {
			t.Errorf("expected 'tuocs', got %q", reversed)
		}

		t.Logf("Query roundtrip: reverse('scout') = %q", reversed)
	})

	t.Run("multiple_handlers", func(t *testing.T) {
		var (
			mu     sync.Mutex
			counts [2]int
		)

		bridge.On("multi", func(_ json.RawMessage) {
			mu.Lock()
			counts[0]++
			mu.Unlock()
		})
		bridge.On("multi", func(_ json.RawMessage) {
			mu.Lock()
			counts[1]++
			mu.Unlock()
		})

		// Fire the event from browser.
		_, _ = page.Eval(`function() { window.__scout.send('multi', {n: 1}) }`)

		time.Sleep(500 * time.Millisecond)

		mu.Lock()
		c0, c1 := counts[0], counts[1]
		mu.Unlock()

		if c0 != 1 || c1 != 1 {
			t.Errorf("expected both handlers called once, got [%d, %d]", c0, c1)
		}

		t.Logf("Multiple handlers: both called %d/%d times", c0, c1)

		bridge.Off("multi")
	})

	t.Run("off_unregisters", func(t *testing.T) {
		called := false

		bridge.On("ephemeral", func(_ json.RawMessage) {
			called = true
		})
		bridge.Off("ephemeral")

		_, _ = page.Eval(`function() { window.__scout.send('ephemeral', null) }`)

		time.Sleep(500 * time.Millisecond)

		if called {
			t.Error("handler called after Off()")
		}
	})

	t.Run("mutation_with_attribute", func(t *testing.T) {
		ch := make(chan []MutationEvent, 1)

		bridge.OnMutation(func(events []MutationEvent) {
			select {
			case ch <- events:
			default:
			}
		})

		_, _ = page.Eval(`function() { window.__scout.observeMutations('#target') }`)

		time.Sleep(200 * time.Millisecond)

		// Change an attribute to trigger a mutation.
		_, _ = page.Eval(`function() { document.getElementById('target').setAttribute('data-test', 'hello') }`)

		select {
		case mutations := <-ch:
			found := false

			for _, m := range mutations {
				if m.Type == "attributes" && m.AttributeName == "data-test" {
					found = true

					t.Logf("Mutation: type=%q attr=%q on %s", m.Type, m.AttributeName, m.Target)
				}
			}

			if !found {
				t.Errorf("expected attribute mutation for data-test, got %+v", mutations)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for attribute mutation")
		}

		bridge.Off("mutation")
	})
}

func TestBridgeDOMJSON(t *testing.T) {
	browser := newBridgeBrowser(t)

	ts := newTestServer()
	defer ts.Close()

	page, err := browser.NewPage(ts.URL + "/bridge-dom")
	if err != nil {
		t.Fatalf("new page: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("wait load: %v", err)
	}

	bridge, err := page.Bridge(WithQueryTimeout(5 * time.Second))
	if err != nil {
		t.Fatalf("bridge init: %v", err)
	}

	injectScoutAPI(page)
	time.Sleep(300 * time.Millisecond)

	node, err := bridge.DOM()
	if err != nil {
		t.Fatalf("DOM(): %v", err)
	}

	if node.Tag != "html" {
		t.Errorf("expected root tag 'html', got %q", node.Tag)
	}

	if len(node.Children) == 0 {
		t.Fatal("expected children on html node")
	}

	t.Logf("DOM root: tag=%s, children=%d", node.Tag, len(node.Children))
}

func TestBridgeDOMMarkdown(t *testing.T) {
	browser := newBridgeBrowser(t)

	ts := newTestServer()
	defer ts.Close()

	page, err := browser.NewPage(ts.URL + "/bridge-dom")
	if err != nil {
		t.Fatalf("new page: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("wait load: %v", err)
	}

	bridge, err := page.Bridge(WithQueryTimeout(5 * time.Second))
	if err != nil {
		t.Fatalf("bridge init: %v", err)
	}

	injectScoutAPI(page)
	time.Sleep(300 * time.Millisecond)

	md, err := bridge.DOMMarkdown()
	if err != nil {
		t.Fatalf("DOMMarkdown(): %v", err)
	}

	if md == "" {
		t.Fatal("expected non-empty markdown")
	}

	// Check for expected content.
	for _, want := range []string{"# Hello World", "**test**", "[link](https://example.com)", "- Item one"} {
		if !containsStr(md, want) {
			t.Errorf("expected markdown to contain %q, got:\n%s", want, md)
		}
	}

	t.Logf("Markdown (%d chars):\n%s", len(md), md)
}

func TestBridgeDOMSelector(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping browser test in -short mode")
	}
	browser := newBridgeBrowser(t)

	ts := newTestServer()
	defer ts.Close()

	page, err := browser.NewPage(ts.URL + "/bridge-dom")
	if err != nil {
		t.Fatalf("new page: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("wait load: %v", err)
	}

	bridge, err := page.Bridge(WithQueryTimeout(5 * time.Second))
	if err != nil {
		t.Fatalf("bridge init: %v", err)
	}

	injectScoutAPI(page)
	time.Sleep(300 * time.Millisecond)

	t.Run("json_scoped", func(t *testing.T) {
		node, err := bridge.DOM(WithDOMSelector("#scoped"))
		if err != nil {
			t.Fatalf("DOM(#scoped): %v", err)
		}

		if node.Tag != "div" {
			t.Errorf("expected tag 'div', got %q", node.Tag)
		}

		t.Logf("scoped DOM: tag=%s, children=%d", node.Tag, len(node.Children))
	})

	t.Run("markdown_scoped", func(t *testing.T) {
		md, err := bridge.DOMMarkdown(WithDOMSelector("#scoped"))
		if err != nil {
			t.Fatalf("DOMMarkdown(#scoped): %v", err)
		}

		if !containsStr(md, "## Scoped Section") {
			t.Errorf("expected '## Scoped Section' in markdown, got:\n%s", md)
		}

		if !containsStr(md, "Inner content.") {
			t.Errorf("expected 'Inner content.' in markdown, got:\n%s", md)
		}

		t.Logf("scoped markdown:\n%s", md)
	})

	t.Run("markdown_main_only", func(t *testing.T) {
		md, err := bridge.DOMMarkdown(WithDOMMainOnly())
		if err != nil {
			t.Fatalf("DOMMarkdown(mainOnly): %v", err)
		}

		if !containsStr(md, "# Hello World") {
			t.Errorf("expected '# Hello World' in main content markdown, got:\n%s", md)
		}

		t.Logf("main-only markdown:\n%s", md)
	})
}

func TestBridgeWithoutExtension(t *testing.T) {
	// Create a browser WITHOUT bridge to verify graceful behavior.
	t.Helper()

	b, err := New(
		WithHeadless(true),
		WithNoSandbox(),
		WithoutBridge(),
		WithTimeout(30*time.Second),
	)
	if err != nil {
		t.Skipf("skipping: browser unavailable: %v", err)
	}

	defer func() { _ = b.Close() }()

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
