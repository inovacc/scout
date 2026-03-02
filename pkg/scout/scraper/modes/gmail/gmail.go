// Package gmail implements the scraper.Mode interface for Gmail account extraction.
// It intercepts Gmail's internal API calls via session hijacking to capture structured
// email, label, contact, and profile data without DOM scraping.
package gmail

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

// gmailProvider implements auth.Provider for Gmail accounts.
type gmailProvider struct{}

func (p *gmailProvider) Name() string { return "gmail" }

func (p *gmailProvider) LoginURL() string { return "https://accounts.google.com/" }

func (p *gmailProvider) DetectAuth(ctx context.Context, page *scout.Page) (bool, error) {
	if page == nil {
		return false, fmt.Errorf("gmail: detect auth: nil page")
	}

	result, err := page.Eval(`() => window.location.href`)
	if err != nil {
		return false, fmt.Errorf("gmail: detect auth: eval url: %w", err)
	}

	url := result.String()
	if strings.Contains(url, "mail.google.com") {
		return true, nil
	}

	// Check for Gmail navigation element.
	_, err = page.Element(`div[role="navigation"]`)
	if err == nil {
		return true, nil
	}

	return false, nil
}

func (p *gmailProvider) CaptureSession(ctx context.Context, page *scout.Page) (*auth.Session, error) {
	if page == nil {
		return nil, fmt.Errorf("gmail: capture session: nil page")
	}

	cookies, err := page.GetCookies()
	if err != nil {
		return nil, fmt.Errorf("gmail: capture session: get cookies: %w", err)
	}

	result, err := page.Eval(`() => window.location.href`)
	if err != nil {
		return nil, fmt.Errorf("gmail: capture session: eval url: %w", err)
	}

	currentURL := result.String()

	tokens := make(map[string]string)
	localStorage := make(map[string]string)

	// Extract auth tokens and state from localStorage if available.
	lsResult, err := page.Eval(`() => {
		try {
			const items = {};
			// Capture relevant localStorage keys for Gmail
			const keysToCapture = [
				'gmai_mailbox_data',
				'gmai_userAccountManager',
				'gmai_deviceId',
			];
			for (const key of keysToCapture) {
				const val = localStorage.getItem(key);
				if (val) items[key] = val;
			}
			return items;
		} catch(e) {}
		return {};
	}`)
	if err == nil {
		raw := lsResult.String()
		if raw != "" && raw != "{}" {
			localStorage["captured_keys"] = raw
		}
	}

	// Try extracting SAPISID or other auth tokens.
	tokenResult, err := page.Eval(`() => {
		try {
			// SAPISID is a secure auth token
			const cookies = document.cookie.split(';');
			for (const cookie of cookies) {
				const [name, value] = cookie.trim().split('=');
				if (name === 'SAPISID' || name === 'APISID' || name === 'SID') {
					return value;
				}
			}
		} catch(e) {}
		return '';
	}`)
	if err == nil {
		tok := tokenResult.String()
		if tok != "" {
			tokens["auth_token"] = tok
		}
	}

	now := time.Now()

	return &auth.Session{
		Provider:     "gmail",
		Version:      "1",
		Timestamp:    now,
		URL:          currentURL,
		Cookies:      cookies,
		Tokens:       tokens,
		LocalStorage: localStorage,
		ExpiresAt:    now.Add(24 * time.Hour),
	}, nil
}

func (p *gmailProvider) ValidateSession(_ context.Context, session *auth.Session) error {
	if session == nil {
		return fmt.Errorf("gmail: validate session: nil session")
	}

	// Check for essential Gmail cookies
	for _, cookie := range session.Cookies {
		if cookie.Name == "SSID" || cookie.Name == "SID" || cookie.Name == "HSID" {
			return nil
		}
	}

	return &scraper.AuthError{Reason: "no valid gmail session cookies (SSID/SID/HSID) found"}
}

// GmailMode implements scraper.Mode for Gmail accounts.
type GmailMode struct {
	provider gmailProvider
}

func (m *GmailMode) Name() string { return "gmail" }
func (m *GmailMode) Description() string {
	return "Scrape Gmail emails, labels, contacts, and profile information"
}
func (m *GmailMode) AuthProvider() scraper.AuthProvider { return &m.provider }

// Scrape creates a browser session, restores cookies, navigates to Gmail,
// and intercepts Gmail API calls to extract structured data.
func (m *GmailMode) Scrape(ctx context.Context, session scraper.SessionData, opts scraper.ScrapeOptions) (<-chan scraper.Result, error) {
	gmailSession, ok := session.(*auth.Session)
	if !ok || gmailSession == nil {
		return nil, fmt.Errorf("gmail: scrape: invalid or nil session")
	}

	if err := m.provider.ValidateSession(ctx, gmailSession); err != nil {
		return nil, fmt.Errorf("gmail: scrape: %w", err)
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
		return nil, fmt.Errorf("gmail: scrape: create browser: %w", err)
	}

	page, err := browser.NewPage("https://mail.google.com")
	if err != nil {
		browser.Close()
		return nil, fmt.Errorf("gmail: scrape: new page: %w", err)
	}

	if err := page.SetCookies(gmailSession.Cookies...); err != nil {
		browser.Close()
		return nil, fmt.Errorf("gmail: scrape: set cookies: %w", err)
	}

	// Reload to apply cookies.
	if _, err := page.Eval(`() => location.reload()`); err != nil {
		browser.Close()
		return nil, fmt.Errorf("gmail: scrape: reload: %w", err)
	}

	if err := page.WaitLoad(); err != nil {
		browser.Close()
		return nil, fmt.Errorf("gmail: scrape: wait load: %w", err)
	}

	hijacker, err := page.NewSessionHijacker(
		scout.WithHijackURLFilter("*mail.google.com/mail*"),
		scout.WithHijackURLFilter("*gmail.com*"),
		scout.WithHijackBodyCapture(),
	)
	if err != nil {
		browser.Close()
		return nil, fmt.Errorf("gmail: scrape: create hijacker: %w", err)
	}

	results := make(chan scraper.Result, 256)
	targetSet := buildTargetSet(opts.Targets)

	go func() {
		defer close(results)
		defer hijacker.Stop()
		defer browser.Close()

		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		count := 0

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
								Message: fmt.Sprintf("captured %d items", count),
							})
						}
					}
				}
			}
		}
	}()

	return results, nil
}

// buildTargetSet creates a lookup set from target label names. An empty set means no filtering.
func buildTargetSet(targets []string) map[string]struct{} {
	if len(targets) == 0 {
		return nil
	}

	set := make(map[string]struct{}, len(targets))
	for _, t := range targets {
		set[strings.ToLower(t)] = struct{}{}
	}

	return set
}

// parseHijackEvent examines a network event and extracts scraper.Result items from Gmail API responses.
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
	case strings.Contains(url, "mail/u/0/") && strings.Contains(url, "search"):
		return parseEmailList(body, targetSet)
	case strings.Contains(url, "mail/u/0/") && strings.Contains(url, "get_thread"):
		return parseEmailThread(body, targetSet)
	case strings.Contains(url, "mail/u/0/") && strings.Contains(url, "labels"):
		return parseLabelsList(body)
	case strings.Contains(url, "contacts") || strings.Contains(url, "contact"):
		return parseContactsList(body)
	default:
		return nil
	}
}

// gmailAPIResponse is a common envelope for Gmail API responses.
type gmailAPIResponse struct {
	Response [][]any `json:"response"`
}

type gmailThread struct { //nolint:unused
	ID       string
	Emails   []gmailEmail
	Labels   []string
	Unread   bool
	Archived bool
}

type gmailEmail struct { //nolint:unused
	ID        string
	From      string
	To        string
	Subject   string
	Body      string
	Timestamp int64
	Labels    []string
}

func parseEmailList(body string, targetSet map[string]struct{}) []scraper.Result {
	// Gmail API responses are complex nested arrays. This is a simplified parser.
	var resp gmailAPIResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil || len(resp.Response) == 0 {
		return nil
	}

	var results []scraper.Result

	// Parse email metadata from response arrays
	for _, item := range resp.Response {
		if len(item) < 5 {
			continue
		}

		// Attempt to extract fields from nested arrays
		threadID := fmt.Sprintf("%v", item[0])
		labels := []string{}

		if len(item) > 4 {
			if labelList, ok := item[4].([]any); ok {
				for _, l := range labelList {
					if lStr, ok := l.(string); ok {
						labels = append(labels, lStr)
					}
				}
			}
		}

		// Filter by target labels if specified
		if targetSet != nil {
			found := false

			for _, label := range labels {
				if _, ok := targetSet[strings.ToLower(label)]; ok {
					found = true
					break
				}
			}

			if !found {
				continue
			}
		}

		results = append(results, scraper.Result{
			Type:      scraper.ResultMessage,
			Source:    "gmail",
			ID:        threadID,
			Timestamp: time.Now(),
			Metadata: map[string]any{
				"labels": labels,
			},
			Raw: item,
		})
	}

	return results
}

func parseEmailThread(body string, targetSet map[string]struct{}) []scraper.Result {
	var resp gmailAPIResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	var results []scraper.Result

	for _, item := range resp.Response {
		if len(item) < 2 {
			continue
		}

		// Extract email fields from nested structures
		threadID := fmt.Sprintf("%v", item[0])

		// Attempt to parse the nested email object
		if emailData, ok := item[1].(map[string]any); ok {
			from := extractStringField(emailData, "from")
			subject := extractStringField(emailData, "subject")
			body := extractStringField(emailData, "body")
			timestamp := extractInt64Field(emailData, "timestamp")

			results = append(results, scraper.Result{
				Type:      scraper.ResultEmail,
				Source:    "gmail",
				ID:        threadID,
				Timestamp: time.Unix(timestamp/1000, 0),
				Author:    from,
				Content:   body,
				Metadata: map[string]any{
					"subject":   subject,
					"thread_id": threadID,
				},
				Raw: item,
			})
		}
	}

	return results
}

type gmailLabel struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Count  int    `json:"count"`
	Unread int    `json:"unread"`
	Color  string `json:"color"`
}

func parseLabelsList(body string) []scraper.Result {
	var labels []gmailLabel
	if err := json.Unmarshal([]byte(body), &labels); err != nil {
		return nil
	}

	results := make([]scraper.Result, 0, len(labels))
	for _, lbl := range labels {
		results = append(results, scraper.Result{
			Type:      scraper.ResultChannel,
			Source:    "gmail",
			ID:        lbl.ID,
			Timestamp: time.Now(),
			Content:   lbl.Name,
			Metadata: map[string]any{
				"label_type": lbl.Type,
				"count":      lbl.Count,
				"unread":     lbl.Unread,
				"color":      lbl.Color,
			},
			Raw: lbl,
		})
	}

	return results
}

type gmailContact struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
	Image string `json:"image"`
}

func parseContactsList(body string) []scraper.Result {
	var contacts []gmailContact
	if err := json.Unmarshal([]byte(body), &contacts); err != nil {
		return nil
	}

	results := make([]scraper.Result, 0, len(contacts))
	for _, contact := range contacts {
		results = append(results, scraper.Result{
			Type:      scraper.ResultProfile,
			Source:    "gmail",
			ID:        contact.ID,
			Timestamp: time.Now(),
			Author:    contact.Name,
			Metadata: map[string]any{
				"email": contact.Email,
				"phone": contact.Phone,
				"image": contact.Image,
			},
			Raw: contact,
		})
	}

	return results
}

// extractStringField safely extracts a string value from a map.
func extractStringField(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}

	return ""
}

// extractInt64Field safely extracts an int64 value from a map.
func extractInt64Field(m map[string]any, key string) int64 {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case float64:
			return int64(val)
		case int64:
			return val
		case int:
			return int64(val)
		}
	}

	return 0
}

func init() {
	scraper.RegisterMode(&GmailMode{})
}
