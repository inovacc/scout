// Package teams implements a scraper.Mode for Microsoft Teams.
// It intercepts Teams and Graph API network traffic to extract
// messages, channels, users, files, meetings, and threads.
package teams

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
	scraper.RegisterMode(&TeamsMode{})
}

// teamsProvider implements auth.Provider for Microsoft Teams.
type teamsProvider struct{}

func (p *teamsProvider) Name() string { return "teams" }

func (p *teamsProvider) LoginURL() string { return "https://teams.microsoft.com" }

// DetectAuth checks if the page has landed on the authenticated Teams UI.
func (p *teamsProvider) DetectAuth(ctx context.Context, page *scout.Page) (bool, error) {
	if page == nil {
		return false, fmt.Errorf("teams: detect auth: nil page")
	}

	// Check for the main app layout element that indicates a logged-in state.
	el, err := page.Element(`[data-tid="app-layout"]`)
	if err == nil && el != nil {
		return true, nil
	}

	// Fallback: check if the URL indicates an authenticated Teams session.
	info := page.Info()
	if strings.Contains(info.URL, "teams.microsoft.com/_") {
		return true, nil
	}

	return false, nil
}

// CaptureSession extracts cookies, tokens, and localStorage from an authenticated Teams page.
func (p *teamsProvider) CaptureSession(ctx context.Context, page *scout.Page) (*auth.Session, error) {
	if page == nil {
		return nil, fmt.Errorf("teams: capture session: nil page")
	}

	cookies, err := page.GetCookies()
	if err != nil {
		return nil, fmt.Errorf("teams: capture session: cookies: %w", err)
	}

	tokens := make(map[string]string)
	localStorage := make(map[string]string)

	// Extract known auth-related localStorage keys.
	tokenKeys := []string{
		"ts.latestToken",
		"ts.skypeToken",
		"ts.authToken",
		"ts.chatToken",
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
			short := strings.TrimPrefix(key, "ts.")
			tokens[short] = val
		}
	}

	info := page.Info()

	return &auth.Session{
		Provider:     "teams",
		Version:      "1",
		Timestamp:    time.Now(),
		URL:          info.URL,
		Cookies:      cookies,
		Tokens:       tokens,
		LocalStorage: localStorage,
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}, nil
}

// ValidateSession checks that the session contains a valid Teams auth token.
func (p *teamsProvider) ValidateSession(_ context.Context, session *auth.Session) error {
	if session == nil {
		return fmt.Errorf("teams: validate session: nil session")
	}

	if session.Provider != "teams" {
		return fmt.Errorf("teams: validate session: wrong provider %q", session.Provider)
	}

	// Require at least one of the known auth tokens.
	for _, key := range []string{"skypeToken", "authToken", "latestToken"} {
		if v, ok := session.Tokens[key]; ok && v != "" {
			return nil
		}
	}

	return &scraper.AuthError{Reason: "teams session missing required auth tokens"}
}

// TeamsMode implements scraper.Mode for Microsoft Teams.
type TeamsMode struct{}

func (m *TeamsMode) Name() string { return "teams" }
func (m *TeamsMode) Description() string {
	return "Microsoft Teams messages, channels, users, files, meetings, and threads"
}
func (m *TeamsMode) AuthProvider() scraper.AuthProvider { return &teamsProvider{} }

// Scrape opens an authenticated Teams session, intercepts API traffic, and emits results.
func (m *TeamsMode) Scrape(ctx context.Context, session scraper.SessionData, opts scraper.ScrapeOptions) (<-chan scraper.Result, error) {
	typedSession, ok := session.(*auth.Session)
	if !ok {
		return nil, fmt.Errorf("teams: scrape: invalid session type")
	}

	if typedSession == nil {
		return nil, fmt.Errorf("teams: scrape: nil session")
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
				Source:  "teams",
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

func (m *TeamsMode) run(ctx context.Context, session *auth.Session, opts scraper.ScrapeOptions, results chan<- scraper.Result) error {
	browserOpts := []scout.Option{
		scout.WithHeadless(opts.Headless),
	}

	if opts.Stealth {
		browserOpts = append(browserOpts, scout.WithStealth())
	}

	b, err := scout.New(browserOpts...)
	if err != nil {
		return fmt.Errorf("teams: create browser: %w", err)
	}

	defer func() { _ = b.Close() }()

	page, err := b.NewPage("about:blank")
	if err != nil {
		return fmt.Errorf("teams: new page: %w", err)
	}

	// Restore session cookies.
	if len(session.Cookies) > 0 {
		if err := page.SetCookies(session.Cookies...); err != nil {
			return fmt.Errorf("teams: restore cookies: %w", err)
		}
	}

	// Restore localStorage tokens.
	if len(session.LocalStorage) > 0 {
		for k, v := range session.LocalStorage {
			if _, err := page.Eval(fmt.Sprintf(`() => localStorage.setItem(%q, %q)`, k, v)); err != nil {
				return fmt.Errorf("teams: restore local storage: %w", err)
			}
		}
	}

	// Set up session hijacker to intercept Teams/Graph API calls.
	hijackOpts := []scout.HijackOption{
		scout.WithHijackURLFilter(
			"*/api/csa/*",
			"*/beta/me/chats*",
			"*graph.microsoft.com*",
			"*/api/mt/*",
			"*/api/v1/*",
		),
		scout.WithHijackBodyCapture(),
	}

	hijacker, err := page.NewSessionHijacker(hijackOpts...)
	if err != nil {
		return fmt.Errorf("teams: create hijacker: %w", err)
	}
	defer hijacker.Stop()

	// Navigate to Teams.
	if err := page.Navigate("https://teams.microsoft.com"); err != nil {
		return fmt.Errorf("teams: navigate: %w", err)
	}

	if err := page.WaitLoad(); err != nil {
		return fmt.Errorf("teams: wait load: %w", err)
	}

	// Navigate to specific targets if provided.
	for _, target := range opts.Targets {
		targetURL := target
		if !strings.HasPrefix(target, "http") {
			targetURL = "https://teams.microsoft.com/_#/" + strings.TrimPrefix(target, "/")
		}

		if err := page.Navigate(targetURL); err != nil {
			return fmt.Errorf("teams: navigate target %q: %w", target, err)
		}

		if err := page.WaitLoad(); err != nil {
			return fmt.Errorf("teams: wait load target %q: %w", target, err)
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

// parseResponse attempts to parse a Teams/Graph API JSON response into Result items.
func (m *TeamsMode) parseResponse(resp *scout.CapturedResponse, results chan<- scraper.Result) int {
	url := resp.URL
	count := 0

	var raw map[string]any
	if err := json.Unmarshal([]byte(resp.Body), &raw); err != nil {
		return 0
	}

	switch {
	case strings.Contains(url, "/beta/me/chats") || strings.Contains(url, "/api/csa/api/v1/teams/users/ME/conversations"):
		count += m.parseChats(raw, results)
	case strings.Contains(url, "/messages"):
		count += m.parseMessages(raw, url, results)
	case strings.Contains(url, "/members"):
		count += m.parseMembers(raw, url, results)
	case strings.Contains(url, "/channels"):
		count += m.parseChannels(raw, results)
	case strings.Contains(url, "/files") || strings.Contains(url, "/driveItem"):
		count += m.parseFiles(raw, url, results)
	case strings.Contains(url, "/onlineMeetings") || strings.Contains(url, "/calendarEvents"):
		count += m.parseMeetings(raw, results)
	default:
		// Emit as raw metadata for unrecognized endpoints.
		results <- scraper.Result{
			Type:      scraper.ResultMessage,
			Source:    "teams",
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

func (m *TeamsMode) parseChats(data map[string]any, results chan<- scraper.Result) int {
	items := extractArray(data, "value")
	count := 0

	for _, item := range items {
		chat, ok := item.(map[string]any)
		if !ok {
			continue
		}

		results <- scraper.Result{
			Type:      scraper.ResultThread,
			Source:    "teams",
			ID:        stringVal(chat, "id"),
			Content:   stringVal(chat, "topic"),
			Timestamp: parseTime(stringVal(chat, "lastUpdatedDateTime")),
			Metadata: map[string]any{
				"chat_type": stringVal(chat, "chatType"),
			},
			Raw: chat,
		}

		count++
	}

	return count
}

func (m *TeamsMode) parseMessages(data map[string]any, url string, results chan<- scraper.Result) int {
	items := extractArray(data, "value")
	if items == nil {
		items = extractArray(data, "messages")
	}

	count := 0

	for _, item := range items {
		msg, ok := item.(map[string]any)
		if !ok {
			continue
		}

		author := ""

		if from, ok := msg["from"].(map[string]any); ok {
			if user, ok := from["user"].(map[string]any); ok {
				author = stringVal(user, "displayName")
			}
		}

		content := ""
		if body, ok := msg["body"].(map[string]any); ok {
			content = stringVal(body, "content")
		}

		results <- scraper.Result{
			Type:      scraper.ResultMessage,
			Source:    "teams",
			ID:        stringVal(msg, "id"),
			Author:    author,
			Content:   content,
			URL:       url,
			Timestamp: parseTime(stringVal(msg, "createdDateTime")),
			Metadata: map[string]any{
				"message_type": stringVal(msg, "messageType"),
				"importance":   stringVal(msg, "importance"),
			},
			Raw: msg,
		}

		count++
	}

	return count
}

func (m *TeamsMode) parseMembers(data map[string]any, url string, results chan<- scraper.Result) int {
	items := extractArray(data, "value")
	if items == nil {
		items = extractArray(data, "members")
	}

	count := 0

	for _, item := range items {
		member, ok := item.(map[string]any)
		if !ok {
			continue
		}

		results <- scraper.Result{
			Type:      scraper.ResultUser,
			Source:    "teams",
			ID:        stringVal(member, "id"),
			Author:    stringVal(member, "displayName"),
			Content:   stringVal(member, "email"),
			URL:       url,
			Timestamp: time.Now(),
			Metadata: map[string]any{
				"roles": member["roles"],
			},
			Raw: member,
		}

		count++
	}

	return count
}

func (m *TeamsMode) parseChannels(data map[string]any, results chan<- scraper.Result) int {
	items := extractArray(data, "value")
	if items == nil {
		items = extractArray(data, "channels")
	}

	count := 0

	for _, item := range items {
		ch, ok := item.(map[string]any)
		if !ok {
			continue
		}

		results <- scraper.Result{
			Type:      scraper.ResultChannel,
			Source:    "teams",
			ID:        stringVal(ch, "id"),
			Content:   stringVal(ch, "displayName"),
			URL:       stringVal(ch, "webUrl"),
			Timestamp: parseTime(stringVal(ch, "createdDateTime")),
			Metadata: map[string]any{
				"description":  stringVal(ch, "description"),
				"channel_type": stringVal(ch, "membershipType"),
			},
			Raw: ch,
		}

		count++
	}

	return count
}

func (m *TeamsMode) parseFiles(data map[string]any, url string, results chan<- scraper.Result) int {
	items := extractArray(data, "value")
	count := 0

	for _, item := range items {
		file, ok := item.(map[string]any)
		if !ok {
			continue
		}

		results <- scraper.Result{
			Type:      scraper.ResultFile,
			Source:    "teams",
			ID:        stringVal(file, "id"),
			Content:   stringVal(file, "name"),
			URL:       stringVal(file, "webUrl"),
			Timestamp: parseTime(stringVal(file, "lastModifiedDateTime")),
			Metadata: map[string]any{
				"size":      file["size"],
				"mime_type": stringVal(file, "file.mimeType"),
			},
			Raw: file,
		}

		count++
	}

	if count == 0 && url != "" {
		// Single file response.
		if name := stringVal(data, "name"); name != "" {
			results <- scraper.Result{
				Type:      scraper.ResultFile,
				Source:    "teams",
				ID:        stringVal(data, "id"),
				Content:   name,
				URL:       stringVal(data, "webUrl"),
				Timestamp: parseTime(stringVal(data, "lastModifiedDateTime")),
				Raw:       data,
			}

			count++
		}
	}

	return count
}

func (m *TeamsMode) parseMeetings(data map[string]any, results chan<- scraper.Result) int {
	items := extractArray(data, "value")
	count := 0

	for _, item := range items {
		meeting, ok := item.(map[string]any)
		if !ok {
			continue
		}

		results <- scraper.Result{
			Type:      scraper.ResultMeeting,
			Source:    "teams",
			ID:        stringVal(meeting, "id"),
			Content:   stringVal(meeting, "subject"),
			URL:       stringVal(meeting, "joinWebUrl"),
			Timestamp: parseTime(stringVal(meeting, "startDateTime")),
			Metadata: map[string]any{
				"end_time":  stringVal(meeting, "endDateTime"),
				"organizer": stringVal(meeting, "organizer"),
			},
			Raw: meeting,
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
