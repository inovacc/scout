// Example: javascript-eval
// Demonstrates JavaScript evaluation with typed result handling.
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

	page, err := browser.NewPage("https://example.com")
	if err != nil {
		log.Fatal(err)
	}

	// Evaluate a string expression.
	res, err := page.Eval(`() => document.title`)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Title:", res.String())

	// Evaluate a numeric expression.
	res, err = page.Eval(`() => document.querySelectorAll("a").length`)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Link count:", res.Int())

	// Evaluate a boolean expression.
	res, err = page.Eval(`() => document.title.includes("Example")`)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Has 'Example':", res.Bool())

	// Decode a complex object into a Go struct.
	res, err = page.Eval(`() => ({
		url: location.href,
		width: window.innerWidth,
		height: window.innerHeight,
	})`)
	if err != nil {
		log.Fatal(err)
	}

	var info struct {
		URL    string `json:"url"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
	}
	if err := res.Decode(&info); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Window: %dx%d at %s\n", info.Width, info.Height, info.URL)
}
