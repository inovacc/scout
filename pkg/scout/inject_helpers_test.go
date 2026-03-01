package scout

import (
	"fmt"
	"net/http"
	"testing"
)

func init() {
	registerTestRoutes(func(mux *http.ServeMux) {
		mux.HandleFunc("/inject-helpers", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Inject Helpers Test</title></head>
<body>
<table id="data-table">
  <thead><tr><th>Name</th><th>Age</th></tr></thead>
  <tbody>
    <tr><td>Alice</td><td>30</td></tr>
    <tr><td>Bob</td><td>25</td></tr>
  </tbody>
</table>
<div id="shadow-host"></div>
<ul id="items">
  <li class="item"><span class="name">Item A</span><span class="price">$10</span></li>
  <li class="item"><span class="name">Item B</span><span class="price">$20</span></li>
</ul>
<button class="btn">One</button>
<button class="btn">Two</button>
<button class="btn">Three</button>
<div id="click-count">0</div>
<form id="test-form">
  <input name="username" value="">
  <input name="email" value="">
</form>
<script>
var host = document.getElementById('shadow-host');
var shadow = host.attachShadow({mode: 'open'});
shadow.innerHTML = '<p class="shadow-text">Inside Shadow</p>';

var clickCount = 0;
document.querySelectorAll('.btn').forEach(function(btn) {
  btn.addEventListener('click', function() {
    clickCount++;
    document.getElementById('click-count').textContent = String(clickCount);
  });
});
</script>
</body></html>`)
		})
	})
}

func TestHelperTableExtract(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	b := newTestBrowser(t)
	defer func() { _ = b.Close() }()

	page, err := b.NewPage(ts.URL + "/inject-helpers")
	if err != nil {
		t.Fatalf("NewPage: %v", err)
	}
	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad: %v", err)
	}

	if err := InjectHelper(page, HelperTableExtract); err != nil {
		t.Fatalf("InjectHelper: %v", err)
	}

	result, err := page.Eval(`JSON.stringify(window.__scout.extractTables())`)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}

	s := result.String()
	if s == "" || s == "[]" {
		t.Fatal("expected non-empty table extraction result")
	}

	// Verify headers and data are present.
	for _, want := range []string{"Name", "Age", "Alice", "30", "Bob", "25"} {
		if !contains(s, want) {
			t.Errorf("result missing %q: %s", want, s)
		}
	}
}

func TestHelperShadowQuery(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	b := newTestBrowser(t)
	defer func() { _ = b.Close() }()

	page, err := b.NewPage(ts.URL + "/inject-helpers")
	if err != nil {
		t.Fatalf("NewPage: %v", err)
	}
	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad: %v", err)
	}

	if err := InjectHelper(page, HelperShadowQuery); err != nil {
		t.Fatalf("InjectHelper: %v", err)
	}

	result, err := page.Eval(`(function() {
		var el = window.__scout.shadowQuery('.shadow-text');
		return el ? el.textContent : 'not found';
	})()`)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}

	if got := result.String(); got != "Inside Shadow" {
		t.Errorf("expected 'Inside Shadow', got %q", got)
	}
}

func TestHelperClickAll(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	b := newTestBrowser(t)
	defer func() { _ = b.Close() }()

	page, err := b.NewPage(ts.URL + "/inject-helpers")
	if err != nil {
		t.Fatalf("NewPage: %v", err)
	}
	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad: %v", err)
	}

	if err := InjectHelper(page, HelperClickAll); err != nil {
		t.Fatalf("InjectHelper: %v", err)
	}

	result, err := page.Eval(`window.__scout.clickAll('.btn')`)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}

	if got := result.Int(); got != 3 {
		t.Errorf("expected 3 clicks, got %d", got)
	}
}

func TestInjectAllHelpers(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	b := newTestBrowser(t)
	defer func() { _ = b.Close() }()

	page, err := b.NewPage(ts.URL + "/inject-helpers")
	if err != nil {
		t.Fatalf("NewPage: %v", err)
	}
	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad: %v", err)
	}

	if err := InjectAllHelpers(page); err != nil {
		t.Fatalf("InjectAllHelpers: %v", err)
	}

	// Verify all functions exist under window.__scout.
	for _, fn := range []string{"extractTables", "infiniteScroll", "shadowQuery", "shadowQueryAll", "waitForSelector", "clickAll"} {
		result, err := page.Eval(fmt.Sprintf(`typeof window.__scout.%s`, fn))
		if err != nil {
			t.Fatalf("Eval typeof %s: %v", fn, err)
		}
		if got := result.String(); got != "function" {
			t.Errorf("window.__scout.%s type = %q, want 'function'", fn, got)
		}
	}
}

func TestRenderTemplate(t *testing.T) {
	tests := []struct {
		name    string
		tmpl    string
		data    map[string]any
		wantSub string
		wantErr bool
	}{
		{
			name:    "extract-list renders container",
			tmpl:    "extract-list",
			data:    map[string]any{"container": "#items", "item": ".item", "fields": map[string]any{"name": ".name"}},
			wantSub: `#items`,
		},
		{
			name:    "fill-form renders fields",
			tmpl:    "fill-form",
			data:    map[string]any{"fields": map[string]any{"username": "alice"}},
			wantSub: `alice`,
		},
		{
			name:    "scroll-and-collect renders selector",
			tmpl:    "scroll-and-collect",
			data:    map[string]any{"selector": ".item", "maxScrolls": 5, "delayMs": 200},
			wantSub: `.item`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpl, ok := BuiltinTemplates[tc.tmpl]
			if !ok {
				t.Fatalf("template %q not found", tc.tmpl)
			}
			got, err := RenderTemplate(tmpl, tc.data)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("RenderTemplate: %v", err)
			}
			if !contains(got, tc.wantSub) {
				t.Errorf("rendered template missing %q:\n%s", tc.wantSub, got)
			}
		})
	}
}

func TestRenderTemplate_Invalid(t *testing.T) {
	bad := ScriptTemplate{Name: "bad", Template: `{{.missing | badFunc}}`}
	_, err := RenderTemplate(bad, map[string]any{})
	if err == nil {
		t.Fatal("expected error for invalid template")
	}
}

func TestInjectTemplate_NotFound(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	b := newTestBrowser(t)
	defer func() { _ = b.Close() }()

	page, err := b.NewPage(ts.URL + "/inject-helpers")
	if err != nil {
		t.Fatalf("NewPage: %v", err)
	}

	_, err = InjectTemplate(page, "nonexistent-template", nil)
	if err == nil {
		t.Fatal("expected error for unknown template")
	}
}

func TestInjectTemplate_ExtractList(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	b := newTestBrowser(t)
	defer func() { _ = b.Close() }()

	page, err := b.NewPage(ts.URL + "/inject-helpers")
	if err != nil {
		t.Fatalf("NewPage: %v", err)
	}
	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad: %v", err)
	}

	result, err := InjectTemplate(page, "extract-list", map[string]any{
		"container": "#items",
		"item":      ".item",
		"fields":    map[string]any{"name": ".name", "price": ".price"},
	})
	if err != nil {
		t.Fatalf("InjectTemplate: %v", err)
	}

	s := result.String()
	for _, want := range []string{"Item A", "Item B", "$10", "$20"} {
		if !contains(s, want) {
			t.Errorf("result missing %q: %s", want, s)
		}
	}
}
