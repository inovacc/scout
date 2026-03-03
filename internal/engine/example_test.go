package engine_test

import (
	"fmt"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/rod/lib/input"
)

func ExampleNew() {
	b, err := scout.New(scout.WithHeadless(true))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	defer func() { _ = b.Close() }()

	page, err := b.NewPage("https://example.com")
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	title, err := page.Title()
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Println(title)
	// Output:
	// Example Domain
}

func ExampleBrowser_NewPage() { //nolint:testableexamples // requires browser
	b, err := scout.New(scout.WithHeadless(true), scout.WithStealth())
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	defer func() { _ = b.Close() }()

	// NewPage creates a tab and navigates to the URL.
	page, err := b.NewPage("https://example.com")
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	defer func() { _ = page.Close() }()

	url, _ := page.URL()
	fmt.Println(url)
}

func ExamplePage_Extract() { //nolint:testableexamples // requires browser
	type Product struct {
		Name  string `scout:"h2.title"`
		Price string `scout:"span.price"`
		Image string `scout:"img.hero@src"`
	}

	b, err := scout.New(scout.WithHeadless(true))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	defer func() { _ = b.Close() }()

	page, err := b.NewPage("https://shop.example.com/product/1")
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	_ = page.WaitLoad()

	var p Product
	if err := page.Extract(&p); err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Printf("Name: %s, Price: %s\n", p.Name, p.Price)
}

func ExamplePage_Eval() { //nolint:testableexamples // requires browser
	b, err := scout.New(scout.WithHeadless(true))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	defer func() { _ = b.Close() }()

	page, err := b.NewPage("https://example.com")
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	result, err := page.Eval("() => document.querySelectorAll('a').length")
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	count := result.Int()
	fmt.Printf("Links: %d\n", count)
}

func ExamplePage_Markdown() { //nolint:testableexamples // requires browser
	b, err := scout.New(scout.WithHeadless(true))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	defer func() { _ = b.Close() }()

	page, err := b.NewPage("https://example.com")
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	_ = page.WaitLoad()

	// Full page as markdown.
	md, err := page.Markdown()
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Println(md)
}

func ExamplePage_MarkdownContent() { //nolint:testableexamples // requires browser
	b, err := scout.New(scout.WithHeadless(true))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	defer func() { _ = b.Close() }()

	page, err := b.NewPage("https://example.com")
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	_ = page.WaitLoad()

	// Main content only — strips nav, footer, sidebar.
	md, err := page.MarkdownContent()
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Println(md)
}

func ExamplePage_Hijack() { //nolint:testableexamples // requires browser
	b, err := scout.New(scout.WithHeadless(true))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	defer func() { _ = b.Close() }()

	page, err := b.NewPage("")
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	// Block all image requests.
	router, err := page.Hijack("*.png", func(ctx *scout.HijackContext) {
		ctx.Response().Fail("BlockedByClient")
	})
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	go router.Run()

	defer func() { _ = router.Stop() }()

	_ = page.Navigate("https://example.com")
}

func ExampleBrowser_Crawl() { //nolint:testableexamples // requires browser
	b, err := scout.New(scout.WithHeadless(true))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	defer func() { _ = b.Close() }()

	results, err := b.Crawl("https://example.com", func(page *scout.Page, result *scout.CrawlResult) error {
		fmt.Printf("Crawled: %s (depth=%d)\n", result.URL, result.Depth)
		return nil
	}, scout.WithCrawlMaxDepth(2), scout.WithCrawlMaxPages(10))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Printf("Total pages: %d\n", len(results))
}

func ExampleBrowser_Map() { //nolint:testableexamples // requires browser
	b, err := scout.New(scout.WithHeadless(true))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	defer func() { _ = b.Close() }()

	urls, err := b.Map("https://example.com",
		scout.WithMapLimit(50),
		scout.WithMapMaxDepth(2),
	)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Printf("Discovered %d URLs\n", len(urls))
}

func ExampleNewRateLimiter() { //nolint:testableexamples // requires browser
	rl := scout.NewRateLimiter(
		scout.WithRateLimit(2),  // 2 requests/sec
		scout.WithMaxRetries(3), // retry up to 3 times
		scout.WithBackoff(500*time.Millisecond),
	)

	err := rl.Do(func() error {
		// Your scraping operation here.
		return nil
	})
	if err != nil {
		fmt.Println("error:", err)
		return
	}
}

func ExampleNewNetworkRecorder() { //nolint:testableexamples // requires browser
	b, err := scout.New(scout.WithHeadless(true))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	defer func() { _ = b.Close() }()

	page, err := b.NewPage("https://example.com")
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	rec := scout.NewNetworkRecorder(page,
		scout.WithCaptureBody(true),
		scout.WithCreatorName("my-tool", "1.0"),
	)
	defer rec.Stop()

	_ = page.Navigate("https://example.com")
	_ = page.WaitLoad()

	harJSON, count, err := rec.ExportHAR()
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Printf("Captured %d entries (%d bytes)\n", count, len(harJSON))
}

func ExamplePage_Element() { //nolint:testableexamples // requires browser
	b, err := scout.New(scout.WithHeadless(true))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	defer func() { _ = b.Close() }()

	page, err := b.NewPage("https://example.com")
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	_ = page.WaitLoad()

	// Find an element by CSS selector.
	el, err := page.Element("h1")
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	text, _ := el.Text()
	fmt.Println(text)
}

func ExampleElement_Click() { //nolint:testableexamples // requires browser
	b, err := scout.New(scout.WithHeadless(true))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	defer func() { _ = b.Close() }()

	page, err := b.NewPage("https://example.com")
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	_ = page.WaitLoad()

	el, err := page.Element("a")
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	// Click the element.
	if err := el.Click(); err != nil {
		fmt.Println("error:", err)
		return
	}

	_ = page.WaitLoad()
	title, _ := page.Title()
	fmt.Println(title)
}

func ExampleElement_Input() { //nolint:testableexamples // requires browser
	b, err := scout.New(scout.WithHeadless(true))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	defer func() { _ = b.Close() }()

	page, err := b.NewPage("https://example.com")
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	el, err := page.Element("input[type=text]")
	if err != nil {
		fmt.Println("not found")
		return
	}

	// Type text into the input field.
	if err := el.Input("hello world"); err != nil {
		fmt.Println("error:", err)
	}
}

func ExampleBrowser_Search() { //nolint:testableexamples // requires browser
	b, err := scout.New(scout.WithHeadless(true))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	defer func() { _ = b.Close() }()

	results, err := b.Search("golang tutorial",
		scout.WithSearchEngine(scout.Google),
		scout.WithSearchMaxPages(1),
	)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	for _, r := range results.Results {
		fmt.Printf("%d. %s\n   %s\n", r.Position, r.Title, r.URL)
	}
}

func ExamplePage_Screenshot() { //nolint:testableexamples // requires browser
	b, err := scout.New(scout.WithHeadless(true))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	defer func() { _ = b.Close() }()

	page, err := b.NewPage("https://example.com")
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	_ = page.WaitLoad()

	// Viewport screenshot.
	data, err := page.Screenshot()
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Printf("Screenshot: %d bytes\n", len(data))
}

func ExamplePage_WaitLoad() { //nolint:testableexamples // requires browser
	b, err := scout.New(scout.WithHeadless(true))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	defer func() { _ = b.Close() }()

	page, err := b.NewPage("https://example.com")
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	// Wait for the DOM load event before extracting content.
	if err := page.WaitLoad(); err != nil {
		fmt.Println("error:", err)
		return
	}

	title, _ := page.Title()
	fmt.Println(title)
}

func ExampleWithBlockPatterns() { //nolint:testableexamples // requires browser
	// Block ads and trackers for faster, cleaner scraping.
	b, err := scout.New(
		scout.WithHeadless(true),
		scout.WithBlockPatterns(scout.BlockAds...),
		scout.WithBlockPatterns(scout.BlockTrackers...),
		scout.WithBlockPatterns(scout.BlockFonts...),
	)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	defer func() { _ = b.Close() }()

	page, err := b.NewPage("https://example.com")
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	_ = page.WaitLoad()
	title, _ := page.Title()
	fmt.Println(title)
}

func ExamplePage_Block() { //nolint:testableexamples // requires browser
	b, err := scout.New(scout.WithHeadless(true))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	defer func() { _ = b.Close() }()

	page, err := b.NewPage("https://example.com")
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	// Block images on this specific page.
	_ = page.Block(scout.BlockImages...)

	_ = page.Navigate("https://example.com")
	_ = page.WaitLoad()
}

func ExampleWithRemoteCDP() { //nolint:testableexamples // requires remote browser
	// Connect to an existing Chrome instance or managed service.
	b, err := scout.New(
		scout.WithRemoteCDP("ws://127.0.0.1:9222"),
	)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	defer func() { _ = b.Close() }()

	page, err := b.NewPage("https://example.com")
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	_ = page.WaitLoad()
	title, _ := page.Title()
	fmt.Println(title)
}

func ExamplePage_KeyPress() { //nolint:testableexamples // requires browser
	b, err := scout.New(scout.WithHeadless(true))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	defer func() { _ = b.Close() }()

	page, err := b.NewPage("https://example.com")
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	// Press Enter at the page level.
	_ = page.KeyPress(input.Enter)

	// Type text at the page level (sends key-by-key).
	_ = page.KeyType('h', 'i')
}

func Example_convertHTMLToMarkdown() { //nolint:testableexamples // requires browser
	// convertHTMLToMarkdown is a pure function (unexported).
	// Use Page.Markdown() or Page.MarkdownContent() for the public API.
	// This example shows the options pattern:
	b, err := scout.New(scout.WithHeadless(true))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	defer func() { _ = b.Close() }()

	page, err := b.NewPage("https://example.com")
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	_ = page.WaitLoad()

	md, err := page.Markdown(
		scout.WithIncludeImages(false),
		scout.WithIncludeLinks(false),
	)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Println(md)
}

func ExampleForm_Fill() { //nolint:testableexamples // requires browser
	b, err := scout.New(scout.WithHeadless(true))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	defer func() { _ = b.Close() }()

	page, err := b.NewPage("https://example.com/login")
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	_ = page.WaitLoad()

	// Detect a form by CSS selector, then fill fields by name/id.
	form, err := page.DetectForm("form#login")
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	if err := form.Fill(map[string]string{
		"username": "alice",
		"password": "s3cret",
	}); err != nil {
		fmt.Println("error:", err)
		return
	}

	if err := form.Submit(); err != nil {
		fmt.Println("error:", err)
	}
}

func ExamplePaginateByClick() { //nolint:testableexamples // requires browser
	type Item struct {
		Title string `scout:"h3.item-title"`
	}

	b, err := scout.New(scout.WithHeadless(true))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	defer func() { _ = b.Close() }()

	page, err := b.NewPage("https://example.com/items")
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	_ = page.WaitLoad()

	// Paginate by clicking a "Next" button, extracting items from each page.
	items, err := scout.PaginateByClick[Item](page, "a.next-page",
		scout.WithPaginateMaxPages(5),
	)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Printf("Collected %d items\n", len(items))
}

func ExamplePage_SaveCookiesToFile() { //nolint:testableexamples // requires browser
	b, err := scout.New(scout.WithHeadless(true))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	defer func() { _ = b.Close() }()

	page, err := b.NewPage("https://example.com")
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	_ = page.WaitLoad()

	// Persist cookies to a file (excluding session cookies).
	if err := page.SaveCookiesToFile("cookies.json", false); err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Println("Cookies saved")
}

func ExampleBrowser_Close() { //nolint:testableexamples // requires browser
	b, err := scout.New(scout.WithHeadless(true))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	// Close is nil-safe and idempotent — safe to call multiple times.
	_ = b.Close()
	_ = b.Close() // no-op, no error

	// Also safe on a nil *Browser.
	var nilBrowser *scout.Browser

	_ = nilBrowser.Close()
}

func ExampleWithStealth() { //nolint:testableexamples // requires browser
	// WithStealth enables anti-bot-detection evasions:
	// automation flag removal, JS property masking, etc.
	b, err := scout.New(
		scout.WithHeadless(true),
		scout.WithStealth(),
	)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	defer func() { _ = b.Close() }()

	page, err := b.NewPage("https://bot.sannysoft.com")
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	_ = page.WaitLoad()
	title, _ := page.Title()
	fmt.Println(title)
}

func ExamplePage_WaitFrameworkReady() { //nolint:testableexamples // requires browser
	b, err := scout.New(scout.WithHeadless(true))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	defer func() { _ = b.Close() }()

	page, err := b.NewPage("https://example.com")
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	// WaitFrameworkReady detects the frontend framework (React, Vue, Angular, etc.)
	// and waits for its specific readiness signal. Falls back to WaitLoad + DOM stable.
	if err := page.WaitFrameworkReady(); err != nil {
		fmt.Println("error:", err)
		return
	}

	title, _ := page.Title()
	fmt.Println(title)
}

func ExamplePage_EvalOnNewDocument() { //nolint:testableexamples // requires browser
	b, err := scout.New(scout.WithHeadless(true))
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	defer func() { _ = b.Close() }()

	page, err := b.NewPage("")
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	// Inject JS that runs before any page script. Returns a cleanup function.
	remove, err := page.EvalOnNewDocument(`window.__injected = true`)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	_ = page.Navigate("https://example.com")
	_ = page.WaitLoad()

	// Clean up the injection when no longer needed.
	if err := remove(); err != nil {
		fmt.Println("error:", err)
	}
}

func ExampleWithWindowSize() { //nolint:testableexamples // requires browser
	// Set a custom viewport size (width x height in pixels).
	b, err := scout.New(
		scout.WithHeadless(true),
		scout.WithWindowSize(1440, 900),
	)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	defer func() { _ = b.Close() }()

	page, err := b.NewPage("https://example.com")
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	_ = page.WaitLoad()
	title, _ := page.Title()
	fmt.Println(title)
}
