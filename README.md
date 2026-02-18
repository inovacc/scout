# Scout

A Go-idiomatic API for headless browser automation, web scraping, and search built on [go-rod](https://github.com/go-rod/rod).

## Features

- **Browser Management** - Launch, configure, and control headless Chromium with functional options (`WithHeadless`, `WithProxy`, `WithStealth`, `WithIncognito`, etc.)
- **Page Navigation** - Navigate, reload, go back/forward, wait for load/idle/DOM stability
- **Element Interaction** - Click, double-click, right-click, hover, tap, input text, select options, file uploads
- **Element Finding** - CSS selectors, XPath, text regex matching, JS evaluation, coordinate-based lookup, DevTools search
- **Screenshots & PDF** - Viewport, full-page, and scroll screenshots in PNG/JPEG; PDF generation with configurable options
- **JavaScript Evaluation** - Execute JS at page and element level with typed result access (`String()`, `Int()`, `Float()`, `Bool()`, `Decode()`)
- **Network Control** - Set headers, manage cookies, intercept/modify requests via hijacking, block URLs by pattern, HTTP basic auth
- **Stealth Mode** - Anti-bot-detection via `go-rod/stealth` to avoid fingerprinting
- **Device Emulation** - Viewport sizing, window bounds, device profile emulation
- **Window Control** - Minimize, maximize, fullscreen, restore; get/set window bounds and state
- **Session & Storage** - localStorage/sessionStorage access, save/load full session state (URL, cookies, storage)
- **DOM Traversal** - Navigate parent/children/siblings, shadow roots, iframes
- **Struct-Tag Extraction** - Extract data into Go structs using `scout:"selector"` tags
- **Table & Meta Extraction** - Parse HTML tables and page metadata (OG, Twitter, JSON-LD)
- **Form Interaction** - Detect, fill, and submit forms; CSRF token extraction; multi-step wizards
- **Rate Limiting** - Token bucket rate limiter with retry and exponential backoff
- **Pagination** - Click-next, URL-pattern, infinite-scroll, and load-more with Go generics
- **Search Engine Integration** - Query Google, Bing, DuckDuckGo and parse SERP results
- **Web Crawling** - BFS crawling with depth/page limits, domain filtering, sitemap parsing
- **HAR Network Recording** - Capture HTTP traffic via CDP events and export as HAR 1.2 format
- **Keyboard Input** - Page-level key press and type sequences via `input.Key` constants
- **gRPC Remote Control** - Multi-session browser control via gRPC with 25+ RPCs and event streaming
- **Scraper Framework** - Pluggable scraper framework with generic auth (browser capture, OAuth2, Electron CDP), encrypted session persistence (AES-256-GCM + Argon2id)
- **HTML-to-Markdown** - Convert page HTML to clean markdown with readability scoring for main content extraction (`page.Markdown()`, `page.MarkdownContent()`)
- **URL Map / Link Discovery** - Lightweight URL-only discovery combining sitemap.xml + on-page link harvesting with path/subdomain/search filters
- **Multi-Browser Support** - Chrome (default), Brave, and Microsoft Edge auto-detection via `WithBrowser()`
- **Chrome Extension Loading** - Load unpacked extensions via `WithExtension(paths...)`
- **Device Identity & Pairing** - Syncthing-style device IDs with Ed25519 keys, mTLS authentication, mDNS peer discovery
- **Platform-Aware Defaults** - Auto-applies `--no-sandbox` on Linux containers; platform-specific session defaults via build constraints
- **Batch Scraper** - Concurrent batch scraping of multiple URLs with page pool, error isolation, and progress reporting (`BatchScrape()`)
- **Multi-Engine Search** - Engine-specific search subcommands for Google, Bing, DuckDuckGo, Wikipedia, Google Scholar, Google News
- **Swagger/OpenAPI Extraction** - Auto-detect Swagger UI / ReDoc pages, fetch and parse OpenAPI 3.x / Swagger 2.0 specs, extract endpoints, schemas, and security definitions
- **Recipe System** - Declarative JSON recipes for extraction and automation playbooks (`scout recipe run/validate`)

## Installation

**Library:**

```bash
go get github.com/inovacc/scout/pkg/scout
```

**CLI:**

```bash
go install github.com/inovacc/scout/cmd/scout@latest
```

Requires Go 1.25+ and a Chromium-based browser available on the system (auto-downloaded by rod if not present).

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

# Clean up
scout session destroy --all
```

## Examples

The [`examples/`](examples/) directory contains 18 runnable programs organized by complexity:

**Simple** — basic-navigation, screenshot, extract-struct, extract-table, extract-meta, javascript-eval, form-fill, cookies-headers

**Advanced** — search-engines, pagination, crawl-site, sitemap-parser, rate-limited-scraper, form-wizard, request-intercept, stealth-scraper, pdf-generator, har-recorder

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

// Navigate and interact — all HTTP traffic is captured
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
| `WithStealth()`                   | Enable anti-bot-detection                                    | disabled        |
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

## CLI Reference

The `scout` CLI provides a unified interface to all library features. It communicates with a background gRPC daemon for session persistence across invocations.

| Command                                     | Description                                                                          |
|---------------------------------------------|--------------------------------------------------------------------------------------|
| `scout session create`                      | Create a browser session (`--headless`, `--stealth`, `--proxy`, `--url`, `--record`) |
| `scout session destroy [id]`                | Destroy a session (`--all` for all)                                                  |
| `scout session list`                        | List tracked sessions                                                                |
| `scout session use <id>`                    | Set active session                                                                   |
| `scout navigate <url>`                      | Navigate to URL                                                                      |
| `scout back` / `forward` / `reload`         | Browser history navigation                                                           |
| `scout click <sel>`                         | Click an element                                                                     |
| `scout type <sel> <text>`                   | Type into an element                                                                 |
| `scout key <key>`                           | Press a keyboard key                                                                 |
| `scout select <sel> <val>`                  | Select a dropdown option                                                             |
| `scout hover <sel>` / `focus` / `clear`     | Element interaction                                                                  |
| `scout title` / `url`                       | Get page title or URL                                                                |
| `scout text <sel>`                          | Get element text                                                                     |
| `scout attr <sel> <attr>`                   | Get element attribute                                                                |
| `scout eval <js>`                           | Execute JavaScript                                                                   |
| `scout html [--selector=sel]`               | Get page/element HTML                                                                |
| `scout screenshot`                          | Capture screenshot (`--full`, `--format`, `--quality`)                               |
| `scout pdf`                                 | Generate PDF                                                                         |
| `scout har start` / `stop` / `export`       | HAR network recording                                                                |
| `scout window get\|min\|max\|full\|restore` | Window control                                                                       |
| `scout storage get\|set\|list\|clear`       | Web storage (`--session-storage`)                                                    |
| `scout cookie get\|set\|clear`              | Cookie management                                                                    |
| `scout header set <key> <val>`              | Set extra headers                                                                    |
| `scout block <pattern>`                     | Block URL pattern                                                                    |
| `scout search <query>`                      | Search engines (`--engine`, `--max-pages`)                                           |
| `scout crawl <url>`                         | BFS crawl (`--max-depth`, `--max-pages`, `--delay`)                                  |
| `scout map <url>`                           | URL discovery (`--search`, `--include-subdomains`, `--limit`)                        |
| `scout markdown --url=<url>`                | Convert page to markdown (`--main-only`, `--no-images`)                              |
| `scout table` / `meta`                      | Extract tables/metadata (`--url`, `--selector`)                                      |
| `scout form detect\|fill\|submit`           | Form interaction                                                                     |
| `scout auth login\|capture\|status\|logout` | Generic auth framework                                                               |
| `scout device pair\|list\|trust`            | Device pairing and identity                                                          |
| `scout batch --urls=u1,u2`                  | Batch scrape URLs (`--concurrency`, `--urls-file`)                                   |
| `scout recipe run --file=f.json`            | Run extraction/automation recipe                                                     |
| `scout recipe validate --file=f.json`       | Validate recipe JSON schema                                                          |
| `scout swagger <url>`                       | Extract Swagger/OpenAPI spec (`--endpoints-only`, `--raw`, `--format`, `--output`)   |
| `scout server`                              | Run gRPC server directly                                                             |
| `scout client`                              | Interactive REPL client                                                              |
| `scout aicontext [--json]`                  | Generate AI context document for the CLI                                             |
| `scout cmdtree [--json]`                    | Visualize full command tree with flags                                               |
| `scout version`                             | Show version info                                                                    |

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

# gRPC
task proto         # Generate protobuf code
task grpc:server   # Run gRPC server (default :50051)
task grpc:client   # Run interactive CLI client
```

## Dependencies

**Core library** (no gRPC — library-only consumers do not pull gRPC deps):

| Package                                                       | Purpose                                                       |
|---------------------------------------------------------------|---------------------------------------------------------------|
| [go-rod/rod](https://github.com/go-rod/rod)                   | Headless browser automation via Chrome DevTools Protocol      |
| pkg/stealth (internalized)                                    | Anti-bot-detection page creation (forked from go-rod/stealth) |
| [ysmood/gson](https://github.com/ysmood/gson)                 | JSON number handling for JS evaluation results                |
| [golang.org/x/time](https://pkg.go.dev/golang.org/x/time)     | Token bucket rate limiter                                     |
| [golang.org/x/crypto](https://pkg.go.dev/golang.org/x/crypto) | Argon2id key derivation for session encryption                |
| [golang.org/x/term](https://pkg.go.dev/golang.org/x/term)     | Secure passphrase input (no-echo terminal)                    |

**gRPC layer and CLI** (`grpc/` and `cmd/` only):

| Package                                                                     | Purpose                                   |
|-----------------------------------------------------------------------------|-------------------------------------------|
| [google.golang.org/grpc](https://pkg.go.dev/google.golang.org/grpc)         | gRPC framework                            |
| [google.golang.org/protobuf](https://pkg.go.dev/google.golang.org/protobuf) | Protocol Buffers runtime                  |
| [google/uuid](https://github.com/google/uuid)                               | Session ID generation                     |
| [spf13/cobra](https://github.com/spf13/cobra)                               | CLI framework                             |
| [grandcat/zeroconf](https://github.com/grandcat/zeroconf)                   | mDNS service discovery for device pairing |

## License

See [LICENSE](LICENSE) file.
