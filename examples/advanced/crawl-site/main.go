// Example: crawl-site
// Demonstrates BFS crawling with depth limits, page limits, and domain filtering.
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/inovacc/scout"
)

func main() {
	browser, err := scout.New()
	if err != nil {
		log.Fatal(err)
	}

	defer func() { _ = browser.Close() }()

	// Crawl quotes.toscrape.com with a handler for each page.
	results, err := browser.Crawl("https://quotes.toscrape.com",
		func(page *scout.Page, result *scout.CrawlResult) error {
			fmt.Printf("[depth=%d] %s - %s (%d links)\n",
				result.Depth, result.URL, result.Title, len(result.Links))

			return nil
		},
		scout.WithCrawlMaxDepth(2),
		scout.WithCrawlMaxPages(10),
		scout.WithCrawlAllowedDomains("quotes.toscrape.com"),
		scout.WithCrawlDelay(300*time.Millisecond),
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\nCrawl complete: %d pages visited\n", len(results))

	for _, r := range results {
		status := "OK"
		if r.Error != nil {
			status = r.Error.Error()
		}

		fmt.Printf("  %s [%s]\n", r.URL, status)
	}
}
