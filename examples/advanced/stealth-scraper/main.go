// Example: stealth-scraper
// Demonstrates stealth mode, custom user agent, and proxy configuration.
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/inovacc/scout/pkg/scout"
)

func main() {
	// Create a browser with stealth mode to avoid bot detection.
	// WithStealth() patches common browser fingerprinting checks.
	browser, err := scout.New(
		scout.WithStealth(),
		scout.WithUserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		scout.WithWindowSize(1920, 1080),
		scout.WithTimeout(60*time.Second),
		// Uncomment to use a proxy:
		// scout.WithProxy("socks5://127.0.0.1:1080"),
	)
	if err != nil {
		log.Fatal(err)
	}

	defer func() { _ = browser.Close() }()

	page, err := browser.NewPage("https://example.com")
	if err != nil {
		log.Fatal(err)
	}

	// Verify the user agent was set.
	res, err := page.Eval(`() => navigator.userAgent`)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("User-Agent:", res.String())

	// Check that common bot detection signals are patched.
	res, err = page.Eval(`() => navigator.webdriver`)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("navigator.webdriver:", res.Bool())

	// Extract page content normally.
	title, err := page.Title()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Page title:", title)

	// Extract links to verify the page loaded correctly.
	links, err := page.ExtractLinks()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Found %d links on page\n", len(links))
}
