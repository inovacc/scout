// Example: rate-limited-scraper
// Demonstrates rate limiting and retry with exponential backoff.
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/inovacc/scout/pkg/scout"
)

func main() {
	browser, err := scout.New()
	if err != nil {
		log.Fatal(err)
	}

	defer func() { _ = browser.Close() }()

	// Create a rate limiter: 1 request/sec, burst of 2, max 3 retries.
	rl := scout.NewRateLimiter(
		scout.WithRateLimit(1.0),
		scout.WithBurstSize(2),
		scout.WithMaxRetries(3),
		scout.WithBackoff(500*time.Millisecond),
		scout.WithMaxBackoff(5*time.Second),
		scout.WithMaxConcurrent(2),
	)

	// Use NavigateWithRetry for automatic rate limiting and retry.
	page, err := browser.NewPage("about:blank")
	if err != nil {
		log.Fatal(err)
	}

	urls := []string{
		"https://quotes.toscrape.com/page/1/",
		"https://quotes.toscrape.com/page/2/",
		"https://quotes.toscrape.com/page/3/",
	}

	for _, url := range urls {
		if err := page.NavigateWithRetry(url, rl); err != nil {
			log.Printf("Failed to navigate to %s: %v", url, err)
			continue
		}

		// Wait for the page to load before reading the title.
		if err := page.WaitLoad(); err != nil {
			log.Printf("WaitLoad failed: %v", err)
			continue
		}

		title, _ := page.Title()
		fmt.Printf("Visited: %s - %s\n", url, title)
	}

	// Use rl.Do() for arbitrary rate-limited operations.
	err = rl.Do(func() error {
		page2, err := browser.NewPage("https://example.com")
		if err != nil {
			return err
		}

		defer func() { _ = page2.Close() }()

		if err := page2.WaitLoad(); err != nil {
			return err
		}

		title, _ := page2.Title()
		fmt.Println("Rate-limited fetch:", title)

		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
}
