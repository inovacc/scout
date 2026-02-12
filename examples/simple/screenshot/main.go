// Example: screenshot
// Demonstrates viewport, full-page, and JPEG screenshot capture.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/inovacc/scout/pkg/scout"
)

func main() {
	browser, err := scout.New(scout.WithWindowSize(1280, 720))
	if err != nil {
		log.Fatal(err)
	}

	defer func() { _ = browser.Close() }()

	page, err := browser.NewPage("https://example.com")
	if err != nil {
		log.Fatal(err)
	}

	// Viewport screenshot (PNG).
	png, err := page.Screenshot()
	if err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile("viewport.png", png, 0o644); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Saved viewport.png", len(png), "bytes")

	// Full-page screenshot (PNG).
	full, err := page.FullScreenshot()
	if err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile("fullpage.png", full, 0o644); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Saved fullpage.png", len(full), "bytes")

	// JPEG screenshot with 80% quality.
	jpeg, err := page.ScreenshotJPEG(80)
	if err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile("viewport.jpg", jpeg, 0o644); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Saved viewport.jpg", len(jpeg), "bytes")
}
