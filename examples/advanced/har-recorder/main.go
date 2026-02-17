package main

import (
	"fmt"
	"log"
	"os"

	"github.com/inovacc/scout/pkg/scout"
)

func main() {
	b, err := scout.New(scout.WithHeadless(true))
	if err != nil {
		log.Fatal(err)
	}

	defer func() { _ = b.Close() }()

	page, err := b.NewPage("about:blank")
	if err != nil {
		log.Fatal(err)
	}

	// Start recording with body capture enabled
	rec := scout.NewNetworkRecorder(page,
		scout.WithCaptureBody(true),
		scout.WithCreatorName("scout-example", "1.0.0"),
	)
	defer rec.Stop()

	// Navigate to a page — all network traffic is recorded
	if err := page.Navigate("https://example.com"); err != nil {
		log.Fatal(err)
	}

	if err := page.WaitLoad(); err != nil {
		log.Fatal(err)
	}

	// Check what was captured
	entries := rec.Entries()
	fmt.Printf("Captured %d network requests\n", len(entries))

	for _, e := range entries {
		fmt.Printf("  %s %s -> %d\n", e.Request.Method, e.Request.URL, e.Response.Status)
	}

	// Export as HAR file (importable in Chrome DevTools -> Network tab)
	data, count, err := rec.ExportHAR()
	if err != nil {
		log.Fatal(err)
	}

	if err := os.WriteFile("capture.har", data, 0o644); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\nHAR exported: capture.har (%d entries)\n", count)
}
