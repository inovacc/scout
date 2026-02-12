// Example: extract-meta
// Demonstrates extraction of page metadata (title, description, OG tags, etc.).
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

	// Use a site with rich metadata (OG, Twitter, etc.).
	page, err := browser.NewPage("https://ogp.me")
	if err != nil {
		log.Fatal(err)
	}

	if err := page.WaitLoad(); err != nil {
		log.Fatal(err)
	}

	// Extract all page metadata.
	meta, err := page.ExtractMeta()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Title:", meta.Title)
	fmt.Println("Description:", meta.Description)
	fmt.Println("Canonical:", meta.Canonical)

	if len(meta.OG) > 0 {
		fmt.Println("\nOpen Graph tags:")

		for k, v := range meta.OG {
			fmt.Printf("  %s = %s\n", k, v)
		}
	}

	if len(meta.Twitter) > 0 {
		fmt.Println("\nTwitter tags:")

		for k, v := range meta.Twitter {
			fmt.Printf("  %s = %s\n", k, v)
		}
	}

	if len(meta.JSONLD) > 0 {
		fmt.Printf("\nJSON-LD: %d entries found\n", len(meta.JSONLD))
	}
}
