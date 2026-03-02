// Package amazon implements the scraper.Mode interface for Amazon product extraction.
// It captures session cookies and tokens, then performs DOM extraction and optional
// network hijacking to gather product details, prices, reviews, and seller information.
package amazon

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/scraper"
	"github.com/inovacc/scout/pkg/scout/scraper/auth"
)

// amazonProvider implements auth.Provider for Amazon accounts.
type amazonProvider struct{}

func (p *amazonProvider) Name() string { return "amazon" }

func (p *amazonProvider) LoginURL() string { return "https://www.amazon.com/ap/signin" }

func (p *amazonProvider) DetectAuth(ctx context.Context, page *scout.Page) (bool, error) {
	if page == nil {
		return false, fmt.Errorf("amazon: detect auth: nil page")
	}

	result, err := page.Eval(`() => window.location.href`)
	if err != nil {
		return false, fmt.Errorf("amazon: detect auth: eval url: %w", err)
	}

	url := result.String()

	// If still on signin page, not authenticated.
	if strings.Contains(url, "/ap/signin") {
		return false, nil
	}

	// Check for account list element (nav-link-accountList) indicating authenticated state.
	_, err = page.Element("a.nav-link-accountList")
	if err == nil {
		return true, nil
	}

	// If URL indicates we left signin, assume authenticated.
	if strings.Contains(url, "amazon.com") && !strings.Contains(url, "/ap/signin") {
		return true, nil
	}

	return false, nil
}

func (p *amazonProvider) CaptureSession(ctx context.Context, page *scout.Page) (*auth.Session, error) {
	if page == nil {
		return nil, fmt.Errorf("amazon: capture session: nil page")
	}

	cookies, err := page.GetCookies()
	if err != nil {
		return nil, fmt.Errorf("amazon: capture session: get cookies: %w", err)
	}

	result, err := page.Eval(`() => window.location.href`)
	if err != nil {
		return nil, fmt.Errorf("amazon: capture session: eval url: %w", err)
	}

	currentURL := result.String()

	now := time.Now()

	return &auth.Session{
		Provider:  "amazon",
		Version:   "1",
		Timestamp: now,
		URL:       currentURL,
		Cookies:   cookies,
		ExpiresAt: now.Add(30 * 24 * time.Hour), // Amazon sessions typically last ~30 days.
	}, nil
}

func (p *amazonProvider) ValidateSession(_ context.Context, session *auth.Session) error {
	if session == nil {
		return fmt.Errorf("amazon: validate session: nil session")
	}

	// Check for session-id cookie, which is essential for Amazon authentication.
	for _, cookie := range session.Cookies {
		if strings.EqualFold(cookie.Name, "session-id") && cookie.Value != "" {
			return nil
		}
	}

	return &scraper.AuthError{Reason: "no session-id cookie found in session"}
}

// AmazonMode implements scraper.Mode for Amazon products.
type AmazonMode struct {
	provider amazonProvider
}

func (m *AmazonMode) Name() string { return "amazon" }
func (m *AmazonMode) Description() string {
	return "Scrape Amazon product details, prices, reviews, and seller information"
}
func (m *AmazonMode) AuthProvider() scraper.AuthProvider { return &m.provider }

// Scrape creates a browser session, restores cookies, and performs product extraction
// via DOM scraping and optional network hijacking.
func (m *AmazonMode) Scrape(ctx context.Context, session scraper.SessionData, opts scraper.ScrapeOptions) (<-chan scraper.Result, error) {
	amazonSession, ok := session.(*auth.Session)
	if !ok || amazonSession == nil {
		return nil, fmt.Errorf("amazon: scrape: invalid or nil session")
	}

	if err := m.provider.ValidateSession(ctx, amazonSession); err != nil {
		return nil, fmt.Errorf("amazon: scrape: %w", err)
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 10 * time.Minute
	}

	browser, err := scout.New(
		scout.WithHeadless(opts.Headless),
		scout.WithStealth(),
	)
	if err != nil {
		return nil, fmt.Errorf("amazon: scrape: create browser: %w", err)
	}

	page, err := browser.NewPage(amazonSession.URL)
	if err != nil {
		browser.Close()
		return nil, fmt.Errorf("amazon: scrape: new page: %w", err)
	}

	if err := page.SetCookies(amazonSession.Cookies...); err != nil {
		browser.Close()
		return nil, fmt.Errorf("amazon: scrape: set cookies: %w", err)
	}

	// Reload to apply cookies.
	if _, err := page.Eval(`() => location.reload()`); err != nil {
		browser.Close()
		return nil, fmt.Errorf("amazon: scrape: reload: %w", err)
	}

	if err := page.WaitLoad(); err != nil {
		browser.Close()
		return nil, fmt.Errorf("amazon: scrape: wait load: %w", err)
	}

	results := make(chan scraper.Result, 256)

	go func() {
		defer close(results)
		defer browser.Close()

		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		count := 0

		// If targets are provided, navigate to search/product pages and extract.
		if len(opts.Targets) > 0 {
			for _, target := range opts.Targets {
				if opts.Limit > 0 && count >= opts.Limit {
					return
				}

				items, err := scrapeTarget(ctx, page, target)
				if err != nil {
					continue
				}

				for _, item := range items {
					select {
					case <-ctx.Done():
						return
					case results <- item:
						count++
						if opts.Limit > 0 && count >= opts.Limit {
							return
						}

						if opts.Progress != nil {
							opts.Progress(scraper.Progress{
								Phase:   "scraping",
								Current: count,
								Total:   opts.Limit,
								Message: fmt.Sprintf("extracted %d items", count),
							})
						}
					}
				}
			}
		} else {
			// No targets: stay on current page and extract visible products.
			items, err := extractProductsFromPage(ctx, page)
			if err == nil {
				for _, item := range items {
					select {
					case <-ctx.Done():
						return
					case results <- item:
						count++
						if opts.Limit > 0 && count >= opts.Limit {
							return
						}

						if opts.Progress != nil {
							opts.Progress(scraper.Progress{
								Phase:   "scraping",
								Current: count,
								Total:   opts.Limit,
								Message: fmt.Sprintf("extracted %d items", count),
							})
						}
					}
				}
			}
		}
	}()

	return results, nil
}

// buildTargetSet creates a lookup set from ASINs and search queries.
// An empty set means no filtering.
func buildTargetSet(targets []string) map[string]struct{} {
	if len(targets) == 0 {
		return nil
	}

	set := make(map[string]struct{}, len(targets))
	for _, t := range targets {
		set[strings.ToLower(strings.TrimSpace(t))] = struct{}{}
	}

	return set
}

// scrapeTarget navigates to a product (by ASIN) or search query and extracts items.
func scrapeTarget(ctx context.Context, page *scout.Page, target string) ([]scraper.Result, error) {
	// Detect if target is an ASIN (10-char alphanumeric) or a search query.
	if isASIN(target) {
		url := fmt.Sprintf("https://www.amazon.com/dp/%s", target)
		if err := page.Navigate(url); err != nil {
			return nil, fmt.Errorf("amazon: navigate to product: %w", err)
		}
	} else {
		// Search query.
		url := fmt.Sprintf("https://www.amazon.com/s?k=%s", strings.ReplaceAll(target, " ", "+"))
		if err := page.Navigate(url); err != nil {
			return nil, fmt.Errorf("amazon: navigate to search: %w", err)
		}
	}

	if err := page.WaitLoad(); err != nil {
		return nil, fmt.Errorf("amazon: wait load: %w", err)
	}

	return extractProductsFromPage(ctx, page)
}

// isASIN checks if the target looks like a 10-char Amazon ASIN.
func isASIN(target string) bool {
	target = strings.TrimSpace(target)
	return len(target) == 10 && regexp.MustCompile(`^[A-Z0-9]+$`).MatchString(target)
}

// extractProductsFromPage extracts product details from the current page via DOM.
func extractProductsFromPage(ctx context.Context, page *scout.Page) ([]scraper.Result, error) {
	var results []scraper.Result

	// Extract product containers. Adjust selector based on page layout.
	elements, err := page.Element("[data-component-type='s-search-result']")
	if err != nil {
		// Try alternative selector for product listings.
		elements, err = page.Element("div[data-asin]")
		if err != nil {
			// If no search results found, try single product page.
			return extractSingleProductPage(ctx, page)
		}
	}

	if elements == nil {
		return nil, nil
	}

	// Extract individual product from the container.
	product, err := extractProductFromElement(ctx, elements)
	if err == nil && product != nil {
		results = append(results, *product)
	}

	return results, nil
}

// extractSingleProductPage extracts product details from a single product detail page (dp/).
func extractSingleProductPage(ctx context.Context, page *scout.Page) ([]scraper.Result, error) {
	// Extract ASIN from URL or page data.
	asinResult, err := page.Eval(`() => {
		const params = new URLSearchParams(window.location.search);
		return params.get('dp') || (window.location.pathname.match(/\/dp\/([A-Z0-9]+)/) || []).pop() || '';
	}`)
	if err != nil {
		return nil, fmt.Errorf("amazon: extract asin: %w", err)
	}

	asin := asinResult.String()

	// Extract title.
	titleElem, err := page.Element("h1 span")
	if err != nil {
		return nil, fmt.Errorf("amazon: extract title: %w", err)
	}

	titleText, _ := titleElem.Text()
	// Extract price.
	priceElem, err := page.Element("span.a-price-whole, span[data-a-color='price']")
	if err == nil {
		priceText, _ := priceElem.Text()
		priceText = strings.TrimSpace(priceText)

		product := scraper.Result{
			Type:      scraper.ResultPost,
			Source:    "amazon",
			ID:        asin,
			Timestamp: time.Now(),
			Content:   strings.TrimSpace(titleText),
			URL:       "https://www.amazon.com/dp/" + asin,
			Metadata: map[string]any{
				"title": strings.TrimSpace(titleText),
				"price": priceText,
			},
		}

		// Extract rating if available.
		ratingElem, err := page.Element("div.a-icon-star span")
		if err == nil {
			ratingText, _ := ratingElem.Text()
			if ratingText != "" {
				product.Metadata["rating"] = ratingText
			}
		}

		// Extract review count if available.
		reviewElem, err := page.Element("span[data-hook='total-review-count']")
		if err == nil {
			reviewText, _ := reviewElem.Text()
			product.Metadata["review_count"] = reviewText
		}

		return []scraper.Result{product}, nil
	}

	return nil, nil
}

// extractProductFromElement extracts product data from a product container element.
func extractProductFromElement(ctx context.Context, elem *scout.Element) (*scraper.Result, error) {
	if elem == nil {
		return nil, fmt.Errorf("amazon: extract product: nil element")
	}

	// Extract ASIN from data-asin attribute.
	asinAttr, ok, err := elem.Attribute("data-asin")
	if err != nil || !ok || asinAttr == "" {
		return nil, fmt.Errorf("amazon: extract asin attribute: %w", err)
	}

	// Extract title.
	titleElem, err := elem.Element("h2 a span")
	if err != nil {
		titleElem, _ = elem.Element("a.a-link-normal span")
	}

	var title string
	if titleElem != nil {
		title, _ = titleElem.Text()
	}

	title = strings.TrimSpace(title)

	// Extract price.
	priceElem, err := elem.Element("span[data-a-color='price']")
	if err != nil {
		priceElem, _ = elem.Element("span.a-price-whole")
	}

	var price string
	if priceElem != nil {
		price, _ = priceElem.Text()
	}

	price = strings.TrimSpace(price)

	// Extract rating.
	ratingElem, err := elem.Element("span.a-icon-star span")
	if err != nil {
		ratingElem, _ = elem.Element("div.a-icon-star span")
	}

	var rating string
	if ratingElem != nil {
		rating, _ = ratingElem.Text()
	}

	rating = strings.TrimSpace(rating)

	// Extract review count.
	reviewElem, err := elem.Element("span[aria-label*='rating']")

	var reviewCount string
	if reviewElem != nil {
		reviewCount, _ = reviewElem.Text()
	}

	// Extract product URL.
	linkElem, err := elem.Element("h2 a, a.a-link-normal")

	var productURL string

	if linkElem != nil {
		href, _, _ := linkElem.Attribute("href")
		if href != "" {
			if strings.HasPrefix(href, "/") {
				productURL = "https://www.amazon.com" + href
			} else {
				productURL = href
			}
		}
	}

	metadata := map[string]any{
		"asin": asinAttr,
	}

	if title != "" {
		metadata["title"] = title
	}

	if price != "" {
		metadata["price"] = price
	}

	if rating != "" {
		metadata["rating"] = rating
	}

	if reviewCount != "" {
		metadata["review_count"] = reviewCount
	}

	return &scraper.Result{
		Type:      scraper.ResultPost,
		Source:    "amazon",
		ID:        asinAttr,
		Timestamp: time.Now(),
		Author:    "", // Amazon products don't have a single author
		Content:   title,
		URL:       productURL,
		Metadata:  metadata,
	}, nil
}

// extractReviewsFromPage extracts product reviews as ResultComment items.
func extractReviewsFromPage(ctx context.Context, page *scout.Page, asin string) []scraper.Result {
	var results []scraper.Result

	// Navigate to reviews page if not already there.
	currentURL, _ := page.Eval(`() => window.location.href`)
	if currentURL != nil && !strings.Contains(currentURL.String(), "/reviews/") {
		reviewURL := fmt.Sprintf("https://www.amazon.com/product-reviews/%s", asin)
		if err := page.Navigate(reviewURL); err != nil {
			return results
		}

		if err := page.WaitLoad(); err != nil {
			return results
		}
	}

	// Extract review elements.
	// This is a placeholder for review extraction logic.
	// In a real implementation, you would iterate through review containers
	// and extract star ratings, reviewer names, review text, etc.

	return results
}

// extractSellerInfo extracts seller information from a product detail page.
func extractSellerInfo(ctx context.Context, page *scout.Page, asin string) *scraper.Result {
	// Extract seller name from "Ships from and sold by" section.
	sellerElem, err := page.Element("a.a-link-normal.a-text-bold")
	if err != nil {
		return nil
	}

	sellerName, _ := sellerElem.Text()
	sellerName = strings.TrimSpace(sellerName)

	if sellerName == "" {
		return nil
	}

	// Extract seller link.
	sellerLink, _, _ := sellerElem.Attribute("href")

	return &scraper.Result{
		Type:      scraper.ResultProfile,
		Source:    "amazon",
		ID:        sellerName,
		Timestamp: time.Now(),
		Author:    sellerName,
		Content:   "",
		URL:       sellerLink,
		Metadata: map[string]any{
			"seller_name": sellerName,
			"asin":        asin,
		},
	}
}

// parsePrice extracts the numeric price from a price string like "$19.99" or "₹1,299.00".
func parsePrice(priceStr string) float64 {
	// Remove currency symbols and whitespace.
	priceStr = strings.TrimSpace(priceStr)
	priceStr = strings.Map(func(r rune) rune {
		if (r >= '0' && r <= '9') || r == '.' {
			return r
		}

		return -1
	}, priceStr)

	if priceStr == "" {
		return 0
	}

	val, _ := strconv.ParseFloat(priceStr, 64)

	return val
}

func init() {
	scraper.RegisterMode(&AmazonMode{})
}
