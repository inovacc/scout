// Package salesforce implements the scraper.Mode interface for Salesforce CRM extraction.
// It intercepts Salesforce's internal API calls via session hijacking to capture structured
// lead, contact, opportunity, account, report, and activity data without DOM scraping.
package salesforce

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

// salesforceProvider implements auth.Provider for Salesforce.
type salesforceProvider struct{}

func (p *salesforceProvider) Name() string { return "salesforce" }

func (p *salesforceProvider) LoginURL() string { return "https://login.salesforce.com/" }

func (p *salesforceProvider) DetectAuth(ctx context.Context, page *scout.Page) (bool, error) {
	if page == nil {
		return false, fmt.Errorf("salesforce: detect auth: nil page")
	}

	result, err := page.Eval(`() => window.location.href`)
	if err != nil {
		return false, fmt.Errorf("salesforce: detect auth: eval url: %w", err)
	}

	url := result.String()
	// Check for authenticated Salesforce instance indicators.
	if strings.Contains(url, ".lightning.force.com") || strings.Contains(url, ".salesforce.com/home") {
		return true, nil
	}

	// Check for Aura framework element which indicates logged-in Lightning Experience.
	_, err = page.Element("[data-aura-rendered-by]")
	if err == nil {
		return true, nil
	}

	return false, nil
}

func (p *salesforceProvider) CaptureSession(ctx context.Context, page *scout.Page) (*auth.Session, error) {
	if page == nil {
		return nil, fmt.Errorf("salesforce: capture session: nil page")
	}

	cookies, err := page.GetCookies()
	if err != nil {
		return nil, fmt.Errorf("salesforce: capture session: get cookies: %w", err)
	}

	result, err := page.Eval(`() => window.location.href`)
	if err != nil {
		return nil, fmt.Errorf("salesforce: capture session: eval url: %w", err)
	}

	currentURL := result.String()

	tokens := make(map[string]string)
	localStorage := make(map[string]string)

	// Extract instance URL and access token from sessionStorage/localStorage.
	tokenResult, err := page.Eval(`() => {
		try {
			// Check for access_token in sessionStorage.
			let token = sessionStorage.getItem('access_token');
			if (token) return token;
			// Check window object for Aura token.
			if (window.Aura && window.Aura.getToken && window.Aura.getToken('sessionId')) {
				return window.Aura.getToken('sessionId');
			}
		} catch(e) {}
		return '';
	}`)
	if err == nil {
		tok := tokenResult.String()
		if tok != "" {
			tokens["access_token"] = tok
		}
	}

	// Extract instance URL.
	instanceResult, err := page.Eval(`() => {
		try {
			const url = window.location.href;
			const match = url.match(/https:\/\/([a-z0-9-]+\.salesforce\.com|[a-z0-9-]+\.lightning\.force\.com)/);
			if (match) return match[0];
			return '';
		} catch(e) {}
		return '';
	}`)
	if err == nil {
		inst := instanceResult.String()
		if inst != "" {
			tokens["instance_url"] = inst
		}
	}

	// Capture localStorage entries that may contain Salesforce auth state.
	lsResult, err := page.Eval(`() => {
		try {
			const keys = Object.keys(localStorage);
			const result = {};
			for (const key of keys) {
				if (key.includes('salesforce') || key.includes('access') || key.includes('token') || key.includes('instance')) {
					try {
						result[key] = localStorage.getItem(key);
					} catch(e) {}
				}
			}
			return JSON.stringify(result);
		} catch(e) {}
		return '{}';
	}`)
	if err == nil {
		raw := lsResult.String()
		if raw != "" && raw != "{}" {
			var lsMap map[string]string
			if json.Unmarshal([]byte(raw), &lsMap) == nil {
				maps.Copy(localStorage, lsMap)
			}
		}
	}

	now := time.Now()

	return &auth.Session{
		Provider:     "salesforce",
		Version:      "1",
		Timestamp:    now,
		URL:          currentURL,
		Cookies:      cookies,
		Tokens:       tokens,
		LocalStorage: localStorage,
		ExpiresAt:    now.Add(24 * time.Hour),
	}, nil
}

func (p *salesforceProvider) ValidateSession(_ context.Context, session *auth.Session) error {
	if session == nil {
		return fmt.Errorf("salesforce: validate session: nil session")
	}

	// Check for required Salesforce authentication tokens.
	for _, cookie := range session.Cookies {
		if cookie.Name == "sid" || cookie.Name == "oid" {
			return nil
		}
	}

	// Also check tokens map for access_token.
	if _, ok := session.Tokens["access_token"]; ok {
		return nil
	}

	return &scraper.AuthError{Reason: "no valid salesforce authentication cookies (sid, oid) or access token found in session"}
}

// SalesforceMode implements scraper.Mode for Salesforce.
type SalesforceMode struct {
	provider salesforceProvider
}

func (m *SalesforceMode) Name() string { return "salesforce" }
func (m *SalesforceMode) Description() string {
	return "Scrape Salesforce leads, contacts, opportunities, accounts, reports, and activities"
}
func (m *SalesforceMode) AuthProvider() scraper.AuthProvider { return &m.provider }

// Scrape creates a browser session, restores cookies, navigates to Salesforce,
// and intercepts REST API calls to extract structured data.
func (m *SalesforceMode) Scrape(ctx context.Context, session scraper.SessionData, opts scraper.ScrapeOptions) (<-chan scraper.Result, error) {
	sfSession, ok := session.(*auth.Session)
	if !ok || sfSession == nil {
		return nil, fmt.Errorf("salesforce: scrape: invalid or nil session")
	}

	if err := m.provider.ValidateSession(ctx, sfSession); err != nil {
		return nil, fmt.Errorf("salesforce: scrape: %w", err)
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
		return nil, fmt.Errorf("salesforce: scrape: create browser: %w", err)
	}

	page, err := browser.NewPage(sfSession.URL)
	if err != nil {
		_ = browser.Close()
		return nil, fmt.Errorf("salesforce: scrape: new page: %w", err)
	}

	if err := page.SetCookies(sfSession.Cookies...); err != nil {
		_ = browser.Close()
		return nil, fmt.Errorf("salesforce: scrape: set cookies: %w", err)
	}

	// Reload to apply cookies.
	if _, err := page.Eval(`() => location.reload()`); err != nil {
		_ = browser.Close()
		return nil, fmt.Errorf("salesforce: scrape: reload: %w", err)
	}

	if err := page.WaitLoad(); err != nil {
		_ = browser.Close()
		return nil, fmt.Errorf("salesforce: scrape: wait load: %w", err)
	}

	hijacker, err := page.NewSessionHijacker(
		scout.WithHijackURLFilter("*/services/data/*", "*/aura*", "*/ui-api/*"),
		scout.WithHijackBodyCapture(),
	)
	if err != nil {
		_ = browser.Close()
		return nil, fmt.Errorf("salesforce: scrape: create hijacker: %w", err)
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

// buildTargetSet creates a lookup set from target object types or IDs.
// An empty set means no filtering (capture all).
func buildTargetSet(targets []string) map[string]struct{} {
	if len(targets) == 0 {
		return nil
	}

	set := make(map[string]struct{}, len(targets))
	for _, t := range targets {
		// Normalize by converting to uppercase (for object types like "Lead", "Contact").
		normalized := strings.ToUpper(t)
		set[normalized] = struct{}{}
	}

	return set
}

// parseHijackEvent examines a network event and extracts scraper.Result items from Salesforce API responses.
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
	case strings.Contains(url, "/services/data/") && strings.Contains(url, "/sobjects/Lead"):
		return parseLeadsResponse(body, targetSet)
	case strings.Contains(url, "/services/data/") && strings.Contains(url, "/sobjects/Contact"):
		return parseContactsResponse(body, targetSet)
	case strings.Contains(url, "/services/data/") && strings.Contains(url, "/sobjects/Opportunity"):
		return parseOpportunitiesResponse(body, targetSet)
	case strings.Contains(url, "/services/data/") && strings.Contains(url, "/sobjects/Account"):
		return parseAccountsResponse(body, targetSet)
	case strings.Contains(url, "/services/data/") && strings.Contains(url, "/analytics/reports/"):
		return parseReportsResponse(body, targetSet)
	case strings.Contains(url, "/services/data/") && strings.Contains(url, "/sobjects/Task"):
		return parseTasksResponse(body, targetSet)
	case strings.Contains(url, "/ui-api/"):
		return parseUIAPIResponse(body, targetSet)
	default:
		return nil
	}
}

// salesforceAPIResponse is the common envelope for Salesforce REST API responses.
type salesforceAPIResponse struct {
	Records        []json.RawMessage `json:"records"`
	TotalSize      int               `json:"totalSize"`
	Done           bool              `json:"done"`
	NextRecordsURL string            `json:"nextRecordsUrl"`
}

// leadRecord represents a Lead object from Salesforce.
type leadRecord struct {
	ID          string `json:"Id"`
	FirstName   string `json:"FirstName"`
	LastName    string `json:"LastName"`
	Company     string `json:"Company"`
	Email       string `json:"Email"`
	Phone       string `json:"Phone"`
	Status      string `json:"Status"`
	CreatedDate string `json:"CreatedDate"`
	Industry    string `json:"Industry"`
	LeadSource  string `json:"LeadSource"`
}

func parseLeadsResponse(body string, targetSet map[string]struct{}) []scraper.Result {
	if targetSet != nil {
		if _, ok := targetSet["LEAD"]; !ok {
			return nil
		}
	}

	var resp salesforceAPIResponse

	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	results := make([]scraper.Result, 0, len(resp.Records))
	for _, recordRaw := range resp.Records {
		var lead leadRecord

		if err := json.Unmarshal(recordRaw, &lead); err != nil {
			continue
		}

		if lead.ID == "" {
			continue
		}

		ts := parseSalesforceTimestamp(lead.CreatedDate)
		results = append(results, scraper.Result{
			Type:      scraper.ResultProfile,
			Source:    "salesforce",
			ID:        lead.ID,
			Timestamp: ts,
			Author:    lead.FirstName + " " + lead.LastName,
			Content:   lead.Company,
			Metadata: map[string]any{
				"first_name":  lead.FirstName,
				"last_name":   lead.LastName,
				"email":       lead.Email,
				"phone":       lead.Phone,
				"status":      lead.Status,
				"industry":    lead.Industry,
				"lead_source": lead.LeadSource,
			},
			Raw: lead,
		})
	}

	return results
}

// contactRecord represents a Contact object from Salesforce.
type contactRecord struct {
	ID             string `json:"Id"`
	FirstName      string `json:"FirstName"`
	LastName       string `json:"LastName"`
	Email          string `json:"Email"`
	Phone          string `json:"Phone"`
	AccountID      string `json:"AccountId"`
	Title          string `json:"Title"`
	Department     string `json:"Department"`
	CreatedDate    string `json:"CreatedDate"`
	MailingCountry string `json:"MailingCountry"`
}

func parseContactsResponse(body string, targetSet map[string]struct{}) []scraper.Result {
	if targetSet != nil {
		if _, ok := targetSet["CONTACT"]; !ok {
			return nil
		}
	}

	var resp salesforceAPIResponse

	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	results := make([]scraper.Result, 0, len(resp.Records))
	for _, recordRaw := range resp.Records {
		var contact contactRecord

		if err := json.Unmarshal(recordRaw, &contact); err != nil {
			continue
		}

		if contact.ID == "" {
			continue
		}

		ts := parseSalesforceTimestamp(contact.CreatedDate)
		results = append(results, scraper.Result{
			Type:      scraper.ResultProfile,
			Source:    "salesforce",
			ID:        contact.ID,
			Timestamp: ts,
			Author:    contact.FirstName + " " + contact.LastName,
			Content:   contact.Title,
			Metadata: map[string]any{
				"email":           contact.Email,
				"phone":           contact.Phone,
				"department":      contact.Department,
				"account_id":      contact.AccountID,
				"mailing_country": contact.MailingCountry,
			},
			Raw: contact,
		})
	}

	return results
}

// opportunityRecord represents an Opportunity object from Salesforce.
type opportunityRecord struct {
	ID               string  `json:"Id"`
	Name             string  `json:"Name"`
	StageName        string  `json:"StageName"`
	Amount           float64 `json:"Amount"`
	CloseDate        string  `json:"CloseDate"`
	CreatedDate      string  `json:"CreatedDate"`
	AccountID        string  `json:"AccountId"`
	Description      string  `json:"Description"`
	Probability      int     `json:"Probability"`
	ForecastCategory string  `json:"ForecastCategory"`
}

func parseOpportunitiesResponse(body string, targetSet map[string]struct{}) []scraper.Result {
	if targetSet != nil {
		if _, ok := targetSet["OPPORTUNITY"]; !ok {
			return nil
		}
	}

	var resp salesforceAPIResponse

	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	results := make([]scraper.Result, 0, len(resp.Records))
	for _, recordRaw := range resp.Records {
		var opp opportunityRecord

		if err := json.Unmarshal(recordRaw, &opp); err != nil {
			continue
		}

		if opp.ID == "" {
			continue
		}

		ts := parseSalesforceTimestamp(opp.CreatedDate)
		results = append(results, scraper.Result{
			Type:      scraper.ResultPost,
			Source:    "salesforce",
			ID:        opp.ID,
			Timestamp: ts,
			Author:    opp.Name,
			Content:   opp.Description,
			Metadata: map[string]any{
				"stage":             opp.StageName,
				"amount":            opp.Amount,
				"close_date":        opp.CloseDate,
				"account_id":        opp.AccountID,
				"probability":       opp.Probability,
				"forecast_category": opp.ForecastCategory,
			},
			Raw: opp,
		})
	}

	return results
}

// accountRecord represents an Account object from Salesforce.
type accountRecord struct {
	ID                string  `json:"Id"`
	Name              string  `json:"Name"`
	Industry          string  `json:"Industry"`
	AnnualRevenue     float64 `json:"AnnualRevenue"`
	NumberOfEmployees int     `json:"NumberOfEmployees"`
	Phone             string  `json:"Phone"`
	Website           string  `json:"Website"`
	CreatedDate       string  `json:"CreatedDate"`
	BillingCity       string  `json:"BillingCity"`
	BillingCountry    string  `json:"BillingCountry"`
}

func parseAccountsResponse(body string, targetSet map[string]struct{}) []scraper.Result {
	if targetSet != nil {
		if _, ok := targetSet["ACCOUNT"]; !ok {
			return nil
		}
	}

	var resp salesforceAPIResponse

	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	results := make([]scraper.Result, 0, len(resp.Records))
	for _, recordRaw := range resp.Records {
		var account accountRecord

		if err := json.Unmarshal(recordRaw, &account); err != nil {
			continue
		}

		if account.ID == "" {
			continue
		}

		ts := parseSalesforceTimestamp(account.CreatedDate)
		results = append(results, scraper.Result{
			Type:      scraper.ResultChannel,
			Source:    "salesforce",
			ID:        account.ID,
			Timestamp: ts,
			Author:    account.Name,
			Content:   account.Industry,
			Metadata: map[string]any{
				"annual_revenue":  account.AnnualRevenue,
				"num_employees":   account.NumberOfEmployees,
				"phone":           account.Phone,
				"website":         account.Website,
				"billing_city":    account.BillingCity,
				"billing_country": account.BillingCountry,
			},
			Raw: account,
		})
	}

	return results
}

// reportRecord represents a report from Salesforce.
type reportRecord struct {
	ID          string `json:"Id"`
	Name        string `json:"Name"`
	Description string `json:"Description"`
	CreatedDate string `json:"CreatedDate"`
	LastRunDate string `json:"LastRunDate"`
	ReportType  string `json:"ReportType"`
	Owner       string `json:"Owner"`
}

func parseReportsResponse(body string, targetSet map[string]struct{}) []scraper.Result { //nolint:unparam
	// Reports are handled by ID or type; check targetSet for report identifiers.
	var resp salesforceAPIResponse

	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	results := make([]scraper.Result, 0, len(resp.Records))
	for _, recordRaw := range resp.Records {
		var report reportRecord

		if err := json.Unmarshal(recordRaw, &report); err != nil {
			continue
		}

		if report.ID == "" {
			continue
		}

		ts := parseSalesforceTimestamp(report.CreatedDate)
		results = append(results, scraper.Result{
			Type:      scraper.ResultFile,
			Source:    "salesforce",
			ID:        report.ID,
			Timestamp: ts,
			Author:    report.Owner,
			Content:   report.Name,
			Metadata: map[string]any{
				"description": report.Description,
				"report_type": report.ReportType,
				"last_run":    report.LastRunDate,
			},
			Raw: report,
		})
	}

	return results
}

// taskRecord represents a Task or Activity from Salesforce.
type taskRecord struct {
	ID           string `json:"Id"`
	Subject      string `json:"Subject"`
	Description  string `json:"Description"`
	Status       string `json:"Status"`
	Priority     string `json:"Priority"`
	CreatedDate  string `json:"CreatedDate"`
	DueDate      string `json:"DueDate"`
	WhoID        string `json:"WhoId"`
	WhatID       string `json:"WhatId"`
	Owner        string `json:"Owner"`
	ActivityType string `json:"Type"`
}

func parseTasksResponse(body string, targetSet map[string]struct{}) []scraper.Result {
	if targetSet != nil {
		if _, ok := targetSet["TASK"]; !ok {
			return nil
		}
	}

	var resp salesforceAPIResponse

	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	results := make([]scraper.Result, 0, len(resp.Records))
	for _, recordRaw := range resp.Records {
		var task taskRecord

		if err := json.Unmarshal(recordRaw, &task); err != nil {
			continue
		}

		if task.ID == "" {
			continue
		}

		ts := parseSalesforceTimestamp(task.CreatedDate)
		results = append(results, scraper.Result{
			Type:      scraper.ResultMessage,
			Source:    "salesforce",
			ID:        task.ID,
			Timestamp: ts,
			Author:    task.Owner,
			Content:   task.Subject,
			Metadata: map[string]any{
				"description":   task.Description,
				"status":        task.Status,
				"priority":      task.Priority,
				"due_date":      task.DueDate,
				"who_id":        task.WhoID,
				"what_id":       task.WhatID,
				"activity_type": task.ActivityType,
			},
			Raw: task,
		})
	}

	return results
}

// uiAPIRecord represents a generic UI API response item.
type uiAPIRecord struct {
	ID          string         `json:"Id"`
	ApiName     string         `json:"ApiName"`
	DisplayName string         `json:"DisplayName"`
	Fields      map[string]any `json:"fields"`
}

func parseUIAPIResponse(body string, targetSet map[string]struct{}) []scraper.Result { //nolint:unparam
	// UI API responses are more generic; try to parse as array or single object.
	var items []uiAPIRecord

	if err := json.Unmarshal([]byte(body), &items); err != nil {
		// Try single object.
		var single uiAPIRecord

		if err := json.Unmarshal([]byte(body), &single); err != nil {
			return nil
		}

		items = []uiAPIRecord{single}
	}

	var results []scraper.Result

	for _, item := range items {
		if item.ID == "" {
			continue
		}

		// Infer result type based on ApiName.
		resultType := scraper.ResultPost

		if item.ApiName != "" {
			switch {
			case strings.Contains(item.ApiName, "Lead"):
				resultType = scraper.ResultProfile
			case strings.Contains(item.ApiName, "Contact"):
				resultType = scraper.ResultProfile
			case strings.Contains(item.ApiName, "Account"):
				resultType = scraper.ResultChannel
			case strings.Contains(item.ApiName, "Opportunity"):
				resultType = scraper.ResultPost
			case strings.Contains(item.ApiName, "Task"):
				resultType = scraper.ResultMessage
			}
		}

		results = append(results, scraper.Result{
			Type:      resultType,
			Source:    "salesforce",
			ID:        item.ID,
			Timestamp: time.Now(),
			Author:    item.DisplayName,
			Content:   item.ApiName,
			Metadata: map[string]any{
				"api_name":     item.ApiName,
				"display_name": item.DisplayName,
				"fields":       item.Fields,
			},
			Raw: item,
		})
	}

	return results
}

// parseSalesforceTimestamp converts a Salesforce ISO 8601 timestamp string to time.Time.
func parseSalesforceTimestamp(ts string) time.Time {
	if ts == "" {
		return time.Time{}
	}

	// Salesforce returns ISO 8601 timestamps like "2026-02-28T10:30:45.000+0000".
	// time.RFC3339Nano handles most variations.
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		// Try a simpler format if RFC3339Nano fails.
		t, _ = time.Parse("2006-01-02T15:04:05", ts)
	}

	return t
}

func init() {
	scraper.RegisterMode(&SalesforceMode{})
}
