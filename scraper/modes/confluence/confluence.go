// Package confluence implements the scraper.Mode interface for Confluence.
// It intercepts Confluence REST API and GraphQL calls via session hijacking to capture
// structured pages, spaces, comments, attachments, and user data without DOM scraping.
package confluence

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/scraper"
	"github.com/inovacc/scout/scraper/auth"
)

// confluenceProvider implements auth.Provider for Confluence Cloud.
type confluenceProvider struct{}

func (p *confluenceProvider) Name() string { return "confluence" }

func (p *confluenceProvider) LoginURL() string { return "https://id.atlassian.com/login" }

// DetectAuth checks if the page has valid Confluence authentication.
func (p *confluenceProvider) DetectAuth(ctx context.Context, page *scout.Page) (bool, error) {
	if page == nil {
		return false, fmt.Errorf("confluence: detect auth: nil page")
	}

	// Check if URL contains /wiki/ path (Confluence Cloud).
	result, err := page.Eval(`() => window.location.href`)
	if err != nil {
		return false, fmt.Errorf("confluence: detect auth: eval url: %w", err)
	}

	url := result.String()
	if strings.Contains(url, "/wiki/") {
		return true, nil
	}

	// Fallback: check for Confluence app navigation element.
	_, err = page.Element(`[data-testid="app-navigation"]`)
	if err == nil {
		return true, nil
	}

	return false, nil
}

// CaptureSession extracts cookies, tokens, and localStorage from an authenticated Confluence page.
func (p *confluenceProvider) CaptureSession(ctx context.Context, page *scout.Page) (*auth.Session, error) {
	if page == nil {
		return nil, fmt.Errorf("confluence: capture session: nil page")
	}

	cookies, err := page.GetCookies()
	if err != nil {
		return nil, fmt.Errorf("confluence: capture session: get cookies: %w", err)
	}

	result, err := page.Eval(`() => window.location.href`)
	if err != nil {
		return nil, fmt.Errorf("confluence: capture session: eval url: %w", err)
	}
	currentURL := result.String()

	tokens := make(map[string]string)
	localStorage := make(map[string]string)

	// Extract Atlassian cloud session token and XSRF token from cookies.
	for _, cookie := range cookies {
		if cookie.Name == "cloud.session.token" && cookie.Value != "" {
			tokens["cloud.session.token"] = cookie.Value
		}
		if cookie.Name == "atlassian.xsrf.token" && cookie.Value != "" {
			tokens["atlassian.xsrf.token"] = cookie.Value
		}
	}

	// Extract localStorage keys that may contain auth or user info.
	lsKeys := []string{
		"confluence.local.storage.cookie",
		"confluence.auth",
		"confluence.user",
	}

	for _, key := range lsKeys {
		lsResult, err := page.Eval(fmt.Sprintf(`() => {
			try { return localStorage.getItem(%q); } catch(e) { return ''; }
		}`, key))
		if err == nil {
			val := lsResult.String()
			if val != "" {
				localStorage[key] = val
			}
		}
	}

	// Extract user info and space info from window object if available.
	userResult, err := page.Eval(`() => {
		try {
			if (window.AJS && window.AJS.params) {
				return JSON.stringify({
					userId: window.AJS.params.userId,
					userName: window.AJS.params.userName,
				});
			}
		} catch(e) {}
		return '';
	}`)
	if err == nil && userResult.String() != "" {
		tokens["user_info"] = userResult.String()
	}

	now := time.Now()
	return &auth.Session{
		Provider:     "confluence",
		Version:      "1",
		Timestamp:    now,
		URL:          currentURL,
		Cookies:      cookies,
		Tokens:       tokens,
		LocalStorage: localStorage,
		ExpiresAt:    now.Add(24 * time.Hour),
	}, nil
}

// ValidateSession checks that the session contains valid Confluence authentication tokens.
func (p *confluenceProvider) ValidateSession(_ context.Context, session *auth.Session) error {
	if session == nil {
		return fmt.Errorf("confluence: validate session: nil session")
	}

	// Check for Atlassian cloud session token.
	if token, ok := session.Tokens["cloud.session.token"]; ok && token != "" {
		return nil
	}

	// Check cookies as fallback.
	for _, cookie := range session.Cookies {
		if cookie.Name == "cloud.session.token" && cookie.Value != "" {
			return nil
		}
	}

	return &scraper.AuthError{Reason: "no valid confluence session token found"}
}

// ConfluenceMode implements scraper.Mode for Confluence Cloud.
type ConfluenceMode struct {
	provider confluenceProvider
}

func (m *ConfluenceMode) Name() string { return "confluence" }
func (m *ConfluenceMode) Description() string {
	return "Scrape Confluence spaces, pages, comments, attachments, and users"
}
func (m *ConfluenceMode) AuthProvider() scraper.AuthProvider { return &m.provider }

// Scrape creates a browser session, restores cookies, navigates to Confluence,
// and intercepts REST API/GraphQL calls to extract structured data.
func (m *ConfluenceMode) Scrape(ctx context.Context, session scraper.SessionData, opts scraper.ScrapeOptions) (<-chan scraper.Result, error) {
	confluenceSession, ok := session.(*auth.Session)
	if !ok || confluenceSession == nil {
		return nil, fmt.Errorf("confluence: scrape: invalid or nil session")
	}

	if err := m.provider.ValidateSession(ctx, confluenceSession); err != nil {
		return nil, fmt.Errorf("confluence: scrape: %w", err)
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
		return nil, fmt.Errorf("confluence: scrape: create browser: %w", err)
	}

	page, err := browser.NewPage(confluenceSession.URL)
	if err != nil {
		browser.Close()
		return nil, fmt.Errorf("confluence: scrape: new page: %w", err)
	}

	if err := page.SetCookies(confluenceSession.Cookies...); err != nil {
		browser.Close()
		return nil, fmt.Errorf("confluence: scrape: set cookies: %w", err)
	}

	// Reload to apply cookies.
	if _, err := page.Eval(`() => location.reload()`); err != nil {
		browser.Close()
		return nil, fmt.Errorf("confluence: scrape: reload: %w", err)
	}

	if err := page.WaitLoad(); err != nil {
		browser.Close()
		return nil, fmt.Errorf("confluence: scrape: wait load: %w", err)
	}

	// Set up hijacker to intercept Confluence API calls.
	hijacker, err := page.NewSessionHijacker(
		scout.WithHijackURLFilter("*/wiki/rest/api/*"),
		scout.WithHijackURLFilter("*/wiki/api/v2/*"),
		scout.WithHijackURLFilter("*graphql*"),
		scout.WithHijackBodyCapture(),
	)
	if err != nil {
		browser.Close()
		return nil, fmt.Errorf("confluence: scrape: create hijacker: %w", err)
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

// buildTargetSet creates a lookup set from target space keys/page IDs.
// An empty set means no filtering.
func buildTargetSet(targets []string) map[string]struct{} {
	if len(targets) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(targets))
	for _, t := range targets {
		set[strings.ToUpper(strings.TrimSpace(t))] = struct{}{}
	}
	return set
}

// parseHijackEvent examines a network event and extracts scraper.Result items from Confluence API responses.
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
	case strings.Contains(url, "/wiki/rest/api/space"):
		return parseSpaces(body, targetSet)
	case strings.Contains(url, "/wiki/rest/api/content"):
		return parsePages(body, targetSet)
	case strings.Contains(url, "/wiki/api/v2/pages"):
		return parsePagesV2(body, targetSet)
	case strings.Contains(url, "/wiki/rest/api/content") && strings.Contains(url, "/child/comment"):
		return parseComments(body)
	case strings.Contains(url, "/wiki/rest/api/user"):
		return parseUsers(body)
	case strings.Contains(url, "graphql") && strings.Contains(body, "data"):
		return parseGraphQL(body)
	default:
		return nil
	}
}

// Confluence API response structures for REST v1.

type spaceResponse struct {
	Size  int               `json:"size"`
	Start int               `json:"start"`
	Limit int               `json:"limit"`
	Space []confluenceSpace `json:"results"`
}

type confluenceSpace struct {
	ID          int                   `json:"id"`
	Key         string                `json:"key"`
	Name        string                `json:"name"`
	Description confluenceDescription `json:"description"`
	CreatedDate time.Time             `json:"createdDate"`
	Homepage    confluenceLink        `json:"homepage"`
	Icon        confluenceLink        `json:"icon"`
	Type        string                `json:"type"`
}

type confluenceDescription struct {
	Plain confluenceValue `json:"plain"`
	View  confluenceValue `json:"view"`
}

type confluenceValue struct {
	Value string `json:"value"`
}

type confluenceLink struct {
	ID    string            `json:"id,omitempty"`
	Title string            `json:"title,omitempty"`
	Links confluenceLinkRel `json:"_links,omitempty"`
}

type confluenceLinkRel struct {
	Self string `json:"self,omitempty"`
	Web  string `json:"webui,omitempty"`
}

func parseSpaces(body string, targetSet map[string]struct{}) []scraper.Result {
	var resp spaceResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	var results []scraper.Result
	for _, space := range resp.Space {
		if targetSet != nil {
			if _, ok := targetSet[strings.ToUpper(space.Key)]; !ok {
				continue
			}
		}

		results = append(results, scraper.Result{
			Type:      scraper.ResultChannel,
			Source:    "confluence",
			ID:        space.Key,
			Timestamp: space.CreatedDate,
			Content:   space.Description.Plain.Value,
			Metadata: map[string]any{
				"name":       space.Name,
				"space_id":   space.ID,
				"space_type": space.Type,
			},
			Raw: space,
		})
	}
	return results
}

type contentResponse struct {
	Size    int              `json:"size"`
	Start   int              `json:"start"`
	Limit   int              `json:"limit"`
	Results []confluencePage `json:"results"`
}

type confluencePage struct {
	ID      string            `json:"id"`
	Type    string            `json:"type"`
	Title   string            `json:"title"`
	Space   confluenceSpace   `json:"space"`
	Body    confluenceBody    `json:"body"`
	Created time.Time         `json:"created"`
	Updated time.Time         `json:"updated"`
	Version confluenceVersion `json:"version"`
	Links   confluenceLinkRel `json:"_links"`
}

type confluenceBody struct {
	Storage confluenceContent `json:"storage"`
	View    confluenceContent `json:"view"`
}

type confluenceContent struct {
	Value string `json:"value"`
}

type confluenceVersion struct {
	Number int            `json:"number"`
	When   time.Time      `json:"when"`
	By     confluenceUser `json:"by"`
}

type confluenceUser struct {
	Username    string `json:"username"`
	UserKey     string `json:"userKey"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email,omitempty"`
}

func parsePages(body string, targetSet map[string]struct{}) []scraper.Result {
	var resp contentResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	var results []scraper.Result
	for _, page := range resp.Results {
		if targetSet != nil {
			spaceKey := strings.ToUpper(page.Space.Key)
			if _, ok := targetSet[spaceKey]; !ok {
				continue
			}
		}

		// Emit page as a post.
		results = append(results, scraper.Result{
			Type:      scraper.ResultPost,
			Source:    "confluence",
			ID:        page.ID,
			Timestamp: page.Created,
			Author:    page.Version.By.DisplayName,
			Content:   page.Body.Storage.Value,
			URL:       page.Links.Web,
			Metadata: map[string]any{
				"title":       page.Title,
				"space_key":   page.Space.Key,
				"space_name":  page.Space.Name,
				"page_type":   page.Type,
				"version":     page.Version.Number,
				"last_update": page.Updated,
			},
			Raw: page,
		})
	}
	return results
}

// Confluence API v2 structures.

type pageV2Response struct {
	Results []pageV2          `json:"results"`
	Links   map[string]string `json:"_links"`
}

type pageV2 struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`
	Title     string            `json:"title"`
	SpaceID   string            `json:"spaceId"`
	CreatedAt time.Time         `json:"createdAt"`
	UpdatedAt time.Time         `json:"updatedAt"`
	CreatedBy pageV2Author      `json:"createdBy"`
	UpdatedBy pageV2Author      `json:"updatedBy"`
	Body      pageV2Body        `json:"body"`
	Links     map[string]string `json:"_links"`
}

type pageV2Author struct {
	AccountID   string `json:"accountId"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
}

type pageV2Body struct {
	Storage pageV2Content `json:"storage"`
	View    pageV2Content `json:"view"`
}

type pageV2Content struct {
	Value          string `json:"value"`
	Representation string `json:"representation"`
}

func parsePagesV2(body string, targetSet map[string]struct{}) []scraper.Result {
	var resp pageV2Response
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	var results []scraper.Result
	for _, page := range resp.Results {
		results = append(results, scraper.Result{
			Type:      scraper.ResultPost,
			Source:    "confluence",
			ID:        page.ID,
			Timestamp: page.CreatedAt,
			Author:    page.CreatedBy.DisplayName,
			Content:   page.Body.Storage.Value,
			URL:       page.Links["webui"],
			Metadata: map[string]any{
				"title":      page.Title,
				"space_id":   page.SpaceID,
				"page_type":  page.Type,
				"updated_at": page.UpdatedAt,
			},
			Raw: page,
		})
	}
	return results
}

type commentResponse struct {
	Size    int                 `json:"size"`
	Start   int                 `json:"start"`
	Limit   int                 `json:"limit"`
	Results []confluenceComment `json:"results"`
}

type confluenceComment struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`
	Body      confluenceBody    `json:"body"`
	Created   time.Time         `json:"created"`
	Updated   time.Time         `json:"updated"`
	Version   confluenceVersion `json:"version"`
	Container confluencePage    `json:"container"`
}

func parseComments(body string) []scraper.Result {
	var resp commentResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	var results []scraper.Result
	for _, comment := range resp.Results {
		results = append(results, scraper.Result{
			Type:      scraper.ResultComment,
			Source:    "confluence",
			ID:        comment.ID,
			Timestamp: comment.Created,
			Author:    comment.Version.By.DisplayName,
			Content:   comment.Body.Storage.Value,
			Metadata: map[string]any{
				"page_id":     comment.Container.ID,
				"page_title":  comment.Container.Title,
				"version":     comment.Version.Number,
				"last_update": comment.Updated,
			},
			Raw: comment,
		})
	}
	return results
}

type usersResponse struct {
	Size    int                    `json:"size"`
	Start   int                    `json:"start"`
	Limit   int                    `json:"limit"`
	Results []confluenceUserDetail `json:"results"`
}

type confluenceUserDetail struct {
	Username    string    `json:"username"`
	UserKey     string    `json:"userKey"`
	DisplayName string    `json:"displayName"`
	Email       string    `json:"email"`
	Active      bool      `json:"active"`
	Created     time.Time `json:"created"`
}

func parseUsers(body string) []scraper.Result {
	var resp usersResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	var results []scraper.Result
	for _, user := range resp.Results {
		results = append(results, scraper.Result{
			Type:      scraper.ResultUser,
			Source:    "confluence",
			ID:        user.UserKey,
			Author:    user.DisplayName,
			Timestamp: user.Created,
			Metadata: map[string]any{
				"username": user.Username,
				"email":    user.Email,
				"active":   user.Active,
			},
			Raw: user,
		})
	}
	return results
}

// GraphQL response parsing for Confluence mutations/queries.
type graphQLResponse struct {
	Data   map[string]any   `json:"data,omitempty"`
	Errors []map[string]any `json:"errors,omitempty"`
}

func parseGraphQL(body string) []scraper.Result {
	var resp graphQLResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	// Basic GraphQL parsing - emits raw data as a post for analysis.
	if len(resp.Data) > 0 {
		return []scraper.Result{
			{
				Type:      scraper.ResultPost,
				Source:    "confluence",
				ID:        fmt.Sprintf("graphql-%d", time.Now().UnixNano()),
				Timestamp: time.Now(),
				Content:   fmt.Sprintf("%v", resp.Data),
				Metadata: map[string]any{
					"source": "graphql",
				},
				Raw: resp.Data,
			},
		}
	}

	return nil
}

func init() {
	scraper.RegisterMode(&ConfluenceMode{})
}
