// Package sharepoint implements the scraper.Mode interface for SharePoint site extraction.
// It intercepts SharePoint's internal API calls via session hijacking to capture structured
// document, list, site, user, and page data without DOM scraping.
package sharepoint

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

// sharepointProvider implements auth.Provider for SharePoint sites.
type sharepointProvider struct{}

func (p *sharepointProvider) Name() string { return "sharepoint" }

func (p *sharepointProvider) LoginURL() string { return "https://login.microsoftonline.com/" }

func (p *sharepointProvider) DetectAuth(ctx context.Context, page *scout.Page) (bool, error) {
	if page == nil {
		return false, fmt.Errorf("sharepoint: detect auth: nil page")
	}

	result, err := page.Eval(`() => window.location.href`)
	if err != nil {
		return false, fmt.Errorf("sharepoint: detect auth: eval url: %w", err)
	}

	url := result.String()
	if strings.Contains(url, ".sharepoint.com") {
		// Check for SharePoint header element that indicates logged-in state.
		_, err := page.Element("[data-automationid=\"SiteHeader\"]")
		if err == nil {
			return true, nil
		}

		// Fallback: check for the nav menu which is present on authenticated pages.
		_, err = page.Element("#O365_MainLink_NavMenu")
		if err == nil {
			return true, nil
		}

		// If we're on a sharepoint.com domain, consider it detected.
		return true, nil
	}

	return false, nil
}

func (p *sharepointProvider) CaptureSession(ctx context.Context, page *scout.Page) (*auth.Session, error) {
	if page == nil {
		return nil, fmt.Errorf("sharepoint: capture session: nil page")
	}

	cookies, err := page.GetCookies()
	if err != nil {
		return nil, fmt.Errorf("sharepoint: capture session: get cookies: %w", err)
	}

	result, err := page.Eval(`() => window.location.href`)
	if err != nil {
		return nil, fmt.Errorf("sharepoint: capture session: eval url: %w", err)
	}

	currentURL := result.String()

	tokens := make(map[string]string)
	localStorage := make(map[string]string)

	// Extract SharePoint-specific tokens from localStorage.
	lsResult, err := page.Eval(`() => {
		try {
			const keys = ["SPOIDCRL", "FedAuth", "rtFa"];
			const result = {};
			for (const key of keys) {
				const val = localStorage.getItem(key);
				if (val) result[key] = val;
			}
			return JSON.stringify(result);
		} catch(e) {}
		return '{}';
	}`)
	if err == nil {
		raw := lsResult.String()
		if raw != "" && raw != "{}" {
			var lsTokens map[string]string
			if json.Unmarshal([]byte(raw), &lsTokens) == nil {
				for k, v := range lsTokens {
					if v != "" {
						localStorage[k] = v
						tokens[k] = v
					}
				}
			}
		}
	}

	// Try to extract from sessionStorage as well.
	ssResult, err := page.Eval(`() => {
		try {
			const keys = ["FedAuth", "rtFa"];
			const result = {};
			for (const key of keys) {
				const val = sessionStorage.getItem(key);
				if (val) result[key] = val;
			}
			return JSON.stringify(result);
		} catch(e) {}
		return '{}';
	}`)
	if err == nil {
		raw := ssResult.String()
		if raw != "" && raw != "{}" {
			var ssTokens map[string]string
			if json.Unmarshal([]byte(raw), &ssTokens) == nil {
				for k, v := range ssTokens {
					if v != "" && tokens[k] == "" {
						tokens[k] = v
					}
				}
			}
		}
	}

	now := time.Now()

	return &auth.Session{
		Provider:     "sharepoint",
		Version:      "1",
		Timestamp:    now,
		URL:          currentURL,
		Cookies:      cookies,
		Tokens:       tokens,
		LocalStorage: localStorage,
		ExpiresAt:    now.Add(24 * time.Hour),
	}, nil
}

func (p *sharepointProvider) ValidateSession(_ context.Context, session *auth.Session) error {
	if session == nil {
		return fmt.Errorf("sharepoint: validate session: nil session")
	}

	// Check for FedAuth or rtFa tokens which indicate a valid SharePoint session.
	if token, ok := session.Tokens["FedAuth"]; ok && token != "" {
		return nil
	}

	if token, ok := session.Tokens["rtFa"]; ok && token != "" {
		return nil
	}

	// Check cookies as fallback.
	for _, cookie := range session.Cookies {
		if cookie.Name == "FedAuth" || cookie.Name == "rtFa" || cookie.Name == "SPOIDCRL" {
			if cookie.Value != "" {
				return nil
			}
		}
	}

	return &scraper.AuthError{Reason: "no valid sharepoint token (FedAuth/rtFa) found in session"}
}

// SharePointMode implements scraper.Mode for SharePoint sites.
type SharePointMode struct {
	provider sharepointProvider
}

func (m *SharePointMode) Name() string { return "sharepoint" }
func (m *SharePointMode) Description() string {
	return "Scrape SharePoint sites, documents, lists, pages, and users"
}
func (m *SharePointMode) AuthProvider() scraper.AuthProvider { return &m.provider }

// Scrape creates a browser session, restores cookies, navigates to the SharePoint site,
// and intercepts SharePoint API calls to extract structured data.
func (m *SharePointMode) Scrape(ctx context.Context, session scraper.SessionData, opts scraper.ScrapeOptions) (<-chan scraper.Result, error) {
	spSession, ok := session.(*auth.Session)
	if !ok || spSession == nil {
		return nil, fmt.Errorf("sharepoint: scrape: invalid or nil session")
	}

	if err := m.provider.ValidateSession(ctx, spSession); err != nil {
		return nil, fmt.Errorf("sharepoint: scrape: %w", err)
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
		return nil, fmt.Errorf("sharepoint: scrape: create browser: %w", err)
	}

	page, err := browser.NewPage(spSession.URL)
	if err != nil {
		_ = browser.Close()
		return nil, fmt.Errorf("sharepoint: scrape: new page: %w", err)
	}

	if err := page.SetCookies(spSession.Cookies...); err != nil {
		_ = browser.Close()
		return nil, fmt.Errorf("sharepoint: scrape: set cookies: %w", err)
	}

	// Reload to apply cookies.
	if _, err := page.Eval(`() => location.reload()`); err != nil {
		_ = browser.Close()
		return nil, fmt.Errorf("sharepoint: scrape: reload: %w", err)
	}

	if err := page.WaitLoad(); err != nil {
		_ = browser.Close()
		return nil, fmt.Errorf("sharepoint: scrape: wait load: %w", err)
	}

	hijacker, err := page.NewSessionHijacker(
		scout.WithHijackURLFilter("*.sharepoint.com/_api/*"),
		scout.WithHijackURLFilter("*.sharepoint.com/_vti_bin/*"),
		scout.WithHijackURLFilter("*graph.microsoft.com*"),
		scout.WithHijackBodyCapture(),
	)
	if err != nil {
		_ = browser.Close()
		return nil, fmt.Errorf("sharepoint: scrape: create hijacker: %w", err)
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

// buildTargetSet creates a lookup set from target site/library names. An empty set means no filtering.
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

// parseHijackEvent examines a network event and extracts scraper.Result items from SharePoint API responses.
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
	case strings.Contains(url, "_api/web/lists"):
		return parseListsResponse(body, targetSet)
	case strings.Contains(url, "_api/web/GetFileByServerRelativeUrl"):
		return parseFileResponse(body, targetSet)
	case strings.Contains(url, "_api/web/lists("):
		return parseListItemsResponse(body, targetSet)
	case strings.Contains(url, "_api/sitepages/pages"):
		return parsePagesResponse(body, targetSet)
	case strings.Contains(url, "_api/web/siteusers"):
		return parseSiteUsersResponse(body)
	case strings.Contains(url, "_api/web/getsiteusers"):
		return parseSiteUsersResponse(body)
	case strings.Contains(url, "graph.microsoft.com") && strings.Contains(url, "/sites/"):
		return parseGraphSitesResponse(body)
	default:
		return nil
	}
}

// SharePoint API response structures.

type spAPIResponse struct { //nolint:unused
	OData string `json:"@odata.type"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type spListsResponse struct {
	Value []spList `json:"value"`
}

type spList struct {
	ID    string `json:"Id"`
	Title string `json:"Title"`
	URL   string `json:"RootFolder"`
}

func parseListsResponse(body string, targetSet map[string]struct{}) []scraper.Result {
	var resp spListsResponse

	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	results := make([]scraper.Result, 0, len(resp.Value))
	for _, list := range resp.Value {
		if targetSet != nil {
			if _, ok := targetSet[strings.ToLower(list.Title)]; !ok {
				continue
			}
		}

		results = append(results, scraper.Result{
			Type:      scraper.ResultChannel,
			Source:    "sharepoint",
			ID:        list.ID,
			Timestamp: time.Now(),
			Content:   list.Title,
			URL:       list.URL,
			Metadata: map[string]any{
				"title": list.Title,
				"type":  "list",
			},
			Raw: list,
		})
	}

	return results
}

type spFileResponse struct {
	Name              string `json:"Name"`
	ServerRelativeURL string `json:"ServerRelativeUrl"`
	Length            int64  `json:"Length"`
	TimeCreated       string `json:"TimeCreated"`
	TimeLastModified  string `json:"TimeLastModified"`
	ModifiedBy        *struct {
		ID    string `json:"Id"`
		Title string `json:"Title"`
	} `json:"ModifiedBy"`
}

func parseFileResponse(body string, targetSet map[string]struct{}) []scraper.Result { //nolint:unparam
	var resp spFileResponse

	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	if resp.Name == "" {
		return nil
	}

	author := ""
	if resp.ModifiedBy != nil {
		author = resp.ModifiedBy.Title
	}

	ts := parseISO8601(resp.TimeLastModified)
	if ts.IsZero() {
		ts = parseISO8601(resp.TimeCreated)
	}

	return []scraper.Result{
		{
			Type:      scraper.ResultFile,
			Source:    "sharepoint",
			ID:        resp.ServerRelativeURL,
			Timestamp: ts,
			Author:    author,
			Content:   resp.Name,
			URL:       resp.ServerRelativeURL,
			Metadata: map[string]any{
				"size":               resp.Length,
				"time_created":       resp.TimeCreated,
				"time_last_modified": resp.TimeLastModified,
			},
			Raw: resp,
		},
	}
}

type spListItemsResponse struct {
	Value []spListItem `json:"value"`
}

type spListItem struct {
	ID       string `json:"Id"`
	Title    string `json:"Title"`
	Body     string `json:"Body"`
	AuthorID *struct {
		ID string `json:"Id"`
	} `json:"AuthorId"`
	Created  string `json:"Created"`
	Modified string `json:"Modified"`
}

func parseListItemsResponse(body string, targetSet map[string]struct{}) []scraper.Result { //nolint:unparam
	var resp spListItemsResponse

	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	results := make([]scraper.Result, 0, len(resp.Value))
	for _, item := range resp.Value {
		ts := parseISO8601(item.Modified)
		if ts.IsZero() {
			ts = parseISO8601(item.Created)
		}

		results = append(results, scraper.Result{
			Type:      scraper.ResultPost,
			Source:    "sharepoint",
			ID:        item.ID,
			Timestamp: ts,
			Content:   item.Body,
			Metadata: map[string]any{
				"title":    item.Title,
				"created":  item.Created,
				"modified": item.Modified,
			},
			Raw: item,
		})
	}

	return results
}

type spPagesResponse struct {
	Value []spPage `json:"value"`
}

type spPage struct {
	ID                   string `json:"id"`
	Name                 string `json:"name"`
	Title                string `json:"title"`
	Description          string `json:"description"`
	WebURL               string `json:"webUrl"`
	CreatedDateTime      string `json:"createdDateTime"`
	LastModifiedDateTime string `json:"lastModifiedDateTime"`
	CreatedBy            *struct {
		User *struct {
			ID          string `json:"id"`
			DisplayName string `json:"displayName"`
		} `json:"user"`
	} `json:"createdBy"`
}

func parsePagesResponse(body string, targetSet map[string]struct{}) []scraper.Result { //nolint:unparam
	var resp spPagesResponse

	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	results := make([]scraper.Result, 0, len(resp.Value))
	for _, page := range resp.Value {
		ts := parseISO8601(page.LastModifiedDateTime)
		if ts.IsZero() {
			ts = parseISO8601(page.CreatedDateTime)
		}

		author := ""
		if page.CreatedBy != nil && page.CreatedBy.User != nil {
			author = page.CreatedBy.User.DisplayName
		}

		results = append(results, scraper.Result{
			Type:      scraper.ResultPost,
			Source:    "sharepoint",
			ID:        page.ID,
			Timestamp: ts,
			Author:    author,
			Content:   page.Description,
			URL:       page.WebURL,
			Metadata: map[string]any{
				"title":    page.Title,
				"name":     page.Name,
				"created":  page.CreatedDateTime,
				"modified": page.LastModifiedDateTime,
			},
			Raw: page,
		})
	}

	return results
}

type spSiteUsersResponse struct {
	Value []spSiteUser `json:"value"`
}

type spSiteUser struct {
	ID          string `json:"Id"`
	Title       string `json:"Title"`
	LoginName   string `json:"LoginName"`
	Email       string `json:"Email"`
	IsSiteAdmin bool   `json:"IsSiteAdmin"`
}

func parseSiteUsersResponse(body string) []scraper.Result {
	var resp spSiteUsersResponse

	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	results := make([]scraper.Result, 0, len(resp.Value))
	for _, user := range resp.Value {
		results = append(results, scraper.Result{
			Type:   scraper.ResultUser,
			Source: "sharepoint",
			ID:     user.ID,
			Author: user.Title,
			Metadata: map[string]any{
				"login_name": user.LoginName,
				"email":      user.Email,
				"is_admin":   user.IsSiteAdmin,
			},
			Raw: user,
		})
	}

	return results
}

type graphSite struct {
	ID              string `json:"id"`
	DisplayName     string `json:"displayName"`
	WebURL          string `json:"webUrl"`
	CreatedDateTime string `json:"createdDateTime"`
}

type graphSitesResponse struct {
	Value []graphSite `json:"value"`
}

func parseGraphSitesResponse(body string) []scraper.Result {
	var resp graphSitesResponse

	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	results := make([]scraper.Result, 0, len(resp.Value))
	for _, site := range resp.Value {
		ts := parseISO8601(site.CreatedDateTime)

		results = append(results, scraper.Result{
			Type:      scraper.ResultProfile,
			Source:    "sharepoint",
			ID:        site.ID,
			Timestamp: ts,
			Content:   site.DisplayName,
			URL:       site.WebURL,
			Metadata: map[string]any{
				"display_name": site.DisplayName,
				"created":      site.CreatedDateTime,
			},
			Raw: site,
		})
	}

	return results
}

// parseISO8601 converts an ISO 8601 datetime string to time.Time.
func parseISO8601(ts string) time.Time {
	if ts == "" {
		return time.Time{}
	}

	// Try standard ISO 8601 formats.
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.0000000",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, ts); err == nil {
			return t
		}
	}

	return time.Time{}
}

func init() {
	scraper.RegisterMode(&SharePointMode{})
}
