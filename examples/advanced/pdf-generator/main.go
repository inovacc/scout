// Example: pdf-generator
// Demonstrates PDF generation with default and custom options.
//
// NOTE: PDF generation requires a Chrome/Chromium version that supports
// the Page.printToPDF CDP method. Some headless configurations or older
// Chrome versions may not support this feature.
package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/inovacc/scout/pkg/scout"
)

func main() {
	browser, err := scout.New(scout.WithTimeout(60 * time.Second))
	if err != nil {
		log.Fatal(err)
	}

	defer func() { _ = browser.Close() }()

	page, err := browser.NewPage("https://example.com")
	if err != nil {
		log.Fatal(err)
	}

	if err := page.WaitLoad(); err != nil {
		log.Fatal(err)
	}

	// Generate a PDF with default settings (portrait, A4).
	fmt.Println("Generating default PDF...")

	pdf, err := page.PDF()
	if err != nil {
		log.Fatal("PDF generation failed:", err)
	}

	if err := os.WriteFile("default.pdf", pdf, 0o644); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Saved default.pdf (%d bytes)\n", len(pdf))

	// Generate a PDF with custom options.
	fmt.Println("Generating custom PDF...")

	customPDF, err := page.PDFWithOptions(scout.PDFOptions{
		Landscape:       true,
		PrintBackground: true,
		Scale:           0.8,
		// A4 paper size in inches
		PaperWidth:  8.27,
		PaperHeight: 11.69,
		// Margins in inches
		MarginTop:    0.5,
		MarginBottom: 0.5,
		MarginLeft:   0.5,
		MarginRight:  0.5,
		// Only print page 1
		PageRanges: "1",
		// Custom header/footer (requires DisplayHeaderFooter)
		DisplayHeaderFooter: true,
		HeaderTemplate:      `<div style="font-size:8px;text-align:center;width:100%">Scout PDF Example</div>`,
		FooterTemplate:      `<div style="font-size:8px;text-align:center;width:100%">Page <span class="pageNumber"></span> of <span class="totalPages"></span></div>`,
	})
	if err != nil {
		log.Fatal("Custom PDF generation failed:", err)
	}

	if err := os.WriteFile("custom.pdf", customPDF, 0o644); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Saved custom.pdf (%d bytes)\n", len(customPDF))
}
