# Shared Command Executor (Phase 6) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `pkg/scout/tools/` the single shared command executor so REPL and MCP become thin adapters with no direct rod/CDP calls, enforcing capability parity structurally.

**Architecture:** Extend the existing `pkg/scout/tools/` layer with page-level verbs (`func Verb(ctx, p *scout.Page, in Input) (*Output, error)`) and browser-level verbs (`func Verb(ctx, b *scout.Browser, in Input) (*Output, error)`), mirroring the shipped convention (typed Input/Output, nil-checks, `tools:` error prefix). Then rewrite the REPL `switch` as a dispatch table over these verbs and re-point the ~12 inline MCP handlers at them. Adapters only parse input and format output.

**Tech Stack:** Go 1.26, `pkg/scout` facade (`*scout.Page`, `*scout.Browser`), existing `pkg/scout/tools/`, `pkg/scout/mcp` (`addTracedTool`), Cobra REPL. Tests use `newTestBrowser` (real Chromium; skips if unavailable).

**Spec:** `docs/superpowers/specs/2026-05-16-06-shared-command-executor-design.md` (see the 2026-06-03 Amendment) + reconciliation `docs/superpowers/specs/2026-06-03-phase6-state-reconciliation.md`.

---

## Conventions (read once — every verb follows this)

- **Page verbs** live in new files under `pkg/scout/tools/`. Signature: `func Verb(_ context.Context, p *scout.Page, in VerbInput) (*VerbOutput, error)`. First line guards nil: `if p == nil { return nil, fmt.Errorf("tools: <verb>: nil page") }`. Wrap the existing `*scout.Page` method (the REPL/MCP code already calls these: `p.Navigate`, `p.Eval`, `p.Element`, `p.ExtractText`, `p.Screenshot`, `p.Markdown`, `p.HTML`, `p.GetCookies`, `p.URL`, `p.Title`, `p.WaitLoad`, `p.NavigateBack`, `p.NavigateForward`, `p.Reload`).
- **Browser verbs** take `*scout.Browser` (existing pattern, see `testsite.go`).
- Error prefix is `tools: <verb>:` (keep the shipped convention — do NOT change to `scout:`).
- Input/Output are typed structs with `json` + `jsonschema` tags (so MCP can reuse them).
- **Test pattern** (real browser): use the package's existing test helper for a browser/page (grep `pkg/scout/tools/*_test.go` for the helper, e.g. `newToolsTestBrowser`; if none exists, Task 1 Step 1 creates one). Each verb test navigates an `httptest` page, calls the verb, asserts the output. Tests `t.Skip` when Chromium is unavailable, matching the project norm.

---

## Task 1: Foundation — `Navigate` page verb (establishes the recipe)

**Files:**
- Create: `pkg/scout/tools/page.go`
- Test: `pkg/scout/tools/page_test.go`

- [ ] **Step 1: Write the failing test** (also creates the shared test helper)

```go
package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/inovacc/scout/pkg/scout"
)

// newPageTestBrowser returns a headless browser + a blank page, skipping when
// Chromium is unavailable (project norm — real browser, no mocks).
func newPageTestBrowser(t *testing.T) (*scout.Browser, *scout.Page) {
	t.Helper()
	b, err := scout.New(scout.WithHeadless(true), scout.WithNoSandbox())
	if err != nil {
		t.Skipf("browser unavailable: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	p, err := b.NewPage("")
	if err != nil {
		t.Skipf("page unavailable: %v", err)
	}
	return b, p
}

func newTestServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><head><title>T</title></head><body>" + body + "</body></html>"))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestNavigate(t *testing.T) {
	_, p := newPageTestBrowser(t)
	srv := newTestServer(t, "<h1>hi</h1>")

	out, err := Navigate(context.Background(), p, NavigateInput{URL: srv.URL})
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if out.Title != "T" {
		t.Errorf("Title = %q, want T", out.Title)
	}
	if out.URL == "" {
		t.Errorf("URL empty")
	}

	if _, err := Navigate(context.Background(), nil, NavigateInput{URL: srv.URL}); err == nil {
		t.Error("nil page should error")
	}
	if _, err := Navigate(context.Background(), p, NavigateInput{URL: ""}); err == nil {
		t.Error("empty url should error")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -run 'TestNavigate' ./pkg/scout/tools/`
Expected: FAIL — `undefined: Navigate`, `undefined: NavigateInput`.

- [ ] **Step 3: Write minimal implementation**

```go
package tools

import (
	"context"
	"fmt"

	"github.com/inovacc/scout/pkg/scout"
)

// NavigateInput targets a URL on the caller's page.
type NavigateInput struct {
	URL string `json:"url" jsonschema:"the URL to navigate to"`
}

// NavigateOutput reports the landed page after load.
type NavigateOutput struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

// Navigate drives the given page to in.URL and waits for load (best-effort).
func Navigate(_ context.Context, p *scout.Page, in NavigateInput) (*NavigateOutput, error) {
	if p == nil {
		return nil, fmt.Errorf("tools: navigate: nil page")
	}
	if in.URL == "" {
		return nil, fmt.Errorf("tools: navigate: url required")
	}
	if err := p.Navigate(in.URL); err != nil {
		return nil, fmt.Errorf("tools: navigate: %w", err)
	}
	_ = p.WaitLoad()
	url, _ := p.URL()
	title, _ := p.Title()
	return &NavigateOutput{URL: url, Title: title}, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -run 'TestNavigate' ./pkg/scout/tools/`
Expected: PASS (or SKIP if no Chromium — that is acceptable; the nil/empty assertions still run because they're reached only after a successful skip-guarded browser; if skipped, run on a machine with Chromium before merge).

- [ ] **Step 5: Lint + commit**

Run: `golangci-lint run ./pkg/scout/tools/` → 0 new issues. Then:
```bash
git add pkg/scout/tools/page.go pkg/scout/tools/page_test.go
git commit -m "feat(tools): page-level Navigate verb + page test harness"
```

---

## Task 2: Navigation cluster — Back, Forward, Reload, Wait

**Files:** Modify `pkg/scout/tools/page.go`; add tests to `pkg/scout/tools/page_test.go`.

- [ ] **Step 1: Add the verbs** (append to `page.go`)

```go
// BackInput / ForwardInput / ReloadInput take no fields (operate on the page).
type BackInput struct{}
type ForwardInput struct{}
type ReloadInput struct{}

// EmptyOutput is returned by verbs with no payload.
type EmptyOutput struct {
	OK bool `json:"ok"`
}

func Back(_ context.Context, p *scout.Page, _ BackInput) (*EmptyOutput, error) {
	if p == nil {
		return nil, fmt.Errorf("tools: back: nil page")
	}
	if err := p.NavigateBack(); err != nil {
		return nil, fmt.Errorf("tools: back: %w", err)
	}
	return &EmptyOutput{OK: true}, nil
}

func Forward(_ context.Context, p *scout.Page, _ ForwardInput) (*EmptyOutput, error) {
	if p == nil {
		return nil, fmt.Errorf("tools: forward: nil page")
	}
	if err := p.NavigateForward(); err != nil {
		return nil, fmt.Errorf("tools: forward: %w", err)
	}
	return &EmptyOutput{OK: true}, nil
}

func Reload(_ context.Context, p *scout.Page, _ ReloadInput) (*EmptyOutput, error) {
	if p == nil {
		return nil, fmt.Errorf("tools: reload: nil page")
	}
	if err := p.Reload(); err != nil {
		return nil, fmt.Errorf("tools: reload: %w", err)
	}
	return &EmptyOutput{OK: true}, nil
}

// WaitInput waits for page load (Selector empty) or for an element to appear.
type WaitInput struct {
	Selector string `json:"selector,omitempty" jsonschema:"CSS selector to wait for; empty waits for page load"`
}

func Wait(_ context.Context, p *scout.Page, in WaitInput) (*EmptyOutput, error) {
	if p == nil {
		return nil, fmt.Errorf("tools: wait: nil page")
	}
	if in.Selector == "" {
		_ = p.WaitLoad()
		return &EmptyOutput{OK: true}, nil
	}
	if _, err := p.Element(in.Selector); err != nil {
		return nil, fmt.Errorf("tools: wait: %w", err)
	}
	return &EmptyOutput{OK: true}, nil
}
```

- [ ] **Step 2: Add tests** (append to `page_test.go`) — one test per verb, e.g.:

```go
func TestReload(t *testing.T) {
	_, p := newPageTestBrowser(t)
	srv := newTestServer(t, "<p>x</p>")
	if _, err := Navigate(context.Background(), p, NavigateInput{URL: srv.URL}); err != nil {
		t.Fatal(err)
	}
	if _, err := Reload(context.Background(), p, ReloadInput{}); err != nil {
		t.Errorf("Reload: %v", err)
	}
	if _, err := Reload(context.Background(), nil, ReloadInput{}); err == nil {
		t.Error("nil page should error")
	}
}

func TestWaitForElement(t *testing.T) {
	_, p := newPageTestBrowser(t)
	srv := newTestServer(t, `<div id="ready">ok</div>`)
	if _, err := Navigate(context.Background(), p, NavigateInput{URL: srv.URL}); err != nil {
		t.Fatal(err)
	}
	if _, err := Wait(context.Background(), p, WaitInput{Selector: "#ready"}); err != nil {
		t.Errorf("Wait: %v", err)
	}
}
```
(Back/Forward need navigation history; a minimal test asserting the nil-page error is acceptable since real back/forward requires two navigations — add a two-nav test if time permits.)

- [ ] **Step 3: Test + lint + commit**

```bash
go test -run 'TestReload|TestWait' ./pkg/scout/tools/ && golangci-lint run ./pkg/scout/tools/
git add pkg/scout/tools/page.go pkg/scout/tools/page_test.go
git commit -m "feat(tools): Back/Forward/Reload/Wait page verbs"
```

---

## Task 3: Interaction cluster — Click, Type, Extract, Eval

**Files:** Modify `pkg/scout/tools/page.go`; tests in `page_test.go`.

- [ ] **Step 1: Add the verbs**

```go
type ClickInput struct {
	Selector string `json:"selector" jsonschema:"CSS selector to click"`
}
type TypeInput struct {
	Selector string `json:"selector" jsonschema:"CSS selector of the input"`
	Text     string `json:"text"     jsonschema:"text to type"`
}
type ExtractInput struct {
	Selector string `json:"selector" jsonschema:"CSS selector to extract text from"`
}
type ExtractOutput struct {
	Text string `json:"text"`
}
type EvalInput struct {
	Expression string `json:"expression" jsonschema:"JavaScript expression to evaluate"`
}
type EvalOutput struct {
	Result string `json:"result"`
}

func Click(_ context.Context, p *scout.Page, in ClickInput) (*EmptyOutput, error) {
	if p == nil {
		return nil, fmt.Errorf("tools: click: nil page")
	}
	if in.Selector == "" {
		return nil, fmt.Errorf("tools: click: selector required")
	}
	el, err := p.Element(in.Selector)
	if err != nil {
		return nil, fmt.Errorf("tools: click: %w", err)
	}
	if err := el.Click(); err != nil {
		return nil, fmt.Errorf("tools: click: %w", err)
	}
	return &EmptyOutput{OK: true}, nil
}

func Type(_ context.Context, p *scout.Page, in TypeInput) (*EmptyOutput, error) {
	if p == nil {
		return nil, fmt.Errorf("tools: type: nil page")
	}
	if in.Selector == "" {
		return nil, fmt.Errorf("tools: type: selector required")
	}
	el, err := p.Element(in.Selector)
	if err != nil {
		return nil, fmt.Errorf("tools: type: %w", err)
	}
	if err := el.Input(in.Text); err != nil {
		return nil, fmt.Errorf("tools: type: %w", err)
	}
	return &EmptyOutput{OK: true}, nil
}

func Extract(_ context.Context, p *scout.Page, in ExtractInput) (*ExtractOutput, error) {
	if p == nil {
		return nil, fmt.Errorf("tools: extract: nil page")
	}
	if in.Selector == "" {
		return nil, fmt.Errorf("tools: extract: selector required")
	}
	text, err := p.ExtractText(in.Selector)
	if err != nil {
		return nil, fmt.Errorf("tools: extract: %w", err)
	}
	return &ExtractOutput{Text: text}, nil
}

func Eval(_ context.Context, p *scout.Page, in EvalInput) (*EvalOutput, error) {
	if p == nil {
		return nil, fmt.Errorf("tools: eval: nil page")
	}
	if in.Expression == "" {
		return nil, fmt.Errorf("tools: eval: expression required")
	}
	res, err := p.Eval(in.Expression)
	if err != nil {
		return nil, fmt.Errorf("tools: eval: %w", err)
	}
	return &EvalOutput{Result: fmt.Sprintf("%v", res)}, nil
}
```

NOTE: confirm `p.Eval` returns `(something, error)` — in REPL it is printed via `Fprintln(out, result)`, so `fmt.Sprintf("%v", res)` is correct. If `p.Eval` already returns a `string`, drop the `Sprintf`.

- [ ] **Step 2: Tests** — `TestClickTypeExtract` navigates a form page (`<input id="f"><button id="b">`), Types into `#f`, Extracts `#b` text, asserts; `TestEval` evaluates `"1+2"` and asserts `out.Result == "3"`. Include nil-page + empty-selector error cases.

- [ ] **Step 3: Test + lint + commit**

```bash
go test -run 'TestClick|TestType|TestExtract|TestEval' ./pkg/scout/tools/ && golangci-lint run ./pkg/scout/tools/
git add pkg/scout/tools/page.go pkg/scout/tools/page_test.go
git commit -m "feat(tools): Click/Type/Extract/Eval page verbs"
```

---

## Task 4: Content-read cluster — HTML, Markdown, Cookies, URL, Title

**Files:** Modify `pkg/scout/tools/page.go`; tests in `page_test.go`.

- [ ] **Step 1: Add the verbs**

```go
type HTMLInput struct{}
type HTMLOutput struct{ HTML string `json:"html"` }
type MarkdownInput struct{}
type MarkdownOutput struct{ Markdown string `json:"markdown"` }
type CookiesInput struct{}
type CookiesOutput struct{ Cookies any `json:"cookies"` } // re-use engine cookie slice via any to avoid a new import
type URLInput struct{}
type URLOutput struct{ URL string `json:"url"` }
type TitleInput struct{}
type TitleOutput struct{ Title string `json:"title"` }

func HTML(_ context.Context, p *scout.Page, _ HTMLInput) (*HTMLOutput, error) {
	if p == nil {
		return nil, fmt.Errorf("tools: html: nil page")
	}
	h, err := p.HTML()
	if err != nil {
		return nil, fmt.Errorf("tools: html: %w", err)
	}
	return &HTMLOutput{HTML: h}, nil
}

func Markdown(_ context.Context, p *scout.Page, _ MarkdownInput) (*MarkdownOutput, error) {
	if p == nil {
		return nil, fmt.Errorf("tools: markdown: nil page")
	}
	m, err := p.Markdown()
	if err != nil {
		return nil, fmt.Errorf("tools: markdown: %w", err)
	}
	return &MarkdownOutput{Markdown: m}, nil
}

func Cookies(_ context.Context, p *scout.Page, _ CookiesInput) (*CookiesOutput, error) {
	if p == nil {
		return nil, fmt.Errorf("tools: cookies: nil page")
	}
	c, err := p.GetCookies()
	if err != nil {
		return nil, fmt.Errorf("tools: cookies: %w", err)
	}
	return &CookiesOutput{Cookies: c}, nil
}

func URL(_ context.Context, p *scout.Page, _ URLInput) (*URLOutput, error) {
	if p == nil {
		return nil, fmt.Errorf("tools: url: nil page")
	}
	u, err := p.URL()
	if err != nil {
		return nil, fmt.Errorf("tools: url: %w", err)
	}
	return &URLOutput{URL: u}, nil
}

func Title(_ context.Context, p *scout.Page, _ TitleInput) (*TitleOutput, error) {
	if p == nil {
		return nil, fmt.Errorf("tools: title: nil page")
	}
	t, err := p.Title()
	if err != nil {
		return nil, fmt.Errorf("tools: title: %w", err)
	}
	return &TitleOutput{Title: t}, nil
}
```

NOTE: if `gofumpt`/lint rejects the inline-struct-tag one-liners (`type X struct{ F string \`json:"f"\` }`), expand them to multi-line struct definitions.

- [ ] **Step 2: Tests** — `TestContentReads` navigates a known page and asserts `Title`==`"T"`, `URL`!=`""`, `HTML` contains `"<body>"`, `Markdown` non-empty, `Cookies` no error. Plus nil-page error per verb.

- [ ] **Step 3: Test + lint + commit**

```bash
go test -run 'TestContentReads' ./pkg/scout/tools/ && golangci-lint run ./pkg/scout/tools/
git add pkg/scout/tools/page.go pkg/scout/tools/page_test.go
git commit -m "feat(tools): HTML/Markdown/Cookies/URL/Title page verbs"
```

---

## Task 5: Capture cluster — Screenshot, Snapshot, PDF

**Files:** Create `pkg/scout/tools/capture.go`; test `pkg/scout/tools/capture_test.go`.

- [ ] **Step 1: Read the current MCP capture handlers** in `pkg/scout/mcp/tools_capture.go` to copy the exact `*scout.Page` calls for screenshot/snapshot/pdf (likely `p.Screenshot()`, `p.Snapshot()`/accessibility, `p.PDF(opts)`). Then add verbs `Screenshot(ctx, p, ScreenshotInput) (*ScreenshotOutput{Data []byte}, error)`, `Snapshot(...)`, `PDF(...)` following the recipe (nil-check, `tools: <verb>:` prefix, wrap the page method). For PDF, mirror the MCP pdf tool's option fields (scale, landscape, etc.) in `PDFInput`.

- [ ] **Step 2: Tests** — `TestScreenshot` navigates and asserts `len(out.Data) > 0` and PNG magic bytes `\x89PNG`. nil-page error cases.

- [ ] **Step 3: Test + lint + commit**

```bash
go test -run 'TestScreenshot|TestSnapshot|TestPDF' ./pkg/scout/tools/ && golangci-lint run ./pkg/scout/tools/
git add pkg/scout/tools/capture.go pkg/scout/tools/capture_test.go
git commit -m "feat(tools): Screenshot/Snapshot/PDF capture verbs"
```

---

## Task 6: Browser cluster — Tabs, NewTab

**Files:** Create `pkg/scout/tools/tabs.go`; test `pkg/scout/tools/tabs_test.go`. These take `*scout.Browser` (page-list operations).

- [ ] **Step 1: Add the verbs**

```go
package tools

import (
	"context"
	"fmt"

	"github.com/inovacc/scout/pkg/scout"
)

type TabInfo struct {
	Index int    `json:"index"`
	URL   string `json:"url"`
	Title string `json:"title"`
}
type TabsInput struct{}
type TabsOutput struct {
	Tabs []TabInfo `json:"tabs"`
}

func Tabs(_ context.Context, b *scout.Browser, _ TabsInput) (*TabsOutput, error) {
	if b == nil {
		return nil, fmt.Errorf("tools: tabs: nil browser")
	}
	pages, err := b.Pages()
	if err != nil {
		return nil, fmt.Errorf("tools: tabs: %w", err)
	}
	out := &TabsOutput{Tabs: make([]TabInfo, 0, len(pages))}
	for i, p := range pages {
		u, _ := p.URL()
		t, _ := p.Title()
		out.Tabs = append(out.Tabs, TabInfo{Index: i, URL: u, Title: t})
	}
	return out, nil
}

type NewTabInput struct {
	URL string `json:"url,omitempty" jsonschema:"optional URL to open in the new tab"`
}
type NewTabOutput struct {
	URL   string `json:"url"`
	Title string `json:"title"`
}

func NewTab(_ context.Context, b *scout.Browser, in NewTabInput) (*NewTabOutput, error) {
	if b == nil {
		return nil, fmt.Errorf("tools: new-tab: nil browser")
	}
	p, err := b.NewPage(in.URL)
	if err != nil {
		return nil, fmt.Errorf("tools: new-tab: %w", err)
	}
	if in.URL != "" {
		_ = p.WaitLoad()
	}
	u, _ := p.URL()
	t, _ := p.Title()
	return &NewTabOutput{URL: u, Title: t}, nil
}
```

- [ ] **Step 2: Tests** — `TestTabs` opens a page, asserts `len(out.Tabs) >= 1`; `TestNewTab` opens a tab with a URL and asserts the returned title. nil-browser error cases.

- [ ] **Step 3: Test + lint + commit**

```bash
go test -run 'TestTabs|TestNewTab' ./pkg/scout/tools/ && golangci-lint run ./pkg/scout/tools/
git add pkg/scout/tools/tabs.go pkg/scout/tools/tabs_test.go
git commit -m "feat(tools): Tabs/NewTab browser verbs"
```

---

## Task 7: REPL adapter — route every command through `tools.*`

**Files:** Modify `cmd/scout/repl.go` (the `switch c` block, lines ~76–423).

- [ ] **Step 1:** Rewrite each `case` so its body parses args, calls the matching `tools.*` verb, and formats output. NO direct `page.*`/`b.*` calls except: obtaining the current page/browser (`page`, `b`), and the adapter-level tab-switch (`tab` keeps `page = pages[idx]` since "current tab" is adapter state). Example for `navigate`:

```go
		case "navigate", "go", "nav":
			if len(parts) < 2 {
				_, _ = fmt.Fprintln(out, "usage: navigate <url>")
				continue
			}
			if page == nil {
				np, err := b.NewPage("")
				if err != nil {
					_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
					continue
				}
				page = np
			}
			res, err := tools.Navigate(cmd.Context(), page, tools.NavigateInput{URL: parts[1]})
			if err != nil {
				_, _ = fmt.Fprintf(out, "ERROR: %v\n", err)
				continue
			}
			_, _ = fmt.Fprintf(out, "Page: %s\n", res.Title)
```

Do the same for eval, click, type, extract, screenshot (verb returns bytes → REPL writes the file), markdown, html, cookies, url, title, wait, back, forward, reload, tabs, newtab, health (`tools.TestSite(cmd.Context(), b, tools.TestSiteInput{URL: u, Depth: 1, Concurrency: 1})`). Keep `help`/`exit` as-is. Keep the `if page == nil { "no page open" }` guards in the adapter.

- [ ] **Step 2:** Add `"github.com/inovacc/scout/pkg/scout/tools"` to the imports of `cmd/scout/repl.go`. Remove now-unused imports (`encoding/json` if cookies formatting moves; keep if still used).

- [ ] **Step 3: Verify behavior unchanged**

Run: `go build ./cmd/scout/ && go vet ./cmd/scout/`. If `cmd/scout` has a REPL integration test (grep `repl_test.go`), run it: `go test -run 'REPL' ./cmd/scout/`. Manually confirm: `echo -e "navigate https://example.com\ntitle\nexit" | go run ./cmd/scout/ repl` prints a title.

- [ ] **Step 4: Lint + commit**

```bash
golangci-lint run ./cmd/scout/   # no NEW issues
git add cmd/scout/repl.go
git commit -m "refactor(repl): route all browser commands through pkg/scout/tools verbs"
```

---

## Task 8: MCP adapter — route remaining inline tools through `tools.*`

**Files:** Modify `pkg/scout/mcp/tools_browser.go`, `tools_capture.go`, `tools_swarm.go`, `tools_websocket.go` (the inline handlers identified in the reconciliation: navigate, click, type, extract, eval, back, forward, wait, screenshot, snapshot, pdf, swarm_crawl, ws_*).

- [ ] **Step 1:** For each inline handler, replace the direct `page.*` call with the `tools.*` verb, keeping the existing `errResult`/`textResult` formatting and the SSRF `state.checkURL` guard (added in the SSRF feature — keep it BEFORE the verb call for navigate/etc.). Example for `navigate`:

```go
		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}
		if err := state.checkURL(ctx, args.URL); err != nil { // keep SSRF guard
			return errResult(err.Error())
		}
		res, err := tools.Navigate(ctx, page, tools.NavigateInput{URL: args.URL})
		if err != nil {
			return errResult(err.Error())
		}
		metrics.Get().NavigationsTotal.Add(1)
		return textResult(fmt.Sprintf("Navigated to %s (%s)", res.URL, res.Title))
```

For `swarm_crawl`, extract a `tools.SwarmCrawl` verb first (read the current inline handler; move its body into `pkg/scout/tools/crawl.go` alongside the existing crawl verb or a new `swarm.go`), then call it. For `ws_*`, port the WebSocket monitor calls into `tools/` verbs (`WSListen`, `WSSend`, `WSConnections`) and re-point.

- [ ] **Step 2:** Add the `tools` import where missing. Keep all tool schemas (`InputSchema`) unchanged so the MCP wire contract is identical.

- [ ] **Step 3: Verify**

Run: `go build ./pkg/scout/mcp/ && go test -run 'TestCheckURL' ./pkg/scout/mcp/ && go vet ./pkg/scout/mcp/`. Run any MCP integration tests present. `golangci-lint run ./pkg/scout/mcp/` — no NEW issues.

- [ ] **Step 4: Commit**

```bash
git add pkg/scout/mcp/
git commit -m "refactor(mcp): route inline tool handlers through pkg/scout/tools verbs"
```

---

## Task 9: MCP parity twins for REPL-only read verbs

**Files:** Modify `pkg/scout/mcp/tools_browser.go` (or a new `tools_content.go`).

- [ ] **Step 1:** Add MCP tools `html`, `markdown`, `cookies`, `page_url`, `page_title` that call `tools.HTML/Markdown/Cookies/URL/Title` on `state.ensurePage`. Each follows the existing `addTracedTool` pattern with a minimal `InputSchema` (`{"type":"object","properties":{}}`). Example:

```go
	addTracedTool(server, &mcp.Tool{
		Name:        "markdown",
		Description: "Get the current page as Markdown",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	}, func(ctx context.Context, _ *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page, err := state.ensurePage(ctx)
		if err != nil {
			return errResult(err.Error())
		}
		res, err := tools.Markdown(ctx, page, tools.MarkdownInput{})
		if err != nil {
			return errResult(err.Error())
		}
		return textResult(res.Markdown)
	})
```

- [ ] **Step 2: Verify + commit**

```bash
go build ./pkg/scout/mcp/ && go vet ./pkg/scout/mcp/ && golangci-lint run ./pkg/scout/mcp/
git add pkg/scout/mcp/
git commit -m "feat(mcp): add html/markdown/cookies/page_url/page_title tools for REPL parity"
```

---

## Task 10: Parity guard + docs

**Files:** Create `pkg/scout/tools/parity_test.go`; modify `CLAUDE.md`, `docs/ARCHITECTURE.md`.

- [ ] **Step 1:** Write a guard test that documents the executor↔adapter mapping. Since REPL (cmd/scout, package main) and MCP (pkg/scout/mcp) can't both be imported from `pkg/scout/tools`, implement the guard as a **table-driven manifest test** inside `pkg/scout/tools`:

```go
package tools

import "testing"

// verbRegistry is the authoritative list of browser verbs and where each is
// exposed. Update this when adding a verb; CI fails if a verb is unmapped.
var verbRegistry = []struct {
	Verb     string
	InREPL   bool
	InMCP    bool
	WaiveMCP string // reason MCP exposure is intentionally skipped, or ""
}{
	{"Navigate", true, true, ""},
	{"Back", true, true, ""},
	{"Forward", true, true, ""},
	{"Reload", true, true, ""},
	{"Wait", true, true, ""},
	{"Click", true, true, ""},
	{"Type", true, true, ""},
	{"Extract", true, true, ""},
	{"Eval", true, true, ""},
	{"HTML", true, true, ""},
	{"Markdown", true, true, ""},
	{"Cookies", true, true, ""},
	{"URL", true, true, ""},
	{"Title", true, true, ""},
	{"Screenshot", true, true, ""},
	{"Snapshot", false, true, "REPL has no snapshot command"},
	{"PDF", false, true, "REPL has no pdf command"},
	{"Tabs", true, true, ""},
	{"NewTab", true, true, ""},
}

func TestVerbParity(t *testing.T) {
	for _, v := range verbRegistry {
		if !v.InREPL && !v.InMCP {
			t.Errorf("verb %s exposed nowhere", v.Verb)
		}
		if !v.InMCP && v.WaiveMCP == "" {
			t.Errorf("verb %s missing from MCP without a waiver reason", v.Verb)
		}
	}
}
```

(This manifest is maintained by hand; pair it with a comment in both adapters: "every browser command must call a `tools.*` verb." A stronger reflection-based check is out of scope.)

- [ ] **Step 2:** Update `CLAUDE.md` (add a bullet: "Shared executor: all REPL + MCP browser commands route through `pkg/scout/tools/` verbs; add a capability = one verb + one REPL case + one MCP `AddTool`") and `docs/ARCHITECTURE.md` (note `pkg/scout/tools/` is the shared command executor).

- [ ] **Step 3: Commit**

```bash
go test ./pkg/scout/tools/
git add pkg/scout/tools/parity_test.go CLAUDE.md docs/ARCHITECTURE.md
git commit -m "test(tools): verb-parity guard; docs(executor): document shared command layer"
```

---

## Final review (whole feature)

After all tasks: run `go build ./cmd/... ./pkg/... && go test ./pkg/scout/tools/ ./pkg/scout/mcp/ ./cmd/scout/` (Chromium-dependent verb tests will run or skip). Confirm the revised success criteria from the spec amendment:
- REPL `repl.go` has no direct `page.*`/`b.*` browser logic except current-page/tab bookkeeping.
- MCP browser handlers call `tools.*` (no inline `page.Navigate`/`page.Eval`).
- `task check` (lint + vet) passes.
Then `superpowers:requesting-code-review`, then `superpowers:finishing-a-development-branch` to merge `feat/shared-executor` into `main`.

**Note:** Tasks 1–6 are mechanical 1:1 verb wraps and can each be a fast subagent. Tasks 7–8 are the integration-heavy ones (read each handler carefully; preserve the SSRF guard in MCP and the `no page open` guards in REPL). Task 9 depends on the verbs from Tasks 1–6.
