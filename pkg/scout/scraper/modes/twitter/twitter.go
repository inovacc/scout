// Package twitter implements the scraper.Mode interface for Twitter/X extraction.
// It intercepts Twitter's internal API calls via session hijacking to capture structured
// tweet, profile, trend, and member data without DOM scraping.
package twitter

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/scraper"
	"github.com/inovacc/scout/pkg/scout/scraper/auth"
)

// twitterProvider implements auth.Provider for Twitter/X.
type twitterProvider struct{}

func (p *twitterProvider) Name() string { return "twitter" }

func (p *twitterProvider) LoginURL() string { return "https://x.com/i/flow/login" }

func (p *twitterProvider) DetectAuth(ctx context.Context, page *scout.Page) (bool, error) {
	if page == nil {
		return false, fmt.Errorf("twitter: detect auth: nil page")
	}

	result, err := page.Eval(`() => window.location.href`)
	if err != nil {
		return false, fmt.Errorf("twitter: detect auth: eval url: %w", err)
	}

	url := result.String()
	if strings.Contains(url, "x.com/home") || strings.Contains(url, "twitter.com/home") {
		return true, nil
	}

	// Check for primary column element which appears when authenticated.
	_, err = page.Element("[data-testid=\"primaryColumn\"]")
	if err == nil {
		return true, nil
	}

	return false, nil
}

func (p *twitterProvider) CaptureSession(ctx context.Context, page *scout.Page) (*auth.Session, error) {
	if page == nil {
		return nil, fmt.Errorf("twitter: capture session: nil page")
	}

	cookies, err := page.GetCookies()
	if err != nil {
		return nil, fmt.Errorf("twitter: capture session: get cookies: %w", err)
	}

	result, err := page.Eval(`() => window.location.href`)
	if err != nil {
		return nil, fmt.Errorf("twitter: capture session: eval url: %w", err)
	}

	currentURL := result.String()

	tokens := make(map[string]string)
	localStorage := make(map[string]string)

	// Extract auth_token, ct0, and twid from cookies.
	for _, cookie := range cookies {
		switch cookie.Name {
		case "auth_token":
			tokens["auth_token"] = cookie.Value
		case "ct0":
			tokens["ct0"] = cookie.Value
		case "twid":
			tokens["twid"] = cookie.Value
		}
	}

	// Try to extract bearer token and user info from localStorage or sessionStorage.
	lsResult, err := page.Eval(`() => {
		try {
			const keys = Object.keys(localStorage);
			const result = {};
			for (const key of keys) {
				result[key] = localStorage.getItem(key);
			}
			return result;
		} catch(e) {}
		return {};
	}`)
	if err == nil {
		raw := lsResult.String()
		if raw != "" && raw != "{}" {
			var lsData map[string]string
			if json.Unmarshal([]byte(raw), &lsData) == nil {
				maps.Copy(localStorage, lsData)
			}
		}
	}

	// Try to extract Twitter app state or user info from global context.
	userResult, err := page.Eval(`() => {
		try {
			if (window.__INITIAL_STATE__ && window.__INITIAL_STATE__.user) {
				return JSON.stringify(window.__INITIAL_STATE__.user);
			}
		} catch(e) {}
		return '';
	}`)
	if err == nil {
		userInfo := userResult.String()
		if userInfo != "" {
			tokens["user_info"] = userInfo
		}
	}

	now := time.Now()

	return &auth.Session{
		Provider:     "twitter",
		Version:      "1",
		Timestamp:    now,
		URL:          currentURL,
		Cookies:      cookies,
		Tokens:       tokens,
		LocalStorage: localStorage,
		ExpiresAt:    now.Add(24 * time.Hour),
	}, nil
}

func (p *twitterProvider) ValidateSession(_ context.Context, session *auth.Session) error {
	if session == nil {
		return fmt.Errorf("twitter: validate session: nil session")
	}

	// Require both auth_token and ct0 for Twitter API access.
	hasAuthToken := false
	hasCT0 := false

	for k, v := range session.Tokens {
		if k == "auth_token" && v != "" {
			hasAuthToken = true
		}

		if k == "ct0" && v != "" {
			hasCT0 = true
		}
	}

	if !hasAuthToken || !hasCT0 {
		return &scraper.AuthError{Reason: "missing auth_token or ct0 tokens in session"}
	}

	return nil
}

// TwitterMode implements scraper.Mode for Twitter/X.
type TwitterMode struct {
	provider twitterProvider
}

func (m *TwitterMode) Name() string { return "twitter" }
func (m *TwitterMode) Description() string {
	return "Scrape Twitter/X tweets, profiles, trends, and members"
}
func (m *TwitterMode) AuthProvider() scraper.AuthProvider { return &m.provider }

// Scrape creates a browser session, restores cookies, navigates to Twitter,
// and intercepts Twitter API calls to extract structured data.
func (m *TwitterMode) Scrape(ctx context.Context, session scraper.SessionData, opts scraper.ScrapeOptions) (<-chan scraper.Result, error) {
	twitterSession, ok := session.(*auth.Session)
	if !ok || twitterSession == nil {
		return nil, fmt.Errorf("twitter: scrape: invalid or nil session")
	}

	if err := m.provider.ValidateSession(ctx, twitterSession); err != nil {
		return nil, fmt.Errorf("twitter: scrape: %w", err)
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
		return nil, fmt.Errorf("twitter: scrape: create browser: %w", err)
	}

	page, err := browser.NewPage(twitterSession.URL)
	if err != nil {
		_ = browser.Close()
		return nil, fmt.Errorf("twitter: scrape: new page: %w", err)
	}

	if err := page.SetCookies(twitterSession.Cookies...); err != nil {
		_ = browser.Close()
		return nil, fmt.Errorf("twitter: scrape: set cookies: %w", err)
	}

	// Reload to apply cookies.
	if _, err := page.Eval(`() => location.reload()`); err != nil {
		_ = browser.Close()
		return nil, fmt.Errorf("twitter: scrape: reload: %w", err)
	}

	if err := page.WaitLoad(); err != nil {
		_ = browser.Close()
		return nil, fmt.Errorf("twitter: scrape: wait load: %w", err)
	}

	hijacker, err := page.NewSessionHijacker(
		scout.WithHijackURLFilter("*api.x.com*"),
		scout.WithHijackURLFilter("*x.com/i/api/*"),
		scout.WithHijackURLFilter("*twitter.com/i/api/*"),
		scout.WithHijackBodyCapture(),
	)
	if err != nil {
		_ = browser.Close()
		return nil, fmt.Errorf("twitter: scrape: create hijacker: %w", err)
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

// buildTargetSet creates a lookup set from target usernames/queries. An empty set means no filtering.
func buildTargetSet(targets []string) map[string]struct{} {
	if len(targets) == 0 {
		return nil
	}

	set := make(map[string]struct{}, len(targets))
	for _, t := range targets {
		set[strings.ToLower(strings.TrimPrefix(t, "@"))] = struct{}{}
	}

	return set
}

// parseHijackEvent examines a network event and extracts scraper.Result items from Twitter API responses.
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
	case strings.Contains(url, "/graphql"):
		return parseGraphQLResponse(body, targetSet)
	case strings.Contains(url, "/search/adaptive.json"):
		return parseSearchResponse(body, targetSet)
	case strings.Contains(url, "/user_by_screen_name/"):
		return parseUserProfileResponse(body)
	case strings.Contains(url, "/followers/list.json"):
		return parseFollowersResponse(body, targetSet)
	case strings.Contains(url, "/home/home_timeline"):
		return parseTimelineResponse(body, targetSet)
	default:
		return nil
	}
}

// twitterAPIResponse is the common envelope for Twitter API responses.
type twitterAPIResponse struct {
	Data   any `json:"data,omitempty"`
	Errors any `json:"errors,omitempty"`
}

// parseGraphQLResponse handles Twitter's GraphQL API responses.
func parseGraphQLResponse(body string, targetSet map[string]struct{}) []scraper.Result {
	var resp twitterAPIResponse

	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	// GraphQL responses contain nested data; we extract tweets and user info where available.
	var results []scraper.Result

	// Try to parse as generic graphQL response containing tweets.
	if data, ok := resp.Data.(map[string]any); ok {
		results = append(results, extractGraphQLTweets(data, targetSet)...)
		results = append(results, extractGraphQLProfiles(data)...)
	}

	return results
}

// extractGraphQLTweets recursively extracts tweet objects from GraphQL response data.
func extractGraphQLTweets(data map[string]any, targetSet map[string]struct{}) []scraper.Result {
	var results []scraper.Result

	// Recursively search for tweet-like objects.
	var search func(any)

	search = func(v any) {
		switch val := v.(type) {
		case map[string]any:
			// Look for tweet-like structures with id_str and full_text.
			if idStr, ok := val["id_str"].(string); ok {
				if fullText, ok := val["full_text"].(string); ok {
					// This looks like a tweet.
					author := ""

					if user, ok := val["user"].(map[string]any); ok {
						if screenName, ok := user["screen_name"].(string); ok {
							author = screenName
						}
					}

					// Check target filter.
					if targetSet != nil && author != "" {
						if _, ok := targetSet[strings.ToLower(author)]; !ok {
							return
						}
					}

					ts := parseTwitterTimestamp(val["created_at"])
					results = append(results, scraper.Result{
						Type:      scraper.ResultPost,
						Source:    "twitter",
						ID:        idStr,
						Timestamp: ts,
						Author:    author,
						Content:   fullText,
						Metadata: map[string]any{
							"retweet_count":  val["retweet_count"],
							"favorite_count": val["favorite_count"],
							"reply_count":    val["reply_count"],
						},
						Raw: val,
					})
				}
			}

			// Recurse into nested objects.
			for _, v := range val {
				search(v)
			}
		case []any:
			// Recurse into array elements.
			for _, v := range val {
				search(v)
			}
		}
	}

	search(data)

	return results
}

// extractGraphQLProfiles extracts profile/user objects from GraphQL response data.
func extractGraphQLProfiles(data map[string]any) []scraper.Result {
	var results []scraper.Result

	// Recursively search for user-like objects.
	var search func(any)

	search = func(v any) {
		switch val := v.(type) {
		case map[string]any:
			// Look for user-like structures with screen_name and followers_count.
			if screenName, ok := val["screen_name"].(string); ok {
				if followersCount, ok := val["followers_count"].(float64); ok {
					// This looks like a user profile.
					description := ""
					if desc, ok := val["description"].(string); ok {
						description = desc
					}

					ts := parseTwitterTimestamp(val["created_at"])
					results = append(results, scraper.Result{
						Type:      scraper.ResultProfile,
						Source:    "twitter",
						ID:        screenName,
						Timestamp: ts,
						Author:    screenName,
						Content:   description,
						Metadata: map[string]any{
							"followers_count": int(followersCount),
							"statuses_count":  val["statuses_count"],
							"verified":        val["verified"],
						},
						Raw: val,
					})
				}
			}

			// Recurse into nested objects.
			for _, v := range val {
				search(v)
			}
		case []any:
			// Recurse into array elements.
			for _, v := range val {
				search(v)
			}
		}
	}

	search(data)

	return results
}

// parseSearchResponse handles Twitter's search/adaptive.json API responses.
func parseSearchResponse(body string, targetSet map[string]struct{}) []scraper.Result {
	var resp struct {
		GlobalObjects struct {
			Tweets map[string]any `json:"tweets"`
			Users  map[string]any `json:"users"`
		} `json:"globalObjects"`
	}

	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	var results []scraper.Result

	// Extract tweets from globalObjects.
	for _, tweetData := range resp.GlobalObjects.Tweets {
		if tweet, ok := tweetData.(map[string]any); ok {
			if result := tweetMapToResult(tweet, targetSet); result != nil {
				results = append(results, *result)
			}
		}
	}

	// Extract users from globalObjects.
	for _, userData := range resp.GlobalObjects.Users {
		if user, ok := userData.(map[string]any); ok {
			if result := userMapToResult(user); result != nil {
				results = append(results, *result)
			}
		}
	}

	return results
}

// tweetMapToResult converts a tweet map to a scraper.Result.
func tweetMapToResult(tweet map[string]any, targetSet map[string]struct{}) *scraper.Result {
	idStr, ok := tweet["id_str"].(string)
	if !ok {
		return nil
	}

	fullText, ok := tweet["full_text"].(string)
	if !ok {
		return nil
	}

	author := ""
	if user, ok := tweet["user"].(string); ok {
		author = user
	} else if userID, ok := tweet["user_id_str"].(string); ok {
		author = userID
	}

	// Check target filter.
	if targetSet != nil && author != "" {
		if _, ok := targetSet[strings.ToLower(author)]; !ok {
			return nil
		}
	}

	ts := parseTwitterTimestamp(tweet["created_at"])

	return &scraper.Result{
		Type:      scraper.ResultPost,
		Source:    "twitter",
		ID:        idStr,
		Timestamp: ts,
		Author:    author,
		Content:   fullText,
		Metadata: map[string]any{
			"retweet_count":  tweet["retweet_count"],
			"favorite_count": tweet["favorite_count"],
			"reply_count":    tweet["reply_count"],
		},
		Raw: tweet,
	}
}

// userMapToResult converts a user map to a scraper.Result.
func userMapToResult(user map[string]any) *scraper.Result {
	screenName, ok := user["screen_name"].(string)
	if !ok {
		return nil
	}

	followersCount, _ := user["followers_count"].(float64)
	description, _ := user["description"].(string)

	ts := parseTwitterTimestamp(user["created_at"])

	return &scraper.Result{
		Type:      scraper.ResultProfile,
		Source:    "twitter",
		ID:        screenName,
		Timestamp: ts,
		Author:    screenName,
		Content:   description,
		Metadata: map[string]any{
			"followers_count": int(followersCount),
			"statuses_count":  user["statuses_count"],
			"verified":        user["verified"],
		},
		Raw: user,
	}
}

// parseUserProfileResponse handles user profile API responses.
func parseUserProfileResponse(body string) []scraper.Result {
	var user map[string]any

	if err := json.Unmarshal([]byte(body), &user); err != nil {
		return nil
	}

	result := userMapToResult(user)
	if result == nil {
		return nil
	}

	return []scraper.Result{*result}
}

// parseFollowersResponse handles followers/list.json API responses.
func parseFollowersResponse(body string, targetSet map[string]struct{}) []scraper.Result { //nolint:unparam
	var resp struct {
		Users []map[string]any `json:"users"`
	}

	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	results := make([]scraper.Result, 0, len(resp.Users))
	for _, user := range resp.Users {
		if result := userMapToResult(user); result != nil {
			results = append(results, *result)
		}
	}

	return results
}

// parseTimelineResponse handles home timeline API responses.
func parseTimelineResponse(body string, targetSet map[string]struct{}) []scraper.Result {
	var resp struct {
		Tweets map[string]any `json:"tweets"`
		Users  map[string]any `json:"users"`
	}

	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	var results []scraper.Result

	// Extract tweets.
	for _, tweetData := range resp.Tweets {
		if tweet, ok := tweetData.(map[string]any); ok {
			if result := tweetMapToResult(tweet, targetSet); result != nil {
				results = append(results, *result)
			}
		}
	}

	// Extract users.
	for _, userData := range resp.Users {
		if user, ok := userData.(map[string]any); ok {
			if result := userMapToResult(user); result != nil {
				results = append(results, *result)
			}
		}
	}

	return results
}

// parseTwitterTimestamp converts a Twitter timestamp string (e.g. "Mon Jan 01 12:34:56 +0000 2024")
// to time.Time. Twitter uses multiple timestamp formats depending on the API endpoint.
func parseTwitterTimestamp(ts any) time.Time {
	if ts == nil {
		return time.Time{}
	}

	tsStr, ok := ts.(string)
	if !ok {
		return time.Time{}
	}

	if tsStr == "" {
		return time.Time{}
	}

	// Try parsing Twitter's standard format: "Mon Jan 01 12:34:56 +0000 2024"
	t, err := time.Parse("Mon Jan 02 15:04:05 -0700 2006", tsStr)
	if err == nil {
		return t
	}

	// Try ISO format.
	t, err = time.Parse(time.RFC3339, tsStr)
	if err == nil {
		return t
	}

	// Try Unix timestamp as string.
	var sec int64
	if _, err := fmt.Sscanf(tsStr, "%d", &sec); err == nil {
		return time.Unix(sec, 0)
	}

	return time.Time{}
}

func init() {
	scraper.RegisterMode(&TwitterMode{})
}
