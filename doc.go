// Package scout provides a Go-idiomatic API for headless browser automation,
// web scraping, and search built on [go-rod]. It wraps rod's types (Browser,
// Page, Element) with a simplified interface and adds higher-level scraping
// capabilities.
//
// Core features: navigation, element interaction, screenshots, PDF generation,
// JavaScript evaluation, network interception, cookies, and stealth mode.
//
// Scraping toolkit: struct-tag extraction ([Page.Extract]), HTML table and
// metadata parsing, form detection and filling, rate limiting with retry,
// generic pagination (click-next, URL-pattern, infinite-scroll, load-more),
// search engine integration (Google, Bing, DuckDuckGo), and BFS web crawling
// with sitemap support.
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
//
// Struct-tag extraction:
//
//	type Product struct {
//		Name  string `scout:"h2.title"`
//		Price string `scout:"span.price"`
//		Image string `scout:"img.hero@src"`
//	}
//	var p Product
//	err := page.Extract(&p)
//
// See the examples/ directory for 17 runnable programs covering all features.
//
// [go-rod]: https://github.com/go-rod/rod
package scout
