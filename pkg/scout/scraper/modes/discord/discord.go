// Package discord implements a scraper.Mode for extracting data from Discord
// by intercepting its internal API traffic via CDP session hijacking.
package discord

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/scraper"
	"github.com/inovacc/scout/pkg/scout/scraper/auth"
)

// apiVersionPattern matches Discord API versioned endpoints.
var apiVersionPattern = regexp.MustCompile(`/api/v\d+/`)

// discordProvider implements auth.Provider for Discord browser-based login.
type discordProvider struct{}

func (p *discordProvider) Name() string { return "discord" }

func (p *discordProvider) LoginURL() string { return "https://discord.com/login" }

func (p *discordProvider) DetectAuth(_ context.Context, page *scout.Page) (bool, error) {
	if page == nil {
		return false, fmt.Errorf("discord: detect auth: nil page")
	}

	url, err := page.URL()
	if err != nil {
		return false, fmt.Errorf("discord: detect auth: %w", err)
	}

	return strings.Contains(url, "/channels/"), nil
}

func (p *discordProvider) CaptureSession(ctx context.Context, page *scout.Page) (*auth.Session, error) {
	if page == nil {
		return nil, fmt.Errorf("discord: capture session: nil page")
	}

	cookies, err := page.GetCookies()
	if err != nil {
		return nil, fmt.Errorf("discord: capture session: cookies: %w", err)
	}

	// Extract localStorage values via Eval
	localStorageResult, err := page.Eval("() => { try { return Object.fromEntries(Object.keys(localStorage).map(k => [k, localStorage.getItem(k)])) } catch(e) { return {} } }")
	if err != nil {
		return nil, fmt.Errorf("discord: capture session: local storage: %w", err)
	}

	localStorage := make(map[string]string)

	if localStorageData, ok := localStorageResult.Value.(map[string]any); ok {
		for k, v := range localStorageData {
			if str, ok := v.(string); ok {
				localStorage[k] = str
			}
		}
	}

	// Extract sessionStorage values via Eval
	sessionStorageResult, err := page.Eval("() => { try { return Object.fromEntries(Object.keys(sessionStorage).map(k => [k, sessionStorage.getItem(k)])) } catch(e) { return {} } }")
	if err != nil {
		return nil, fmt.Errorf("discord: capture session: session storage: %w", err)
	}

	sessionStorage := make(map[string]string)

	if sessionStorageData, ok := sessionStorageResult.Value.(map[string]any); ok {
		for k, v := range sessionStorageData {
			if str, ok := v.(string); ok {
				sessionStorage[k] = str
			}
		}
	}

	url, err := page.URL()
	if err != nil {
		return nil, fmt.Errorf("discord: capture session: url: %w", err)
	}

	tokens := make(map[string]string)
	if tok, ok := localStorage["token"]; ok {
		// Discord stores the token with surrounding quotes in localStorage.
		tokens["token"] = strings.Trim(tok, `"`)
	}

	sess := &auth.Session{
		Provider:       "discord",
		Version:        "1",
		Timestamp:      time.Now(),
		URL:            url,
		Cookies:        cookies,
		Tokens:         tokens,
		LocalStorage:   localStorage,
		SessionStorage: sessionStorage,
	}

	// If we have no token the session is unusable.
	if _, ok := tokens["token"]; !ok {
		return nil, fmt.Errorf("discord: capture session: token not found in localStorage")
	}

	_ = ctx // reserved for future network validation

	return sess, nil
}

func (p *discordProvider) ValidateSession(_ context.Context, session *auth.Session) error {
	if session == nil {
		return fmt.Errorf("discord: validate session: nil session")
	}

	if _, ok := session.Tokens["token"]; !ok {
		return &scraper.AuthError{Reason: "discord session missing token"}
	}

	if !session.ExpiresAt.IsZero() && time.Now().After(session.ExpiresAt) {
		return &scraper.AuthError{Reason: "discord session expired"}
	}

	return nil
}

// DiscordMode implements scraper.Mode for Discord.
type DiscordMode struct {
	provider *discordProvider
}

func (m *DiscordMode) Name() string { return "discord" }
func (m *DiscordMode) Description() string {
	return "Discord message and channel scraper via API interception"
}
func (m *DiscordMode) AuthProvider() scraper.AuthProvider { return m.provider }

// Scrape launches a browser with the restored Discord session, navigates to the
// channels view, and intercepts Discord API responses to emit structured results.
func (m *DiscordMode) Scrape(ctx context.Context, session scraper.SessionData, opts scraper.ScrapeOptions) (<-chan scraper.Result, error) {
	authSession, ok := session.(*auth.Session)
	if !ok || authSession == nil {
		return nil, fmt.Errorf("discord: scrape: invalid session type")
	}

	token, ok := authSession.Tokens["token"]
	if !ok || token == "" {
		return nil, &scraper.AuthError{Reason: "discord session missing token"}
	}

	browserOpts := []scout.Option{
		scout.WithHeadless(opts.Headless),
	}
	if opts.Stealth {
		browserOpts = append(browserOpts, scout.WithStealth())
	}

	if opts.Timeout > 0 {
		browserOpts = append(browserOpts, scout.WithTimeout(opts.Timeout))
	}

	browser, err := scout.New(browserOpts...)
	if err != nil {
		return nil, fmt.Errorf("discord: scrape: browser: %w", err)
	}

	// Restore cookies on the Discord domain.
	page, err := browser.NewPage("about:blank")
	if err != nil {
		_ = browser.Close()
		return nil, fmt.Errorf("discord: scrape: new page: %w", err)
	}

	if len(authSession.Cookies) > 0 {
		if err := page.SetCookies(authSession.Cookies...); err != nil {
			_ = browser.Close()
			return nil, fmt.Errorf("discord: scrape: set cookies: %w", err)
		}
	}

	// Inject the token into localStorage before navigating.
	tokenJS := fmt.Sprintf(`localStorage.setItem("token", %q)`, `"`+token+`"`)

	if err := page.Navigate("https://discord.com/app"); err != nil {
		_ = browser.Close()
		return nil, fmt.Errorf("discord: scrape: navigate app: %w", err)
	}

	if _, err := page.EvalOnNewDocument(tokenJS); err != nil {
		_ = browser.Close()
		return nil, fmt.Errorf("discord: scrape: inject token: %w", err)
	}

	// Set up the session hijacker to intercept Discord API calls.
	hijackOpts := []scout.HijackOption{
		scout.WithHijackURLFilter("*/api/*"),
		scout.WithHijackBodyCapture(),
	}

	hijacker, err := page.NewSessionHijacker(hijackOpts...)
	if err != nil {
		_ = browser.Close()
		return nil, fmt.Errorf("discord: scrape: hijacker: %w", err)
	}

	// Navigate to channels overview (DMs).
	target := "https://discord.com/channels/@me"
	if len(opts.Targets) > 0 {
		// If a guild ID is provided as first target, navigate there.
		target = "https://discord.com/channels/" + opts.Targets[0]
	}

	if err := page.Navigate(target); err != nil {
		hijacker.Stop()

		_ = browser.Close()

		return nil, fmt.Errorf("discord: scrape: navigate channels: %w", err)
	}

	if err := page.WaitLoad(); err != nil {
		hijacker.Stop()

		_ = browser.Close()

		return nil, fmt.Errorf("discord: scrape: wait load: %w", err)
	}

	results := make(chan scraper.Result, 256)

	go m.processEvents(ctx, hijacker, browser, results, opts)

	return results, nil
}

// processEvents reads hijacked network events and converts API responses into Result items.
func (m *DiscordMode) processEvents(
	ctx context.Context,
	hijacker *scout.SessionHijacker,
	browser *scout.Browser,
	results chan<- scraper.Result,
	opts scraper.ScrapeOptions,
) {
	defer close(results)
	defer hijacker.Stop()
	defer func() { _ = browser.Close() }()

	count := 0

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-hijacker.Events():
			if !ok {
				return
			}

			if ev.Type != scout.HijackEventResponse || ev.Response == nil {
				continue
			}

			resp := ev.Response
			if resp.Body == "" || resp.Status < 200 || resp.Status >= 300 {
				continue
			}

			if !apiVersionPattern.MatchString(resp.URL) {
				continue
			}

			emitted := m.parseResponse(resp.URL, resp.Body, results, opts)
			count += emitted

			if opts.Limit > 0 && count >= opts.Limit {
				return
			}
		}
	}
}

// parseResponse inspects the API URL path and decodes the JSON body into appropriate Result types.
// It returns the number of results emitted.
func (m *DiscordMode) parseResponse(url, body string, results chan<- scraper.Result, opts scraper.ScrapeOptions) int {
	// Strip the versioned API prefix to get the resource path.
	idx := apiVersionPattern.FindStringIndex(url)
	if idx == nil {
		return 0
	}

	resourcePath := url[idx[1]:]

	// Remove query parameters.
	if qi := strings.IndexByte(resourcePath, '?'); qi >= 0 {
		resourcePath = resourcePath[:qi]
	}

	parts := strings.Split(strings.Trim(resourcePath, "/"), "/")
	if len(parts) == 0 {
		return 0
	}

	switch {
	case containsSegment(parts, "messages"):
		return m.parseMessages(body, url, results, opts)
	case containsSegment(parts, "channels"):
		return m.parseChannels(body, url, results)
	case containsSegment(parts, "members"):
		return m.parseMembers(body, url, results)
	case containsSegment(parts, "threads"):
		return m.parseThreads(body, url, results)
	case containsSegment(parts, "pins"):
		return m.parsePins(body, url, results)
	case endsWith(parts, "users", "@me"):
		return m.parseUser(body, url, results)
	default:
		return 0
	}
}

// Discord API JSON structures (partial, relevant fields only).

type discordMessage struct {
	ID        string        `json:"id"`
	ChannelID string        `json:"channel_id"`
	Content   string        `json:"content"`
	Author    discordAuthor `json:"author"`
	Timestamp string        `json:"timestamp"`
	Type      int           `json:"type"`
}

type discordAuthor struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	Discriminator string `json:"discriminator"`
	GlobalName    string `json:"global_name"`
}

type discordChannel struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    int    `json:"type"`
	GuildID string `json:"guild_id"`
	Topic   string `json:"topic"`
}

type discordMember struct {
	User   discordAuthor `json:"user"`
	Nick   string        `json:"nick"`
	Roles  []string      `json:"roles"`
	Joined string        `json:"joined_at"`
}

type discordThread struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ParentID string `json:"parent_id"`
	GuildID  string `json:"guild_id"`
}

func (m *DiscordMode) parseMessages(body, source string, results chan<- scraper.Result, opts scraper.ScrapeOptions) int {
	// Messages can come as an array or a single object.
	var msgs []discordMessage
	if err := json.Unmarshal([]byte(body), &msgs); err != nil {
		// Try single message.
		var single discordMessage
		if err2 := json.Unmarshal([]byte(body), &single); err2 != nil {
			return 0
		}

		msgs = []discordMessage{single}
	}

	count := 0

	for _, msg := range msgs {
		ts, _ := time.Parse(time.RFC3339, msg.Timestamp)

		results <- scraper.Result{
			Type:      scraper.ResultMessage,
			Source:    msg.ChannelID,
			ID:        msg.ID,
			Timestamp: ts,
			Author:    authorName(msg.Author),
			Content:   msg.Content,
			URL:       source,
			Metadata: map[string]any{
				"channel_id":   msg.ChannelID,
				"author_id":    msg.Author.ID,
				"message_type": msg.Type,
			},
		}

		count++

		if opts.Limit > 0 && count >= opts.Limit {
			break
		}
	}

	return count
}

func (m *DiscordMode) parseChannels(body, source string, results chan<- scraper.Result) int {
	var channels []discordChannel
	if err := json.Unmarshal([]byte(body), &channels); err != nil {
		var single discordChannel
		if err2 := json.Unmarshal([]byte(body), &single); err2 != nil {
			return 0
		}

		channels = []discordChannel{single}
	}

	count := 0

	for _, ch := range channels {
		results <- scraper.Result{
			Type:      scraper.ResultChannel,
			Source:    ch.GuildID,
			ID:        ch.ID,
			Timestamp: time.Now(),
			Content:   ch.Topic,
			URL:       source,
			Metadata: map[string]any{
				"name":         ch.Name,
				"channel_type": ch.Type,
				"guild_id":     ch.GuildID,
			},
		}

		count++
	}

	return count
}

func (m *DiscordMode) parseMembers(body, source string, results chan<- scraper.Result) int {
	var members []discordMember
	if err := json.Unmarshal([]byte(body), &members); err != nil {
		return 0
	}

	count := 0

	for _, member := range members {
		joined, _ := time.Parse(time.RFC3339, member.Joined)

		results <- scraper.Result{
			Type:      scraper.ResultMember,
			Source:    source,
			ID:        member.User.ID,
			Timestamp: joined,
			Author:    authorName(member.User),
			Metadata: map[string]any{
				"nick":  member.Nick,
				"roles": member.Roles,
			},
		}

		count++
	}

	return count
}

func (m *DiscordMode) parseThreads(body, source string, results chan<- scraper.Result) int {
	// Threads can be wrapped in {"threads": [...]} or be a plain array.
	var wrapper struct {
		Threads []discordThread `json:"threads"`
	}
	if err := json.Unmarshal([]byte(body), &wrapper); err == nil && len(wrapper.Threads) > 0 {
		return m.emitThreads(wrapper.Threads, source, results)
	}

	var threads []discordThread
	if err := json.Unmarshal([]byte(body), &threads); err != nil {
		return 0
	}

	return m.emitThreads(threads, source, results)
}

func (m *DiscordMode) emitThreads(threads []discordThread, source string, results chan<- scraper.Result) int {
	count := 0

	for _, t := range threads {
		results <- scraper.Result{
			Type:      scraper.ResultThread,
			Source:    t.GuildID,
			ID:        t.ID,
			Timestamp: time.Now(),
			Content:   t.Name,
			URL:       source,
			Metadata: map[string]any{
				"parent_id": t.ParentID,
				"guild_id":  t.GuildID,
			},
		}

		count++
	}

	return count
}

func (m *DiscordMode) parsePins(body, source string, results chan<- scraper.Result) int {
	var msgs []discordMessage
	if err := json.Unmarshal([]byte(body), &msgs); err != nil {
		return 0
	}

	count := 0

	for _, msg := range msgs {
		ts, _ := time.Parse(time.RFC3339, msg.Timestamp)

		results <- scraper.Result{
			Type:      scraper.ResultPin,
			Source:    msg.ChannelID,
			ID:        msg.ID,
			Timestamp: ts,
			Author:    authorName(msg.Author),
			Content:   msg.Content,
			URL:       source,
			Metadata: map[string]any{
				"channel_id": msg.ChannelID,
				"author_id":  msg.Author.ID,
			},
		}

		count++
	}

	return count
}

func (m *DiscordMode) parseUser(body, source string, results chan<- scraper.Result) int {
	var user discordAuthor
	if err := json.Unmarshal([]byte(body), &user); err != nil {
		return 0
	}

	results <- scraper.Result{
		Type:      scraper.ResultUser,
		Source:    source,
		ID:        user.ID,
		Timestamp: time.Now(),
		Author:    authorName(user),
		Metadata: map[string]any{
			"username":      user.Username,
			"discriminator": user.Discriminator,
			"global_name":   user.GlobalName,
		},
	}

	return 1
}

// authorName returns the best display name for a Discord user.
func authorName(a discordAuthor) string {
	if a.GlobalName != "" {
		return a.GlobalName
	}

	if a.Discriminator != "" && a.Discriminator != "0" {
		return a.Username + "#" + a.Discriminator
	}

	return a.Username
}

// containsSegment checks if any path segment equals the given value.
func containsSegment(parts []string, segment string) bool {
	for _, p := range parts {
		if p == segment {
			return true
		}
	}

	return false
}

// endsWith checks if the path ends with the given segments.
func endsWith(parts []string, segments ...string) bool {
	if len(parts) < len(segments) {
		return false
	}

	tail := parts[len(parts)-len(segments):]
	for i, s := range segments {
		if tail[i] != s {
			return false
		}
	}

	return true
}

func init() {
	scraper.RegisterMode(&DiscordMode{
		provider: &discordProvider{},
	})
}
