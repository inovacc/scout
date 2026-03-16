package main

import (
	"fmt"
	"os"
	"time"

	"github.com/inovacc/scout/pkg/scout"
)

func main() {
	url := "https://developers.londonbridge.pro/"
	if len(os.Args) > 1 {
		url = os.Args[1]
	}

	browser, err := scout.New(
		scout.WithHeadless(true),
		scout.WithNoSandbox(),
		scout.WithStealth(),
		scout.WithBrowser("brave"),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "launch: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = browser.Close() }()

	page, err := browser.NewPage(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "page: %v\n", err)
		os.Exit(1)
	}

	_ = page.WaitLoad()
	time.Sleep(2 * time.Second)

	// Scroll down in increments to trigger lazy-loaded content
	for i := 0; i < 5; i++ {
		_, _ = page.Eval(`window.scrollBy(0, window.innerHeight)`)
		time.Sleep(800 * time.Millisecond)
	}

	// Wait for lazy-loaded content to render
	time.Sleep(1 * time.Second)

	// Full-page screenshot captures entire scrollable area
	data, err := page.FullScreenshot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "screenshot: %v\n", err)
		os.Exit(1)
	}

	outFile := "scroll_capture.png"
	if err := os.WriteFile(outFile, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Captured %s → %s (%d bytes)\n", url, outFile, len(data))
}
