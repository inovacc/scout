# Scout Examples

A gallery of runnable examples demonstrating the [Scout](https://github.com/inovacc/scout) library API. Each example is self-contained and can be run directly.

## Quick Start

```bash
# Simple examples
go run ./examples/simple/<name>/

# Advanced examples
go run ./examples/advanced/<name>/
```

All examples require a Chromium-based browser. Scout auto-downloads Chrome for Testing if none is cached in `~/.scout/browsers/`.

## Simple Examples

| Example | Description | Key APIs |
|---------|-------------|----------|
| [basic-navigation](simple/basic-navigation/) | Navigate pages, read title/URL/HTML, go back/forward | `New`, `NewPage`, `Title`, `URL`, `HTML`, `Navigate`, `NavigateBack` |
| [screenshot](simple/screenshot/) | Capture viewport, full-page, and JPEG screenshots | `Screenshot`, `FullScreenshot`, `ScreenshotJPEG`, `WithWindowSize` |
| [scroll-capture](simple/scroll-capture/) | Scroll through lazy-loaded content, then capture full page | `Eval` (scrollBy), `FullScreenshot`, `WithStealth`, `WithBrowser` |
| [extract-struct](simple/extract-struct/) | Extract data into Go structs using `scout:"selector"` tags | `Extract`, struct tags, nested structs, slice fields |
| [extract-table](simple/extract-table/) | Parse HTML tables into rows or maps, plus simple text extraction | `ExtractTable`, `ExtractTableMap`, `ExtractText` |
| [extract-meta](simple/extract-meta/) | Extract page metadata (OG, Twitter, JSON-LD) | `ExtractMeta`, Open Graph, Twitter cards, JSON-LD |
| [javascript-eval](simple/javascript-eval/) | Evaluate JS and get typed results (string, int, bool, struct) | `Eval`, `.String`, `.Int`, `.Bool`, `.Decode` |
| [form-fill](simple/form-fill/) | Detect forms, fill fields via struct tags, handle CSRF, submit | `DetectForm`, `FillStruct`, `CSRFToken`, `Submit` |
| [cookies-headers](simple/cookies-headers/) | Set/get cookies and custom request headers with cleanup | `SetCookies`, `GetCookies`, `ClearCookies`, `SetHeaders` |

## Advanced Examples

| Example | Description | Key APIs |
|---------|-------------|----------|
| [ai-analysis](advanced/ai-analysis/) | Extract page content via bridge, send to local LLM for analysis | `Bridge`, `DOMMarkdown`, `NewOpenAIProvider`, `Complete` |
| [bridge-dom](advanced/bridge-dom/) | Crawl a site extracting DOM JSON and Markdown for every page | `SitemapExtract`, `WithSitemapMaxDepth`, `WithSitemapDOMDepth` |
| [bridge-record](advanced/bridge-record/) | Record browser interactions and export as a runbook JSON | `NewBridgeRecorder`, `Start`, `Stop`, `ToRunbook` |
| [crawl-site](advanced/crawl-site/) | BFS crawl with depth/page limits and domain filtering | `Crawl`, `CrawlHandler`, `WithCrawlMaxDepth`, `WithCrawlAllowedDomains` |
| [form-wizard](advanced/form-wizard/) | Automate multi-step form workflows | `NewFormWizard`, `WizardStep`, `Run` |
| [har-recorder](advanced/har-recorder/) | Record all network traffic and export as a HAR file | `NewNetworkRecorder`, `WithCaptureBody`, `ExportHAR` |
| [pagination](advanced/pagination/) | Paginate via URL patterns or "next" button clicks with dedup | `PaginateByURL`, `PaginateByClick`, generics, `WithPaginateDedup` |
| [pdf-generator](advanced/pdf-generator/) | Generate PDFs with custom layout, headers, footers, and margins | `PDF`, `PDFWithOptions`, `PDFOptions` |
| [rate-limited-scraper](advanced/rate-limited-scraper/) | Rate limiting with retry and exponential backoff | `NewRateLimiter`, `NavigateWithRetry`, `WithBackoff` |
| [request-intercept](advanced/request-intercept/) | Intercept requests, modify responses, block URL patterns | `Hijack`, `HijackContext`, `SetBlockedURLs` |
| [search-engines](advanced/search-engines/) | Search Google/Bing/DuckDuckGo and parse SERP results | `Search`, `SearchAll`, `WithSearchEngine`, `WithSearchMaxPages` |
| [sitemap-parser](advanced/sitemap-parser/) | Parse sitemap.xml to discover site URLs with metadata | `ParseSitemap`, `SitemapURL` |
| [stealth-scraper](advanced/stealth-scraper/) | Stealth mode with custom user agent and proxy support | `WithStealth`, `WithUserAgent`, `WithProxy` |

## Cookbook

Quick recipes for common tasks. Each snippet assumes you already have a `browser` created with `scout.New()` and deferred `browser.Close()`.

---

### Screenshot any URL

```go
b, _ := scout.New()
defer func() { _ = b.Close() }()

page, _ := b.NewPage("https://example.com")
_ = page.WaitLoad()

data, _ := page.Screenshot()
_ = os.WriteFile("screenshot.png", data, 0o644)
```

---

### Extract structured data with tags

Define a struct with `scout:"selector"` tags and call `Extract`:

```go
type Product struct {
    Name  string `scout:"h1.product-name"`
    Price string `scout:"span.price"`
    Desc  string `scout:"div.description"`
}

page, _ := browser.NewPage("https://example.com/product/1")
_ = page.WaitLoad()

var product Product
_ = page.Extract(&product)
fmt.Printf("%s: %s\n", product.Name, product.Price)
```

---

### Fill and submit a form

Use `form:"field_name"` tags to map struct fields to form inputs:

```go
type Login struct {
    Email    string `form:"email"`
    Password string `form:"password"`
}

page, _ := browser.NewPage("https://example.com/login")
form, _ := page.DetectForm("form#login")

_ = form.FillStruct(&Login{
    Email:    "user@example.com",
    Password: "secret",
})
_ = form.Submit()
```

---

### Crawl a site with depth control

```go
results, _ := browser.Crawl("https://example.com",
    func(page *scout.Page, result *scout.CrawlResult) error {
        fmt.Printf("%s - %s\n", result.URL, result.Title)
        return nil
    },
    scout.WithCrawlMaxDepth(3),
    scout.WithCrawlMaxPages(50),
    scout.WithCrawlDelay(500*time.Millisecond),
)
fmt.Printf("Visited %d pages\n", len(results))
```

---

### Record network traffic as HAR

```go
page, _ := browser.NewPage("about:blank")

rec := scout.NewNetworkRecorder(page, scout.WithCaptureBody(true))
defer rec.Stop()

_ = page.Navigate("https://example.com")
_ = page.WaitLoad()

data, count, _ := rec.ExportHAR()
_ = os.WriteFile("trace.har", data, 0o644)
fmt.Printf("Captured %d requests\n", count)
```

---

### Intercept and block requests

```go
page, _ := browser.NewPage("about:blank")

router, _ := page.Hijack("*", func(ctx *scout.HijackContext) {
    fmt.Printf("[%s] %s\n", ctx.Request().Method(), ctx.Request().URL())
    ctx.ContinueRequest()
})
go router.Run()
defer func() { _ = router.Stop() }()

// Block analytics and ads
_ = page.SetBlockedURLs("*google-analytics.com*", "*doubleclick.net*")
_ = page.Navigate("https://example.com")
```

---

### Stealth mode with custom identity

```go
browser, _ := scout.New(
    scout.WithStealth(),
    scout.WithUserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) ..."),
    scout.WithWindowSize(1920, 1080),
    // scout.WithProxy("socks5://127.0.0.1:1080"),
)
defer func() { _ = browser.Close() }()

page, _ := browser.NewPage("https://bot-detection-site.com")
// navigator.webdriver is false, fingerprint checks are patched
```

---

### Paginate and collect items

Use generics to paginate and extract typed items with automatic deduplication:

```go
type Item struct {
    Title string `scout:"h2.title"`
    Link  string `scout:"a@href"`
}

items, _ := scout.PaginateByURL[Item](browser,
    func(page int) string {
        return fmt.Sprintf("https://example.com/items?page=%d", page)
    },
    scout.WithPaginateMaxPages(5),
    scout.WithPaginateDedup("Title"),
)
fmt.Printf("Collected %d items\n", len(items))
```

---

### Generate a PDF from any page

```go
page, _ := browser.NewPage("https://example.com/report")
_ = page.WaitLoad()

pdf, _ := page.PDFWithOptions(scout.PDFOptions{
    Landscape:       true,
    PrintBackground: true,
    MarginTop:       0.5,
    MarginBottom:    0.5,
    MarginLeft:      0.5,
    MarginRight:     0.5,
})
_ = os.WriteFile("report.pdf", pdf, 0o644)
```

## Notes

- **Search engines**: Google may return 0 results due to CAPTCHA/bot detection. DuckDuckGo and Bing tend to be more reliable for automated searches.
- **PDF generation**: Requires a Chrome/Chromium version that supports `Page.printToPDF` via CDP. Some headless configurations may not support this.
- **AI analysis**: Requires a local LLM server (LM Studio, Ollama, etc.) running an OpenAI-compatible API.
- **External sites**: Examples target public sites (quotes.toscrape.com, books.toscrape.com, example.com, ogp.me). These may change over time.
- **Browser auto-download**: If no browser is cached, Scout downloads Chrome for Testing automatically on first run.

## Prerequisites

- Go 1.23+
- Chromium-based browser (auto-downloaded if not cached)
- For `ai-analysis`: a local LLM server with OpenAI-compatible API
- For `stealth-scraper` with proxy: a SOCKS5 or HTTP proxy server
