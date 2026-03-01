# MCP Tier 1-3 Tools Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add 14 MCP tools (cookie, header, markdown, table, meta, form_detect, form_fill, form_submit, crawl, detect, block, hijack, storage, har, swagger) organized across 3 new files.

**Architecture:** Each tier gets its own file (`tools_content.go`, `tools_network.go`, `tools_interact.go`) with a `register*Tools(server, state)` function called from `NewServer()`. All tools follow the existing handler pattern: unmarshal args → ensurePage/ensureBrowser → call scout API → return JSON via `jsonResult()`. Tools that need a URL navigate first then extract; tools that operate on current page use `ensurePage()` directly.

**Tech Stack:** Go, `pkg/scout` APIs (Page/Browser methods), `go-sdk/mcp`.

---

### Task 1: Tier 1 content tools — markdown, table, meta

**Files:**
- Create: `pkg/scout/mcp/tools_content.go`
- Create: `pkg/scout/mcp/tools_content_test.go`
- Modify: `pkg/scout/mcp/server.go` (add `registerContentTools(server, state)` call)
- Modify: `pkg/scout/mcp/server_test.go` (update expected tools list)

**Step 1: Create `tools_content.go` with 3 tools**

Register function `registerContentTools(server *mcp.Server, state *mcpState)` adding:

- **`markdown`** — Extract page content as markdown
  - Input: `mainOnly` (bool, optional), `includeImages` (bool, optional, default true), `includeLinks` (bool, optional, default true)
  - Calls: `page.Markdown(opts...)` with `scout.WithMainContentOnly()`, `scout.WithIncludeImages()`, `scout.WithIncludeLinks()`
  - Returns: text result with markdown string

- **`table`** — Extract table data from page
  - Input: `selector` (string, optional, default "table")
  - Calls: `page.ExtractTable(selector)`
  - Returns: JSON with `{headers: [...], rows: [[...], ...]}`

- **`meta`** — Extract page metadata (title, description, OG, Twitter)
  - Input: none
  - Calls: `page.ExtractMeta()`
  - Returns: JSON with `{title, description, canonical, og: {}, twitter: {}}`

**Step 2: Create `tools_content_test.go`**

Tests using `connectTestClient` + `newTestHTTPServer` (add table/meta HTML endpoints):
- `TestMarkdownTool` — navigate to test page, call markdown, verify contains "Hello Scout"
- `TestTableTool` — serve HTML with `<table>`, extract, verify headers/rows
- `TestMetaTool` — serve HTML with `<meta>` tags, extract, verify fields

**Step 3: Wire into server.go**

Add `registerContentTools(server, state)` after `registerDiagTools` call in `NewServer()`.

**Step 4: Update `server_test.go` expected tools**

Add `"markdown"`, `"table"`, `"meta"` to the expected list in `TestListTools`.

**Step 5: Run tests, commit**

```
go test -v -count=1 ./pkg/scout/mcp/ -timeout 60s
git add pkg/scout/mcp/tools_content.go pkg/scout/mcp/tools_content_test.go pkg/scout/mcp/server.go pkg/scout/mcp/server_test.go
git commit -m "feat(mcp): add markdown, table, meta content extraction tools"
```

---

### Task 2: Tier 1+2 network tools — cookie, header, block

**Files:**
- Create: `pkg/scout/mcp/tools_network.go`
- Create: `pkg/scout/mcp/tools_network_test.go`
- Modify: `pkg/scout/mcp/server.go` (add `registerNetworkTools(server, state)` call)
- Modify: `pkg/scout/mcp/server_test.go` (update expected tools list)

**Step 1: Create `tools_network.go` with 3 tools**

Register function `registerNetworkTools(server *mcp.Server, state *mcpState)` adding:

- **`cookie`** — Get, set, or clear cookies
  - Input: `action` (string, required: "get"|"set"|"clear"), `url` (string, optional for get), `name` (string, for set), `value` (string, for set), `domain` (string, optional for set), `path` (string, optional for set, default "/")
  - Actions:
    - `get`: calls `page.GetCookies(urls...)` → JSON array
    - `set`: calls `page.SetCookies(scout.Cookie{Name, Value, Domain, Path})` → confirmation
    - `clear`: calls `page.ClearCookies()` → confirmation

- **`header`** — Set custom HTTP headers for all subsequent requests
  - Input: `headers` (object, required — key-value map)
  - Calls: `page.SetHeaders(headers)` — store cleanup func in mcpState for later removal
  - Returns: confirmation text listing headers set

- **`block`** — Block requests matching URL patterns
  - Input: `patterns` (array of strings, required)
  - Calls: `page.SetBlockedURLs(patterns...)`
  - Returns: confirmation text

**Step 2: Create `tools_network_test.go`**

- `TestCookieToolSet` — set cookie, then get, verify it exists
- `TestCookieToolClear` — set then clear, verify empty
- `TestHeaderTool` — set headers, verify no error
- `TestBlockTool` — set block pattern, verify no error

**Step 3: Wire into server.go, update expected tools**

**Step 4: Run tests, commit**

```
git commit -m "feat(mcp): add cookie, header, block network tools"
```

---

### Task 3: Tier 2 interaction tools — form_detect, form_fill, form_submit

**Files:**
- Create: `pkg/scout/mcp/tools_form.go`
- Create: `pkg/scout/mcp/tools_form_test.go`
- Modify: `pkg/scout/mcp/server.go` (add `registerFormTools(server, state)` call)
- Modify: `pkg/scout/mcp/server_test.go` (update expected tools list)

**Step 1: Create `tools_form.go` with 3 tools**

Register function `registerFormTools(server *mcp.Server, state *mcpState)`:

- **`form_detect`** — Detect forms on the current page
  - Input: `selector` (string, optional — specific form selector)
  - If selector: `page.DetectForm(selector)` → single form JSON
  - Else: `page.DetectForms()` → array of forms JSON
  - Returns: JSON with form fields (name, type, value)

- **`form_fill`** — Fill a form with data
  - Input: `selector` (string, optional, default "form"), `data` (object, required — field name→value map)
  - Calls: `page.DetectForm(selector)` → `form.Fill(data)`
  - Returns: confirmation text

- **`form_submit`** — Submit a form
  - Input: `selector` (string, optional, default "form")
  - Calls: `page.DetectForm(selector)` → `form.Submit()`
  - Returns: confirmation text

**Step 2: Create `tools_form_test.go`**

Serve HTML with `<form><input name="q" type="text"><button type="submit">Go</button></form>`:
- `TestFormDetectTool` — detect form, verify fields
- `TestFormFillTool` — fill form, verify no error
- `TestFormSubmitTool` — submit form, verify no error

**Step 3: Wire, update expected, test, commit**

```
git commit -m "feat(mcp): add form_detect, form_fill, form_submit tools"
```

---

### Task 4: Tier 2 analysis tools — crawl, detect

**Files:**
- Create: `pkg/scout/mcp/tools_analysis.go`
- Create: `pkg/scout/mcp/tools_analysis_test.go`
- Modify: `pkg/scout/mcp/server.go` (add `registerAnalysisTools(server, state)` call)
- Modify: `pkg/scout/mcp/server_test.go` (update expected tools list)

**Step 1: Create `tools_analysis.go` with 2 tools**

Register function `registerAnalysisTools(server *mcp.Server, state *mcpState)`:

- **`crawl`** — Crawl a website and return discovered pages
  - Input: `url` (string, required), `maxDepth` (int, optional, default 2), `maxPages` (int, optional, default 50)
  - Calls: `browser.Crawl(url, handler, opts...)` collecting results into slice
  - Crawl options: `scout.WithMaxDepth(n)`, `scout.WithMaxPages(n)`
  - Returns: JSON array of `{url, title, depth, links}` for discovered pages

- **`detect`** — Detect technologies, frameworks, and rendering mode
  - Input: none (operates on current page)
  - Calls: `page.DetectFrameworks()`, `page.DetectPWA()`, `page.DetectRenderMode()`, `page.DetectTechStack()`
  - Returns: JSON with `{frameworks, pwa, renderMode, techStack}`

**Step 2: Create `tools_analysis_test.go`**

- `TestDetectTool` — navigate to test page, call detect, verify JSON structure has frameworks array

Crawl test is harder (needs multiple linked pages in httptest). Create `/crawl-root` with links to `/crawl-page1` and `/crawl-page2`:
- `TestCrawlTool` — crawl test server, verify found pages >= 2

**Step 3: Wire, update expected, test, commit**

```
git commit -m "feat(mcp): add crawl, detect analysis tools"
```

---

### Task 5: Tier 3 inspection tools — hijack, storage, har, swagger

**Files:**
- Create: `pkg/scout/mcp/tools_inspect.go`
- Create: `pkg/scout/mcp/tools_inspect_test.go`
- Modify: `pkg/scout/mcp/server.go` (add `registerInspectTools(server, state)` call)
- Modify: `pkg/scout/mcp/server_test.go` (update expected tools list)

**Step 1: Create `tools_inspect.go` with 4 tools**

Register function `registerInspectTools(server *mcp.Server, state *mcpState)`:

- **`storage`** — Get, set, list, or clear web storage (localStorage/sessionStorage)
  - Input: `action` (string, required: "get"|"set"|"list"|"clear"), `key` (string, for get/set), `value` (string, for set), `sessionStorage` (bool, optional — use sessionStorage instead of localStorage)
  - Dispatches to `page.LocalStorageGet/Set/GetAll/Clear` or `page.SessionStorageGet/Set/GetAll/Clear`
  - Returns: JSON for get/list, confirmation text for set/clear

- **`hijack`** — Start capturing network traffic from current page
  - Input: `urlFilter` (string, optional), `captureBody` (bool, optional)
  - Calls: `page.NewSessionHijacker(opts...)` with `WithHijackURLFilter`, `WithHijackBodyCapture`
  - Collects events for up to 10 seconds or 100 events (whichever first)
  - Returns: JSON array of captured events

- **`har`** — Record network traffic as HAR
  - Input: `action` (string, required: "start"|"stop"|"export"), `captureBody` (bool, optional for start)
  - `start`: begins HAR recording via page eval injecting performance observer
  - `stop`: stops recording
  - `export`: returns collected HAR JSON
  - Note: This uses `page.Eval()` with Performance/Resource Timing API since MCP doesn't use gRPC daemon. Simpler implementation: capture via `performance.getEntriesByType('resource')`.

- **`swagger`** — Detect and extract Swagger/OpenAPI specs from a URL
  - Input: `url` (string, required), `endpointsOnly` (bool, optional)
  - Calls: `browser.ExtractSwagger(url, opts...)` with `scout.WithSwaggerEndpointsOnly()`
  - Returns: JSON with spec info, endpoints, schemas

**Step 2: Create `tools_inspect_test.go`**

- `TestStorageToolSetGet` — set value via eval, get via storage tool, verify
- `TestStorageToolList` — set values, list, verify keys
- `TestStorageToolClear` — set then clear, verify empty
- `TestSwaggerTool` — serve minimal OpenAPI JSON, extract, verify info

Hijack and HAR are harder to test without real browser. Test with browser-required skip pattern:
- `TestHijackTool` — start hijack on test page with XHR, verify events captured
- `TestHarTool` — start/export, verify entries

**Step 3: Wire, update expected, test, commit**

```
git commit -m "feat(mcp): add storage, hijack, har, swagger inspection tools"
```

---

### Task 6: Update help text and documentation

**Files:**
- Modify: `cmd/scout/mcp.go` (update Tools list in Long description)

**Step 1:** Update the `Long` description's Tools line to list all 32 tools:
```
Tools: navigate, click, type, screenshot, snapshot, extract, eval, back, forward, wait,
       search, fetch, pdf, session_list, session_reset, open, ping, curl,
       markdown, table, meta, cookie, header, block,
       form_detect, form_fill, form_submit, crawl, detect,
       storage, hijack, har, swagger
```

**Step 2:** Commit

```
git commit -m "docs(mcp): update help text with all 32 tools"
```

---

## File Structure After Implementation

```
pkg/scout/mcp/
├── doc.go                    # Package docs
├── server.go                 # Core server + 16 original tools + wiring
├── server_test.go            # Original tests + TestListTools updated
├── diag.go                   # ping, curl (existing)
├── diag_test.go              # ping, curl tests (existing)
├── tools_content.go          # markdown, table, meta
├── tools_content_test.go
├── tools_network.go          # cookie, header, block
├── tools_network_test.go
├── tools_form.go             # form_detect, form_fill, form_submit
├── tools_form_test.go
├── tools_analysis.go         # crawl, detect
├── tools_analysis_test.go
├── tools_inspect.go          # storage, hijack, har, swagger
├── tools_inspect_test.go
```

## Tool Count Progression

| After Task | Total MCP Tools |
|-----------|----------------|
| Existing  | 18 (including ping, curl, open) |
| Task 1    | 21 (+markdown, table, meta) |
| Task 2    | 24 (+cookie, header, block) |
| Task 3    | 27 (+form_detect, form_fill, form_submit) |
| Task 4    | 29 (+crawl, detect) |
| Task 5    | 33 (+storage, hijack, har, swagger) |
| Task 6    | 33 (docs only) |
