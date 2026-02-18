# Scout Examples

Runnable examples demonstrating the [Scout](https://github.com/inovacc/scout) library API.

## Prerequisites

- Go 1.25+
- Chromium-based browser installed (Chrome, Chromium, or Edge)

## Running

```bash
cd examples/simple/basic-navigation
go run .
```

## Simple Examples

| Example                                      | Description                                                   | Key APIs                                                             |
|----------------------------------------------|---------------------------------------------------------------|----------------------------------------------------------------------|
| [basic-navigation](simple/basic-navigation/) | Navigate pages, read title/URL/HTML, go back/forward          | `New`, `NewPage`, `Title`, `URL`, `HTML`, `Navigate`, `NavigateBack` |
| [screenshot](simple/screenshot/)             | Capture viewport, full-page, and JPEG screenshots             | `Screenshot`, `FullScreenshot`, `ScreenshotJPEG`                     |
| [extract-struct](simple/extract-struct/)     | Extract data into Go structs using `scout:"selector"` tags    | `Extract`, struct tags                                               |
| [extract-table](simple/extract-table/)       | Parse HTML tables into rows or maps                           | `ExtractTable`, `ExtractTableMap`                                    |
| [extract-meta](simple/extract-meta/)         | Extract page metadata (OG, Twitter, JSON-LD)                  | `ExtractMeta`                                                        |
| [javascript-eval](simple/javascript-eval/)   | Evaluate JS and get typed results (string, int, bool, object) | `Eval`, `EvalResult.String`, `.Int`, `.Bool`, `.Decode`              |
| [form-fill](simple/form-fill/)               | Detect forms, fill fields, and submit                         | `DetectForm`, `Fill`, `FillStruct`, `Submit`, `CSRFToken`            |
| [cookies-headers](simple/cookies-headers/)   | Set/get cookies and custom request headers                    | `SetCookies`, `GetCookies`, `ClearCookies`, `SetHeaders`             |

## Advanced Examples

| Example                                                | Description                                             | Key APIs                                     |
|--------------------------------------------------------|---------------------------------------------------------|----------------------------------------------|
| [search-engines](advanced/search-engines/)             | Search Google/Bing/DuckDuckGo and parse SERP results    | `Search`, `SearchAll`, `WithSearchEngine`    |
| [pagination](advanced/pagination/)                     | Paginate through pages using URL patterns or click-next | `PaginateByURL`, `PaginateByClick`, generics |
| [crawl-site](advanced/crawl-site/)                     | BFS crawl with depth/page limits and domain filtering   | `Crawl`, `CrawlHandler`, `WithCrawlMaxDepth` |
| [sitemap-parser](advanced/sitemap-parser/)             | Parse sitemap.xml to discover site URLs                 | `ParseSitemap`, `SitemapURL`                 |
| [rate-limited-scraper](advanced/rate-limited-scraper/) | Rate limiting with retry and exponential backoff        | `NewRateLimiter`, `Do`, `NavigateWithRetry`  |
| [form-wizard](advanced/form-wizard/)                   | Automate multi-step form workflows                      | `NewFormWizard`, `WizardStep`, `Run`         |
| [request-intercept](advanced/request-intercept/)       | Intercept requests, modify responses, block URLs        | `Hijack`, `HijackRouter`, `SetBlockedURLs`   |
| [stealth-scraper](advanced/stealth-scraper/)           | Stealth mode with custom UA to avoid detection          | `WithStealth`, `WithUserAgent`, `WithProxy`  |
| [pdf-generator](advanced/pdf-generator/)               | Generate PDFs with custom layout options                | `PDF`, `PDFWithOptions`, `PDFOptions`        |

## Notes

- **Search engines**: Google may show 0 results due to CAPTCHA/bot detection. DuckDuckGo and Bing tend to be more reliable for automated searches.
- **PDF generation**: Requires a Chrome/Chromium version that supports `Page.printToPDF` via CDP. Some headless configurations may not support this.
- **External sites**: Examples target public sites (quotes.toscrape.com, books.toscrape.com, example.com, ogp.me). These may change over time.
