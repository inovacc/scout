// Package tiktok implements the scraper.Mode interface for TikTok.
// It extracts video metadata, comments, profiles, and trending content
// using browser automation and API interception via session hijacking.
package tiktok

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

func init() {
	scraper.RegisterMode(&TikTokMode{})
}

// tiktokProvider implements auth.Provider for TikTok authentication.
type tiktokProvider struct{}

func (p *tiktokProvider) Name() string { return "tiktok" }

func (p *tiktokProvider) LoginURL() string { return "https://www.tiktok.com/login" }

// DetectAuth checks whether the current page reflects a logged-in TikTok session.
func (p *tiktokProvider) DetectAuth(ctx context.Context, page *scout.Page) (bool, error) {
	pageURL, err := page.URL()
	if err != nil {
		return false, fmt.Errorf("tiktok: detect auth: %w", err)
	}

	if strings.Contains(pageURL, "/login") {
		return false, nil
	}

	// Check for avatar/profile link that appears when logged in.
	result, err := page.Eval(`() => {
		const avatar = document.querySelector('[data-e2e="profile-icon"], [class*="avatar"], a[href*="/@"]');
		return avatar !== null;
	}`)
	if err != nil {
		return false, fmt.Errorf("tiktok: detect auth eval: %w", err)
	}

	return result.Bool(), nil
}

// CaptureSession extracts cookies and tokens from an authenticated TikTok page.
func (p *tiktokProvider) CaptureSession(ctx context.Context, page *scout.Page) (*auth.Session, error) {
	cookies, err := page.GetCookies("https://www.tiktok.com")
	if err != nil {
		return nil, fmt.Errorf("tiktok: capture cookies: %w", err)
	}

	pageURL, err := page.URL()
	if err != nil {
		return nil, fmt.Errorf("tiktok: capture url: %w", err)
	}

	tokens := make(map[string]string)

	for _, c := range cookies {
		switch c.Name {
		case "sessionid", "sessionid_ss", "sid_tt", "tt_webid_v2", "tt_csrf_token",
			"passport_csrf_token", "msToken", "odin_tt":
			tokens[c.Name] = c.Value
		}
	}

	return &auth.Session{
		Provider:  "tiktok",
		Version:   "1.0",
		Timestamp: time.Now().UTC(),
		URL:       pageURL,
		Cookies:   cookies,
		Tokens:    tokens,
	}, nil
}

// ValidateSession checks if the session tokens are present.
func (p *tiktokProvider) ValidateSession(_ context.Context, session *auth.Session) error {
	if session == nil {
		return &auth.AuthError{Reason: "nil session"}
	}

	if session.Tokens["sessionid"] == "" && session.Tokens["sid_tt"] == "" {
		return &auth.AuthError{Reason: "missing session tokens (sessionid or sid_tt)"}
	}

	return nil
}

// TikTokMode implements scraper.Mode for TikTok.
type TikTokMode struct{}

func (m *TikTokMode) Name() string        { return "tiktok" }
func (m *TikTokMode) Description() string  { return "TikTok video metadata, comments, profiles, and trending content" }
func (m *TikTokMode) AuthProvider() scraper.AuthProvider { return &tiktokProvider{} }

// ResultTypes for TikTok content.
const (
	ResultVideo   scraper.ResultType = "video"
	ResultSound   scraper.ResultType = "sound"
	ResultHashtag scraper.ResultType = "hashtag"
)

// Scrape extracts TikTok content based on targets.
// Targets can be profile URLs, hashtags, or video URLs.
func (m *TikTokMode) Scrape(ctx context.Context, session scraper.SessionData, opts scraper.ScrapeOptions) (<-chan scraper.Result, error) {
	results := make(chan scraper.Result, 256)

	go func() {
		defer close(results)

		browser, err := scout.New(
			scout.WithHeadless(opts.Headless),
			scout.WithStealth(),
			scout.WithTimeout(0),
		)
		if err != nil {
			return
		}

		defer func() { _ = browser.Close() }()

		var sessionCookies []scout.Cookie
		if session != nil {
			if s, ok := session.(*auth.Session); ok && len(s.Cookies) > 0 {
				sessionCookies = s.Cookies
			}
		}

		targets := opts.Targets
		if len(targets) == 0 {
			targets = []string{"https://www.tiktok.com/foryou"}
		}

		count := 0

		for _, target := range targets {
			if ctx.Err() != nil {
				return
			}

			if opts.Limit > 0 && count >= opts.Limit {
				return
			}

			url := resolveTarget(target)

			page, err := browser.NewPage(url)
			if err != nil {
				continue
			}

			if len(sessionCookies) > 0 {
				_ = page.SetCookies(sessionCookies...)
				_ = page.Navigate(url)
			}

			_ = page.WaitLoad()

			// Wait for content to render.
			time.Sleep(2 * time.Second)

			// Extract video items from the page.
			videos, err := extractVideos(page)
			if err != nil {
				continue
			}

			for _, v := range videos {
				if ctx.Err() != nil {
					return
				}

				if opts.Limit > 0 && count >= opts.Limit {
					return
				}

				select {
				case <-ctx.Done():
					return
				case results <- v:
					count++
				}
			}

			if opts.Progress != nil {
				opts.Progress(scraper.Progress{
					Phase:   "scrape",
					Current: count,
					Message: fmt.Sprintf("extracted %d items from %s", count, target),
				})
			}
		}
	}()

	return results, nil
}

func resolveTarget(target string) string {
	if strings.HasPrefix(target, "http") {
		return target
	}

	if strings.HasPrefix(target, "@") {
		return "https://www.tiktok.com/" + target
	}

	if strings.HasPrefix(target, "#") {
		return "https://www.tiktok.com/tag/" + strings.TrimPrefix(target, "#")
	}

	// Assume it's a username.
	return "https://www.tiktok.com/@" + target
}

func extractVideos(page *scout.Page) ([]scraper.Result, error) {
	raw, err := page.Eval(`() => {
		const items = document.querySelectorAll('[data-e2e="user-post-item"], [class*="DivItemContainerV2"], [class*="video-feed-item"], div[class*="DivVideoCardDesc"]');
		const results = [];
		items.forEach((item, i) => {
			const link = item.querySelector('a[href*="/video/"]') || item.closest('a[href*="/video/"]');
			const desc = item.querySelector('[data-e2e="video-desc"], [class*="desc"], span[class*="SpanText"]');
			const author = item.querySelector('[data-e2e="video-author-uniqueid"], [class*="author"], a[href*="/@"]');
			const stats = item.querySelector('[data-e2e="video-views"], [class*="views"], strong[data-e2e]');

			results.push({
				url: link ? link.href : '',
				description: desc ? desc.textContent.trim() : '',
				author: author ? author.textContent.trim().replace('@', '') : '',
				views: stats ? stats.textContent.trim() : '',
				index: i
			});
		});
		return JSON.stringify(results);
	}`)
	if err != nil {
		return nil, fmt.Errorf("tiktok: extract videos: %w", err)
	}

	var items []struct {
		URL         string `json:"url"`
		Description string `json:"description"`
		Author      string `json:"author"`
		Views       string `json:"views"`
		Index       int    `json:"index"`
	}

	s := raw.String()
	if s == "" || s == "null" {
		return nil, nil
	}

	if err := json.Unmarshal([]byte(s), &items); err != nil {
		return nil, fmt.Errorf("tiktok: parse videos: %w", err)
	}

	var results []scraper.Result

	for _, item := range items {
		if item.URL == "" {
			continue
		}

		results = append(results, scraper.Result{
			Type:      ResultVideo,
			Source:    "tiktok",
			ID:        extractVideoID(item.URL),
			Timestamp: time.Now(),
			Author:    item.Author,
			Content:   item.Description,
			URL:       item.URL,
			Metadata: map[string]any{
				"views": item.Views,
			},
		})
	}

	return results, nil
}

func extractVideoID(url string) string {
	parts := strings.Split(url, "/video/")
	if len(parts) >= 2 {
		id := strings.Split(parts[1], "?")[0]
		return strings.TrimRight(id, "/")
	}

	return url
}
