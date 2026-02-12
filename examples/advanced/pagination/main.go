// Example: pagination
// Demonstrates PaginateByURL and PaginateByClick with generic struct extraction.
// Targets quotes.toscrape.com which has 10 pages of quotes.
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/inovacc/scout/pkg/scout"
)

// Quote is extracted from each div.quote container using scout tags.
// For extractAll (used by Paginate*), selectors must resolve within the
// parent element of the first field's match. On quotes.toscrape.com,
// span.text's parent is div.quote, which also contains small.author.
type Quote struct {
	Text   string `scout:"span.text"`
	Author string `scout:"small.author"`
}

func main() {
	browser, err := scout.New()
	if err != nil {
		log.Fatal(err)
	}

	defer func() { _ = browser.Close() }()

	// --- PaginateByURL: generate URLs for each page ---
	fmt.Println("=== PaginateByURL ===")

	quotes, err := scout.PaginateByURL[Quote](browser,
		func(page int) string {
			return fmt.Sprintf("https://quotes.toscrape.com/page/%d/", page)
		},
		scout.WithPaginateMaxPages(3),
		scout.WithPaginateDelay(500*time.Millisecond),
		scout.WithPaginateDedup("Text"),
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Collected %d quotes across 3 pages\n", len(quotes))

	for i, q := range quotes {
		if i >= 5 {
			break
		}

		fmt.Printf("  %d. %s — %s\n", i+1, truncate(q.Text, 60), q.Author)
	}

	// --- PaginateByClick: click "next" button to paginate ---
	fmt.Println("\n=== PaginateByClick ===")

	page, err := browser.NewPage("https://quotes.toscrape.com/page/1/")
	if err != nil {
		log.Fatal(err)
	}

	if err := page.WaitLoad(); err != nil {
		log.Fatal(err)
	}

	clickQuotes, err := scout.PaginateByClick[Quote](page, "li.next a",
		scout.WithPaginateMaxPages(2),
		scout.WithPaginateDedup("Text"),
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Collected %d quotes via click pagination\n", len(clickQuotes))

	for i, q := range clickQuotes {
		if i >= 5 {
			break
		}

		fmt.Printf("  %d. %s — %s\n", i+1, truncate(q.Text, 60), q.Author)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}

	return s[:n] + "..."
}
