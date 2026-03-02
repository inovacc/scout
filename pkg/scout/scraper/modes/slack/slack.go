// Package slack implements the scraper.Mode interface for Slack workspace extraction.
// It intercepts Slack's internal API calls via session hijacking to capture structured
// channel, message, user, and file data without DOM scraping.
package slack

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

// slackProvider implements auth.Provider for Slack workspaces.
type slackProvider struct{}

func (p *slackProvider) Name() string { return "slack" }

func (p *slackProvider) LoginURL() string { return "https://slack.com/signin" }

func (p *slackProvider) DetectAuth(ctx context.Context, page *scout.Page) (bool, error) {
	if page == nil {
		return false, fmt.Errorf("slack: detect auth: nil page")
	}

	result, err := page.Eval(`() => window.location.href`)
	if err != nil {
		return false, fmt.Errorf("slack: detect auth: eval url: %w", err)
	}

	url := result.String()
	if strings.Contains(url, "/client") {
		return true, nil
	}

	// Check for workspace primary view element.
	_, err = page.Element(".p-workspace__primary_view")
	if err == nil {
		return true, nil
	}

	return false, nil
}

func (p *slackProvider) CaptureSession(ctx context.Context, page *scout.Page) (*auth.Session, error) {
	if page == nil {
		return nil, fmt.Errorf("slack: capture session: nil page")
	}

	cookies, err := page.GetCookies()
	if err != nil {
		return nil, fmt.Errorf("slack: capture session: get cookies: %w", err)
	}

	result, err := page.Eval(`() => window.location.href`)
	if err != nil {
		return nil, fmt.Errorf("slack: capture session: eval url: %w", err)
	}
	currentURL := result.String()

	tokens := make(map[string]string)
	localStorage := make(map[string]string)

	// Extract localConfig_v2 which typically contains the API token.
	lsResult, err := page.Eval(`() => {
		try {
			const config = localStorage.getItem('localConfig_v2');
			if (config) return config;
		} catch(e) {}
		return '';
	}`)
	if err == nil {
		raw := lsResult.String()
		if raw != "" {
			localStorage["localConfig_v2"] = raw
			// Parse to extract xoxc-/xoxs- tokens.
			var configMap map[string]any
			if json.Unmarshal([]byte(raw), &configMap) == nil {
				extractTokens(configMap, tokens)
			}
		}
	}

	// Also try extracting token from boot_data or global JS variables.
	tokenResult, err := page.Eval(`() => {
		try {
			if (window.boot_data && window.boot_data.api_token) return window.boot_data.api_token;
			if (window.TS && window.TS.boot_data && window.TS.boot_data.api_token) return window.TS.boot_data.api_token;
		} catch(e) {}
		return '';
	}`)
	if err == nil {
		tok := tokenResult.String()
		if tok != "" && (strings.HasPrefix(tok, "xoxc-") || strings.HasPrefix(tok, "xoxs-")) {
			tokens["api_token"] = tok
		}
	}

	now := time.Now()
	return &auth.Session{
		Provider:     "slack",
		Version:      "1",
		Timestamp:    now,
		URL:          currentURL,
		Cookies:      cookies,
		Tokens:       tokens,
		LocalStorage: localStorage,
		ExpiresAt:    now.Add(24 * time.Hour),
	}, nil
}

func (p *slackProvider) ValidateSession(_ context.Context, session *auth.Session) error {
	if session == nil {
		return fmt.Errorf("slack: validate session: nil session")
	}

	for _, tok := range session.Tokens {
		if strings.HasPrefix(tok, "xoxc-") || strings.HasPrefix(tok, "xoxs-") {
			return nil
		}
	}

	return &scraper.AuthError{Reason: "no valid slack token (xoxc-/xoxs-) found in session"}
}

// extractTokens recursively searches a parsed JSON map for xoxc-/xoxs- token strings.
func extractTokens(m map[string]any, tokens map[string]string) {
	for k, v := range m {
		switch val := v.(type) {
		case string:
			if strings.HasPrefix(val, "xoxc-") || strings.HasPrefix(val, "xoxs-") {
				tokens[k] = val
			}
		case map[string]any:
			extractTokens(val, tokens)
		}
	}
}

// SlackMode implements scraper.Mode for Slack workspaces.
type SlackMode struct {
	provider slackProvider
}

func (m *SlackMode) Name() string { return "slack" }
func (m *SlackMode) Description() string {
	return "Scrape Slack workspace channels, messages, users, and files"
}
func (m *SlackMode) AuthProvider() scraper.AuthProvider { return &m.provider }

// Scrape creates a browser session, restores cookies, navigates to the workspace,
// and intercepts Slack API calls to extract structured data.
func (m *SlackMode) Scrape(ctx context.Context, session scraper.SessionData, opts scraper.ScrapeOptions) (<-chan scraper.Result, error) {
	slackSession, ok := session.(*auth.Session)
	if !ok || slackSession == nil {
		return nil, fmt.Errorf("slack: scrape: invalid or nil session")
	}

	if err := m.provider.ValidateSession(ctx, slackSession); err != nil {
		return nil, fmt.Errorf("slack: scrape: %w", err)
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
		return nil, fmt.Errorf("slack: scrape: create browser: %w", err)
	}

	page, err := browser.NewPage(slackSession.URL)
	if err != nil {
		browser.Close()
		return nil, fmt.Errorf("slack: scrape: new page: %w", err)
	}

	if err := page.SetCookies(slackSession.Cookies...); err != nil {
		browser.Close()
		return nil, fmt.Errorf("slack: scrape: set cookies: %w", err)
	}

	// Reload to apply cookies.
	if _, err := page.Eval(`() => location.reload()`); err != nil {
		browser.Close()
		return nil, fmt.Errorf("slack: scrape: reload: %w", err)
	}

	if err := page.WaitLoad(); err != nil {
		browser.Close()
		return nil, fmt.Errorf("slack: scrape: wait load: %w", err)
	}

	hijacker, err := page.NewSessionHijacker(
		scout.WithHijackURLFilter("*/api/*"),
		scout.WithHijackBodyCapture(),
	)
	if err != nil {
		browser.Close()
		return nil, fmt.Errorf("slack: scrape: create hijacker: %w", err)
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

// buildTargetSet creates a lookup set from target channel names. An empty set means no filtering.
func buildTargetSet(targets []string) map[string]struct{} {
	if len(targets) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(targets))
	for _, t := range targets {
		set[strings.ToLower(strings.TrimPrefix(t, "#"))] = struct{}{}
	}
	return set
}

// parseHijackEvent examines a network event and extracts scraper.Result items from Slack API responses.
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
	case strings.Contains(url, "/api/conversations.list"):
		return parseChannelsList(body, targetSet)
	case strings.Contains(url, "/api/conversations.history"):
		return parseConversationHistory(body, targetSet)
	case strings.Contains(url, "/api/users.list"):
		return parseUsersList(body)
	case strings.Contains(url, "/api/files.list"):
		return parseFilesList(body)
	default:
		return nil
	}
}

// slackAPIResponse is the common envelope for Slack API responses.
type slackAPIResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

type channelsListResponse struct {
	slackAPIResponse
	Channels []slackChannel `json:"channels"`
}

type slackChannel struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Topic      slackTopic  `json:"topic"`
	Purpose    slackTopic  `json:"purpose"`
	NumMembers int         `json:"num_members"`
	Created    json.Number `json:"created"`
}

type slackTopic struct {
	Value string `json:"value"`
}

func parseChannelsList(body string, targetSet map[string]struct{}) []scraper.Result {
	var resp channelsListResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil || !resp.OK {
		return nil
	}

	results := make([]scraper.Result, 0, len(resp.Channels))
	for _, ch := range resp.Channels {
		if targetSet != nil {
			if _, ok := targetSet[strings.ToLower(ch.Name)]; !ok {
				continue
			}
		}

		ts := parseSlackTimestamp(ch.Created.String())
		results = append(results, scraper.Result{
			Type:      scraper.ResultChannel,
			Source:    "slack",
			ID:        ch.ID,
			Timestamp: ts,
			Content:   ch.Topic.Value,
			Metadata: map[string]any{
				"name":        ch.Name,
				"purpose":     ch.Purpose.Value,
				"num_members": ch.NumMembers,
			},
			Raw: ch,
		})
	}
	return results
}

type historyResponse struct {
	slackAPIResponse
	Messages []slackMessage `json:"messages"`
}

type slackMessage struct {
	Type      string          `json:"type"`
	User      string          `json:"user"`
	Text      string          `json:"text"`
	TS        string          `json:"ts"`
	ThreadTS  string          `json:"thread_ts,omitempty"`
	Reactions []slackReaction `json:"reactions,omitempty"`
	Files     []slackFile     `json:"files,omitempty"`
	Channel   string          `json:"channel,omitempty"`
}

type slackReaction struct {
	Name  string   `json:"name"`
	Users []string `json:"users"`
	Count int      `json:"count"`
}

type slackFile struct {
	ID                 string      `json:"id"`
	Name               string      `json:"name"`
	Title              string      `json:"title"`
	Mimetype           string      `json:"mimetype"`
	Size               int64       `json:"size"`
	URLPrivateDownload string      `json:"url_private_download"`
	User               string      `json:"user"`
	Timestamp          json.Number `json:"timestamp"`
}

func parseConversationHistory(body string, targetSet map[string]struct{}) []scraper.Result {
	var resp historyResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil || !resp.OK {
		return nil
	}

	var results []scraper.Result
	for _, msg := range resp.Messages {
		if msg.Type != "message" {
			continue
		}

		ts := parseSlackTimestamp(msg.TS)
		result := scraper.Result{
			Type:      scraper.ResultMessage,
			Source:    msg.Channel,
			ID:        msg.TS,
			Timestamp: ts,
			Author:    msg.User,
			Content:   msg.Text,
			Metadata:  make(map[string]any),
			Raw:       msg,
		}

		if msg.ThreadTS != "" {
			result.Metadata["thread_ts"] = msg.ThreadTS
		}

		results = append(results, result)

		// Emit reactions as separate results.
		for _, r := range msg.Reactions {
			results = append(results, scraper.Result{
				Type:      scraper.ResultReaction,
				Source:    msg.Channel,
				ID:        msg.TS + ":" + r.Name,
				Timestamp: ts,
				Content:   r.Name,
				Metadata: map[string]any{
					"count":      r.Count,
					"users":      r.Users,
					"message_ts": msg.TS,
				},
			})
		}

		// Emit inline files as separate results.
		for _, f := range msg.Files {
			results = append(results, fileToResult(f))
		}
	}

	return results
}

type usersListResponse struct {
	slackAPIResponse
	Members []slackUser `json:"members"`
}

type slackUser struct {
	ID       string           `json:"id"`
	Name     string           `json:"name"`
	RealName string           `json:"real_name"`
	Deleted  bool             `json:"deleted"`
	IsBot    bool             `json:"is_bot"`
	Profile  slackUserProfile `json:"profile"`
}

type slackUserProfile struct {
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Image72     string `json:"image_72"`
}

func parseUsersList(body string) []scraper.Result {
	var resp usersListResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil || !resp.OK {
		return nil
	}

	results := make([]scraper.Result, 0, len(resp.Members))
	for _, u := range resp.Members {
		results = append(results, scraper.Result{
			Type:   scraper.ResultUser,
			Source: "slack",
			ID:     u.ID,
			Author: u.RealName,
			Metadata: map[string]any{
				"name":         u.Name,
				"display_name": u.Profile.DisplayName,
				"email":        u.Profile.Email,
				"is_bot":       u.IsBot,
				"deleted":      u.Deleted,
			},
			Raw: u,
		})
	}
	return results
}

type filesListResponse struct {
	slackAPIResponse
	Files []slackFile `json:"files"`
}

func parseFilesList(body string) []scraper.Result {
	var resp filesListResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil || !resp.OK {
		return nil
	}

	results := make([]scraper.Result, 0, len(resp.Files))
	for _, f := range resp.Files {
		results = append(results, fileToResult(f))
	}
	return results
}

func fileToResult(f slackFile) scraper.Result {
	ts := parseSlackTimestamp(f.Timestamp.String())
	return scraper.Result{
		Type:      scraper.ResultFile,
		Source:    "slack",
		ID:        f.ID,
		Timestamp: ts,
		Author:    f.User,
		Content:   f.Title,
		URL:       f.URLPrivateDownload,
		Metadata: map[string]any{
			"name":     f.Name,
			"mimetype": f.Mimetype,
			"size":     f.Size,
		},
		Raw: f,
	}
}

// parseSlackTimestamp converts a Slack epoch timestamp string (e.g. "1234567890.123456") to time.Time.
func parseSlackTimestamp(ts string) time.Time {
	if ts == "" {
		return time.Time{}
	}

	// Slack timestamps are Unix epoch seconds, possibly with a fractional part.
	parts := strings.SplitN(ts, ".", 2)
	var sec, nsec int64
	if _, err := fmt.Sscanf(parts[0], "%d", &sec); err != nil {
		return time.Time{}
	}
	if len(parts) == 2 {
		// Pad or truncate to nanoseconds (9 digits).
		frac := parts[1]
		for len(frac) < 9 {
			frac += "0"
		}
		frac = frac[:9]
		_, _ = fmt.Sscanf(frac, "%d", &nsec)
	}

	return time.Unix(sec, nsec)
}

func init() {
	scraper.RegisterMode(&SlackMode{})
}
