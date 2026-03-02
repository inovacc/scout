// Package notion implements the scraper.Mode interface for Notion workspace extraction.
// It intercepts Notion's internal API calls via session hijacking to capture structured
// page, database, block, comment, and user data without DOM scraping.
package notion

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

// notionProvider implements auth.Provider for Notion workspaces.
type notionProvider struct{}

func (p *notionProvider) Name() string { return "notion" }

func (p *notionProvider) LoginURL() string { return "https://www.notion.so/login" }

func (p *notionProvider) DetectAuth(ctx context.Context, page *scout.Page) (bool, error) {
	if page == nil {
		return false, fmt.Errorf("notion: detect auth: nil page")
	}

	result, err := page.Eval(`() => window.location.href`)
	if err != nil {
		return false, fmt.Errorf("notion: detect auth: eval url: %w", err)
	}

	url := result.String()
	if strings.Contains(url, "/login") {
		return false, nil
	}

	// Check for Notion sidebar element indicating authenticated state.
	_, err = page.Element(`.notion-sidebar`)
	if err == nil {
		return true, nil
	}

	// Fallback: check for any element with sidebar in class name.
	_, err = page.Element(`[class*="sidebar"]`)
	if err == nil {
		return true, nil
	}

	return false, nil
}

func (p *notionProvider) CaptureSession(ctx context.Context, page *scout.Page) (*auth.Session, error) {
	if page == nil {
		return nil, fmt.Errorf("notion: capture session: nil page")
	}

	cookies, err := page.GetCookies()
	if err != nil {
		return nil, fmt.Errorf("notion: capture session: get cookies: %w", err)
	}

	result, err := page.Eval(`() => window.location.href`)
	if err != nil {
		return nil, fmt.Errorf("notion: capture session: eval url: %w", err)
	}

	currentURL := result.String()

	tokens := make(map[string]string)
	localStorage := make(map[string]string)

	// Extract token_v2 from cookies or localStorage.
	for _, cookie := range cookies {
		if cookie.Name == "token_v2" {
			tokens["token_v2"] = cookie.Value
		}

		if cookie.Name == "notion_user_id" {
			tokens["notion_user_id"] = cookie.Value
		}
	}

	// Also try to get token_v2 from localStorage.
	lsTokenResult, err := page.Eval(`() => {
		try {
			const token = localStorage.getItem('token_v2');
			if (token) return token;
		} catch(e) {}
		return '';
	}`)
	if err == nil {
		tok := lsTokenResult.String()
		if tok != "" && strings.Contains(tok, "_") {
			tokens["token_v2_ls"] = tok
		}
	}

	// Extract notion_user_id from localStorage.
	userIDResult, err := page.Eval(`() => {
		try {
			const userID = localStorage.getItem('notion_user_id');
			if (userID) return userID;
		} catch(e) {}
		return '';
	}`)
	if err == nil {
		uid := userIDResult.String()
		if uid != "" {
			tokens["notion_user_id_ls"] = uid
		}
	}

	// Capture workspace name from localStorage or page.
	wsNameResult, err := page.Eval(`() => {
		try {
			const ws = localStorage.getItem('workspace_name');
			if (ws) return ws;
			const uuid = localStorage.getItem('workspace_uuid');
			if (uuid) return uuid;
		} catch(e) {}
		return '';
	}`)
	if err == nil {
		wsName := wsNameResult.String()
		if wsName != "" {
			localStorage["workspace_name"] = wsName
		}
	}

	now := time.Now()

	return &auth.Session{
		Provider:     "notion",
		Version:      "1",
		Timestamp:    now,
		URL:          currentURL,
		Cookies:      cookies,
		Tokens:       tokens,
		LocalStorage: localStorage,
		ExpiresAt:    now.Add(24 * time.Hour),
	}, nil
}

func (p *notionProvider) ValidateSession(_ context.Context, session *auth.Session) error {
	if session == nil {
		return fmt.Errorf("notion: validate session: nil session")
	}

	// Check for token_v2 in tokens or cookies.
	if tok, ok := session.Tokens["token_v2"]; ok && tok != "" {
		return nil
	}

	if tok, ok := session.Tokens["token_v2_ls"]; ok && tok != "" {
		return nil
	}

	// Check cookies for token_v2.
	for _, cookie := range session.Cookies {
		if cookie.Name == "token_v2" && cookie.Value != "" {
			return nil
		}
	}

	return &scraper.AuthError{Reason: "no valid notion token_v2 found in session"}
}

// NotionMode implements scraper.Mode for Notion workspaces.
type NotionMode struct {
	provider notionProvider
}

func (m *NotionMode) Name() string { return "notion" }
func (m *NotionMode) Description() string {
	return "Scrape Notion workspace pages, databases, blocks, comments, and users"
}
func (m *NotionMode) AuthProvider() scraper.AuthProvider { return &m.provider }

// Scrape creates a browser session, restores cookies, navigates to notion.so,
// and intercepts Notion API calls to extract structured data.
func (m *NotionMode) Scrape(ctx context.Context, session scraper.SessionData, opts scraper.ScrapeOptions) (<-chan scraper.Result, error) {
	notionSession, ok := session.(*auth.Session)
	if !ok || notionSession == nil {
		return nil, fmt.Errorf("notion: scrape: invalid or nil session")
	}

	if err := m.provider.ValidateSession(ctx, notionSession); err != nil {
		return nil, fmt.Errorf("notion: scrape: %w", err)
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
		return nil, fmt.Errorf("notion: scrape: create browser: %w", err)
	}

	page, err := browser.NewPage("https://www.notion.so")
	if err != nil {
		_ = browser.Close()
		return nil, fmt.Errorf("notion: scrape: new page: %w", err)
	}

	if err := page.SetCookies(notionSession.Cookies...); err != nil {
		_ = browser.Close()
		return nil, fmt.Errorf("notion: scrape: set cookies: %w", err)
	}

	// Reload to apply cookies.
	if _, err := page.Eval(`() => location.reload()`); err != nil {
		_ = browser.Close()
		return nil, fmt.Errorf("notion: scrape: reload: %w", err)
	}

	if err := page.WaitLoad(); err != nil {
		_ = browser.Close()
		return nil, fmt.Errorf("notion: scrape: wait load: %w", err)
	}

	hijacker, err := page.NewSessionHijacker(
		scout.WithHijackURLFilter("*notion.so/api/v3/*"),
		scout.WithHijackURLFilter("*msgstore.www.notion.so*"),
		scout.WithHijackBodyCapture(),
	)
	if err != nil {
		_ = browser.Close()
		return nil, fmt.Errorf("notion: scrape: create hijacker: %w", err)
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

// buildTargetSet creates a lookup set from target page IDs or workspace names.
// An empty set means no filtering.
func buildTargetSet(targets []string) map[string]struct{} {
	if len(targets) == 0 {
		return nil
	}

	set := make(map[string]struct{}, len(targets))
	for _, t := range targets {
		set[strings.ToLower(strings.TrimSpace(t))] = struct{}{}
	}

	return set
}

// parseHijackEvent examines a network event and extracts scraper.Result items from Notion API responses.
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
	case strings.Contains(url, "/getPageAsNested"):
		return parseGetPageAsNested(body, targetSet)
	case strings.Contains(url, "/queryCollection"):
		return parseQueryCollection(body, targetSet)
	case strings.Contains(url, "/getRecordValues"):
		return parseGetRecordValues(body, targetSet)
	case strings.Contains(url, "/loadPageChunk"):
		return parseLoadPageChunk(body, targetSet)
	case strings.Contains(url, "/queryCollectionPages"):
		return parseQueryCollectionPages(body, targetSet)
	default:
		return nil
	}
}

// Common response envelope structures
type notionAPIResponse struct {
	RecordMap map[string]any `json:"recordMap,omitempty"`
}

type getPageAsNestedResponse struct {
	notionAPIResponse
}

type queryCollectionResponse struct {
	notionAPIResponse

	BlockIDs    []string `json:"blockIds,omitempty"`
	Total       int      `json:"total,omitempty"`
	Aggregators []any    `json:"aggregators,omitempty"`
}

type getRecordValuesResponse struct {
	Results []map[string]any `json:"results,omitempty"`
}

type loadPageChunkResponse struct {
	RecordMap map[string]any `json:"recordMap,omitempty"`
	Cursor    map[string]any `json:"cursor,omitempty"`
}

type queryCollectionPagesResponse struct {
	Results []map[string]any `json:"results,omitempty"`
}

// parseGetPageAsNested extracts pages from getPageAsNested response.
func parseGetPageAsNested(body string, targetSet map[string]struct{}) []scraper.Result {
	var resp getPageAsNestedResponse

	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	if resp.RecordMap == nil {
		return nil
	}

	var results []scraper.Result

	for recordType, records := range resp.RecordMap {
		if recordType != "block" {
			continue
		}

		recordMap, ok := records.(map[string]any)
		if !ok {
			continue
		}

		for pageID, pageData := range recordMap {
			if targetSet != nil {
				if _, ok := targetSet[strings.ToLower(pageID)]; !ok {
					continue
				}
			}

			result := blockToResult(pageID, pageData)
			if result != nil {
				results = append(results, *result)
			}
		}
	}

	return results
}

// parseQueryCollection extracts pages from queryCollection response.
func parseQueryCollection(body string, targetSet map[string]struct{}) []scraper.Result {
	var resp queryCollectionResponse

	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	if resp.RecordMap == nil {
		return nil
	}

	var results []scraper.Result

	for recordType, records := range resp.RecordMap {
		if recordType != "block" {
			continue
		}

		recordMap, ok := records.(map[string]any)
		if !ok {
			continue
		}

		for pageID, pageData := range recordMap {
			if targetSet != nil {
				if _, ok := targetSet[strings.ToLower(pageID)]; !ok {
					continue
				}
			}

			result := blockToResult(pageID, pageData)
			if result != nil {
				results = append(results, *result)
			}
		}
	}

	return results
}

// parseGetRecordValues extracts records from getRecordValues response.
func parseGetRecordValues(body string, targetSet map[string]struct{}) []scraper.Result {
	var resp getRecordValuesResponse

	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	var results []scraper.Result

	for _, item := range resp.Results {
		if blockData, ok := item["block"].(map[string]any); ok {
			for blockID, block := range blockData {
				if targetSet != nil {
					if _, ok := targetSet[strings.ToLower(blockID)]; !ok {
						continue
					}
				}

				if result := blockToResult(blockID, block); result != nil {
					results = append(results, *result)
				}
			}
		}

		if userData, ok := item["user"].(map[string]any); ok {
			for userID, user := range userData {
				if result := userToResult(userID, user); result != nil {
					results = append(results, *result)
				}
			}
		}
	}

	return results
}

// parseLoadPageChunk extracts blocks from loadPageChunk response.
func parseLoadPageChunk(body string, targetSet map[string]struct{}) []scraper.Result {
	var resp loadPageChunkResponse

	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	if resp.RecordMap == nil {
		return nil
	}

	var results []scraper.Result

	for recordType, records := range resp.RecordMap {
		if recordType != "block" {
			continue
		}

		recordMap, ok := records.(map[string]any)
		if !ok {
			continue
		}

		for pageID, pageData := range recordMap {
			if targetSet != nil {
				if _, ok := targetSet[strings.ToLower(pageID)]; !ok {
					continue
				}
			}

			result := blockToResult(pageID, pageData)
			if result != nil {
				results = append(results, *result)
			}
		}
	}

	return results
}

// parseQueryCollectionPages extracts pages from queryCollectionPages response.
func parseQueryCollectionPages(body string, targetSet map[string]struct{}) []scraper.Result {
	var resp queryCollectionPagesResponse

	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	var results []scraper.Result

	for _, item := range resp.Results {
		if blockID, ok := item["id"].(string); ok {
			if targetSet != nil {
				if _, ok := targetSet[strings.ToLower(blockID)]; !ok {
					continue
				}
			}

			if result := blockToResult(blockID, item); result != nil {
				results = append(results, *result)
			}
		}
	}

	return results
}

// blockToResult converts a Notion block record to a scraper.Result.
func blockToResult(blockID string, blockData any) *scraper.Result {
	blockMap, ok := blockData.(map[string]any)
	if !ok {
		return nil
	}

	value, ok := blockMap["value"].(map[string]any)
	if !ok {
		return nil
	}

	// Extract common properties.
	var (
		title, blockType string
		timestamp        time.Time
	)

	if properties, ok := value["properties"].(map[string]any); ok {
		if titleArray, ok := properties["title"].([]any); ok && len(titleArray) > 0 {
			if titleData, ok := titleArray[0].([]any); ok && len(titleData) > 0 {
				title, _ = titleData[0].(string)
			}
		}
	}

	if t, ok := value["type"].(string); ok {
		blockType = t
	}

	if createdTime, ok := value["created_time"].(float64); ok {
		timestamp = time.UnixMilli(int64(createdTime))
	}

	// Determine result type based on block type.
	resultType := scraper.ResultMessage

	switch blockType {
	case "page":
		resultType = scraper.ResultPost
	case "database":
		resultType = scraper.ResultChannel
	case "comment", "callout":
		resultType = scraper.ResultComment
	case "synced_block":
		resultType = scraper.ResultMessage
	}

	return &scraper.Result{
		Type:      resultType,
		Source:    "notion",
		ID:        blockID,
		Timestamp: timestamp,
		Content:   title,
		Metadata: map[string]any{
			"type": blockType,
		},
		Raw: value,
	}
}

// userToResult converts a Notion user record to a scraper.Result.
func userToResult(userID string, userData any) *scraper.Result {
	userMap, ok := userData.(map[string]any)
	if !ok {
		return nil
	}

	value, ok := userMap["value"].(map[string]any)
	if !ok {
		return nil
	}

	var name, email string

	if n, ok := value["name"].(string); ok {
		name = n
	}

	if e, ok := value["email"].(string); ok {
		email = e
	}

	return &scraper.Result{
		Type:   scraper.ResultUser,
		Source: "notion",
		ID:     userID,
		Author: name,
		Metadata: map[string]any{
			"email": email,
		},
		Raw: value,
	}
}

func init() {
	scraper.RegisterMode(&NotionMode{})
}
