// Package gmaps implements the scraper.Mode interface for Google Maps place extraction.
// It intercepts Google Maps' internal API calls via session hijacking to capture structured
// place data, reviews, and business information without DOM scraping.
package gmaps

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/scraper"
	"github.com/inovacc/scout/pkg/scout/scraper/auth"
)

// gmapsProvider implements auth.Provider for Google Maps.
type gmapsProvider struct{}

func (p *gmapsProvider) Name() string { return "gmaps" }

func (p *gmapsProvider) LoginURL() string { return "https://accounts.google.com/" }

func (p *gmapsProvider) DetectAuth(ctx context.Context, page *scout.Page) (bool, error) {
	if page == nil {
		return false, fmt.Errorf("gmaps: detect auth: nil page")
	}

	result, err := page.Eval(`() => window.location.href`)
	if err != nil {
		return false, fmt.Errorf("gmaps: detect auth: eval url: %w", err)
	}

	url := result.String()
	if strings.Contains(url, "google.com/maps") {
		// Check for search box element indicating loaded maps page.
		_, err := page.Element("#searchboxinput")
		if err == nil {
			return true, nil
		}
	}

	return false, nil
}

func (p *gmapsProvider) CaptureSession(ctx context.Context, page *scout.Page) (*auth.Session, error) {
	if page == nil {
		return nil, fmt.Errorf("gmaps: capture session: nil page")
	}

	cookies, err := page.GetCookies()
	if err != nil {
		return nil, fmt.Errorf("gmaps: capture session: get cookies: %w", err)
	}

	result, err := page.Eval(`() => window.location.href`)
	if err != nil {
		return nil, fmt.Errorf("gmaps: capture session: eval url: %w", err)
	}

	currentURL := result.String()

	now := time.Now()

	return &auth.Session{
		Provider:  "gmaps",
		Version:   "1",
		Timestamp: now,
		URL:       currentURL,
		Cookies:   cookies,
		ExpiresAt: now.Add(24 * time.Hour),
	}, nil
}

func (p *gmapsProvider) ValidateSession(_ context.Context, session *auth.Session) error {
	if session == nil {
		return fmt.Errorf("gmaps: validate session: nil session")
	}

	// Check for essential authentication cookies: SID or NID
	for _, cookie := range session.Cookies {
		if cookie.Name == "SID" || cookie.Name == "NID" {
			return nil
		}
	}

	return &scraper.AuthError{Reason: "no valid google auth cookies (SID/NID) found in session"}
}

// GMapsMode implements scraper.Mode for Google Maps place extraction.
type GMapsMode struct {
	provider gmapsProvider
}

func (m *GMapsMode) Name() string { return "gmaps" }
func (m *GMapsMode) Description() string {
	return "Scrape Google Maps places, reviews, and business information"
}
func (m *GMapsMode) AuthProvider() scraper.AuthProvider { return &m.provider }

// Scrape creates a browser session, navigates to Google Maps with search targets,
// and intercepts Maps API calls to extract structured place data.
func (m *GMapsMode) Scrape(ctx context.Context, session scraper.SessionData, opts scraper.ScrapeOptions) (<-chan scraper.Result, error) {
	// Session can be nil for public Maps scraping.
	var gmapsSession *auth.Session

	if session != nil {
		var ok bool

		gmapsSession, ok = session.(*auth.Session)
		if !ok {
			return nil, fmt.Errorf("gmaps: scrape: invalid session type")
		}

		if err := m.provider.ValidateSession(ctx, gmapsSession); err != nil {
			return nil, fmt.Errorf("gmaps: scrape: %w", err)
		}
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 10 * time.Minute
	}

	browser, err := scout.New(scout.WithHeadless(opts.Headless),
		scout.WithStealth(),
	)
	if err != nil {
		return nil, fmt.Errorf("gmaps: scrape: create browser: %w", err)
	}

	var startURL string
	if gmapsSession != nil && gmapsSession.URL != "" {
		startURL = gmapsSession.URL
	} else {
		startURL = "https://www.google.com/maps"
	}

	page, err := browser.NewPage(startURL)
	if err != nil {
		_ = browser.Close()
		return nil, fmt.Errorf("gmaps: scrape: new page: %w", err)
	}

	if gmapsSession != nil && len(gmapsSession.Cookies) > 0 {
		if err := page.SetCookies(gmapsSession.Cookies...); err != nil {
			_ = browser.Close()
			return nil, fmt.Errorf("gmaps: scrape: set cookies: %w", err)
		}

		// Reload to apply cookies.
		if _, err := page.Eval(`() => location.reload()`); err != nil {
			_ = browser.Close()
			return nil, fmt.Errorf("gmaps: scrape: reload: %w", err)
		}
	}

	if err := page.WaitLoad(); err != nil {
		_ = browser.Close()
		return nil, fmt.Errorf("gmaps: scrape: wait load: %w", err)
	}

	hijacker, err := page.NewSessionHijacker(scout.WithHijackURLFilter("*google.com/maps/preview/*"),
		scout.WithHijackURLFilter("*google.com/maps/rpc/*"),
		scout.WithHijackBodyCapture(),
	)
	if err != nil {
		_ = browser.Close()
		return nil, fmt.Errorf("gmaps: scrape: create hijacker: %w", err)
	}

	results := make(chan scraper.Result, 256)
	targetSet := buildTargetSet(opts.Targets)

	go func() {
		defer close(results)
		defer hijacker.Stop()
		defer func() { _ = browser.Close() }()

		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		count := 0
		// If targets provided, perform searches.
		if len(opts.Targets) > 0 {
			for _, target := range opts.Targets {
				select {
				case <-ctx.Done():
					return
				default:
				}

				searchURL := fmt.Sprintf("https://www.google.com/maps/search/%s", encodeSearchQuery(target))

				if err := page.Navigate(searchURL); err != nil {
					continue
				}

				if err := page.WaitLoad(); err != nil {
					continue
				}

				time.Sleep(2 * time.Second)
			}
		}

		// Collect results from hijacked network events.
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-hijacker.Events():
				if !ok {
					return
				}

				if opts.Limit > 0 && count >= opts.Limit {
					return
				}

				items := parseHijackEvent(ev, targetSet)
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
								Message: fmt.Sprintf("captured %d places/reviews", count),
							})
						}
					}
				}
			}
		}
	}()

	return results, nil
}

// buildTargetSet creates a lookup set from target search queries.
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

// encodeSearchQuery URL-encodes a search query for Google Maps.
func encodeSearchQuery(q string) string {
	return strings.ReplaceAll(strings.TrimSpace(q), " ", "+")
}

// parseHijackEvent examines a network event and extracts scraper.Result items from Maps API responses.
func parseHijackEvent(ev scout.HijackEvent, targetSet map[string]struct{}) []scraper.Result {
	if ev.Type != scout.HijackEventResponse || ev.Response == nil {
		return nil
	}

	url := ev.Response.URL

	body := ev.Response.Body
	if body == "" {
		return nil
	}

	switch {
	case strings.Contains(url, "/maps/preview/") || strings.Contains(url, "/maps/rpc/"):
		return parseMapsPLACES(body, targetSet)
	default:
		return nil
	}
}

// parseMapsPLACES parses Google Maps RPC responses for business listings and reviews.
func parseMapsPLACES(body string, targetSet map[string]struct{}) []scraper.Result { //nolint:unparam
	// Google Maps RPC responses are complex and often JSON-in-JSON nested.
	// This is a simplified parser for common response structures.
	var results []scraper.Result

	// Try to parse as generic JSON to detect response type.
	var rawResp any

	if err := json.Unmarshal([]byte(body), &rawResp); err != nil {
		return nil
	}

	// Parse business profile responses.
	if profile := parseBusinessProfile(body); profile != nil {
		results = append(results, *profile)
	}

	// Parse review responses.
	reviews := parseReviews(body)
	results = append(results, reviews...)

	// Parse photos.
	photos := parsePhotos(body)
	results = append(results, photos...)

	return results
}

// businessProfile represents extracted Google Maps business data.
type businessProfile struct {
	ID      string
	Name    string
	Address string
	Rating  float64
	Reviews int
	Phone   string
	Website string
	Hours   map[string]string
	Types   []string
}

func parseBusinessProfile(body string) *scraper.Result {
	var profile businessProfile

	// Try to extract place name.
	namePattern := `"name":"([^"]+)"`
	if nameIdx := strings.Index(body, namePattern); nameIdx >= 0 {
		end := strings.Index(body[nameIdx+10:], `"`)
		if end > 0 {
			profile.Name = body[nameIdx+10 : nameIdx+10+end]
		}
	}

	// Try to extract address.
	addressPattern := `"formatted_address":"([^"]+)"`
	if addrIdx := strings.Index(body, addressPattern); addrIdx >= 0 {
		end := strings.Index(body[addrIdx+23:], `"`)
		if end > 0 {
			profile.Address = body[addrIdx+23 : addrIdx+23+end]
		}
	}

	// Try to extract phone.
	phonePattern := `"international_phone_number":"([^"]+)"`
	if phoneIdx := strings.Index(body, phonePattern); phoneIdx >= 0 {
		end := strings.Index(body[phoneIdx+34:], `"`)
		if end > 0 {
			profile.Phone = body[phoneIdx+34 : phoneIdx+34+end]
		}
	}

	// Try to extract rating.
	ratingPattern := `"rating":`
	if ratingIdx := strings.Index(body, ratingPattern); ratingIdx >= 0 {
		ratingStart := ratingIdx + len(ratingPattern)

		ratingEnd := strings.IndexAny(body[ratingStart:], ",}")
		if ratingEnd > 0 {
			ratingStr := body[ratingStart : ratingStart+ratingEnd]

			var rating float64
			if _, err := fmt.Sscanf(ratingStr, "%f", &rating); err == nil {
				profile.Rating = rating
			}
		}
	}

	if profile.Name == "" && profile.Address == "" {
		return nil
	}

	return &scraper.Result{
		Type:      scraper.ResultProfile,
		Source:    "gmaps",
		ID:        profile.ID,
		Timestamp: time.Now(),
		Content:   profile.Name,
		Metadata: map[string]any{
			"name":    profile.Name,
			"address": profile.Address,
			"rating":  profile.Rating,
			"phone":   profile.Phone,
			"reviews": profile.Reviews,
		},
		Raw: profile,
	}
}

// parseReviews extracts review data from Maps API responses.
func parseReviews(body string) []scraper.Result {
	var results []scraper.Result

	// Simple pattern matching for review text.
	reviewPattern := `"review":"([^"]+)"`

	idx := 0
	for {
		foundIdx := strings.Index(body[idx:], reviewPattern)
		if foundIdx < 0 {
			break
		}

		actualIdx := idx + foundIdx
		textStart := actualIdx + len(reviewPattern) - 1

		textEnd := strings.Index(body[textStart+11:], `"`)
		if textEnd < 0 {
			idx = actualIdx + 1
			continue
		}

		reviewText := body[textStart+11 : textStart+11+textEnd]
		results = append(results, scraper.Result{
			Type:      scraper.ResultComment,
			Source:    "gmaps",
			ID:        fmt.Sprintf("review_%d", len(results)),
			Timestamp: time.Now(),
			Content:   reviewText,
			Metadata: map[string]any{
				"type": "review",
			},
		})

		idx = actualIdx + 1
	}

	return results
}

// parsePhotos extracts photo URLs from Maps API responses.
func parsePhotos(body string) []scraper.Result {
	var results []scraper.Result

	// Simple pattern matching for photo URLs.
	photoPattern := `"url":"(https://[^"]+)"`

	idx := 0
	for {
		foundIdx := strings.Index(body[idx:], photoPattern)
		if foundIdx < 0 {
			break
		}

		actualIdx := idx + foundIdx
		urlStart := actualIdx + len(photoPattern) - 1

		urlEnd := strings.Index(body[urlStart+7:], `"`)
		if urlEnd < 0 {
			idx = actualIdx + 1
			continue
		}

		photoURL := body[urlStart+7 : urlStart+7+urlEnd]
		if strings.Contains(photoURL, "google.com") || strings.Contains(photoURL, "maps") {
			results = append(results, scraper.Result{
				Type:      scraper.ResultFile,
				Source:    "gmaps",
				ID:        fmt.Sprintf("photo_%d", len(results)),
				Timestamp: time.Now(),
				URL:       photoURL,
				Metadata: map[string]any{
					"type": "photo",
				},
			})
		}

		idx = actualIdx + 1
	}

	return results
}

func init() {
	scraper.RegisterMode(&GMapsMode{})
}
