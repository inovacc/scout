# Scout

A Go browser automation library and CLI for headless browser control, web scraping, and AI-powered extraction. Built on an internalized rod fork with a public facade, gRPC service layer, MCP server, plugin system, and unified Cobra CLI.

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
- **Swarm Mode** - Distributed crawling with coordinator/worker architecture and domain-partitioned queues
- **HAR Network Recording** - Capture HTTP traffic via CDP events and export as HAR 1.2 format
- **LLM-Powered Extraction** - 6 built-in providers (Ollama, OpenAI, Anthropic, OpenRouter, DeepSeek, Gemini) with review pipeline
- **Research Presets** - Shallow, Medium, and Deep research modes with caching and incremental research
- **Scraper Framework** - 20 pluggable modes with AES-256-GCM encrypted session persistence
- **MCP Server** - 18 built-in tools + plugin-contributed tools for LLM browser control via stdio or SSE
- **Plugin System** - Subprocess JSON-RPC plugins with Go SDK, marketplace, and 12 built-in plugins
- **gRPC Remote Control** - Multi-session browser control with 25+ RPCs and event streaming
- **Electron Support** - `WithElectronApp(path)` with auto-download runtime
- **REPL Mode** - Interactive browser shell with 20 commands, no daemon required
- **Health Check** - Site-wide broken link, console error, and network failure detection
- **Visual Monitoring** - Pixel-level visual regression testing with baseline management
- **Reports** - AI-consumable markdown reports for health checks, gather, and crawl results
- **Chrome Extensions** - Load unpacked, download from Chrome Web Store, embedded bridge extension
- **Cloud Upload** - OAuth2 upload to Google Drive and OneDrive

## Installation

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

```bash
# Start a browser session
scout session create --url=https://example.com

# Inspect the page
scout title
scout url
scout text "h1"

# Take a screenshot
scout screenshot --output=page.png

# Navigate
scout navigate https://example.org
scout back
scout forward

# Interact with elements
scout click "button#submit"
scout type "input[name=q]" "search query"
scout key Enter

# Extract data
scout eval "document.title"
scout html --selector="div.content"
scout table --url=https://example.com --selector="table"
scout meta --url=https://example.com

# Search engines (standalone)
scout search "golang web scraping" --engine=google

# Crawl a site (standalone)
scout crawl https://example.com --max-depth=2

# REPL mode (no daemon)
scout repl https://example.com

# Clean up
scout session destroy --all
```

## Examples

The [`examples/`](examples/) directory contains 18 runnable programs organized by complexity:

**Simple** -- basic-navigation, screenshot, extract-struct, extract-table, extract-meta, javascript-eval, form-fill, cookies-headers

**Advanced** -- search-engines, pagination, crawl-site, sitemap-parser, rate-limited-scraper, form-wizard, request-intercept, stealth-scraper, pdf-generator, har-recorder

```bash
cd examples/simple/basic-navigation && go run .
```

See [`examples/README.md`](examples/README.md) for the full index with descriptions and key APIs.

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
| `WithRemoteCDP(endpoint)`         | Connect to existing Chrome DevTools endpoint                 | none            |
| `WithElectronApp(path)`           | Launch an Electron application                               | none            |

## MCP Server (LLM Integration)

Scout includes a [Model Context Protocol](https://modelcontextprotocol.io/) server exposing 18 built-in browser automation tools for LLMs like Claude, plus additional tools contributed by plugins.

```bash
# Install for Claude Code (local project)
scout mcp --install

# Install globally
scout mcp --install --global

# Start manually (stdio)
scout mcp

# Start with HTTP+SSE transport
scout mcp --sse --addr=localhost:8080
```

### Built-in Tools (18)

| Category | Tools | Description |
|----------|-------|-------------|
| **Navigation** | `navigate`, `back`, `forward`, `wait`, `open` | Page navigation and browser control |
| **Interaction** | `click`, `type`, `eval` | Element interaction and JS execution |
| **Content** | `extract`, `screenshot`, `snapshot`, `pdf` | Content extraction, screenshots, PDF export |
| **Session** | `session_list`, `session_reset` | Session management |
| **Crawling** | `swarm_crawl` | Distributed BFS crawling |
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
| `browser-automation` | General-purpose browser automation workflows |

## Agent HTTP API

Scout provides an HTTP API for AI agent integration via `pkg/scout/agent/`.

```bash
# Start the agent HTTP server
scout agent serve
```

### Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/tools` | List available tools (OpenAI/Anthropic schema) |
| `POST` | `/tools/{name}` | Execute a tool by name |
| `GET` | `/health` | Health check |

```bash
# Example: navigate to a URL
curl -X POST http://localhost:8080/tools/navigate \
  -H "Content-Type: application/json" \
  -d '{"url": "https://example.com"}'
```

The agent framework also provides `OpenAITools()` and `AnthropicTools()` for embedding Scout tools directly into AI agent pipelines.

## Mobile Automation

Scout supports mobile browser automation via ADB (Android Debug Bridge).

```bash
# List connected devices
scout mobile devices

# Connect to a device
scout mobile connect <device-id>

# Touch gestures
scout mobile tap 500 300
scout mobile swipe 500 800 500 200
scout mobile scroll down
```

Mobile sessions use Chrome on Android via `adb forward` for CDP connections.

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

## Cloud Deployment

Scout can be deployed to Kubernetes using the included Helm chart.

```bash
# Deploy with Helm
helm install scout deploy/helm/scout/

# Or use the CLI
scout cloud deploy
scout cloud status
scout cloud scale --replicas=3
```

## Monitoring

Scout exposes runtime metrics for observability.

```bash
# Prometheus metrics endpoint
curl http://localhost:9551/metrics

# JSON metrics
curl http://localhost:9551/metrics/json
```

OpenTelemetry tracing is available when `SCOUT_TRACE=1` or `OTEL_EXPORTER_OTLP_ENDPOINT` is set. All MCP tools are auto-instrumented.

## CLI Reference

The `scout` CLI provides 50+ subcommands. Run `scout cmdtree` for the full command tree or `scout aicontext` for AI-consumable documentation.

| Command | Description |
|---------|-------------|
| `scout session create/destroy/list/use/reset` | Session lifecycle management |
| `scout navigate/back/forward/reload` | Page navigation |
| `scout click/type/key/select/hover` | Element interaction |
| `scout title/url/text/attr/eval/html` | Page inspection |
| `scout screenshot/pdf` | Visual capture |
| `scout markdown --url=<url>` | HTML-to-Markdown conversion |
| `scout table/meta` | Structured data extraction |
| `scout form detect/fill/submit` | Form interaction |
| `scout search <query>` | Multi-engine search |
| `scout crawl <url>` | BFS crawling |
| `scout swarm start <url>` | Distributed crawling |
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
| `scout mcp` | MCP server |
| `scout agent serve` | Agent HTTP API server |
| `scout mobile devices/connect` | Mobile automation |
| `scout plugin list/install/search` | Plugin management |
| `scout browser list` | Browser management |
| `scout cloud deploy/status/scale` | Cloud deployment |
| `scout report list/show/delete` | Report management |
| `scout upload auth/file/status` | Cloud upload (Drive, OneDrive) |
| `scout connect --cdp ws://...` | Remote CDP connection |
| `scout server` | Run gRPC server directly |
| `scout version` | Version info |

## gRPC Service

Multi-session browser control via gRPC on port 9551 with mTLS authentication and device pairing.

```bash
# Start gRPC server
scout server

# Or via Task
task grpc:server
```

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
task proto         # Generate protobuf code
```

## Dependencies

**Core library** (no gRPC -- library-only consumers do not pull gRPC deps):

| Package | Purpose |
|---------|---------|
| internal/engine/lib (internalized rod) | Headless browser automation via Chrome DevTools Protocol |
| internal/engine/stealth (internalized) | Anti-bot-detection (17 evasions, forked from go-rod/stealth) |
| [ysmood/gson](https://github.com/ysmood/gson) | JSON number handling for JS evaluation results |
| [golang.org/x/time](https://pkg.go.dev/golang.org/x/time) | Token bucket rate limiter |
| [golang.org/x/crypto](https://pkg.go.dev/golang.org/x/crypto) | Argon2id key derivation for session encryption |
| [ollama/ollama](https://github.com/ollama/ollama) | Ollama Go client for local LLM provider |
| [go-sdk/mcp](https://github.com/modelcontextprotocol/go-sdk) | Model Context Protocol server for LLM integration |

**gRPC layer and CLI** (`grpc/` and `cmd/` only):

| Package | Purpose |
|---------|---------|
| [google.golang.org/grpc](https://pkg.go.dev/google.golang.org/grpc) | gRPC framework |
| [google.golang.org/protobuf](https://pkg.go.dev/google.golang.org/protobuf) | Protocol Buffers runtime |
| [google/uuid](https://github.com/google/uuid) | Session ID generation |
| [spf13/cobra](https://github.com/spf13/cobra) | CLI framework |
| [grandcat/zeroconf](https://github.com/grandcat/zeroconf) | mDNS service discovery for device pairing |
| [google/gops](https://github.com/google/gops) | Process discovery and orphan detection |
| [go.opentelemetry.io/otel](https://opentelemetry.io) | Distributed tracing |

For full API reference, see [docs/API.md](docs/API.md).

## License

See [LICENSE](LICENSE) file.
