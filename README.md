# Scout

Browser automation, web scraping, and site testing for AI agents.

Scout ships as a **Claude Code plugin** (with Codex CLI and Gemini CLI on the roadmap). Install the plugin, restart your AI host, and Scout's full surface — agents, skills, slash commands, and MCP tools — is available to your agent without writing a line of code.

## Quick start

```bash
# 1. Install the scout binary
go install github.com/inovacc/scout/cmd/scout@latest

# 2. Install the plugin into your AI host (default: Claude Code)
scout plugin install

# 3. Restart Claude Code. That's it.
```

After restart you have:

- **6 skills** (auto-triggered): `scrape`, `screenshot`, `test-site`, `gather`, `crawl`, `monitor`
- **6 slash commands** (user-invoked): `/scout:scrape`, `/scout:screenshot`, `/scout:test-site`, `/scout:gather`, `/scout:crawl`, `/scout:monitor`
- **5 agents** (via Task tool): `site-tester`, `web-scraper`, `site-mapper`, `session-capture`, `flow-porter`
- **MCP tools** (direct): `gather`, `test_site`, `sitemap`, `report_*`, `runbook_*`, `screenshot`, `snapshot`, `navigate`, `click`, `type`, `eval`, `ws_listen`, … (full list: `scout plugin extract --target ./out && cat ./out/.mcp.json`)

Verify the install: `scout plugin doctor --host claude`. Other hosts: `scout plugin install --host all`.

## Features

- **Browser Management** - Launch, configure, and control headless Chromium with functional options (`WithHeadless`, `WithProxy`, `WithStealth`, `WithIncognito`, etc.)
- **Multi-Browser Support** - Chrome (default), Brave, and Microsoft Edge with auto-download and `~/.scout/browsers/` cache isolation
- **Page Navigation** - Navigate, reload, go back/forward, wait for load/idle/DOM stability
- **Element Interaction** - Click, double-click, right-click, hover, tap, input text, select options, file uploads
- **Element Finding** - CSS selectors, XPath, text regex matching, JS evaluation, coordinate-based lookup, DevTools search
- **Screenshots & PDF** - Viewport, full-page, and scroll screenshots in PNG/JPEG; PDF generation with configurable options
- **JavaScript Evaluation** - Execute JS at page and element level with typed result access (`String()`, `Int()`, `Float()`, `Bool()`, `Decode()`)
- **Network Control** - Set headers, manage cookies, intercept/modify requests via hijacking, block URLs by pattern, HTTP basic auth
- **Stealth Mode** - 17 anti-bot evasions including canvas/audio noise, WebGL, WebRTC, timezone, fonts, battery, and toString integrity
- **Session Hijacking** - Real-time HTTP + WebSocket traffic capture via CDP events with channel-based event streaming and HAR export
- **Fingerprint Rotation** - Per-session, per-page, per-domain, or interval-based fingerprint strategies with persistent store
- **Device Emulation** - Viewport sizing, window bounds, device profile emulation
- **Struct-Tag Extraction** - Extract data into Go structs using `scout:"selector"` tags
- **Form Interaction** - Detect, fill, and submit forms; CSRF token extraction; multi-step wizards
- **Pagination** - Click-next, URL-pattern, infinite-scroll, and load-more with Go generics
- **Search Engine Integration** - Google, Bing, DuckDuckGo, Wikipedia, Google Scholar, Google News
- **Web Crawling** - BFS crawling with depth/page limits, domain filtering, sitemap parsing
- **HAR Network Recording** - Capture HTTP traffic via CDP events and export as HAR 1.2 format
- **LLM-Powered Extraction** - 6 built-in providers (Ollama, OpenAI, Anthropic, OpenRouter, DeepSeek, Gemini) with review pipeline
- **Research Presets** - Shallow, Medium, and Deep research modes with caching and incremental research
- **Scraper Framework** - 20 pluggable modes with AES-256-GCM encrypted session persistence
- **MCP Server** - 17 built-in tools + plugin-contributed tools for LLM browser control via stdio
- **Plugin System** - Subprocess JSON-RPC plugins with Go SDK, marketplace, and 12 built-in plugins
- **Electron Support** - `WithElectronApp(path)` with auto-download runtime
- **REPL Mode** - Interactive browser shell with 20 commands, no daemon required
- **Health Check** - Site-wide broken link, console error, and network failure detection
- **Visual Monitoring** - Pixel-level visual regression testing with baseline management
- **Reports** - AI-consumable markdown reports for health checks, gather, and crawl results
- **Chrome Extensions** - Load unpacked, download from Chrome Web Store, embedded bridge extension

## Installation (library + standalone CLI)

> **For most users, prefer the plugin install above.** This section is for embedding Scout in your own Go application, scripting from a shell, or running the standalone CLI without an AI host.

**CLI (Go):**

```bash
go install github.com/inovacc/scout/cmd/scout@latest
```

**CLI (npm):**

```bash
npm install -g @inovacc/scout-browser
```

**Library:**

```bash
go get github.com/inovacc/scout/pkg/scout
```

Requires Go 1.25+ for building from source. A Chromium-based browser is auto-downloaded if not present.

The full CLI surface (~60 verbs, 188 leaf commands) remains available as thin shims over the same `pkg/scout/tools/` package the MCP server uses. Every ported verb's `--help` text points at its MCP equivalent (e.g. `scout gather --help` → "Also available as MCP tool `mcp__scout__gather`").

## Quick Start

### As a Library

```go
package main

import (
	"fmt"
	"log"

	"github.com/inovacc/scout/pkg/scout"
)

func main() {
	b, err := scout.New(scout.WithHeadless(true))
	if err != nil {
		log.Fatal(err)
	}
	defer b.Close()

	page, err := b.NewPage("https://example.com")
	if err != nil {
		log.Fatal(err)
	}

	title, err := page.Title()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(title)
}
```

### As a CLI

Single-shot verbs each run one browser process and take a URL directly:

```bash
# Inspect a page
scout title https://example.com
scout url https://example.com
scout text https://example.com "h1"

# Take a screenshot
scout screenshot https://example.com --output=page.png

# Extract data
scout eval https://example.com "document.title"
scout html https://example.com --selector="div.content"
scout table https://example.com --selector="table"
scout meta https://example.com

# Search engines (standalone)
scout search "golang web scraping" --engine=google

# Crawl a site (standalone)
scout crawl https://example.com --max-depth=2

# Multi-step automation (one process): interactive shell,
# the stdio MCP server, or a strategy/runbook file
scout repl https://example.com

# Inspect captured session files
scout session list
```

## Examples

The [`examples/`](examples/) directory contains 18 runnable programs organized by complexity:

**Simple** -- basic-navigation, screenshot, extract-struct, extract-table, extract-meta, javascript-eval, form-fill, cookies-headers

**Advanced** -- search-engines, pagination, crawl-site, sitemap-parser, rate-limited-scraper, form-wizard, request-intercept, stealth-scraper, pdf-generator, har-recorder

```bash
cd examples/simple/basic-navigation && go run .
```

See [`examples/README.md`](examples/README.md) for the full index with descriptions and key APIs.

## Flow capture → replay

Record a browser flow once, then replay its underlying API calls deterministically with **no browser**:

```bash
# 1. Capture a flow's network traffic (drives a real browser via the session hijacker)
scout flow capture https://app.example.com/checkout -o capture.json

# 2. Analyze it into a reviewed flow.yaml + report (LLM infers token/CSRF/ID chains)
scout flow analyze capture.json --llm ollama -o flow.yaml   # review the report on stderr

# 3. Replay the REST/GraphQL calls with no browser (auth pulled from a vault profile)
scout flow run flow.yaml --profile <vault-profile-id>

# 4. Verify parity: re-run and diff status vs the golden capture
scout flow verify flow.yaml --golden capture.json
```

The generated `flow.yaml` is the deterministic contract — the LLM runs only at `analyze` time. Values chain between steps via `${var}` (extracted from a response) and `${secret.NAME}` (resolved from a vault profile at send time, never embedded). Generated specs are **secret-free**: the analyzer parameterizes secret-bearing headers and URL-query tokens to `${secret.*}` and redacts secret values from the LLM digest. v1 supports REST/JSON + GraphQL.

## Extraction

Extract data into Go structs using struct tags:

```go
type Product struct {
    Name  string   `scout:"h2.title"`
    Price string   `scout:"span.price"`
    Image string   `scout:"img.hero@src"`
    Tags  []string `scout:"span.tag"`
}

var p Product
err := page.Extract(&p)
```

Extract tables:

```go
table, err := page.ExtractTable("table#data")
// table.Headers = ["Name", "Age", "City"]
// table.Rows = [["Alice", "30", "NYC"], ...]

maps, err := page.ExtractTableMap("table#data")
// maps[0]["Name"] = "Alice"
```

Extract metadata:

```go
meta, err := page.ExtractMeta()
// meta.Title, meta.Description, meta.OG["og:image"], meta.JSONLD
```

## Forms

```go
form, err := page.DetectForm("#login")
err = form.Fill(map[string]string{
    "username": "user",
    "password": "pass",
})
err = form.Submit()

// Or with struct tags
type Login struct {
    User string `form:"username"`
    Pass string `form:"password"`
}
err = form.FillStruct(Login{User: "user", Pass: "pass"})
```

## Pagination

```go
type Item struct {
    Name  string `scout:"span.name"`
    Price int    `scout:"span.price"`
}

// URL-pattern pagination
items, err := scout.PaginateByURL[Item](browser, func(page int) string {
    return fmt.Sprintf("https://shop.com/items?page=%d", page)
}, scout.WithPaginateMaxPages(5))

// Click-next pagination
items, err := scout.PaginateByClick[Item](page, "a.next")

// Infinite scroll
items, err := scout.PaginateByScroll[Item](page, "div.item",
    scout.WithPaginateMaxPages(20))

// Load-more button
items, err := scout.PaginateByLoadMore[Item](page, "button.load-more")
```

## Search

```go
results, err := browser.Search("golang web scraping",
    scout.WithSearchEngine(scout.Google),
)
for _, r := range results.Results {
    fmt.Printf("%d. %s - %s\n", r.Position, r.Title, r.URL)
}
```

## Crawling

```go
results, err := browser.Crawl("https://example.com", func(page *scout.Page, result *scout.CrawlResult) error {
    fmt.Printf("Crawled: %s (depth=%d, links=%d)\n", result.URL, result.Depth, len(result.Links))
    return nil
},
    scout.WithCrawlMaxDepth(2),
    scout.WithCrawlMaxPages(50),
)
```

## HAR Network Recording

```go
recorder := scout.NewNetworkRecorder(page,
    scout.WithCaptureBody(true),
    scout.WithCreatorName("my-tool", "1.0"),
)
defer recorder.Stop()

// Navigate and interact -- all HTTP traffic is captured
page.Navigate("https://example.com")

// Export as HAR 1.2
harJSON, entryCount, err := recorder.ExportHAR()
os.WriteFile("capture.har", harJSON, 0644)
```

## Rate Limiting

```go
rl := scout.NewRateLimiter(
    scout.WithRateLimit(2),     // 2 requests/sec
    scout.WithMaxRetries(3),
    scout.WithBackoff(time.Second),
)

err := rl.Do(func() error {
    return page.Navigate("https://example.com")
})

// Or use the convenience method
err := page.NavigateWithRetry("https://example.com", rl)
```

## Browser Options

| Option                            | Description                                                  | Default         |
|-----------------------------------|--------------------------------------------------------------|-----------------|
| `WithHeadless(bool)`              | Run in headless mode                                         | `true`          |
| `WithStealth()`                   | Enable anti-bot-detection (17 evasions)                      | disabled        |
| `WithProxy(url)`                  | Set proxy server                                             | none            |
| `WithUserAgent(ua)`               | Custom User-Agent                                            | browser default |
| `WithWindowSize(w, h)`            | Browser window size                                          | 1920x1080       |
| `WithTimeout(d)`                  | Default operation timeout                                    | 30s             |
| `WithSlowMotion(d)`               | Delay between actions (debugging)                            | none            |
| `WithIgnoreCerts()`               | Skip TLS verification                                        | disabled        |
| `WithExecPath(path)`              | Path to browser binary                                       | auto-detect     |
| `WithUserDataDir(dir)`            | Persistent session directory                                 | temp            |
| `WithIncognito()`                 | Incognito mode                                               | disabled        |
| `WithEnv(env...)`                 | Set environment variables for browser                        | none            |
| `WithNoSandbox()`                 | Disable sandbox (containers)                                 | disabled        |
| `WithWindowState(state)`          | Initial window state (normal/minimized/maximized/fullscreen) | normal          |
| `WithLaunchFlag(name, values...)` | Add custom Chrome CLI flag                                   | none            |
| `WithXvfb(args...)`               | Enable Xvfb for headful mode without display (Unix only)     | disabled        |
| `WithExtension(paths...)`         | Load unpacked Chrome extensions by directory path            | none            |
| `WithExtensionByID(ids...)`       | Load downloaded Chrome extensions by Web Store ID            | none            |
| `WithBridge()`                    | Enable Scout Bridge extension for Go<>browser communication  | enabled         |
| `WithBrowser(BrowserType)`        | Select browser: chrome, brave, edge                          | chrome          |
| `WithDevTools()`                  | Open Chrome DevTools for each tab                            | disabled        |
| `WithFingerprintRotation(cfg)`    | Enable fingerprint rotation strategy                         | disabled        |
| `WithResearchPreset(preset)`      | Set research depth: Shallow, Medium, Deep                    | none            |
| `WithRemoteCDP(endpoint)`         | Connect to existing Chrome DevTools endpoint (dormant)       | none            |
| `WithElectronApp(path)`           | Launch an Electron application                               | none            |

## MCP Server (LLM Integration)

Scout includes a [Model Context Protocol](https://modelcontextprotocol.io/) server exposing 17 built-in browser automation tools for LLMs like Claude, plus additional tools contributed by plugins. The server runs over **stdio only**.

```bash
# Install for Claude Code (local project)
scout mcp --install

# Install globally
scout mcp --install --global

# Start manually (stdio)
scout mcp
```

### Built-in Tools (17)

| Category | Tools | Description |
|----------|-------|-------------|
| **Navigation** | `navigate`, `back`, `forward`, `wait`, `open` | Page navigation and browser control |
| **Interaction** | `click`, `type`, `eval` | Element interaction and JS execution |
| **Content** | `extract`, `screenshot`, `snapshot`, `pdf` | Content extraction, screenshots, PDF export |
| **Session** | `session_list`, `session_reset` | Session management |
| **WebSocket** | `ws_listen`, `ws_send`, `ws_connections` | Monitor, send, and list WebSocket traffic |

Additional tools (markdown, table, meta, forms, network, etc.) are available via the 12 built-in plugins.

### Resources

| URI | Description |
|-----|-------------|
| `scout://page/markdown` | Current page as markdown |
| `scout://page/url` | Current page URL |
| `scout://page/title` | Current page title |

## Claude Code Plugin

Scout works as a Claude Code plugin for AI-assisted browser automation during development.

```bash
# Run Claude Code with Scout plugin (from project root)
claude --plugin-dir .
```

### Skills

| Skill | Description |
|-------|-------------|
| `/scout:scrape` | Scrape a URL and extract structured data |
| `/scout:screenshot` | Capture a screenshot of a URL |
| `/scout:test-site` | Run health check on a site (broken links, errors) |
| `/scout:gather` | One-shot page intelligence (DOM, HAR, links, meta) |
| `/scout:crawl` | Crawl a site with depth/page limits |
| `/scout:monitor` | Visual regression monitoring |

### Agents

| Agent | Description |
|-------|-------------|
| `web-scraper` | Autonomous web scraping with extraction strategies |
| `site-tester` | Automated site health and quality testing |
| `site-mapper` | Site structure discovery and URL mapping |
| `session-capture` | Capture browser session traffic for replay |
| `flow-porter` | Port captured flows into deterministic replay specs |

## Plugin System

Scout uses subprocess-based plugins communicating via JSON-RPC 2.0. Twelve built-in plugins provide extended functionality.

```bash
# List installed plugins
scout plugin list

# Search the marketplace
scout plugin search "content"

# Install from GitHub
scout plugin install github:owner/plugin-name

# Install from local path or URL
scout plugin install ./my-plugin
scout plugin install https://example.com/plugin.tar.gz
```

### Built-in Plugins

`diag`, `reports`, `content`, `search`, `network`, `forms`, `crawl`, `guide`, `comm`, `email-docs`, `content-social`, `enterprise`

### Building Plugins

Use the Go SDK in `pkg/scout/plugin/sdk/`:

```go
srv := sdk.NewServer("my-plugin", "1.0.0")
sdk.RegisterMode(srv, "my_mode", myModeHandler)
sdk.RegisterTool(srv, "my_tool", myToolHandler)
srv.Run()
```

Plugins declare capabilities (`scraper_mode`, `extractor`, `mcp_tool`) in their `plugin.json` manifest.

## Secrets vault

Store secrets (API keys, cookies, auth headers) encrypted at rest and inject them into a browser without ever leaking plaintext to child processes.

```bash
scout vault init                                   # create <scouthome>/profiles/vault.bin (Argon2id + AES-256-GCM)
scout vault set --name openai api_key=sk-live-xyz  # prints an opaque profile ID
scout vault set --from-profile ./session.scoutprofile --name web   # import browser cookies/storage/headers
scout vault list                                   # metadata only — never secret values
scout vault use <id> --url https://example.com     # inject cookies/headers into a page via CDP
scout vault rotate                                 # re-key under a new passphrase (atomic)
scout vault rm <id>
```

Secrets are held in swap-locked, explicitly-zeroed buffers and are never passed via environment variables. Set `SCOUT_VAULT_PASSPHRASE` for non-interactive use (a stderr warning notes it is visible to child processes; prefer the interactive prompt).

## Observability

OpenTelemetry tracing remains available (no-op by default) when `SCOUT_TRACE=1` or `OTEL_EXPORTER_OTLP_ENDPOINT` is set. All MCP tools are auto-instrumented.

## CLI Reference

The `scout` CLI provides 50+ subcommands. Run `scout cmdtree` for the full command tree or `scout aicontext` for AI-consumable documentation.

| Command | Description |
|---------|-------------|
| `scout session list/list-local/prune/clean/rm/reset` | File-based session management |
| `scout title/url/text/attr/eval/html <url>` | Single-shot page inspection |
| `scout screenshot/pdf <url>` | Visual capture |
| `scout markdown <url>` | HTML-to-Markdown conversion |
| `scout table/meta <url>` | Structured data extraction |
| `scout form detect/fill/submit` | Form interaction |
| `scout search <query>` | Multi-engine search |
| `scout crawl <url>` | BFS crawling |
| `scout map <url>` | URL discovery |
| `scout gather <url>` | One-shot page intelligence |
| `scout test-site <url>` | Site health check |
| `scout repl [url]` | Interactive browser shell |
| `scout batch --urls=u1,u2` | Batch scraping |
| `scout har start/stop/export` | Network recording |
| `scout hijack watch <url>` | Session hijack monitoring |
| `scout extract-ai --url=<url>` | AI-powered extraction |
| `scout recipe run/validate` | Declarative recipes |
| `scout swagger <url>` | OpenAPI extraction |
| `scout sitemap extract <url>` | Full-site DOM + Markdown extraction |
| `scout auth login/capture/status` | Auth framework |
| `scout mcp` | MCP server (stdio) |
| `scout plugin list/install/search` | Plugin management |
| `scout browser list` | Browser management |
| `scout report list/show/delete` | Report management |
| `scout version` | Version info |

## Development

Requires [Task](https://taskfile.dev) for build automation.

```bash
task build         # Build scout CLI binary to bin/
task test          # Run all tests with -race and coverage
task test:unit     # Run tests with -short flag
task check         # Full quality check: fmt, vet, lint, test
task lint          # Run golangci-lint
task lint:fix      # Run golangci-lint with --fix
task fmt           # Format code (go fmt + goimports)
```

## Dependencies

**Core library:**

| Package | Purpose |
|---------|---------|
| internal/engine/lib (internalized rod) | Headless browser automation via Chrome DevTools Protocol |
| internal/engine/stealth (internalized) | Anti-bot-detection (17 evasions, forked from go-rod/stealth) |
| [ysmood/gson](https://github.com/ysmood/gson) | JSON number handling for JS evaluation results |
| [golang.org/x/time](https://pkg.go.dev/golang.org/x/time) | Token bucket rate limiter |
| [golang.org/x/crypto](https://pkg.go.dev/golang.org/x/crypto) | Argon2id key derivation for session encryption |
| [ollama/ollama](https://github.com/ollama/ollama) | Ollama Go client for local LLM provider |
| [go-sdk/mcp](https://github.com/modelcontextprotocol/go-sdk) | Model Context Protocol server for LLM integration |

**CLI** (`cmd/` only):

| Package | Purpose |
|---------|---------|
| [google/uuid](https://github.com/google/uuid) | Session ID generation |
| [spf13/cobra](https://github.com/spf13/cobra) | CLI framework |
| [google/gops](https://github.com/google/gops) | Process discovery and orphan detection |
| [go.opentelemetry.io/otel](https://opentelemetry.io) | Distributed tracing |

For full API reference, see [docs/API.md](docs/API.md).

## License

See [LICENSE](LICENSE) file.
