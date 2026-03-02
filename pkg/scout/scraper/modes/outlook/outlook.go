// Package outlook implements a scraper.Mode for Microsoft Outlook.
// It intercepts Outlook and Microsoft Graph API network traffic to extract
// emails, folders, contacts, calendar events, and meeting data.
package outlook

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
	scraper.RegisterMode(&OutlookMode{})
}

// outlookProvider implements auth.Provider for Microsoft Outlook.
type outlookProvider struct{}

func (p *outlookProvider) Name() string { return "outlook" }

func (p *outlookProvider) LoginURL() string { return "https://login.microsoftonline.com/" }

// DetectAuth checks if the page has landed on the authenticated Outlook UI.
func (p *outlookProvider) DetectAuth(ctx context.Context, page *scout.Page) (bool, error) {
	if page == nil {
		return false, fmt.Errorf("outlook: detect auth: nil page")
	}

	result, err := page.Eval(`() => window.location.href`)
	if err != nil {
		return false, fmt.Errorf("outlook: detect auth: eval url: %w", err)
	}

	url := result.String()
	// Check for Outlook mail UI URLs.
	if strings.Contains(url, "outlook.live.com/mail") || strings.Contains(url, "outlook.office.com/mail") {
		return true, nil
	}

	// Fallback: check for the main Outlook app container element.
	_, err = page.Element(`[data-app="mail"]`)
	if err == nil {
		return true, nil
	}

	return false, nil
}

// CaptureSession extracts cookies, tokens, and localStorage from an authenticated Outlook page.
func (p *outlookProvider) CaptureSession(ctx context.Context, page *scout.Page) (*auth.Session, error) {
	if page == nil {
		return nil, fmt.Errorf("outlook: capture session: nil page")
	}

	cookies, err := page.GetCookies()
	if err != nil {
		return nil, fmt.Errorf("outlook: capture session: cookies: %w", err)
	}

	tokens := make(map[string]string)
	localStorage := make(map[string]string)

	// Extract known auth-related localStorage keys.
	tokenKeys := []string{
		"o365SessionInfo",
		"authToken",
		"accessToken",
		"refreshToken",
		"OATS.expiration_ms",
	}

	for _, key := range tokenKeys {
		result, err := page.Eval(fmt.Sprintf(`() => localStorage.getItem(%q)`, key))
		if err != nil {
			continue
		}
		val := result.String()
		if val != "" {
			localStorage[key] = val
			// Also populate Tokens map with short names for easy validation.
			short := strings.TrimPrefix(key, "o365")
			if short == key {
				short = strings.TrimPrefix(key, "Token")
			}
			tokens[short] = val
		}
	}

	// Try extracting from sessionStorage as well.
	sessionStorageKeys := []string{
		"msal.account.keys",
		"msal.idtoken",
	}
	sessionStorage := make(map[string]string)

	for _, key := range sessionStorageKeys {
		result, err := page.Eval(fmt.Sprintf(`() => sessionStorage.getItem(%q)`, key))
		if err != nil {
			continue
		}
		val := result.String()
		if val != "" {
			sessionStorage[key] = val
		}
	}

	info := page.Info()

	return &auth.Session{
		Provider:       "outlook",
		Version:        "1",
		Timestamp:      time.Now(),
		URL:            info.URL,
		Cookies:        cookies,
		Tokens:         tokens,
		LocalStorage:   localStorage,
		SessionStorage: sessionStorage,
		ExpiresAt:      time.Now().Add(24 * time.Hour),
	}, nil
}

// ValidateSession checks that the session contains valid Outlook auth tokens.
func (p *outlookProvider) ValidateSession(_ context.Context, session *auth.Session) error {
	if session == nil {
		return fmt.Errorf("outlook: validate session: nil session")
	}

	if session.Provider != "outlook" {
		return fmt.Errorf("outlook: validate session: wrong provider %q", session.Provider)
	}

	// Require at least one of the known auth tokens or localStorage entries.
	if len(session.Tokens) > 0 {
		return nil
	}

	if len(session.LocalStorage) > 0 {
		return nil
	}

	if len(session.Cookies) > 0 {
		// At least some cookies must be present for a valid session.
		return nil
	}

	return &scraper.AuthError{Reason: "outlook session missing required auth tokens or cookies"}
}

// OutlookMode implements scraper.Mode for Microsoft Outlook.
type OutlookMode struct {
	provider outlookProvider
}

func (m *OutlookMode) Name() string { return "outlook" }
func (m *OutlookMode) Description() string {
	return "Outlook emails, folders, contacts, calendar events, and meetings"
}
func (m *OutlookMode) AuthProvider() scraper.AuthProvider { return &m.provider }

// Scrape opens an authenticated Outlook session, intercepts API traffic, and emits results.
func (m *OutlookMode) Scrape(ctx context.Context, session scraper.SessionData, opts scraper.ScrapeOptions) (<-chan scraper.Result, error) {
	typedSession, ok := session.(*auth.Session)
	if !ok {
		return nil, fmt.Errorf("outlook: scrape: invalid session type")
	}
	if typedSession == nil {
		return nil, fmt.Errorf("outlook: scrape: nil session")
	}

	if err := m.provider.ValidateSession(ctx, typedSession); err != nil {
		return nil, fmt.Errorf("outlook: scrape: %w", err)
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 10 * time.Minute
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)

	results := make(chan scraper.Result, 256)

	go func() {
		defer close(results)
		defer cancel()

		if err := m.run(ctx, typedSession, opts, results); err != nil && ctx.Err() == nil {
			results <- scraper.Result{
				Type:    scraper.ResultMessage,
				Source:  "outlook",
				Content: fmt.Sprintf("scrape error: %v", err),
				Metadata: map[string]any{
					"error": true,
				},
				Timestamp: time.Now(),
			}
		}
	}()

	return results, nil
}

func (m *OutlookMode) run(ctx context.Context, session *auth.Session, opts scraper.ScrapeOptions, results chan<- scraper.Result) error {
	browserOpts := []scout.Option{
		scout.WithHeadless(opts.Headless),
	}

	if opts.Stealth {
		browserOpts = append(browserOpts, scout.WithStealth())
	}

	b, err := scout.New(browserOpts...)
	if err != nil {
		return fmt.Errorf("outlook: create browser: %w", err)
	}
	defer b.Close()

	page, err := b.NewPage("about:blank")
	if err != nil {
		return fmt.Errorf("outlook: new page: %w", err)
	}

	// Restore session cookies.
	if len(session.Cookies) > 0 {
		if err := page.SetCookies(session.Cookies...); err != nil {
			return fmt.Errorf("outlook: restore cookies: %w", err)
		}
	}

	// Restore localStorage tokens.
	if len(session.LocalStorage) > 0 {
		for k, v := range session.LocalStorage {
			if _, err := page.Eval(fmt.Sprintf(`() => localStorage.setItem(%q, %q)`, k, v)); err != nil {
				return fmt.Errorf("outlook: restore local storage: %w", err)
			}
		}
	}

	// Restore sessionStorage tokens.
	if len(session.SessionStorage) > 0 {
		for k, v := range session.SessionStorage {
			if _, err := page.Eval(fmt.Sprintf(`() => sessionStorage.setItem(%q, %q)`, k, v)); err != nil {
				return fmt.Errorf("outlook: restore session storage: %w", err)
			}
		}
	}

	// Set up session hijacker to intercept Outlook/Graph API calls.
	hijackOpts := []scout.HijackOption{
		scout.WithHijackURLFilter(
			"*outlook.live.com*",
			"*outlook.office.com*",
			"*outlook.office365.com*",
			"*graph.microsoft.com*",
			"*/api/v2/*",
		),
		scout.WithHijackBodyCapture(),
	}

	hijacker, err := page.NewSessionHijacker(hijackOpts...)
	if err != nil {
		return fmt.Errorf("outlook: create hijacker: %w", err)
	}
	defer hijacker.Stop()

	// Navigate to Outlook mail.
	if err := page.Navigate("https://outlook.live.com/mail/"); err != nil {
		return fmt.Errorf("outlook: navigate: %w", err)
	}

	if err := page.WaitLoad(); err != nil {
		return fmt.Errorf("outlook: wait load: %w", err)
	}

	// Navigate to specific targets if provided.
	for _, target := range opts.Targets {
		targetURL := target
		if !strings.HasPrefix(target, "http") {
			targetURL = "https://outlook.live.com/mail/" + strings.TrimPrefix(target, "/")
		}
		if err := page.Navigate(targetURL); err != nil {
			return fmt.Errorf("outlook: navigate target %q: %w", target, err)
		}
		if err := page.WaitLoad(); err != nil {
			return fmt.Errorf("outlook: wait load target %q: %w", target, err)
		}
	}

	// Process intercepted events until context cancellation or limit reached.
	count := 0
	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-hijacker.Events():
			if !ok {
				return nil
			}
			if ev.Response == nil || ev.Response.Body == "" {
				continue
			}
			emitted := m.parseResponse(ev.Response, results)
			count += emitted
			if opts.Limit > 0 && count >= opts.Limit {
				return nil
			}
			if opts.Progress != nil {
				opts.Progress(scraper.Progress{
					Phase:   "intercepting",
					Current: count,
					Total:   opts.Limit,
					Message: fmt.Sprintf("captured %d items from %s", count, ev.Response.URL),
				})
			}
		}
	}
}

// parseResponse attempts to parse an Outlook/Graph API JSON response into Result items.
func (m *OutlookMode) parseResponse(resp *scout.CapturedResponse, results chan<- scraper.Result) int {
	url := resp.URL
	count := 0

	var raw map[string]any
	if err := json.Unmarshal([]byte(resp.Body), &raw); err != nil {
		return 0
	}

	switch {
	case strings.Contains(url, "/api/v2/me/mailfolders"):
		count += m.parseFolders(raw, results)
	case strings.Contains(url, "/api/v2/me/messages") || strings.Contains(url, "/api/v2/me/mailfolders") && strings.Contains(url, "messages"):
		count += m.parseEmails(raw, url, results)
	case strings.Contains(url, "/api/v2/me/contacts"):
		count += m.parseContacts(raw, results)
	case strings.Contains(url, "/me/events") || strings.Contains(url, "/api/v2/me/calendarview"):
		count += m.parseMeetings(raw, results)
	case strings.Contains(url, "graph.microsoft.com") && strings.Contains(url, "/messages"):
		count += m.parseGraphEmails(raw, url, results)
	case strings.Contains(url, "graph.microsoft.com") && strings.Contains(url, "/events"):
		count += m.parseGraphEvents(raw, results)
	case strings.Contains(url, "graph.microsoft.com") && strings.Contains(url, "/contacts"):
		count += m.parseGraphContacts(raw, results)
	default:
		// Emit as raw metadata for unrecognized endpoints.
		results <- scraper.Result{
			Type:      scraper.ResultMessage,
			Source:    "outlook",
			ID:        resp.RequestID,
			URL:       url,
			Timestamp: resp.Timestamp,
			Metadata:  map[string]any{"raw_endpoint": true},
			Raw:       raw,
		}
		count++
	}

	return count
}

// parseFolders extracts folder/mailbox data from Outlook API responses.
func (m *OutlookMode) parseFolders(data map[string]any, results chan<- scraper.Result) int {
	items := extractArray(data, "value")
	count := 0
	for _, item := range items {
		folder, ok := item.(map[string]any)
		if !ok {
			continue
		}
		results <- scraper.Result{
			Type:      scraper.ResultChannel,
			Source:    "outlook",
			ID:        stringVal(folder, "id"),
			Content:   stringVal(folder, "displayName"),
			Timestamp: parseTime(stringVal(folder, "createdDateTime")),
			Metadata: map[string]any{
				"unread_count": folder["unreadItemCount"],
				"total_count":  folder["totalItemCount"],
				"folder_type":  stringVal(folder, "parentFolderId"),
				"child_count":  folder["childFolderCount"],
			},
			Raw: folder,
		}
		count++
	}
	return count
}

// parseEmails extracts email message data from Outlook API responses.
func (m *OutlookMode) parseEmails(data map[string]any, url string, results chan<- scraper.Result) int {
	items := extractArray(data, "value")
	if items == nil {
		items = extractArray(data, "messages")
	}
	count := 0
	for _, item := range items {
		email, ok := item.(map[string]any)
		if !ok {
			continue
		}

		author := ""
		if from, ok := email["from"].(map[string]any); ok {
			if emailAddr, ok := from["emailAddress"].(map[string]any); ok {
				author = stringVal(emailAddr, "address")
			}
		}

		var toAddrs []string
		if toRecipients, ok := email["toRecipients"].([]any); ok {
			for _, recipient := range toRecipients {
				if recMap, ok := recipient.(map[string]any); ok {
					if emailAddr, ok := recMap["emailAddress"].(map[string]any); ok {
						toAddrs = append(toAddrs, stringVal(emailAddr, "address"))
					}
				}
			}
		}

		subject := stringVal(email, "subject")
		var bodyContent string
		if body, ok := email["body"].(map[string]any); ok {
			bodyContent = stringVal(body, "content")
		}

		results <- scraper.Result{
			Type:      scraper.ResultEmail,
			Source:    "outlook",
			ID:        stringVal(email, "id"),
			Author:    author,
			Content:   subject,
			URL:       url,
			Timestamp: parseTime(stringVal(email, "receivedDateTime")),
			Metadata: map[string]any{
				"body":            bodyContent,
				"to_recipients":   toAddrs,
				"has_attachments": email["hasAttachments"],
				"is_read":         email["isRead"],
				"importance":      stringVal(email, "importance"),
			},
			Raw: email,
		}
		count++
	}
	return count
}

// parseContacts extracts contact/profile data from Outlook API responses.
func (m *OutlookMode) parseContacts(data map[string]any, results chan<- scraper.Result) int {
	items := extractArray(data, "value")
	if items == nil {
		items = extractArray(data, "contacts")
	}
	count := 0
	for _, item := range items {
		contact, ok := item.(map[string]any)
		if !ok {
			continue
		}

		var emails []string
		if emailAddrs, ok := contact["emailAddresses"].([]any); ok {
			for _, e := range emailAddrs {
				if eMap, ok := e.(map[string]any); ok {
					if addr := stringVal(eMap, "address"); addr != "" {
						emails = append(emails, addr)
					}
				}
			}
		}

		results <- scraper.Result{
			Type:      scraper.ResultProfile,
			Source:    "outlook",
			ID:        stringVal(contact, "id"),
			Author:    stringVal(contact, "displayName"),
			Content:   stringVal(contact, "givenName"),
			Timestamp: parseTime(stringVal(contact, "createdDateTime")),
			Metadata: map[string]any{
				"surname":      stringVal(contact, "surname"),
				"emails":       emails,
				"phone_number": stringVal(contact, "mobilePhone"),
				"company":      stringVal(contact, "companyName"),
			},
			Raw: contact,
		}
		count++
	}
	return count
}

// parseMeetings extracts meeting/calendar event data from Outlook API responses.
func (m *OutlookMode) parseMeetings(data map[string]any, results chan<- scraper.Result) int {
	items := extractArray(data, "value")
	count := 0
	for _, item := range items {
		meeting, ok := item.(map[string]any)
		if !ok {
			continue
		}

		var attendees []string
		if attendeesArr, ok := meeting["attendees"].([]any); ok {
			for _, att := range attendeesArr {
				if attMap, ok := att.(map[string]any); ok {
					if emailAddr, ok := attMap["emailAddress"].(map[string]any); ok {
						if addr := stringVal(emailAddr, "address"); addr != "" {
							attendees = append(attendees, addr)
						}
					}
				}
			}
		}

		results <- scraper.Result{
			Type:      scraper.ResultMeeting,
			Source:    "outlook",
			ID:        stringVal(meeting, "id"),
			Content:   stringVal(meeting, "subject"),
			URL:       stringVal(meeting, "webLink"),
			Timestamp: parseTime(stringVal(meeting, "start.dateTime")),
			Metadata: map[string]any{
				"organizer": stringVal(meeting, "organizer.emailAddress.address"),
				"attendees": attendees,
				"end_time":  stringVal(meeting, "end.dateTime"),
				"is_online": meeting["isOnlineMeeting"],
				"location":  stringVal(meeting, "location.displayName"),
			},
			Raw: meeting,
		}
		count++
	}
	return count
}

// parseGraphEmails parses emails from Microsoft Graph API responses.
func (m *OutlookMode) parseGraphEmails(data map[string]any, url string, results chan<- scraper.Result) int {
	items := extractArray(data, "value")
	count := 0
	for _, item := range items {
		email, ok := item.(map[string]any)
		if !ok {
			continue
		}

		author := ""
		if from, ok := email["from"].(map[string]any); ok {
			if emailAddr, ok := from["emailAddress"].(map[string]any); ok {
				author = stringVal(emailAddr, "address")
			}
		}

		subject := stringVal(email, "subject")
		var bodyContent string
		if body, ok := email["bodyPreview"].(string); ok {
			bodyContent = body
		}

		results <- scraper.Result{
			Type:      scraper.ResultEmail,
			Source:    "outlook",
			ID:        stringVal(email, "id"),
			Author:    author,
			Content:   subject,
			URL:       url,
			Timestamp: parseTime(stringVal(email, "receivedDateTime")),
			Metadata: map[string]any{
				"body_preview":    bodyContent,
				"has_attachments": email["hasAttachments"],
				"is_read":         email["isRead"],
			},
			Raw: email,
		}
		count++
	}
	return count
}

// parseGraphEvents parses calendar events from Microsoft Graph API responses.
func (m *OutlookMode) parseGraphEvents(data map[string]any, results chan<- scraper.Result) int {
	items := extractArray(data, "value")
	count := 0
	for _, item := range items {
		event, ok := item.(map[string]any)
		if !ok {
			continue
		}

		results <- scraper.Result{
			Type:      scraper.ResultMeeting,
			Source:    "outlook",
			ID:        stringVal(event, "id"),
			Content:   stringVal(event, "subject"),
			URL:       stringVal(event, "webLink"),
			Timestamp: parseTime(stringVal(event, "start.dateTime")),
			Metadata: map[string]any{
				"organizer":      stringVal(event, "organizer.emailAddress.address"),
				"end_time":       stringVal(event, "end.dateTime"),
				"is_reminder_on": event["isReminderOn"],
			},
			Raw: event,
		}
		count++
	}
	return count
}

// parseGraphContacts parses contacts from Microsoft Graph API responses.
func (m *OutlookMode) parseGraphContacts(data map[string]any, results chan<- scraper.Result) int {
	items := extractArray(data, "value")
	count := 0
	for _, item := range items {
		contact, ok := item.(map[string]any)
		if !ok {
			continue
		}

		results <- scraper.Result{
			Type:      scraper.ResultProfile,
			Source:    "outlook",
			ID:        stringVal(contact, "id"),
			Author:    stringVal(contact, "displayName"),
			Content:   stringVal(contact, "givenName"),
			Timestamp: parseTime(stringVal(contact, "createdDateTime")),
			Metadata: map[string]any{
				"surname":      stringVal(contact, "surname"),
				"mobile_phone": stringVal(contact, "mobilePhone"),
				"company_name": stringVal(contact, "companyName"),
			},
			Raw: contact,
		}
		count++
	}
	return count
}

// extractArray returns the []any for a top-level key, or nil.
func extractArray(data map[string]any, key string) []any {
	val, ok := data[key]
	if !ok {
		return nil
	}
	arr, ok := val.([]any)
	if !ok {
		return nil
	}
	return arr
}

// stringVal returns a string value from a map, or "".
func stringVal(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// parseTime parses an ISO 8601 timestamp, returning zero time on failure.
func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.999Z",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
