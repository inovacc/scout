// Package firecrawl provides a Go client for the Firecrawl v2 REST API.
//
// Firecrawl converts web pages into LLM-ready markdown, supports crawling,
// search, URL mapping, batch scraping, and AI-powered extraction — all
// without running a browser.
//
// Basic usage:
//
//	client, err := firecrawl.New("fc-xxx")
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	doc, err := client.Scrape(ctx, "https://example.com",
//		firecrawl.WithFormats(firecrawl.FormatMarkdown),
//	)
package firecrawl
