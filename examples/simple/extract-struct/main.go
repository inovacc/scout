// Example: extract-struct
// Demonstrates struct-tag-based extraction using scout:"selector" tags.
// Targets quotes.toscrape.com to extract quote data.
package main

import (
	"fmt"
	"log"

	"github.com/inovacc/scout/pkg/scout"
)

// QuotePage holds data extracted from the quotes.toscrape.com homepage.
type QuotePage struct {
	Heading string  `scout:"h1 a"`
	Quotes  []Quote `scout:"div.quote"`
}

// Quote represents a single quote listing.
// Selectors are resolved relative to the parent div.quote container.
type Quote struct {
	Text   string   `scout:"span.text"`
	Author string   `scout:"small.author"`
	Tags   []string `scout:"a.tag"`
}

func main() {
	browser, err := scout.New()
	if err != nil {
		log.Fatal(err)
	}

	defer func() { _ = browser.Close() }()

	page, err := browser.NewPage("https://quotes.toscrape.com")
	if err != nil {
		log.Fatal(err)
	}

	if err := page.WaitLoad(); err != nil {
		log.Fatal(err)
	}

	// Extract structured data using scout tags.
	var data QuotePage
	if err := page.Extract(&data); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Heading:", data.Heading)
	fmt.Printf("Found %d quotes\n", len(data.Quotes))

	for i, q := range data.Quotes {
		if i >= 5 {
			break // Print first 5 only
		}

		fmt.Printf("  %d. %s\n     — %s (tags: %v)\n", i+1, q.Text, q.Author, q.Tags)
	}
}
