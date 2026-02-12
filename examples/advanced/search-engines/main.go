// Example: search-engines
// Demonstrates searching Google, Bing, and DuckDuckGo with SERP parsing.
package main

import (
	"fmt"
	"log"

	"github.com/inovacc/scout/pkg/scout"
)

func main() {
	browser, err := scout.New()
	if err != nil {
		log.Fatal(err)
	}

	defer func() { _ = browser.Close() }()

	// Search Google (default engine).
	results, err := browser.Search("golang web scraping",
		scout.WithSearchEngine(scout.Google),
		scout.WithSearchLanguage("en"),
		scout.WithSearchRegion("us"),
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Google: %d results for %q\n", len(results.Results), results.Query)

	for _, r := range results.Results[:min(3, len(results.Results))] {
		fmt.Printf("  #%d %s\n      %s\n", r.Position, r.Title, r.URL)
	}

	// Search across multiple pages with SearchAll.
	allResults, err := browser.SearchAll("go-rod browser automation",
		scout.WithSearchEngine(scout.DuckDuckGo),
		scout.WithSearchMaxPages(2),
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\nDuckDuckGo: %d total results across pages\n", len(allResults))

	for i, r := range allResults {
		if i >= 5 {
			break
		}

		fmt.Printf("  #%d %s\n", r.Position, r.Title)
	}
}
