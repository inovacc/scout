// Example: extract-table
// Demonstrates HTML table extraction into structured data.
// Also shows ExtractTexts and ExtractAttribute for simpler extractions.
package main

import (
	"fmt"
	"log"

	"github.com/inovacc/scout"
)

func main() {
	browser, err := scout.New()
	if err != nil {
		log.Fatal(err)
	}

	defer func() { _ = browser.Close() }()

	page, err := browser.NewPage("https://books.toscrape.com/catalogue/a-light-in-the-attic_1000/index.html")
	if err != nil {
		log.Fatal(err)
	}

	if err := page.WaitLoad(); err != nil {
		log.Fatal(err)
	}

	// Extract the product information table.
	table, err := page.ExtractTable("table.table-striped")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Headers:", table.Headers)
	fmt.Printf("Rows (%d):\n", len(table.Rows))

	for _, row := range table.Rows {
		fmt.Println(" ", row)
	}

	// Extract as maps keyed by header text.
	rows, err := page.ExtractTableMap("table.table-striped")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\nAs map (%d entries):\n", len(rows))

	for i, row := range rows {
		if i >= 3 {
			fmt.Printf("  ... and %d more\n", len(rows)-3)
			break
		}

		for k, v := range row {
			fmt.Printf("  %s = %s\n", k, v)
		}
	}

	// Simpler extractions: text and attributes.
	title, err := page.ExtractText("h1")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\nBook title:", title)

	price, err := page.ExtractText("p.price_color")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Price:", price)
}
