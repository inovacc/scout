// Package scout provides a Go-idiomatic API for headless browser automation
// using go-rod. It supports navigation, element interaction, screenshots, PDF
// generation, JavaScript evaluation, network interception, and stealth mode.
//
// Basic usage:
//
//	b, err := scout.New(scout.WithHeadless(true))
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer b.Close()
//
//	page, err := b.NewPage("https://example.com")
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	title, err := page.Title()
//	if err != nil {
//		log.Fatal(err)
//	}
//	fmt.Println(title)
package scout
