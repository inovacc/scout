// Example: bridge-dom
// Crawls a site and extracts DOM JSON + Markdown for every page using SitemapExtract.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/inovacc/scout/pkg/scout"
)

func main() {
	browser, err := scout.New(
		scout.WithHeadless(true),
		scout.WithNoSandbox(),
		scout.WithBridge(),
		scout.WithTimeout(30*time.Second),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer func() { _ = browser.Close() }()

	result, err := browser.SitemapExtract("https://quotes.toscrape.com",
		scout.WithSitemapMaxDepth(1),
		scout.WithSitemapMaxPages(5),
		scout.WithSitemapDelay(1*time.Second),
		scout.WithSitemapDOMDepth(8),
		scout.WithSitemapOutputDir("output"),
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Extracted %d pages\n\n", result.Total)

	for _, p := range result.Pages {
		status := "OK"
		if p.Error != "" {
			status = "ERR: " + p.Error
		}

		fmt.Printf("[depth=%d] %s  %s  links=%d  md=%d bytes\n",
			p.Depth, status, p.URL, len(p.Links), len(p.Markdown))
	}

	// Also write full result as JSON for inspection.
	data, _ := json.MarshalIndent(result, "", "  ")
	_ = os.WriteFile("output/full-result.json", data, 0o644)

	fmt.Println("\nDone. Files in output/")
}
