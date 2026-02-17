package scout_test

import (
	"fmt"
	"log"
	"time"

	"github.com/inovacc/scout/pkg/scout"
)

func ExampleNew() {
	b, err := scout.New(scout.WithHeadless(true))
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = b.Close() }()

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

func ExampleBrowser_NewPage() {
	b, err := scout.New(scout.WithHeadless(true), scout.WithStealth())
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = b.Close() }()

	// NewPage creates a tab and navigates to the URL.
	page, err := b.NewPage("https://example.com")
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = page.Close() }()

	url, _ := page.URL()
	fmt.Println(url)
}

func ExamplePage_Extract() {
	type Product struct {
		Name  string `scout:"h2.title"`
		Price string `scout:"span.price"`
		Image string `scout:"img.hero@src"`
	}

	b, err := scout.New(scout.WithHeadless(true))
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = b.Close() }()

	page, err := b.NewPage("https://shop.example.com/product/1")
	if err != nil {
		log.Fatal(err)
	}
	_ = page.WaitLoad()

	var p Product
	if err := page.Extract(&p); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Name: %s, Price: %s\n", p.Name, p.Price)
}

func ExamplePage_Eval() {
	b, err := scout.New(scout.WithHeadless(true))
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = b.Close() }()

	page, err := b.NewPage("https://example.com")
	if err != nil {
		log.Fatal(err)
	}

	result, err := page.Eval("() => document.querySelectorAll('a').length")
	if err != nil {
		log.Fatal(err)
	}

	count := result.Int()
	fmt.Printf("Links: %d\n", count)
}

func ExamplePage_Markdown() {
	b, err := scout.New(scout.WithHeadless(true))
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = b.Close() }()

	page, err := b.NewPage("https://example.com")
	if err != nil {
		log.Fatal(err)
	}
	_ = page.WaitLoad()

	// Full page as markdown.
	md, err := page.Markdown()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(md)
}

func ExamplePage_MarkdownContent() {
	b, err := scout.New(scout.WithHeadless(true))
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = b.Close() }()

	page, err := b.NewPage("https://example.com")
	if err != nil {
		log.Fatal(err)
	}
	_ = page.WaitLoad()

	// Main content only — strips nav, footer, sidebar.
	md, err := page.MarkdownContent()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(md)
}

func ExamplePage_Hijack() {
	b, err := scout.New(scout.WithHeadless(true))
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = b.Close() }()

	page, err := b.NewPage("")
	if err != nil {
		log.Fatal(err)
	}

	// Block all image requests.
	router, err := page.Hijack("*.png", func(ctx *scout.HijackContext) {
		ctx.Response().Fail("BlockedByClient")
	})
	if err != nil {
		log.Fatal(err)
	}
	go router.Run()
	defer func() { _ = router.Stop() }()

	_ = page.Navigate("https://example.com")
}

func ExampleBrowser_Crawl() {
	b, err := scout.New(scout.WithHeadless(true))
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = b.Close() }()

	results, err := b.Crawl("https://example.com", func(page *scout.Page, result *scout.CrawlResult) error {
		fmt.Printf("Crawled: %s (depth=%d)\n", result.URL, result.Depth)
		return nil
	}, scout.WithCrawlMaxDepth(2), scout.WithCrawlMaxPages(10))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Total pages: %d\n", len(results))
}

func ExampleBrowser_Map() {
	b, err := scout.New(scout.WithHeadless(true))
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = b.Close() }()

	urls, err := b.Map("https://example.com",
		scout.WithMapLimit(50),
		scout.WithMapMaxDepth(2),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Discovered %d URLs\n", len(urls))
}

func ExampleNewRateLimiter() {
	rl := scout.NewRateLimiter(
		scout.WithRateLimit(2),     // 2 requests/sec
		scout.WithMaxRetries(3),    // retry up to 3 times
		scout.WithBackoff(500*time.Millisecond),
	)

	err := rl.Do(func() error {
		// Your scraping operation here.
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
}

func ExampleNewNetworkRecorder() {
	b, err := scout.New(scout.WithHeadless(true))
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = b.Close() }()

	page, err := b.NewPage("https://example.com")
	if err != nil {
		log.Fatal(err)
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
		log.Fatal(err)
	}
	fmt.Printf("Captured %d entries (%d bytes)\n", count, len(harJSON))
}

func Example_convertHTMLToMarkdown() {
	// convertHTMLToMarkdown is a pure function (unexported).
	// Use Page.Markdown() or Page.MarkdownContent() for the public API.
	// This example shows the options pattern:
	b, err := scout.New(scout.WithHeadless(true))
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = b.Close() }()

	page, err := b.NewPage("https://example.com")
	if err != nil {
		log.Fatal(err)
	}
	_ = page.WaitLoad()

	md, err := page.Markdown(
		scout.WithIncludeImages(false),
		scout.WithIncludeLinks(false),
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(md)
}
