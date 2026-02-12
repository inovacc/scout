// Example: request-intercept
// Demonstrates request interception: modifying responses and blocking URLs.
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

	page, err := browser.NewPage("about:blank")
	if err != nil {
		log.Fatal(err)
	}

	// Intercept all requests matching a pattern.
	router, err := page.Hijack("*", func(ctx *scout.HijackContext) {
		req := ctx.Request()
		url := req.URL().String()

		// Log every request.
		fmt.Printf("[%s] %s\n", req.Method(), url)

		// Load the original response so we can inspect/modify it.
		if err := ctx.LoadResponse(true); err != nil {
			ctx.ContinueRequest()
			return
		}

		// Modify specific responses by adding custom headers.
		resp := ctx.Response()
		resp.SetHeader("X-Intercepted", "true")

		ctx.ContinueRequest()
	})
	if err != nil {
		log.Fatal(err)
	}

	// Run the hijack router in a goroutine (it blocks).
	go router.Run()

	defer func() { _ = router.Stop() }()

	// Navigate — all requests will be intercepted and logged.
	if err := page.Navigate("https://example.com"); err != nil {
		log.Fatal(err)
	}

	if err := page.WaitLoad(); err != nil {
		log.Fatal(err)
	}

	title, _ := page.Title()
	fmt.Println("\nPage title:", title)

	// Block specific URL patterns (e.g., analytics, ads).
	if err := page.SetBlockedURLs(
		"*google-analytics.com*",
		"*doubleclick.net*",
	); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Blocked analytics and ad URLs")
}
