// Example: basic-navigation
// Demonstrates browser creation, page navigation, and reading page info.
package main

import (
	"fmt"
	"log"

	"github.com/inovacc/scout/pkg/scout"
)

func main() {
	// Create a headless browser with default options.
	browser, err := scout.New()
	if err != nil {
		log.Fatal(err)
	}

	defer func() { _ = browser.Close() }()

	// Open a new page and navigate to a URL.
	page, err := browser.NewPage("https://example.com")
	if err != nil {
		log.Fatal(err)
	}

	// Read page title.
	title, err := page.Title()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Title:", title)

	// Read current URL.
	url, err := page.URL()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("URL:", url)

	// Get page HTML (first 200 chars).
	html, err := page.HTML()
	if err != nil {
		log.Fatal(err)
	}

	if len(html) > 200 {
		html = html[:200] + "..."
	}

	fmt.Println("HTML:", html)

	// Navigate to another page.
	if err := page.Navigate("https://example.org"); err != nil {
		log.Fatal(err)
	}

	title, _ = page.Title()
	fmt.Println("After navigate:", title)

	// Go back.
	if err := page.NavigateBack(); err != nil {
		log.Fatal(err)
	}

	title, _ = page.Title()
	fmt.Println("After back:", title)
}
