// Example: cookies-headers
// Demonstrates setting/getting cookies and custom request headers.
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

	page, err := browser.NewPage("https://example.com")
	if err != nil {
		log.Fatal(err)
	}

	// Set cookies.
	if err := page.SetCookies(
		scout.Cookie{
			Name:    "session_id",
			Value:   "abc123",
			URL:     "https://example.com",
			Expires: time.Now().Add(24 * time.Hour),
		},
		scout.Cookie{
			Name:  "theme",
			Value: "dark",
			URL:   "https://example.com",
		},
	); err != nil {
		log.Fatal(err)
	}

	// Read cookies back.
	cookies, err := page.GetCookies()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Cookies:")

	for _, c := range cookies {
		fmt.Printf("  %s = %s\n", c.Name, c.Value)
	}

	// Set custom headers. The cleanup function restores original headers.
	cleanup, err := page.SetHeaders(map[string]string{
		"X-Custom-Header": "scout-example",
		"Accept-Language": "en-US",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer cleanup()

	// Navigate with custom headers active.
	if err := page.Navigate("https://example.com"); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Navigated with custom headers")

	// Clear all cookies.
	if err := page.ClearCookies(); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Cookies cleared")
}
