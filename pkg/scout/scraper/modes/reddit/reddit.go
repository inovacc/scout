// Package reddit implements the scraper.Mode interface for Reddit.
// It extracts posts, comments, subreddit metadata, and user information
// from public Reddit pages using browser automation.
package reddit

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/scraper"
	"github.com/inovacc/scout/pkg/scout/scraper/auth"
)

func init() {
	scraper.RegisterMode(&RedditMode{})
}

// redditProvider implements auth.Provider for Reddit authentication.
type redditProvider struct{}

func (p *redditProvider) Name() string { return "reddit" }

func (p *redditProvider) LoginURL() string { return "https://www.reddit.com/login" }

// DetectAuth checks whether the current page reflects a logged-in Reddit session.
func (p *redditProvider) DetectAuth(ctx context.Context, page *scout.Page) (bool, error) {
	pageURL, err := page.URL()
	if err != nil {
		return false, fmt.Errorf("reddit: detect auth: %w", err)
	}

	// If we are still on the login page, auth is not complete.
	if strings.Contains(pageURL, "/login") {
		return false, nil
	}

	// Check for the presence of a user-menu element that only renders when logged in.
	result, err := page.Eval(`() => {
		const btn = document.querySelector('#USER_DROPDOWN_ID, [id*="email-collection"], button[aria-label="Open user menu"]');
		return btn !== null;
	}`)
	if err != nil {
		return false, fmt.Errorf("reddit: detect auth eval: %w", err)
	}

	return result.Bool(), nil
}

// CaptureSession extracts cookies and localStorage from an authenticated Reddit page.
func (p *redditProvider) CaptureSession(ctx context.Context, page *scout.Page) (*auth.Session, error) {
	cookies, err := page.GetCookies("https://www.reddit.com")
	if err != nil {
		return nil, fmt.Errorf("reddit: capture cookies: %w", err)
	}

	pageURL, err := page.URL()
	if err != nil {
		return nil, fmt.Errorf("reddit: capture url: %w", err)
	}

	tokens := make(map[string]string)

	for _, c := range cookies {
		switch c.Name {
		case "reddit_session", "token_v2", "edgebucket", "loid":
			tokens[c.Name] = c.Value
		}
	}

	// Attempt to read localStorage for access token.
	lsResult, err := page.Eval(`() => {
		try {
			const obj = {};
			for (let i = 0; i < localStorage.length; i++) {
				const key = localStorage.key(i);
				if (key.includes('token') || key.includes('auth') || key.includes('user')) {
					obj[key] = localStorage.getItem(key);
				}
			}
			return JSON.stringify(obj);
		} catch(e) { return "{}"; }
	}`)

	localStorage := make(map[string]string)

	if err == nil && lsResult != nil {
		raw := lsResult.String()
		// Parse the JSON string manually to populate localStorage.
		raw = strings.Trim(raw, "\"")
		if raw != "{}" && raw != "" {
			// Simple key-value extraction; non-critical if it fails.
			pairs := strings.SplitSeq(strings.Trim(raw, "{}"), ",")
			for pair := range pairs {
				kv := strings.SplitN(pair, ":", 2)
				if len(kv) == 2 {
					k := strings.Trim(kv[0], ` "`)

					v := strings.Trim(kv[1], ` "`)
					if k != "" {
						localStorage[k] = v
					}
				}
			}
		}
	}

	return &auth.Session{
		Provider:     "reddit",
		Version:      "1",
		Timestamp:    time.Now(),
		URL:          pageURL,
		Cookies:      cookies,
		Tokens:       tokens,
		LocalStorage: localStorage,
		ExpiresAt:    time.Now().Add(24 * time.Hour),
	}, nil
}

// ValidateSession checks that a previously captured session has the required tokens.
func (p *redditProvider) ValidateSession(_ context.Context, session *auth.Session) error {
	if session == nil {
		return fmt.Errorf("reddit: validate session: session is nil")
	}

	if session.ExpiresAt.Before(time.Now()) {
		return fmt.Errorf("reddit: validate session: session expired")
	}

	_, hasSession := session.Tokens["reddit_session"]

	_, hasToken := session.Tokens["token_v2"]
	if !hasSession && !hasToken {
		return fmt.Errorf("reddit: validate session: no reddit_session or token_v2 token found")
	}

	return nil
}

// RedditMode implements scraper.Mode for Reddit.
type RedditMode struct {
	provider *redditProvider
}

func (m *RedditMode) Name() string        { return "reddit" }
func (m *RedditMode) Description() string { return "Scrape Reddit posts, comments, and subreddits" }

func (m *RedditMode) AuthProvider() scraper.AuthProvider {
	if m.provider == nil {
		m.provider = &redditProvider{}
	}

	return m.provider
}

// Scrape performs Reddit extraction. If session is nil, only public content is scraped.
// Targets should be subreddit names (e.g. "golang", "programming"). If empty, the front page is scraped.
func (m *RedditMode) Scrape(ctx context.Context, session scraper.SessionData, opts scraper.ScrapeOptions) (<-chan scraper.Result, error) {
	results := make(chan scraper.Result, 64)

	var authSession *auth.Session

	if session != nil {
		var ok bool

		authSession, ok = session.(*auth.Session)
		if !ok {
			close(results)
			return nil, fmt.Errorf("reddit: invalid session type: expected *auth.Session")
		}
	}

	browserOpts := []scout.Option{
		scout.WithHeadless(opts.Headless),
		scout.WithTimeout(opts.Timeout),
	}
	if opts.Stealth {
		browserOpts = append(browserOpts, scout.WithStealth())
	}

	browser, err := scout.New(browserOpts...)
	if err != nil {
		close(results)
		return nil, fmt.Errorf("reddit: create browser: %w", err)
	}

	go func() {
		defer close(results)
		defer func() { _ = browser.Close() }()

		m.scrapeTargets(ctx, browser, authSession, opts, results)
	}()

	return results, nil
}

// scrapeTargets iterates over subreddit targets and extracts content.
func (m *RedditMode) scrapeTargets(ctx context.Context, browser *scout.Browser, session *auth.Session, opts scraper.ScrapeOptions, results chan<- scraper.Result) {
	targets := opts.Targets
	if len(targets) == 0 {
		targets = []string{""} // empty string = front page
	}

	emitted := 0

	for _, target := range targets {
		if ctx.Err() != nil {
			return
		}

		if opts.Limit > 0 && emitted >= opts.Limit {
			return
		}

		n, err := m.scrapeSubreddit(ctx, browser, session, target, opts, results, opts.Limit-emitted)
		if err != nil {
			m.emitProgress(opts.Progress, "error", fmt.Sprintf("failed to scrape r/%s: %v", target, err))
			continue
		}

		emitted += n
	}
}

// scrapeSubreddit navigates to a subreddit and extracts posts from the page.
func (m *RedditMode) scrapeSubreddit(ctx context.Context, browser *scout.Browser, session *auth.Session, subreddit string, opts scraper.ScrapeOptions, results chan<- scraper.Result, remaining int) (int, error) {
	url := "https://www.reddit.com"
	source := "frontpage"

	if subreddit != "" {
		url = fmt.Sprintf("https://www.reddit.com/r/%s/", subreddit)
		source = "r/" + subreddit
	}

	page, err := browser.NewPage(url)
	if err != nil {
		return 0, fmt.Errorf("reddit: new page: %w", err)
	}

	defer func() { _ = page.Close() }()

	// Restore session cookies if available.
	if session != nil && len(session.Cookies) > 0 {
		if err := page.SetCookies(session.Cookies...); err != nil {
			return 0, fmt.Errorf("reddit: set cookies: %w", err)
		}
		// Reload to apply cookies.

		if err := page.Navigate(url); err != nil {
			return 0, fmt.Errorf("reddit: reload with cookies: %w", err)
		}
	}

	if err := page.WaitLoad(); err != nil {
		return 0, fmt.Errorf("reddit: wait load: %w", err)
	}

	// Set up session hijacker for API interception if body capture is enabled.
	if opts.CaptureBody {
		hijacker, err := page.NewSessionHijacker(
			scout.WithHijackURLFilter(
				"*gql.reddit.com*",
				"*gateway.reddit.com*",
				"*oauth.reddit.com/api*",
			),
			scout.WithHijackBodyCapture(),
		)
		if err != nil {
			return 0, fmt.Errorf("reddit: create hijacker: %w", err)
		}

		defer hijacker.Stop()

		// Drain hijack events in background; could be used for richer extraction.
		go func() {
			for range hijacker.Events() {
				// Events are consumed but not processed in this implementation.
				// Future versions can parse API responses for richer data.
			}
		}()
	}

	// Emit subreddit metadata if scraping a specific subreddit.
	if subreddit != "" {
		m.emitSubredditInfo(ctx, page, subreddit, source, results)
	}

	m.emitProgress(opts.Progress, "scraping", fmt.Sprintf("extracting posts from %s", source))

	return m.extractPosts(ctx, page, source, remaining, results)
}

// extractPosts pulls post data from the rendered Reddit page via DOM queries.
func (m *RedditMode) extractPosts(ctx context.Context, page *scout.Page, source string, limit int, results chan<- scraper.Result) (int, error) {
	// Reddit new UI uses shreddit-post or article elements; old Reddit uses .thing.
	// We use JS evaluation for robustness across both.
	postsJS := `() => {
		const posts = [];
		// New Reddit (shreddit)
		document.querySelectorAll('shreddit-post, article[data-testid="post-container"], [data-testid="post-container"]').forEach(el => {
			const titleEl = el.querySelector('a[slot="title"], [data-testid="post-title"], h3');
			const authorEl = el.querySelector('a[data-testid="post_author"], [data-testid="post-author-header"], faceplate-tracker[source="post"] a[href*="/user/"]');
			const scoreEl = el.querySelector('[score], faceplate-number, [data-testid="post-score"]');
			const commentEl = el.querySelector('a[href*="/comments/"], [data-testid="comment-count"]');
			const linkEl = el.querySelector('a[slot="title"], a[data-testid="post-title"], h3 a');

			const title = titleEl ? titleEl.textContent.trim() : '';
			const author = authorEl ? authorEl.textContent.trim().replace(/^u\//, '') : '';
			const score = scoreEl ? (scoreEl.getAttribute('number') || scoreEl.textContent.trim()) : '0';
			const commentCount = commentEl ? commentEl.textContent.trim() : '0';
			const href = linkEl ? linkEl.getAttribute('href') || '' : '';
			const postId = el.getAttribute('id') || el.getAttribute('data-post-id') || href;

			if (title) {
				posts.push({
					id: postId,
					title: title,
					author: author,
					score: score,
					comments: commentCount,
					url: href.startsWith('/') ? 'https://www.reddit.com' + href : href
				});
			}
		});

		// Old Reddit fallback
		if (posts.length === 0) {
			document.querySelectorAll('.thing.link').forEach(el => {
				const titleEl = el.querySelector('a.title');
				const authorEl = el.querySelector('.author');
				const scoreEl = el.querySelector('.score.unvoted');
				const commentEl = el.querySelector('.comments');

				posts.push({
					id: el.getAttribute('data-fullname') || '',
					title: titleEl ? titleEl.textContent.trim() : '',
					author: authorEl ? authorEl.textContent.trim() : '',
					score: scoreEl ? scoreEl.textContent.trim() : '0',
					comments: commentEl ? commentEl.textContent.trim() : '0',
					url: titleEl ? titleEl.getAttribute('href') || '' : ''
				});
			});
		}

		return JSON.stringify(posts);
	}`

	result, err := page.Eval(postsJS)
	if err != nil {
		return 0, fmt.Errorf("reddit: extract posts: %w", err)
	}

	raw := result.String()
	if raw == "" || raw == "null" || raw == "[]" {
		return 0, nil
	}

	// Parse the JSON array of post objects.
	type postData struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		Author   string `json:"author"`
		Score    string `json:"score"`
		Comments string `json:"comments"`
		URL      string `json:"url"`
	}

	var posts []postData

	if err := parseJSONArray(raw, &posts); err != nil {
		return 0, fmt.Errorf("reddit: parse posts: %w", err)
	}

	emitted := 0

	for _, p := range posts {
		if ctx.Err() != nil {
			return emitted, nil //nolint:nilerr
		}

		if limit > 0 && emitted >= limit {
			return emitted, nil
		}

		score, _ := parseScore(p.Score)
		commentCount, _ := parseScore(p.Comments)

		results <- scraper.Result{
			Type:      scraper.ResultPost,
			Source:    source,
			ID:        p.ID,
			Timestamp: time.Now(),
			Author:    p.Author,
			Content:   p.Title,
			URL:       p.URL,
			Metadata: map[string]any{
				"score":         score,
				"comment_count": commentCount,
			},
		}

		emitted++
	}

	return emitted, nil
}

// emitSubredditInfo extracts and emits metadata about a subreddit.
func (m *RedditMode) emitSubredditInfo(ctx context.Context, page *scout.Page, subreddit, source string, results chan<- scraper.Result) {
	if ctx.Err() != nil {
		return
	}

	infoJS := `() => {
		const desc = document.querySelector('[data-testid="subreddit-description"], .md.wiki, #sr-header-area .redditname');
		const members = document.querySelector('[data-testid="members-count"], .subscribers .number, faceplate-number[pretty]');
		return JSON.stringify({
			description: desc ? desc.textContent.trim() : '',
			members: members ? (members.getAttribute('number') || members.textContent.trim()) : '0'
		});
	}`

	result, err := page.Eval(infoJS)
	if err != nil {
		return
	}

	type subredditInfo struct {
		Description string `json:"description"`
		Members     string `json:"members"`
	}

	var info subredditInfo

	if err := parseJSONObject(result.String(), &info); err != nil {
		return
	}

	memberCount, _ := parseScore(info.Members)

	results <- scraper.Result{
		Type:      scraper.ResultSubreddit,
		Source:    source,
		ID:        subreddit,
		Timestamp: time.Now(),
		Content:   info.Description,
		URL:       fmt.Sprintf("https://www.reddit.com/r/%s/", subreddit),
		Metadata: map[string]any{
			"members": memberCount,
		},
	}
}

// emitProgress sends a progress update if a callback is configured.
func (m *RedditMode) emitProgress(fn scraper.ProgressFunc, phase, message string) {
	if fn != nil {
		fn(scraper.Progress{
			Phase:   phase,
			Message: message,
		})
	}
}

// parseScore converts a score string (e.g. "1.2k", "500") to an integer.
func parseScore(s string) (int, error) {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")

	// Remove non-numeric suffixes like " comments", " points".
	for _, suffix := range []string{" comments", " comment", " points", " point"} {
		s = strings.TrimSuffix(s, suffix)
	}

	multiplier := 1
	if strings.HasSuffix(s, "k") || strings.HasSuffix(s, "K") {
		multiplier = 1000
		s = s[:len(s)-1]
	} else if strings.HasSuffix(s, "m") || strings.HasSuffix(s, "M") {
		multiplier = 1_000_000
		s = s[:len(s)-1]
	}

	if s == "" || s == "•" || s == "Vote" {
		return 0, nil
	}

	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("reddit: parse score %q: %w", s, err)
	}

	return int(f * float64(multiplier)), nil
}

// parseJSONArray unmarshals a JSON array string into a typed slice.
func parseJSONArray[T any](raw string, out *[]T) error {
	raw = strings.Trim(raw, "\"")
	raw = strings.ReplaceAll(raw, `\"`, `"`)

	return json.Unmarshal([]byte(raw), out)
}

// parseJSONObject unmarshals a JSON object string into a typed struct.
func parseJSONObject[T any](raw string, out *T) error {
	raw = strings.Trim(raw, "\"")
	raw = strings.ReplaceAll(raw, `\"`, `"`)

	return json.Unmarshal([]byte(raw), out)
}
