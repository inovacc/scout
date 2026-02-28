// Package grafana implements the scraper.Mode interface for Grafana/Datadog dashboard extraction.
// It intercepts Grafana's internal API calls via session hijacking to capture structured
// dashboard, datasource, alert, and panel data without DOM scraping.
package grafana

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

// grafanaProvider implements auth.Provider for Grafana instances.
type grafanaProvider struct{}

func (p *grafanaProvider) Name() string { return "grafana" }

func (p *grafanaProvider) LoginURL() string { return "https://grafana.com/auth/sign-in" }

func (p *grafanaProvider) DetectAuth(ctx context.Context, page *scout.Page) (bool, error) {
	if page == nil {
		return false, fmt.Errorf("grafana: detect auth: nil page")
	}

	result, err := page.Eval(`() => window.location.href`)
	if err != nil {
		return false, fmt.Errorf("grafana: detect auth: eval url: %w", err)
	}

	url := result.String()
	if strings.Contains(url, "/d/") || strings.Contains(url, "/dashboards") {
		return true, nil
	}

	// Check for Grafana dashboard container element.
	_, err = page.Element(".dashboard-container")
	if err == nil {
		return true, nil
	}

	return false, nil
}

func (p *grafanaProvider) CaptureSession(ctx context.Context, page *scout.Page) (*auth.Session, error) {
	if page == nil {
		return nil, fmt.Errorf("grafana: capture session: nil page")
	}

	cookies, err := page.GetCookies()
	if err != nil {
		return nil, fmt.Errorf("grafana: capture session: get cookies: %w", err)
	}

	result, err := page.Eval(`() => window.location.href`)
	if err != nil {
		return nil, fmt.Errorf("grafana: capture session: eval url: %w", err)
	}
	currentURL := result.String()

	tokens := make(map[string]string)
	localStorage := make(map[string]string)

	// Extract auth tokens from localStorage.
	lsResult, err := page.Eval(`() => {
		try {
			const authToken = localStorage.getItem('grafana_session');
			if (authToken) return authToken;
		} catch(e) {}
		return '';
	}`)
	if err == nil {
		raw := lsResult.String()
		if raw != "" {
			localStorage["grafana_session"] = raw
			tokens["grafana_session"] = raw
		}
	}

	// Try to extract API token from window globals or localStorage.
	tokenResult, err := page.Eval(`() => {
		try {
			if (window.grafanaBootData && window.grafanaBootData.user && window.grafanaBootData.user.orgRole) {
				return JSON.stringify(window.grafanaBootData.user);
			}
			const apiToken = localStorage.getItem('grafana_api_token');
			if (apiToken) return apiToken;
		} catch(e) {}
		return '';
	}`)
	if err == nil {
		tok := tokenResult.String()
		if tok != "" {
			tokens["api_token"] = tok
		}
	}

	// Extract session expiry if available.
	expiryResult, err := page.Eval(`() => {
		try {
			const expiry = localStorage.getItem('grafana_session_expiry');
			if (expiry) return expiry;
		} catch(e) {}
		return '';
	}`)
	var expiresAt time.Time
	if err == nil {
		expiryStr := expiryResult.String()
		if expiryStr != "" {
			localStorage["grafana_session_expiry"] = expiryStr
			// Try to parse as Unix timestamp.
			if et, err := time.Parse(time.RFC3339, expiryStr); err == nil {
				expiresAt = et
			}
		}
	}

	// Default expiration if not set.
	if expiresAt.IsZero() {
		expiresAt = time.Now().Add(24 * time.Hour)
	}

	now := time.Now()
	return &auth.Session{
		Provider:     "grafana",
		Version:      "1",
		Timestamp:    now,
		URL:          currentURL,
		Cookies:      cookies,
		Tokens:       tokens,
		LocalStorage: localStorage,
		ExpiresAt:    expiresAt,
	}, nil
}

func (p *grafanaProvider) ValidateSession(_ context.Context, session *auth.Session) error {
	if session == nil {
		return fmt.Errorf("grafana: validate session: nil session")
	}

	// Check for grafana_session cookie or token.
	if len(session.Tokens) > 0 {
		if _, ok := session.Tokens["grafana_session"]; ok {
			return nil
		}
		if _, ok := session.Tokens["api_token"]; ok {
			return nil
		}
	}

	// Check cookies for grafana_session.
	for _, cookie := range session.Cookies {
		if cookie.Name == "grafana_session" && cookie.Value != "" {
			return nil
		}
	}

	return &scraper.AuthError{Reason: "no valid grafana session or token found in session"}
}

// GrafanaMode implements scraper.Mode for Grafana instances.
type GrafanaMode struct {
	provider grafanaProvider
}

func (m *GrafanaMode) Name() string        { return "grafana" }
func (m *GrafanaMode) Description() string  { return "Scrape Grafana dashboards, datasources, alerts, and panels" }
func (m *GrafanaMode) AuthProvider() scraper.AuthProvider { return &m.provider }

// Scrape creates a browser session, restores cookies, navigates to the Grafana instance,
// and intercepts Grafana API calls to extract structured data.
func (m *GrafanaMode) Scrape(ctx context.Context, session scraper.SessionData, opts scraper.ScrapeOptions) (<-chan scraper.Result, error) {
	grafanaSession, ok := session.(*auth.Session)
	if !ok || grafanaSession == nil {
		return nil, fmt.Errorf("grafana: scrape: invalid or nil session")
	}

	if err := m.provider.ValidateSession(ctx, grafanaSession); err != nil {
		return nil, fmt.Errorf("grafana: scrape: %w", err)
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
		return nil, fmt.Errorf("grafana: scrape: create browser: %w", err)
	}

	page, err := browser.NewPage(grafanaSession.URL)
	if err != nil {
		browser.Close()
		return nil, fmt.Errorf("grafana: scrape: new page: %w", err)
	}

	if err := page.SetCookies(grafanaSession.Cookies...); err != nil {
		browser.Close()
		return nil, fmt.Errorf("grafana: scrape: set cookies: %w", err)
	}

	// Reload to apply cookies.
	if _, err := page.Eval(`() => location.reload()`); err != nil {
		browser.Close()
		return nil, fmt.Errorf("grafana: scrape: reload: %w", err)
	}

	if err := page.WaitLoad(); err != nil {
		browser.Close()
		return nil, fmt.Errorf("grafana: scrape: wait load: %w", err)
	}

	hijacker, err := page.NewSessionHijacker(
		scout.WithHijackURLFilter("*/api/*"),
		scout.WithHijackBodyCapture(),
	)
	if err != nil {
		browser.Close()
		return nil, fmt.Errorf("grafana: scrape: create hijacker: %w", err)
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

// buildTargetSet creates a lookup set from target dashboard UIDs or folder names.
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

// parseHijackEvent examines a network event and extracts scraper.Result items from Grafana API responses.
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
	case strings.Contains(url, "/api/dashboards/") && strings.Contains(url, "find"):
		return parseDashboardsList(body, targetSet)
	case strings.Contains(url, "/api/dashboards/uid/"):
		return parseDashboardDetail(body, targetSet)
	case strings.Contains(url, "/api/datasources"):
		return parseDatasourcesList(body, targetSet)
	case strings.Contains(url, "/api/alerts"):
		return parseAlertsList(body, targetSet)
	case strings.Contains(url, "/api/search"):
		return parseSearchResults(body, targetSet)
	case strings.Contains(url, "/api/ds/query"):
		return parsePanelQuery(body, targetSet)
	case strings.Contains(url, "/api/annotations"):
		return parseAnnotations(body, targetSet)
	default:
		return nil
	}
}

// grafanaAPIResponse is the common envelope for Grafana API responses.
type grafanaAPIResponse struct {
	Message string `json:"message,omitempty"`
}

type dashboardListResponse struct {
	grafanaAPIResponse
	Results []grafanaDashboard `json:"results,omitempty"`
}

type grafanaDashboard struct {
	ID       int64  `json:"id"`
	UID      string `json:"uid"`
	Title    string `json:"title"`
	FolderID int64  `json:"folderId"`
	Folder   string `json:"folder,omitempty"`
	URL      string `json:"url,omitempty"`
	Type     string `json:"type,omitempty"`
	Tags     []string `json:"tags,omitempty"`
}

func parseDashboardsList(body string, targetSet map[string]struct{}) []scraper.Result {
	var resp dashboardListResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	results := make([]scraper.Result, 0, len(resp.Results))
	for _, dash := range resp.Results {
		// Apply target filter.
		if targetSet != nil {
			dashUID := strings.ToLower(dash.UID)
			if _, ok := targetSet[dashUID]; !ok {
				continue
			}
		}

		ts := time.Now()
		results = append(results, scraper.Result{
			Type:      scraper.ResultPost,
			Source:    "grafana",
			ID:        dash.UID,
			Timestamp: ts,
			Content:   dash.Title,
			URL:       dash.URL,
			Metadata: map[string]any{
				"id":        dash.ID,
				"folder":    dash.Folder,
				"folder_id": dash.FolderID,
				"type":      dash.Type,
				"tags":      dash.Tags,
			},
			Raw: dash,
		})
	}
	return results
}

type dashboardDetailResponse struct {
	grafanaAPIResponse
	Dashboard grafanaDashboardDetail `json:"dashboard"`
}

type grafanaDashboardDetail struct {
	ID       int64  `json:"id"`
	UID      string `json:"uid"`
	Title    string `json:"title"`
	Tags     []string `json:"tags,omitempty"`
	Panels   []grafanaPanel `json:"panels,omitempty"`
	Timezone string `json:"timezone,omitempty"`
	SchemaVersion int `json:"schemaVersion"`
}

type grafanaPanel struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Type     string `json:"type"`
	GridPos  map[string]int `json:"gridPos,omitempty"`
	Targets  []map[string]any `json:"targets,omitempty"`
}

func parseDashboardDetail(body string, targetSet map[string]struct{}) []scraper.Result {
	var resp dashboardDetailResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	dash := resp.Dashboard

	// Apply target filter.
	if targetSet != nil {
		dashUID := strings.ToLower(dash.UID)
		if _, ok := targetSet[dashUID]; !ok {
			return nil
		}
	}

	var results []scraper.Result
	ts := time.Now()

	// Emit dashboard as ResultPost.
	results = append(results, scraper.Result{
		Type:      scraper.ResultPost,
		Source:    "grafana",
		ID:        dash.UID,
		Timestamp: ts,
		Content:   dash.Title,
		Metadata: map[string]any{
			"id":               dash.ID,
			"tags":             dash.Tags,
			"timezone":         dash.Timezone,
			"schema_version":   dash.SchemaVersion,
			"panel_count":      len(dash.Panels),
		},
		Raw: dash,
	})

	// Emit panels as ResultFile.
	for _, panel := range dash.Panels {
		results = append(results, scraper.Result{
			Type:      scraper.ResultFile,
			Source:    "grafana",
			ID:        fmt.Sprintf("%s_panel_%d", dash.UID, panel.ID),
			Timestamp: ts,
			Content:   panel.Title,
			Metadata: map[string]any{
				"dashboard_uid": dash.UID,
				"panel_type":    panel.Type,
				"panel_id":      panel.ID,
				"grid_pos":      panel.GridPos,
				"targets":       panel.Targets,
			},
			Raw: panel,
		})
	}

	return results
}

type datasourcesListResponse struct {
	grafanaAPIResponse
}

type grafanaDatasource struct {
	ID       int64  `json:"id"`
	UID      string `json:"uid"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	URL      string `json:"url"`
	IsDefault bool  `json:"isDefault"`
	Database string `json:"database,omitempty"`
	User     string `json:"user,omitempty"`
}

func parseDatasourcesList(body string, targetSet map[string]struct{}) []scraper.Result {
	var resp []grafanaDatasource
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	results := make([]scraper.Result, 0, len(resp))
	ts := time.Now()

	for _, ds := range resp {
		results = append(results, scraper.Result{
			Type:      scraper.ResultChannel,
			Source:    "grafana",
			ID:        ds.UID,
			Timestamp: ts,
			Content:   ds.Name,
			URL:       ds.URL,
			Metadata: map[string]any{
				"id":         ds.ID,
				"type":       ds.Type,
				"is_default": ds.IsDefault,
				"database":   ds.Database,
				"user":       ds.User,
			},
			Raw: ds,
		})
	}
	return results
}

type alertsListResponse struct {
	grafanaAPIResponse
	Results []grafanaAlert `json:"results,omitempty"`
}

type grafanaAlert struct {
	ID       int64  `json:"id"`
	DashboardID int64 `json:"dashboardId"`
	DashboardUID string `json:"dashboardUid"`
	Name     string `json:"name"`
	State    string `json:"state"`
	Message  string `json:"message,omitempty"`
	Created  int64  `json:"created"`
	Updated  int64  `json:"updated"`
}

func parseAlertsList(body string, targetSet map[string]struct{}) []scraper.Result {
	var resp alertsListResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	results := make([]scraper.Result, 0, len(resp.Results))
	for _, alert := range resp.Results {
		ts := time.Unix(alert.Created/1000, (alert.Created%1000)*1000000)

		results = append(results, scraper.Result{
			Type:      scraper.ResultMessage,
			Source:    "grafana",
			ID:        fmt.Sprintf("alert_%d", alert.ID),
			Timestamp: ts,
			Content:   alert.Message,
			Author:    alert.Name,
			Metadata: map[string]any{
				"id":               alert.ID,
				"dashboard_id":     alert.DashboardID,
				"dashboard_uid":    alert.DashboardUID,
				"state":            alert.State,
				"created":          alert.Created,
				"updated":          alert.Updated,
			},
			Raw: alert,
		})
	}
	return results
}

type searchResultsResponse struct {
	grafanaAPIResponse
	Results []grafanaSearchResult `json:"results,omitempty"`
}

type grafanaSearchResult struct {
	ID       int64  `json:"id"`
	UID      string `json:"uid"`
	Title    string `json:"title"`
	Type     string `json:"type"`
	URL      string `json:"url"`
	FolderID int64  `json:"folderId,omitempty"`
	Folder   string `json:"folder,omitempty"`
	Tags     []string `json:"tags,omitempty"`
}

func parseSearchResults(body string, targetSet map[string]struct{}) []scraper.Result {
	var resp searchResultsResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	results := make([]scraper.Result, 0, len(resp.Results))
	ts := time.Now()

	for _, item := range resp.Results {
		// Apply target filter for dashboards.
		if targetSet != nil {
			if item.Type == "dash-db" {
				itemUID := strings.ToLower(item.UID)
				if _, ok := targetSet[itemUID]; !ok {
					continue
				}
			}
		}

		resultType := scraper.ResultPost
		if item.Type == "folder" {
			resultType = scraper.ResultChannel
		}

		results = append(results, scraper.Result{
			Type:      resultType,
			Source:    "grafana",
			ID:        item.UID,
			Timestamp: ts,
			Content:   item.Title,
			URL:       item.URL,
			Metadata: map[string]any{
				"id":        item.ID,
				"type":      item.Type,
				"folder":    item.Folder,
				"folder_id": item.FolderID,
				"tags":      item.Tags,
			},
			Raw: item,
		})
	}
	return results
}

type panelQueryResponse struct {
	grafanaAPIResponse
	Results []grafanaQueryResult `json:"results,omitempty"`
}

type grafanaQueryResult struct {
	FrameMeta map[string]any `json:"frameMeta,omitempty"`
	Meta      map[string]any `json:"meta,omitempty"`
	Status    int `json:"status,omitempty"`
}

func parsePanelQuery(body string, targetSet map[string]struct{}) []scraper.Result {
	var resp panelQueryResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	results := make([]scraper.Result, 0, len(resp.Results))
	ts := time.Now()

	for i, result := range resp.Results {
		results = append(results, scraper.Result{
			Type:      scraper.ResultFile,
			Source:    "grafana",
			ID:        fmt.Sprintf("query_result_%d", i),
			Timestamp: ts,
			Metadata: map[string]any{
				"frame_meta": result.FrameMeta,
				"meta":       result.Meta,
				"status":     result.Status,
			},
			Raw: result,
		})
	}
	return results
}

type annotationsResponse struct {
	grafanaAPIResponse
	Results []grafanaAnnotation `json:"results,omitempty"`
}

type grafanaAnnotation struct {
	ID        int64  `json:"id"`
	DashboardID int64 `json:"dashboardId"`
	AlertID   int64  `json:"alertId,omitempty"`
	Text      string `json:"text"`
	Time      int64  `json:"time"`
	TimeEnd   int64  `json:"timeEnd,omitempty"`
	Tags      []string `json:"tags,omitempty"`
}

func parseAnnotations(body string, targetSet map[string]struct{}) []scraper.Result {
	var resp annotationsResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	results := make([]scraper.Result, 0, len(resp.Results))
	for _, ann := range resp.Results {
		ts := time.Unix(ann.Time/1000, (ann.Time%1000)*1000000)

		results = append(results, scraper.Result{
			Type:      scraper.ResultComment,
			Source:    "grafana",
			ID:        fmt.Sprintf("annotation_%d", ann.ID),
			Timestamp: ts,
			Content:   ann.Text,
			Metadata: map[string]any{
				"id":           ann.ID,
				"dashboard_id": ann.DashboardID,
				"alert_id":     ann.AlertID,
				"time_end":     ann.TimeEnd,
				"tags":         ann.Tags,
			},
			Raw: ann,
		})
	}
	return results
}

func init() {
	scraper.RegisterMode(&GrafanaMode{})
}
