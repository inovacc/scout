# REPL and MCP Server UX Analysis - Research

**Researched:** 2026-03-29
**Domain:** Browser automation CLI (REPL) + MCP server UX
**Confidence:** HIGH (direct source code analysis)

## Summary

Scout exposes browser automation through two interactive interfaces: a CLI REPL (`cmd/scout/repl.go`) and an MCP server (`pkg/scout/mcp/`). Both share the same underlying engine (`internal/engine/`) but have divergent feature sets, no shared command logic, and inconsistent capabilities. The REPL is minimal with no readline support, while the MCP server is well-structured but has stale documentation (CLI help says 33 tools, actual count is 18).

**Primary finding:** The REPL and MCP server duplicate interaction patterns independently. The REPL lacks modern terminal UX (no tab completion, no history, no syntax highlighting). The MCP server is solid architecturally but has gaps in error context and some capabilities only exist in one interface.

## 1. REPL Current State

### File: `cmd/scout/repl.go` (454 lines)

**20 Commands** (matching help text):
| Command | Aliases | Arguments | What It Does |
|---------|---------|-----------|--------------|
| `navigate` | `go`, `nav` | `<url>` | Opens new page, closes old one |
| `eval` | - | `<js expression>` | Evaluates JS, prints result |
| `click` | - | `<selector>` | Clicks CSS-selected element |
| `type` | - | `<selector> <text>` | Types into element via `el.Input()` |
| `extract` | - | `<selector>` | Extracts text via `page.ExtractText()` |
| `screenshot` | - | `[file]` | Saves PNG (default: `screenshot.png`) |
| `markdown` | `md` | - | Gets page as markdown |
| `html` | - | - | Gets full page HTML |
| `cookies` | - | - | JSON-dumps cookies |
| `url` | - | - | Shows current URL |
| `title` | - | - | Shows page title |
| `wait` | - | `[selector]` | Waits for load or element |
| `back` | - | - | History back |
| `forward` | - | - | History forward |
| `reload` | - | - | Reloads page |
| `tabs` | - | - | Lists tabs with `*` for active |
| `tab` | - | `<index>` | Switches active tab |
| `newtab` | - | `[url]` | Opens new tab |
| `health` | - | - | Runs health check (depth=1, concurrency=1) |
| `help` | - | - | Prints command list |
| `exit` | `quit` | - | Exits REPL |

### Input Handling

- Uses `bufio.Scanner(os.Stdin)` -- raw line reading, no readline library
- Command parsing: `strings.SplitN(line, " ", 3)` -- splits into max 3 parts
- Dynamic prompt: `[host/path] > ` when page is open, `> ` otherwise
- No input history (up-arrow does nothing)
- No tab completion
- No multi-line input support
- No command abbreviation beyond hardcoded aliases (`go`, `nav`, `md`)

### UX Flow

1. `scout repl [url]` launches browser with `baseOpts(cmd)` (headless/sandbox/browser/stealth flags)
2. If URL provided, opens page and waits for load
3. Enter command loop: print prompt, read line, dispatch
4. `navigate` creates a NEW page each time (calls `b.NewPage()`), closes old page
5. All output goes to `cmd.OutOrStdout()`
6. Errors displayed as `ERROR: <message>` inline

### What's Missing or Clunky

1. **No readline/history** -- `bufio.Scanner` is the most basic input possible. No up-arrow history, no Ctrl+R search, no Ctrl+A/E line editing.
2. **No tab completion** -- for commands, CSS selectors, or URLs.
3. **`navigate` creates new page** -- doesn't reuse current page via `page.Navigate()`. This means losing page state (cookies, session storage) on every navigation.
4. **`type` command parsing bug** -- `SplitN(line, " ", 3)` means `type #input hello world` works, but the text with spaces is the 3rd part only. Can't type into selectors that contain spaces.
5. **No `snapshot` command** -- MCP has `snapshot` (accessibility tree), REPL does not.
6. **No `pdf` command** -- MCP has `pdf`, REPL does not.
7. **No `open` command** -- MCP has `open` (headed browser for inspection), REPL does not.
8. **No full-page screenshot** -- MCP has `fullPage` option, REPL always does viewport screenshot.
9. **No `elements` command** -- can't query multiple elements or list matches.
10. **No output formatting** -- raw text dumps, no color, no paging for long output.
11. **No `.` or `!!` to repeat last command**.
12. **No scripting/macro support** -- can't pipe commands from a file (well, you can via stdin, but no explicit support).
13. **Duplicate `truncate` function** -- defined in both `helpers.go:137` and `tools_websocket.go:155` with slightly different implementations (helpers subtracts 3 from maxLen, websocket does not).

## 2. MCP Server Current State

### File: `pkg/scout/mcp/server.go` + tool files

**Architecture:**
- `mcpState` struct holds lazy-initialized browser + current page (singleton pattern)
- `ensureBrowser()` / `ensurePage()` with mutex for thread safety
- `addTracedTool()` wrapper adds OpenTelemetry spans + metrics to every tool
- Idle timer auto-shutdown via `idle.Timer`
- Two transport modes: stdio (`Serve()`) and HTTP+SSE (`ServeSSE()`)

**18 Built-in Tools** (verified by counting `addTracedTool` calls):

| Tool | File | Input Schema | Description |
|------|------|-------------|-------------|
| `navigate` | tools_browser.go | `{url: string}` | Navigate to URL, 15s WaitLoad timeout |
| `click` | tools_browser.go | `{selector: string}` | Click element by CSS |
| `type` | tools_browser.go | `{selector: string, text: string}` | Type into element |
| `extract` | tools_browser.go | `{selector: string}` | Extract text from element |
| `eval` | tools_browser.go | `{expression: string}` | Evaluate JavaScript |
| `back` | tools_browser.go | `{}` | Navigate back |
| `forward` | tools_browser.go | `{}` | Navigate forward |
| `wait` | tools_browser.go | `{selector?: string}` | Wait for load or selector |
| `screenshot` | tools_capture.go | `{fullPage?: bool}` | Take screenshot (returns image content) |
| `snapshot` | tools_capture.go | `{interactableOnly?: bool, maxDepth?: int, iframes?: bool, filter?: string}` | Accessibility tree |
| `pdf` | tools_capture.go | `{landscape?: bool, printBackground?: bool, scale?: number}` | Generate PDF |
| `session_list` | tools_session.go | `{}` | Current session info (URL, title) |
| `session_reset` | tools_session.go | `{}` | Close browser, force re-init |
| `open` | tools_session.go | `{url: string, devtools?: bool}` | Open headed browser for inspection |
| `swarm_crawl` | tools_swarm.go | `{url: string, workers?: int, depth?: int, maxPages?: int}` | Parallel BFS crawl |
| `ws_listen` | tools_websocket.go | `{urlFilter?: string, duration?: int}` | Monitor WS traffic |
| `ws_send` | tools_websocket.go | `{script: string}` | Send WS message via JS |
| `ws_connections` | tools_websocket.go | `{}` | List active WS connections |

**3 Resources:**
| URI | Name | Returns |
|-----|------|---------|
| `scout://page/markdown` | Page Markdown | Page content as markdown |
| `scout://page/url` | Page URL | Current URL string |
| `scout://page/title` | Page Title | Current page title |

**Plus dynamic tools:**
- Plugin-provided tools via `cfg.PluginManager.RegisterMCPTools(server)`
- WebMCP tools via `RegisterWebMCPTools()` -- namespaced as `webmcp_<origin>_<name>`

### Tool UX Analysis

**Input schemas:** All defined as raw JSON strings (not typed Go structs with jsonschema tags). This works but is fragile -- no compile-time validation of schema correctness. The MCP Go SDK supports typed input structs with `jsonschema` tags which would be more maintainable.

**Output format:**
- Success: `textResult(msg)` returns plain text
- Structured data: `jsonResult(v)` returns JSON-indented string
- Errors: `errResult(msg)` sets `IsError: true` with text content
- Binary: Screenshots return `ImageContent` with `image/png` MIME type
- PDFs return `ImageContent` with `application/pdf` MIME type

**Error messages:** Inconsistent prefixing:
- Some use `"scout-mcp: <tool>: <error>"` (e.g., `ws_listen`, `pdf`, `open`)
- Some use bare error message (e.g., `click`, `type`, `extract`)
- `navigate` uses `"scout: navigate to <url>: <error>"`

**Stale CLI help text:** `cmd/scout/mcp.go:36-44` lists 33 tools in categories (Browser, Content, Network, Forms, Analysis, Inspection, Session) but only 18 actually exist. The other 15 were migrated to plugins but the help text was not updated.

### MCP Navigate vs REPL Navigate

Critical difference: MCP `navigate` calls `page.Navigate(args.URL)` on the existing page (reuses session), while REPL `navigate` calls `b.NewPage(parts[1])` creating a fresh page each time. MCP approach is correct for maintaining state; REPL approach loses cookies/storage.

### MCP Navigate WaitLoad

MCP `navigate` has a smart 15-second timeout for `WaitLoad()` to handle SPAs that never fire the load event. REPL just calls `page.WaitLoad()` with no timeout, which can hang indefinitely.

## 3. REPL Improvement Opportunities

### P0 -- Critical UX Fixes

1. **Add readline support**: Use `github.com/chzyer/readline` or `github.com/peterh/liner` for:
   - Command history (persisted to `~/.scout/repl_history`)
   - Line editing (Ctrl+A, Ctrl+E, Ctrl+K, etc.)
   - Up/down arrow for history navigation
   - Ctrl+R reverse search

2. **Fix navigate to reuse page**: Change `navigate` to call `page.Navigate(url)` instead of `b.NewPage(url)`. Only create new page if none exists.

3. **Add WaitLoad timeout**: Match MCP's 15-second timeout pattern to avoid hanging on SPAs.

### P1 -- Feature Parity with MCP

4. **Add `snapshot` command**: Expose accessibility tree (matches MCP `snapshot` tool).
5. **Add `pdf [file]` command**: Generate PDF (matches MCP `pdf` tool).
6. **Add `fullscreenshot [file]` command**: Full-page screenshot (matches MCP `screenshot --fullPage`).
7. **Add `open <url>` command**: Open headed browser (matches MCP `open` tool).

### P2 -- Enhanced UX

8. **Tab completion**: Complete command names, and optionally CSS selectors from current page DOM.
9. **Color output**: Use ANSI colors for errors (red), success confirmations (green), URLs (blue).
10. **Command history file**: Persist to `~/.scout/repl_history`.
11. **Alias support**: Allow `.scoutrc` or similar for custom aliases.
12. **Repeat last command**: `!!` or `.` to re-execute.
13. **Pipe-friendly mode**: Detect non-TTY stdin and skip prompts for scripting.
14. **`elements <selector>` command**: List all matching elements with index, tag, text preview.

### P3 -- Nice-to-Have

15. **`set` command**: Change headless/stealth/timeout at runtime.
16. **`export` command**: Save current page state (cookies, localStorage) to file.
17. **`import` command**: Load saved state.
18. **`watch <selector> <interval>`**: Poll element for changes.

## 4. MCP Improvement Opportunities

### Error Consistency

- Standardize all error messages to format: `"scout-mcp: <tool_name>: <context>: <error>"`
- Currently 3 different conventions used across tools

### Input Schema Modernization

- Migrate from `json.RawMessage` string literals to typed Go structs with `jsonschema` tags
- Current approach is error-prone (typos in JSON strings won't be caught at compile time)
- The Go MCP SDK supports: `type MyInput struct { URL string \x60json:"url" jsonschema:"URL to navigate to"\x60 }`

### Tool Description Improvements

| Tool | Current Description | Suggested Improvement |
|------|--------------------|-----------------------|
| `extract` | "Extract text from an element by CSS selector" | "Extract visible text content from a DOM element matched by CSS selector. Returns innerText." |
| `wait` | "Wait for a page condition (load, selector)" | "Wait for page load event (no args) or for a CSS selector to appear in DOM. Blocks until condition met or timeout." |
| `snapshot` | Good -- already explains it's an alternative to screenshot | No change needed |
| `session_list` | "List current session info (URL, title of current page)" | "Get current browser session status. Returns JSON with status, URL, and title. Returns 'no active session' if no browser is running." |

### Missing Capabilities

1. **No `markdown` tool** -- exists as a resource (`scout://page/markdown`) but not as a tool. AI agents typically prefer tools over resources. Should add a `markdown` tool.
2. **No `html` tool** -- page HTML only accessible via `eval` with `document.documentElement.outerHTML`.
3. **No `cookies` tool** -- page cookies not directly accessible via MCP tool.
4. **No `reload` tool** -- available in REPL but not MCP.
5. **No `tabs`/`tab` tools** -- no multi-tab management in MCP.
6. **No `title`/`url` tools** -- only available as resources, not tools. Resources require a separate protocol round-trip and not all MCP clients support resources well.

### Tool Combination Opportunities

- `navigate` could accept optional `waitFor` selector to combine navigate+wait in one call
- `click` could accept optional `waitAfter` selector to wait for navigation/element after click
- A `fill_form` composite tool could combine multiple `type` + `click` operations

### CLI Help Text Fix

`cmd/scout/mcp.go` lines 36-44 list 33 tools but only 18 exist. The help text must be updated to reflect the actual tool set after plugin migration.

## 5. Shared Capabilities Analysis

### In REPL but NOT in MCP (tools)

| Capability | REPL Command | Why Missing from MCP |
|------------|-------------|---------------------|
| Get page HTML | `html` | Only via `eval` workaround |
| Get cookies | `cookies` | Not exposed as tool |
| Reload page | `reload` | Not exposed as tool |
| List tabs | `tabs` | No multi-tab support |
| Switch tab | `tab <n>` | No multi-tab support |
| Open new tab | `newtab` | No multi-tab support |
| Health check | `health` | Not exposed (was likely a plugin) |
| Get page markdown | `markdown` | Only as resource, not tool |
| Get page title | `title` | Only as resource, not tool |
| Get page URL | `url` | Only as resource, not tool |

### In MCP but NOT in REPL

| Capability | MCP Tool | Why Missing from REPL |
|------------|---------|----------------------|
| Accessibility snapshot | `snapshot` | Never added |
| PDF generation | `pdf` | Never added |
| Full-page screenshot | `screenshot(fullPage)` | Never added |
| Open headed browser | `open` | REPL IS the headed browser |
| Swarm crawl | `swarm_crawl` | Complex, maybe too much for REPL |
| WebSocket monitoring | `ws_listen` | Never added |
| WebSocket send | `ws_send` | Never added |
| WebSocket connections | `ws_connections` | Never added |
| Session reset | `session_reset` | REPL manages own lifecycle |

### Code Sharing Opportunity

Both interfaces independently implement the same operations against the `*scout.Page` API. A shared "command executor" layer could:

1. Define a `Command` interface with `Name()`, `Execute(page, args) (string, error)`
2. Register commands once
3. REPL wraps with readline + prompt
4. MCP wraps with `addTracedTool` + JSON schema

This would eliminate the current duplication where, for example:
- REPL `click` (repl.go:129-149) and MCP `click` (tools_browser.go:58-85) do the exact same thing
- REPL `extract` (repl.go:174-190) and MCP `extract` (tools_browser.go:117-147) do the exact same thing

Key files for shared layer:
- `pkg/scout/mcp/tools_browser.go` -- 8 tools
- `pkg/scout/mcp/tools_capture.go` -- 3 tools
- `cmd/scout/repl.go` -- 20 commands (overlapping set)

## 6. Browser Control Flow Analysis

### "Open, Navigate, Interact, Extract, Close" Flow

**REPL Flow:**
```
scout repl https://example.com
  -> scout.New(baseOpts...)           # launch browser
  -> b.NewPage(url)                   # create page + navigate
  -> page.WaitLoad()                  # wait for DOM
  [user types commands]
  -> page.Element(sel) / page.Click() # interact
  -> page.ExtractText(sel)            # extract
  -> exit                             # b.Close() via defer
```

**MCP Flow:**
```
[MCP client sends navigate tool call]
  -> state.ensureBrowser()            # lazy launch (first call only)
  -> state.ensurePage()               # lazy create page (first call only)
  -> page.Navigate(url)               # navigate existing page
  -> page.WaitLoad() with 15s timeout # smart wait
[MCP client sends extract tool call]
  -> state.ensurePage()               # returns existing page
  -> page.Element(sel).Text()         # extract
[MCP client sends session_reset]
  -> state.reset()                    # close everything
  -> time.Sleep(500ms)                # let OS release resources
```

### Friction Points

1. **REPL page lifecycle is destructive**: Every `navigate` command creates a new page and closes the old one. This loses all page state (cookies, service workers, localStorage). MCP correctly reuses the page.

2. **MCP is single-page only**: `mcpState` holds exactly one `page`. No multi-tab support. If an AI agent needs to compare two pages, it must navigate back and forth (losing scroll position, dynamic state).

3. **REPL has no error recovery**: If `b.NewPage()` fails, the error is printed but `page` may be left as `nil` or stale. No retry logic.

4. **MCP has no navigate-and-extract shortcut**: Common AI agent pattern is "go to URL and get content." This requires two tool calls (navigate + extract/snapshot/markdown). A combined tool would reduce latency.

5. **MCP 500ms sleep on reset**: `state.reset()` has a hardcoded `time.Sleep(500 * time.Millisecond)` after closing the browser. This blocks the calling goroutine. Could be improved with a readiness check instead.

6. **MCP screenshot returns binary**: Screenshot tool returns raw `ImageContent` bytes. Some MCP clients may not handle binary content well. No option to save to file and return path instead.

7. **REPL `type` command can't handle selectors with spaces**: `SplitN(line, " ", 3)` means `type "my selector" hello` won't work. Should support quoted arguments.

8. **Neither interface exposes `WaitStable`/`WaitDOMStable`/`WaitIdle`**: These are available on the Page API (`page.go:436-489`) but neither REPL nor MCP exposes them. They're critical for SPAs where `WaitLoad` fires too early.

## Duplicate Code: `truncate` Function

Two implementations exist:
- `cmd/scout/helpers.go:137`: `s[:maxLen-3] + "..."` (reserves space for ellipsis)
- `pkg/scout/mcp/tools_websocket.go:155`: `s[:maxLen] + "..."` (appends beyond maxLen)

The helpers.go version is correct (output never exceeds maxLen). The websocket version can produce strings longer than maxLen.

## Sources

### Primary (HIGH confidence)
- Direct source code analysis of all files listed above
- `cmd/scout/repl.go` -- full REPL implementation (454 lines)
- `pkg/scout/mcp/server.go` -- MCP server core (349 lines)
- `pkg/scout/mcp/tools_browser.go` -- 8 browser tools
- `pkg/scout/mcp/tools_capture.go` -- 3 capture tools
- `pkg/scout/mcp/tools_session.go` -- 3 session tools
- `pkg/scout/mcp/tools_swarm.go` -- 1 swarm tool
- `pkg/scout/mcp/tools_websocket.go` -- 3 WebSocket tools
- `pkg/scout/mcp/resources.go` -- 3 MCP resources
- `cmd/scout/mcp.go` -- CLI command definition
- `cmd/scout/helpers.go` -- shared CLI helpers
- `internal/engine/page.go` -- Page API surface (60+ methods)

## Metadata

**Confidence breakdown:**
- REPL state: HIGH -- complete source read
- MCP state: HIGH -- complete source read of all tool files
- Shared capabilities gap: HIGH -- direct comparison
- Improvement recommendations: HIGH -- based on observed code patterns
- Browser control flow: HIGH -- traced through actual code paths

**Research date:** 2026-03-29
**Valid until:** 2026-04-28 (stable codebase, not fast-moving dependencies)
