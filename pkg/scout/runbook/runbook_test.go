package runbook

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/rod/lib/input"
)

// newTestBrowser creates a headless browser for testing. Skips if unavailable.
func newTestBrowser(t *testing.T) *scout.Browser {
	t.Helper()
	b, err := scout.New(scout.WithHeadless(true), scout.WithNoSandbox(), scout.WithoutBridge())
	if err != nil {
		t.Skipf("browser unavailable: %v", err)
	}
	t.Cleanup(func() { b.Close() })
	return b
}

// newTestServer creates an httptest server with runbook test fixture routes.
func newTestServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/runbook-extract", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<html><body>
			<div class="item"><h2>Item A</h2><a href="/a">Link A</a><span class="price">$10</span></div>
			<div class="item"><h2>Item B</h2><a href="/b">Link B</a><span class="price">$20</span></div>
			<div class="item"><h2>Item C</h2><a href="/c">Link C</a><span class="price">$30</span></div>
			<p class="total">3 items</p>
		</body></html>`)
	})

	mux.HandleFunc("/runbook-extract-page2", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<html><body>
			<div class="item"><h2>Item D</h2><a href="/d">Link D</a></div>
			<div class="item"><h2>Item E</h2><a href="/e">Link E</a></div>
		</body></html>`)
	})

	mux.HandleFunc("/runbook-paginated", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<html><body>
			<div class="item"><h2>Page1 Item</h2></div>
			<a class="next" href="/runbook-paginated-2">Next</a>
		</body></html>`)
	})

	mux.HandleFunc("/runbook-paginated-2", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<html><body>
			<div class="item"><h2>Page2 Item</h2></div>
		</body></html>`)
	})

	mux.HandleFunc("/runbook-dedup", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<html><body>
			<div class="item"><h2>Dup</h2></div>
			<div class="item"><h2>Dup</h2></div>
			<div class="item"><h2>Unique</h2></div>
		</body></html>`)
	})

	mux.HandleFunc("/runbook-auto", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<html><body>
			<input id="name" type="text" />
			<button id="btn" onclick="document.getElementById('result').textContent='clicked'">Go</button>
			<div id="result">pending</div>
			<span class="info">hello</span>
			<span class="info">world</span>
		</body></html>`)
	})

	mux.HandleFunc("/runbook-wait", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<html><body>
			<div class="item"><h2>Waited</h2></div>
		</body></html>`)
	})

	return httptest.NewServer(mux)
}

// ── Parse & Validate tests (existing) ──────────────────────────────────────

func TestParse_ExtractRunbook(t *testing.T) {
	data := []byte(`{
		"version": "1",
		"name": "test_extract",
		"type": "extract",
		"url": "https://example.com",
		"wait_for": ".item",
		"items": {
			"container": ".item",
			"fields": {
				"title": "h2",
				"link": "a@href"
			}
		}
	}`)

	r, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if r.Name != "test_extract" {
		t.Errorf("name = %q, want %q", r.Name, "test_extract")
	}
	if r.Type != "extract" {
		t.Errorf("type = %q, want %q", r.Type, "extract")
	}
	if r.Items.Container != ".item" {
		t.Errorf("container = %q, want %q", r.Items.Container, ".item")
	}
	if len(r.Items.Fields) != 2 {
		t.Errorf("fields count = %d, want 2", len(r.Items.Fields))
	}
}

func TestParse_AutomateRunbook(t *testing.T) {
	data := []byte(`{
		"version": "1",
		"name": "test_automate",
		"type": "automate",
		"steps": [
			{"action": "navigate", "url": "https://example.com"},
			{"action": "click", "selector": "#btn"},
			{"action": "screenshot", "name": "result"}
		]
	}`)

	r, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if r.Type != "automate" {
		t.Errorf("type = %q, want %q", r.Type, "automate")
	}
	if len(r.Steps) != 3 {
		t.Errorf("steps count = %d, want 3", len(r.Steps))
	}
	if r.Steps[0].Action != "navigate" {
		t.Errorf("step 0 action = %q, want %q", r.Steps[0].Action, "navigate")
	}
}

func TestValidate_MissingVersion(t *testing.T) {
	r := &Runbook{Name: "test", Type: "extract", URL: "https://x.com",
		Items: &ItemSpec{Container: ".c", Fields: map[string]string{"a": "b"}}}
	if err := r.Validate(); err == nil {
		t.Error("expected error for missing version")
	}
}

func TestValidate_MissingName(t *testing.T) {
	r := &Runbook{Version: "1", Type: "extract", URL: "https://x.com",
		Items: &ItemSpec{Container: ".c", Fields: map[string]string{"a": "b"}}}
	if err := r.Validate(); err == nil {
		t.Error("expected error for missing name")
	}
}

func TestValidate_UnknownType(t *testing.T) {
	r := &Runbook{Version: "1", Name: "test", Type: "unknown"}
	if err := r.Validate(); err == nil {
		t.Error("expected error for unknown type")
	}
}

func TestValidate_ExtractMissingURL(t *testing.T) {
	r := &Runbook{Version: "1", Name: "test", Type: "extract",
		Items: &ItemSpec{Container: ".c", Fields: map[string]string{"a": "b"}}}
	if err := r.Validate(); err == nil {
		t.Error("expected error for extract missing url")
	}
}

func TestValidate_ExtractMissingItems(t *testing.T) {
	r := &Runbook{Version: "1", Name: "test", Type: "extract", URL: "https://x.com"}
	if err := r.Validate(); err == nil {
		t.Error("expected error for extract missing items")
	}
}

func TestValidate_AutomateMissingSteps(t *testing.T) {
	r := &Runbook{Version: "1", Name: "test", Type: "automate"}
	if err := r.Validate(); err == nil {
		t.Error("expected error for automate missing steps")
	}
}

func TestValidate_AutomateStepMissingAction(t *testing.T) {
	r := &Runbook{Version: "1", Name: "test", Type: "automate",
		Steps: []Step{{URL: "https://x.com"}}}
	if err := r.Validate(); err == nil {
		t.Error("expected error for step missing action")
	}
}

func TestParse_InvalidJSON(t *testing.T) {
	_, err := Parse([]byte(`{invalid`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParse_Pagination(t *testing.T) {
	data := []byte(`{
		"version": "1",
		"name": "paginated",
		"type": "extract",
		"url": "https://example.com",
		"items": {
			"container": ".item",
			"fields": {"title": "h2"}
		},
		"pagination": {
			"strategy": "click",
			"next_selector": "a.next",
			"max_pages": 3,
			"delay_ms": 500
		}
	}`)

	r, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if r.Pagination == nil {
		t.Fatal("pagination should not be nil")
	}
	if r.Pagination.Strategy != "click" {
		t.Errorf("strategy = %q, want %q", r.Pagination.Strategy, "click")
	}
	if r.Pagination.MaxPages != 3 {
		t.Errorf("max_pages = %d, want 3", r.Pagination.MaxPages)
	}
}

// ── Validate: missing container and fields ─────────────────────────────────

func TestValidate_ExtractMissingContainer(t *testing.T) {
	r := &Runbook{Version: "1", Name: "test", Type: "extract", URL: "https://x.com",
		Items: &ItemSpec{Fields: map[string]string{"a": "b"}}}
	if err := r.Validate(); err == nil {
		t.Error("expected error for missing container")
	}
}

func TestValidate_ExtractMissingFields(t *testing.T) {
	r := &Runbook{Version: "1", Name: "test", Type: "extract", URL: "https://x.com",
		Items: &ItemSpec{Container: ".c"}}
	if err := r.Validate(); err == nil {
		t.Error("expected error for missing fields")
	}
}

// ── mapKeyName ─────────────────────────────────────────────────────────────

func TestMapKeyName(t *testing.T) {
	tests := []struct {
		name string
		want input.Key
	}{
		{"Enter", input.Enter},
		{"Tab", input.Tab},
		{"Escape", input.Escape},
		{"Space", input.Space},
		{"Backspace", input.Backspace},
		{"a", input.Key('a')},
		{"", 0},
		{"UnknownLong", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapKeyName(tt.name)
			if got != tt.want {
				t.Errorf("mapKeyName(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

// ── LoadFile ───────────────────────────────────────────────────────────────

func TestLoadFile_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "runbook.json")
	data := []byte(`{"version":"1","name":"test","type":"extract","url":"https://x.com","items":{"container":".c","fields":{"a":"b"}}}`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}

	r, err := LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}
	if r.Name != "test" {
		t.Errorf("name = %q, want %q", r.Name, "test")
	}
}

func TestLoadFile_NotFound(t *testing.T) {
	_, err := LoadFile("/nonexistent/runbook.json")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoadFile_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte(`{bad`), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadFile(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// ── Run dispatch ───────────────────────────────────────────────────────────

func TestRun_UnknownType(t *testing.T) {
	r := &Runbook{Version: "1", Name: "test", Type: "bogus"}
	_, err := Apply(context.Background(), nil, r)
	if err == nil {
		t.Error("expected error for unknown type")
	}
}

// ── runExtract (browser tests) ─────────────────────────────────────────────

func TestRunExtract_BasicFields(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	r := &Runbook{
		Version: "1", Name: "basic", Type: "extract",
		URL:   ts.URL + "/runbook-extract",
		Items: &ItemSpec{Container: ".item", Fields: map[string]string{"title": "h2", "link": "a@href"}},
	}

	result, err := Apply(context.Background(), b, r)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(result.Items) != 3 {
		t.Fatalf("items count = %d, want 3", len(result.Items))
	}
	if result.Items[0]["title"] != "Item A" {
		t.Errorf("item 0 title = %q, want %q", result.Items[0]["title"], "Item A")
	}
	if result.Items[1]["link"] != "/b" {
		t.Errorf("item 1 link = %q, want %q", result.Items[1]["link"], "/b")
	}
}

func TestRunExtract_SiblingField(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	r := &Runbook{
		Version: "1", Name: "sibling", Type: "extract",
		URL:   ts.URL + "/runbook-extract",
		Items: &ItemSpec{Container: ".item", Fields: map[string]string{"title": "h2", "total": "+.total"}},
	}

	result, err := Apply(context.Background(), b, r)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(result.Items) != 3 {
		t.Fatalf("items count = %d, want 3", len(result.Items))
	}
	// Sibling field resolves from page level so all items get the same value
	if result.Items[0]["total"] != "3 items" {
		t.Errorf("item 0 total = %q, want %q", result.Items[0]["total"], "3 items")
	}
}

func TestRunExtract_SiblingAttribute(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	r := &Runbook{
		Version: "1", Name: "sibling-attr", Type: "extract",
		URL:   ts.URL + "/runbook-extract",
		Items: &ItemSpec{Container: ".item", Fields: map[string]string{"title": "h2", "href": "+a@href"}},
	}

	result, err := Apply(context.Background(), b, r)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(result.Items) < 1 {
		t.Fatal("expected at least 1 item")
	}
	// Sibling attribute: first <a> on the page
	if result.Items[0]["href"] != "/a" {
		t.Errorf("item 0 href = %q, want %q", result.Items[0]["href"], "/a")
	}
}

func TestRunExtract_WaitFor(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	r := &Runbook{
		Version: "1", Name: "wait", Type: "extract",
		URL:     ts.URL + "/runbook-wait",
		WaitFor: ".item",
		Items:   &ItemSpec{Container: ".item", Fields: map[string]string{"title": "h2"}},
	}

	result, err := Apply(context.Background(), b, r)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(result.Items) != 1 {
		t.Fatalf("items count = %d, want 1", len(result.Items))
	}
	if result.Items[0]["title"] != "Waited" {
		t.Errorf("title = %q, want %q", result.Items[0]["title"], "Waited")
	}
}

func TestRunExtract_PaginationClick(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	r := &Runbook{
		Version: "1", Name: "paginated", Type: "extract",
		URL:   ts.URL + "/runbook-paginated",
		Items: &ItemSpec{Container: ".item", Fields: map[string]string{"title": "h2"}},
		Pagination: &Pagination{
			Strategy:     "click",
			NextSelector: "a.next",
			MaxPages:     2,
		},
	}

	result, err := Apply(context.Background(), b, r)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(result.Items) != 2 {
		t.Fatalf("items count = %d, want 2", len(result.Items))
	}
	if result.Items[0]["title"] != "Page1 Item" {
		t.Errorf("item 0 = %q, want %q", result.Items[0]["title"], "Page1 Item")
	}
	if result.Items[1]["title"] != "Page2 Item" {
		t.Errorf("item 1 = %q, want %q", result.Items[1]["title"], "Page2 Item")
	}
}

func TestRunExtract_PaginationScroll(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	r := &Runbook{
		Version: "1", Name: "scroll", Type: "extract",
		URL:   ts.URL + "/runbook-extract",
		Items: &ItemSpec{Container: ".item", Fields: map[string]string{"title": "h2"}},
		Pagination: &Pagination{
			Strategy: "scroll",
			MaxPages: 2,
		},
	}

	result, err := Apply(context.Background(), b, r)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Scroll pagination still collects items from both "pages" (same content since static)
	if len(result.Items) < 3 {
		t.Errorf("expected at least 3 items, got %d", len(result.Items))
	}
}

func TestRunExtract_Deduplication(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	r := &Runbook{
		Version: "1", Name: "dedup", Type: "extract",
		URL:   ts.URL + "/runbook-dedup",
		Items: &ItemSpec{Container: ".item", Fields: map[string]string{"title": "h2"}},
		Pagination: &Pagination{
			Strategy:   "scroll",
			MaxPages:   1,
			DedupField: "title",
		},
	}

	result, err := Apply(context.Background(), b, r)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(result.Items) != 2 {
		t.Fatalf("items count = %d, want 2 (deduped)", len(result.Items))
	}
}

func TestRunExtract_ContainerNotFound(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	r := &Runbook{
		Version: "1", Name: "nope", Type: "extract",
		URL:   ts.URL + "/runbook-auto",
		Items: &ItemSpec{Container: ".nonexistent", Fields: map[string]string{"x": "y"}},
	}

	result, err := Apply(context.Background(), b, r)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(result.Items) != 0 {
		t.Errorf("items count = %d, want 0", len(result.Items))
	}
}

func TestRunExtract_ContextCancelled(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	r := &Runbook{
		Version: "1", Name: "cancel", Type: "extract",
		URL:   ts.URL + "/runbook-extract",
		Items: &ItemSpec{Container: ".item", Fields: map[string]string{"title": "h2"}},
		Pagination: &Pagination{
			Strategy: "scroll",
			MaxPages: 10,
		},
	}

	// Navigating with cancelled context may or may not error depending on timing.
	// The key thing is it doesn't hang.
	_, _ = Apply(ctx, b, r)
}

// ── advancePage ────────────────────────────────────────────────────────────

func TestAdvancePage_UnknownStrategy(t *testing.T) {
	// advancePage with unknown strategy returns false
	p := &Pagination{Strategy: "unknown"}
	if advancePage(nil, p) {
		t.Error("expected false for unknown strategy")
	}
}

func TestAdvancePage_ClickNoSelector(t *testing.T) {
	p := &Pagination{Strategy: "click"}
	if advancePage(nil, p) {
		t.Error("expected false when next_selector is empty")
	}
}

// ── runAutomate (browser tests) ────────────────────────────────────────────

func TestRunAutomate_NavigateTypeClick(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	r := &Runbook{
		Version: "1", Name: "auto", Type: "automate",
		Steps: []Step{
			{Action: "navigate", URL: ts.URL + "/runbook-auto"},
			{Action: "type", Selector: "#name", Text: "hello"},
			{Action: "click", Selector: "#btn"},
			{Action: "eval", Script: "() => document.getElementById('result').textContent", As: "result"},
		},
	}

	result, err := Apply(context.Background(), b, r)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if v, ok := result.Variables["result"]; !ok || v != "clicked" {
		t.Errorf("result variable = %v, want %q", v, "clicked")
	}
}

func TestRunAutomate_Screenshot(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	r := &Runbook{
		Version: "1", Name: "screenshot", Type: "automate",
		Steps: []Step{
			{Action: "navigate", URL: ts.URL + "/runbook-auto"},
			{Action: "screenshot", Name: "viewport"},
			{Action: "screenshot", Name: "full", FullPage: true},
		},
	}

	result, err := Apply(context.Background(), b, r)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(result.Screenshots["viewport"]) == 0 {
		t.Error("viewport screenshot is empty")
	}
	if len(result.Screenshots["full"]) == 0 {
		t.Error("full page screenshot is empty")
	}
}

func TestRunAutomate_ScreenshotDefaultName(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	r := &Runbook{
		Version: "1", Name: "ss-default", Type: "automate",
		Steps: []Step{
			{Action: "navigate", URL: ts.URL + "/runbook-auto"},
			{Action: "screenshot"},
		},
	}

	result, err := Apply(context.Background(), b, r)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if _, ok := result.Screenshots["step_1"]; !ok {
		t.Error("expected screenshot with default name 'step_1'")
	}
}

func TestRunAutomate_Extract(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	r := &Runbook{
		Version: "1", Name: "extract", Type: "automate",
		Steps: []Step{
			{Action: "navigate", URL: ts.URL + "/runbook-auto"},
			{Action: "extract", Selector: ".info", As: "infos"},
		},
	}

	result, err := Apply(context.Background(), b, r)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	infos, ok := result.Variables["infos"].([]string)
	if !ok {
		t.Fatalf("infos variable type = %T, want []string", result.Variables["infos"])
	}
	if len(infos) != 2 {
		t.Fatalf("infos count = %d, want 2", len(infos))
	}
	if infos[0] != "hello" || infos[1] != "world" {
		t.Errorf("infos = %v, want [hello world]", infos)
	}
}

func TestRunAutomate_Eval(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	r := &Runbook{
		Version: "1", Name: "eval", Type: "automate",
		Steps: []Step{
			{Action: "navigate", URL: ts.URL + "/runbook-auto"},
			{Action: "eval", Script: "() => 1 + 2", As: "sum"},
		},
	}

	result, err := Apply(context.Background(), b, r)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// JSON numbers come back as float64
	if v, ok := result.Variables["sum"]; !ok {
		t.Error("sum variable not set")
	} else if v != float64(3) {
		t.Errorf("sum = %v (%T), want 3", v, v)
	}
}

func TestRunAutomate_Wait(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	r := &Runbook{
		Version: "1", Name: "wait", Type: "automate",
		Steps: []Step{
			{Action: "navigate", URL: ts.URL + "/runbook-auto"},
			{Action: "wait", Selector: "#result"},
		},
	}

	_, err := Apply(context.Background(), b, r)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
}

func TestRunAutomate_Key(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	r := &Runbook{
		Version: "1", Name: "key", Type: "automate",
		Steps: []Step{
			{Action: "navigate", URL: ts.URL + "/runbook-auto"},
			{Action: "key", Text: "Tab"},
		},
	}

	_, err := Apply(context.Background(), b, r)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
}

func TestRunAutomate_NoPageErrors(t *testing.T) {
	b := newTestBrowser(t)

	actions := []string{"click", "type", "wait", "screenshot", "extract", "eval", "key"}
	for _, action := range actions {
		t.Run(action, func(t *testing.T) {
			r := &Runbook{
				Version: "1", Name: "nopage", Type: "automate",
				Steps: []Step{{Action: action, Selector: "#x", Text: "x", Script: "1"}},
			}
			_, err := Apply(context.Background(), b, r)
			if err == nil {
				t.Errorf("expected error for %s without page", action)
			}
		})
	}
}

func TestRunAutomate_UnknownAction(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	r := &Runbook{
		Version: "1", Name: "unknown", Type: "automate",
		Steps: []Step{
			{Action: "navigate", URL: ts.URL + "/runbook-auto"},
			{Action: "bogus"},
		},
	}

	_, err := Apply(context.Background(), b, r)
	if err == nil {
		t.Error("expected error for unknown action")
	}
}

func TestRunAutomate_ContextCancelled(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	r := &Runbook{
		Version: "1", Name: "cancel", Type: "automate",
		Steps: []Step{
			{Action: "navigate", URL: ts.URL + "/runbook-auto"},
			{Action: "click", Selector: "#btn"},
		},
	}

	_, err := Apply(ctx, b, r)
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestRunAutomate_NavigateExistingPage(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	r := &Runbook{
		Version: "1", Name: "re-nav", Type: "automate",
		Steps: []Step{
			{Action: "navigate", URL: ts.URL + "/runbook-auto"},
			{Action: "navigate", URL: ts.URL + "/runbook-wait"},
			{Action: "wait", Selector: ".item"},
		},
	}

	_, err := Apply(context.Background(), b, r)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
}

// ── Run end-to-end ─────────────────────────────────────────────────────────

func TestRun_ExtractEndToEnd(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	data := fmt.Sprintf(`{
		"version": "1",
		"name": "e2e_extract",
		"type": "extract",
		"url": "%s/runbook-extract",
		"items": {
			"container": ".item",
			"fields": {"title": "h2", "price": ".price"}
		}
	}`, ts.URL)

	r, err := Parse([]byte(data))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	result, err := Apply(context.Background(), b, r)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(result.Items) != 3 {
		t.Fatalf("items = %d, want 3", len(result.Items))
	}
	if result.Items[2]["price"] != "$30" {
		t.Errorf("item 2 price = %q, want %q", result.Items[2]["price"], "$30")
	}
}

func TestRun_AutomateEndToEnd(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	data := fmt.Sprintf(`{
		"version": "1",
		"name": "e2e_automate",
		"type": "automate",
		"steps": [
			{"action": "navigate", "url": "%s/runbook-auto"},
			{"action": "click", "selector": "#btn"},
			{"action": "eval", "script": "() => document.getElementById('result').textContent", "as": "result"}
		]
	}`, ts.URL)

	r, err := Parse([]byte(data))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	result, err := Apply(context.Background(), b, r)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if result.Variables["result"] != "clicked" {
		t.Errorf("result = %v, want %q", result.Variables["result"], "clicked")
	}
}

func TestParse_NamedSelectors(t *testing.T) {
	data := `{
		"version": "1",
		"name": "sel_test",
		"type": "extract",
		"url": "http://example.com",
		"selectors": {
			"item": ".product",
			"title": "h2",
			"link": "a"
		},
		"wait_for": "$item",
		"items": {
			"container": "$item",
			"fields": {
				"name": "$title",
				"url": "$link@href",
				"price": ".price",
				"total": "+$title"
			}
		}
	}`

	r, err := Parse([]byte(data))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if r.WaitFor != ".product" {
		t.Errorf("wait_for = %q, want %q", r.WaitFor, ".product")
	}
	if r.Items.Container != ".product" {
		t.Errorf("container = %q, want %q", r.Items.Container, ".product")
	}
	if r.Items.Fields["name"] != "h2" {
		t.Errorf("fields[name] = %q, want %q", r.Items.Fields["name"], "h2")
	}
	if r.Items.Fields["url"] != "a@href" {
		t.Errorf("fields[url] = %q, want %q", r.Items.Fields["url"], "a@href")
	}
	if r.Items.Fields["price"] != ".price" {
		t.Errorf("fields[price] = %q, want %q (should be unchanged)", r.Items.Fields["price"], ".price")
	}
	if r.Items.Fields["total"] != "+h2" {
		t.Errorf("fields[total] = %q, want %q", r.Items.Fields["total"], "+h2")
	}
}

func TestParse_NamedSelectorsUnknownRef(t *testing.T) {
	data := `{
		"version": "1",
		"name": "bad_ref",
		"type": "extract",
		"url": "http://example.com",
		"selectors": {"item": ".product"},
		"items": {
			"container": ".product",
			"fields": {"name": "$unknown"}
		}
	}`

	_, err := Parse([]byte(data))
	if err == nil {
		t.Fatal("expected error for unknown $ref, got nil")
	}
}

func TestParse_NamedSelectorsInSteps(t *testing.T) {
	data := `{
		"version": "1",
		"name": "step_ref",
		"type": "automate",
		"selectors": {"btn": "#submit", "input": "#email"},
		"steps": [
			{"action": "navigate", "url": "http://example.com"},
			{"action": "type", "selector": "$input", "text": "test"},
			{"action": "click", "selector": "$btn"}
		]
	}`

	r, err := Parse([]byte(data))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if r.Steps[1].Selector != "#email" {
		t.Errorf("step[1].selector = %q, want %q", r.Steps[1].Selector, "#email")
	}
	if r.Steps[2].Selector != "#submit" {
		t.Errorf("step[2].selector = %q, want %q", r.Steps[2].Selector, "#submit")
	}
}

func TestParse_NamedSelectorsInPagination(t *testing.T) {
	data := `{
		"version": "1",
		"name": "pag_ref",
		"type": "extract",
		"url": "http://example.com",
		"selectors": {"next": "a.next-page", "item": ".card"},
		"items": {
			"container": "$item",
			"fields": {"title": "h2"}
		},
		"pagination": {
			"strategy": "click",
			"next_selector": "$next",
			"max_pages": 3
		}
	}`

	r, err := Parse([]byte(data))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if r.Items.Container != ".card" {
		t.Errorf("container = %q, want %q", r.Items.Container, ".card")
	}
	if r.Pagination.NextSelector != "a.next-page" {
		t.Errorf("next_selector = %q, want %q", r.Pagination.NextSelector, "a.next-page")
	}
}

func TestRun_ExtractWithNamedSelectors(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()
	b := newTestBrowser(t)

	data := fmt.Sprintf(`{
		"version": "1",
		"name": "e2e_selectors",
		"type": "extract",
		"url": "%s/runbook-extract",
		"selectors": {
			"item": ".item",
			"heading": "h2",
			"link": "a"
		},
		"items": {
			"container": "$item",
			"fields": {
				"title": "$heading",
				"url": "$link@href"
			}
		}
	}`, ts.URL)

	r, err := Parse([]byte(data))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	result, err := Apply(context.Background(), b, r)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(result.Items) != 3 {
		t.Fatalf("got %d items, want 3", len(result.Items))
	}

	if result.Items[0]["title"] != "Item A" {
		t.Errorf("items[0].title = %q, want %q", result.Items[0]["title"], "Item A")
	}
	if result.Items[0]["url"] != "/a" {
		t.Errorf("items[0].url = %q, want %q", result.Items[0]["url"], "/a")
	}
}
