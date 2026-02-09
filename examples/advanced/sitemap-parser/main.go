// Example: sitemap-parser
// Demonstrates parsing a sitemap.xml file to discover site URLs.
package main

import (
	"fmt"
	"log"

	"github.com/inovacc/scout"
)

func main() {
	browser, err := scout.New()
	if err != nil {
		log.Fatal(err)
	}

	defer func() { _ = browser.Close() }()

	// Parse sitemap.xml from a public site.
	urls, err := browser.ParseSitemap("https://www.sitemaps.org/sitemap.xml")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Found %d URLs in sitemap\n", len(urls))

	for i, u := range urls {
		if i >= 20 {
			fmt.Printf("  ... and %d more\n", len(urls)-20)
			break
		}

		fmt.Printf("  %s", u.Loc)

		if u.LastMod != "" {
			fmt.Printf(" (modified: %s)", u.LastMod)
		}

		if u.Priority != "" {
			fmt.Printf(" [priority: %s]", u.Priority)
		}

		fmt.Println()
	}
}
