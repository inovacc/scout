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
- **DOM Traversal** - Navigate parent/children/siblings, shadow roots, iframes
- **Struct-Tag Extraction** - Extract data into Go structs using `scout:"selector"` tags
- **Table & Meta Extraction** - Parse HTML tables and page metadata (OG, Twitter, JSON-LD)
- **Form Interaction** - Detect, fill, and submit forms; CSRF token extraction; multi-step wizards
- **Rate Limiting** - Token bucket rate limiter with retry and exponential backoff
- **Pagination** - Click-next, URL-pattern, infinite-scroll, and load-more with Go generics
- **Search Engine Integration** - Query Google, Bing, DuckDuckGo and parse SERP results
- **Web Crawling** - BFS crawling with depth/page limits, domain filtering, sitemap parsing

## Installation

```bash
go get github.com/inovacc/scout
```

Requires Go 1.25+ and a Chromium-based browser available on the system (auto-downloaded by rod if not present).

## Quick Start

```go
package main

import (
	"fmt"
	"log"

	"github.com/inovacc/scout"
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

| Option | Description | Default |
|--------|-------------|---------|
| `WithHeadless(bool)` | Run in headless mode | `true` |
| `WithStealth()` | Enable anti-bot-detection | disabled |
| `WithProxy(url)` | Set proxy server | none |
| `WithUserAgent(ua)` | Custom User-Agent | browser default |
| `WithWindowSize(w, h)` | Browser window size | 1920x1080 |
| `WithTimeout(d)` | Default operation timeout | 30s |
| `WithSlowMotion(d)` | Delay between actions (debugging) | none |
| `WithIgnoreCerts()` | Skip TLS verification | disabled |
| `WithExecPath(path)` | Path to browser binary | auto-detect |
| `WithUserDataDir(dir)` | Persistent session directory | temp |
| `WithIncognito()` | Incognito mode | disabled |
| `WithNoSandbox()` | Disable sandbox (containers) | disabled |

## Development

Requires [Task](https://taskfile.dev) for build automation.

```bash
task test          # Run all tests with -race and coverage
task test:unit     # Run tests with -short flag
task check         # Full quality check: fmt, vet, lint, test
task lint          # Run golangci-lint
task fmt           # Format code (go fmt + goimports)
```

## Dependencies

| Package | Purpose |
|---------|---------|
| [go-rod/rod](https://github.com/go-rod/rod) | Headless browser automation via Chrome DevTools Protocol |
| [go-rod/stealth](https://github.com/go-rod/stealth) | Anti-bot-detection page creation |
| [ysmood/gson](https://github.com/ysmood/gson) | JSON number handling for JS evaluation results |
| [golang.org/x/time](https://pkg.go.dev/golang.org/x/time) | Token bucket rate limiter |

## License

See [LICENSE](LICENSE) file.
