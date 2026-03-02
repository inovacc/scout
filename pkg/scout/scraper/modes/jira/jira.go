// Package jira implements the scraper.Mode interface for Jira instance extraction.
// It intercepts Jira's internal API calls via session hijacking to capture structured
// issue, sprint, board, and user data without DOM scraping.
package jira

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

// jiraProvider implements auth.Provider for Jira instances.
type jiraProvider struct{}

func (p *jiraProvider) Name() string { return "jira" }

func (p *jiraProvider) LoginURL() string { return "https://id.atlassian.com/login" }

func (p *jiraProvider) DetectAuth(ctx context.Context, page *scout.Page) (bool, error) {
	if page == nil {
		return false, fmt.Errorf("jira: detect auth: nil page")
	}

	result, err := page.Eval(`() => window.location.href`)
	if err != nil {
		return false, fmt.Errorf("jira: detect auth: eval url: %w", err)
	}

	url := result.String()
	if strings.Contains(url, "/jira/") || strings.Contains(url, "/browse/") {
		return true, nil
	}

	// Check for Jira global navigation element (present on authenticated pages).
	_, err = page.Element("[data-testid=\"global-navigation\"]")
	if err == nil {
		return true, nil
	}

	return false, nil
}

func (p *jiraProvider) CaptureSession(ctx context.Context, page *scout.Page) (*auth.Session, error) {
	if page == nil {
		return nil, fmt.Errorf("jira: capture session: nil page")
	}

	cookies, err := page.GetCookies()
	if err != nil {
		return nil, fmt.Errorf("jira: capture session: get cookies: %w", err)
	}

	result, err := page.Eval(`() => window.location.href`)
	if err != nil {
		return nil, fmt.Errorf("jira: capture session: eval url: %w", err)
	}

	currentURL := result.String()

	tokens := make(map[string]string)
	localStorage := make(map[string]string)
	sessionStorage := make(map[string]string)

	// Extract cloud.session.token and atlassian.xsrf.token from cookies.
	for _, cookie := range cookies {
		if cookie.Name == "cloud.session.token" && cookie.Value != "" {
			tokens["cloud.session.token"] = cookie.Value
		}

		if cookie.Name == "atlassian.xsrf.token" && cookie.Value != "" {
			tokens["atlassian.xsrf.token"] = cookie.Value
		}
	}

	// Extract localStorage data for Jira state.
	lsResult, err := page.Eval(`() => {
		try {
			const keys = Object.keys(localStorage);
			const data = {};
			for (const key of keys) {
				if (key.startsWith('jira-') || key.includes('atlassian')) {
					try {
						data[key] = localStorage.getItem(key);
					} catch(e) {}
				}
			}
			return data;
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

	// Extract sessionStorage data if available.
	ssResult, err := page.Eval(`() => {
		try {
			const keys = Object.keys(sessionStorage);
			const data = {};
			for (const key of keys) {
				if (key.startsWith('jira-')) {
					try {
						data[key] = sessionStorage.getItem(key);
					} catch(e) {}
				}
			}
			return data;
		} catch(e) {}
		return {};
	}`)
	if err == nil {
		raw := ssResult.String()
		if raw != "" && raw != "{}" {
			var ssData map[string]string
			if json.Unmarshal([]byte(raw), &ssData) == nil {
				maps.Copy(sessionStorage, ssData)
			}
		}
	}

	now := time.Now()

	return &auth.Session{
		Provider:       "jira",
		Version:        "1",
		Timestamp:      now,
		URL:            currentURL,
		Cookies:        cookies,
		Tokens:         tokens,
		LocalStorage:   localStorage,
		SessionStorage: sessionStorage,
		ExpiresAt:      now.Add(24 * time.Hour),
	}, nil
}

func (p *jiraProvider) ValidateSession(_ context.Context, session *auth.Session) error {
	if session == nil {
		return fmt.Errorf("jira: validate session: nil session")
	}

	// Check for at least one session token.
	if token, ok := session.Tokens["cloud.session.token"]; ok && token != "" {
		return nil
	}

	// Also check cookies for cloud.session.token.
	for _, cookie := range session.Cookies {
		if cookie.Name == "cloud.session.token" && cookie.Value != "" {
			return nil
		}
	}

	return &scraper.AuthError{Reason: "no valid jira session token found"}
}

// JiraMode implements scraper.Mode for Jira instances.
type JiraMode struct {
	provider jiraProvider
}

func (m *JiraMode) Name() string { return "jira" }
func (m *JiraMode) Description() string {
	return "Scrape Jira boards, projects, issues, sprints, and users"
}
func (m *JiraMode) AuthProvider() scraper.AuthProvider { return &m.provider }

// Scrape creates a browser session, restores cookies, navigates to a Jira instance,
// and intercepts Jira API calls to extract structured data.
func (m *JiraMode) Scrape(ctx context.Context, session scraper.SessionData, opts scraper.ScrapeOptions) (<-chan scraper.Result, error) {
	jiraSession, ok := session.(*auth.Session)
	if !ok || jiraSession == nil {
		return nil, fmt.Errorf("jira: scrape: invalid or nil session")
	}

	if err := m.provider.ValidateSession(ctx, jiraSession); err != nil {
		return nil, fmt.Errorf("jira: scrape: %w", err)
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
		return nil, fmt.Errorf("jira: scrape: create browser: %w", err)
	}

	page, err := browser.NewPage(jiraSession.URL)
	if err != nil {
		_ = browser.Close()
		return nil, fmt.Errorf("jira: scrape: new page: %w", err)
	}

	if err := page.SetCookies(jiraSession.Cookies...); err != nil {
		_ = browser.Close()
		return nil, fmt.Errorf("jira: scrape: set cookies: %w", err)
	}

	// Reload to apply cookies.
	if _, err := page.Eval(`() => location.reload()`); err != nil {
		_ = browser.Close()
		return nil, fmt.Errorf("jira: scrape: reload: %w", err)
	}

	if err := page.WaitLoad(); err != nil {
		_ = browser.Close()
		return nil, fmt.Errorf("jira: scrape: wait load: %w", err)
	}

	hijacker, err := page.NewSessionHijacker(
		scout.WithHijackURLFilter("*/rest/api/*"),
		scout.WithHijackURLFilter("*/rest/agile/*"),
		scout.WithHijackURLFilter("*/rest/greenhopper/*"),
		scout.WithHijackURLFilter("*graphql*"),
		scout.WithHijackBodyCapture(),
	)
	if err != nil {
		_ = browser.Close()
		return nil, fmt.Errorf("jira: scrape: create hijacker: %w", err)
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

// buildTargetSet creates a lookup set from target project keys or board IDs.
// An empty set means no filtering.
func buildTargetSet(targets []string) map[string]struct{} {
	if len(targets) == 0 {
		return nil
	}

	set := make(map[string]struct{}, len(targets))
	for _, t := range targets {
		// Normalize to uppercase for project keys, keep as-is for board IDs.
		set[strings.ToUpper(t)] = struct{}{}
		set[strings.ToLower(t)] = struct{}{}
	}

	return set
}

// parseHijackEvent examines a network event and extracts scraper.Result items from Jira API responses.
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
	case strings.Contains(url, "/rest/api/2/search") || strings.Contains(url, "/rest/api/3/issues/search"):
		return parseIssuesSearch(body, targetSet)
	case strings.Contains(url, "/rest/api/2/issue/") || strings.Contains(url, "/rest/api/3/issues/"):
		return parseIssueDetail(body, targetSet)
	case strings.Contains(url, "/rest/agile/1.0/board") || strings.Contains(url, "/rest/greenhopper/1.0/board"):
		return parseBoards(body, targetSet)
	case strings.Contains(url, "/rest/agile/1.0/sprint") || strings.Contains(url, "/rest/greenhopper/1.0/sprint"):
		return parseSprints(body, targetSet)
	case strings.Contains(url, "/rest/api/2/user") || strings.Contains(url, "/rest/api/3/users"):
		return parseUsers(body)
	case strings.Contains(url, "/rest/api/2/project") || strings.Contains(url, "/rest/api/3/projects"):
		return parseProjects(body, targetSet)
	default:
		return nil
	}
}

// jiraAPIResponse is the common envelope for Jira API responses.
type jiraAPIResponse struct {
	Errors map[string]any `json:"errors,omitempty"`
}

type issuesSearchResponse struct {
	jiraAPIResponse

	Issues     []jiraIssue `json:"issues"`
	Total      int         `json:"total"`
	MaxResults int         `json:"maxResults"`
}

type jiraIssue struct {
	Key    string          `json:"key"`
	ID     string          `json:"id"`
	Fields jiraIssueFields `json:"fields"`
	Expand string          `json:"expand,omitempty"`
	Self   string          `json:"self,omitempty"`
}

type jiraIssueFields struct {
	Summary     string               `json:"summary"`
	Description string               `json:"description"`
	Status      jiraStatus           `json:"status"`
	Assignee    *jiraUser            `json:"assignee"`
	Reporter    *jiraUser            `json:"reporter"`
	Created     string               `json:"created"`
	Updated     string               `json:"updated"`
	Project     jiraProject          `json:"project"`
	Priority    jiraPriority         `json:"priority"`
	Labels      []string             `json:"labels"`
	Comments    jiraCommentContainer `json:"comment"`
	Attachment  []jiraAttachment     `json:"attachment"`
}

type jiraCommentContainer struct {
	Comments []jiraComment `json:"comments"`
	Total    int           `json:"total"`
}

type jiraComment struct {
	ID      string   `json:"id"`
	Author  jiraUser `json:"author"`
	Body    string   `json:"body"`
	Created string   `json:"created"`
	Updated string   `json:"updated"`
}

type jiraAttachment struct {
	ID       string   `json:"id"`
	Filename string   `json:"filename"`
	MimeType string   `json:"mimeType"`
	Size     int64    `json:"size"`
	Created  string   `json:"created"`
	Author   jiraUser `json:"author"`
	Content  string   `json:"content"`
}

type jiraStatus struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type jiraPriority struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

type jiraProject struct {
	Key  string `json:"key"`
	ID   string `json:"id"`
	Name string `json:"name"`
}

type jiraUser struct {
	Name         string `json:"name,omitempty"`
	Key          string `json:"key,omitempty"`
	EmailAddress string `json:"emailAddress,omitempty"`
	DisplayName  string `json:"displayName"`
	AccountID    string `json:"accountId,omitempty"`
}

func parseIssuesSearch(body string, targetSet map[string]struct{}) []scraper.Result {
	var resp issuesSearchResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	results := make([]scraper.Result, 0, len(resp.Issues))
	for _, issue := range resp.Issues {
		if targetSet != nil {
			if _, ok := targetSet[issue.Fields.Project.Key]; !ok {
				continue
			}
		}

		ts := parseJiraTimestamp(issue.Fields.Created)

		author := ""
		if issue.Fields.Reporter != nil {
			author = issue.Fields.Reporter.DisplayName
		}

		result := scraper.Result{
			Type:      scraper.ResultMessage,
			Source:    "jira",
			ID:        issue.Key,
			Timestamp: ts,
			Author:    author,
			Content:   issue.Fields.Summary,
			Metadata: map[string]any{
				"issue_id": issue.ID,
				"status":   issue.Fields.Status.Name,
				"priority": issue.Fields.Priority.Name,
				"project":  issue.Fields.Project.Key,
				"labels":   issue.Fields.Labels,
				"updated":  issue.Fields.Updated,
			},
			Raw: issue,
		}

		if issue.Fields.Assignee != nil {
			result.Metadata["assignee"] = issue.Fields.Assignee.DisplayName
		}

		results = append(results, result)

		// Emit comments as separate results.
		for _, comment := range issue.Fields.Comments.Comments {
			commentTS := parseJiraTimestamp(comment.Created)
			results = append(results, scraper.Result{
				Type:      scraper.ResultComment,
				Source:    "jira",
				ID:        issue.Key + ":" + comment.ID,
				Timestamp: commentTS,
				Author:    comment.Author.DisplayName,
				Content:   comment.Body,
				Metadata: map[string]any{
					"issue_key": issue.Key,
					"updated":   comment.Updated,
				},
			})
		}

		// Emit attachments as separate results.
		for _, att := range issue.Fields.Attachment {
			attTS := parseJiraTimestamp(att.Created)
			results = append(results, scraper.Result{
				Type:      scraper.ResultFile,
				Source:    "jira",
				ID:        att.ID,
				Timestamp: attTS,
				Author:    att.Author.DisplayName,
				Content:   att.Filename,
				URL:       att.Content,
				Metadata: map[string]any{
					"issue_key": issue.Key,
					"mimetype":  att.MimeType,
					"size":      att.Size,
				},
			})
		}
	}

	return results
}

func parseIssueDetail(body string, targetSet map[string]struct{}) []scraper.Result {
	var issue jiraIssue
	if err := json.Unmarshal([]byte(body), &issue); err != nil {
		return nil
	}

	if targetSet != nil {
		if _, ok := targetSet[issue.Fields.Project.Key]; !ok {
			return nil
		}
	}

	ts := parseJiraTimestamp(issue.Fields.Created)

	author := ""
	if issue.Fields.Reporter != nil {
		author = issue.Fields.Reporter.DisplayName
	}

	result := scraper.Result{
		Type:      scraper.ResultMessage,
		Source:    "jira",
		ID:        issue.Key,
		Timestamp: ts,
		Author:    author,
		Content:   issue.Fields.Summary,
		Metadata: map[string]any{
			"issue_id": issue.ID,
			"status":   issue.Fields.Status.Name,
			"priority": issue.Fields.Priority.Name,
			"project":  issue.Fields.Project.Key,
			"labels":   issue.Fields.Labels,
			"updated":  issue.Fields.Updated,
		},
		Raw: issue,
	}

	if issue.Fields.Assignee != nil {
		result.Metadata["assignee"] = issue.Fields.Assignee.DisplayName
	}

	results := []scraper.Result{result}

	// Emit comments.
	for _, comment := range issue.Fields.Comments.Comments {
		commentTS := parseJiraTimestamp(comment.Created)
		results = append(results, scraper.Result{
			Type:      scraper.ResultComment,
			Source:    "jira",
			ID:        issue.Key + ":" + comment.ID,
			Timestamp: commentTS,
			Author:    comment.Author.DisplayName,
			Content:   comment.Body,
			Metadata: map[string]any{
				"issue_key": issue.Key,
				"updated":   comment.Updated,
			},
		})
	}

	// Emit attachments.
	for _, att := range issue.Fields.Attachment {
		attTS := parseJiraTimestamp(att.Created)
		results = append(results, scraper.Result{
			Type:      scraper.ResultFile,
			Source:    "jira",
			ID:        att.ID,
			Timestamp: attTS,
			Author:    att.Author.DisplayName,
			Content:   att.Filename,
			URL:       att.Content,
			Metadata: map[string]any{
				"issue_key": issue.Key,
				"mimetype":  att.MimeType,
				"size":      att.Size,
			},
		})
	}

	return results
}

type boardsResponse struct {
	jiraAPIResponse

	Values []jiraBoard `json:"values"`
	Total  int         `json:"total"`
}

type jiraBoard struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	Self string `json:"self"`
}

func parseBoards(body string, targetSet map[string]struct{}) []scraper.Result {
	var resp boardsResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	results := make([]scraper.Result, 0, len(resp.Values))
	for _, board := range resp.Values {
		boardID := fmt.Sprintf("%d", board.ID)
		if targetSet != nil {
			if _, ok := targetSet[boardID]; !ok {
				continue
			}
		}

		results = append(results, scraper.Result{
			Type:      scraper.ResultChannel,
			Source:    "jira",
			ID:        boardID,
			Timestamp: time.Now(),
			Content:   board.Name,
			Metadata: map[string]any{
				"type": board.Type,
			},
			Raw: board,
		})
	}

	return results
}

type sprintsResponse struct {
	jiraAPIResponse

	Values []jiraSprint `json:"values"`
	Total  int          `json:"total"`
}

type jiraSprint struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	State       string  `json:"state"`
	StartDate   *string `json:"startDate"`
	EndDate     *string `json:"endDate"`
	CreatedDate string  `json:"createdDate"`
	Self        string  `json:"self"`
}

func parseSprints(body string, targetSet map[string]struct{}) []scraper.Result {
	var resp sprintsResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		return nil
	}

	results := make([]scraper.Result, 0, len(resp.Values))
	for _, sprint := range resp.Values {
		ts := parseJiraTimestamp(sprint.CreatedDate)

		result := scraper.Result{
			Type:      scraper.ResultThread,
			Source:    "jira",
			ID:        fmt.Sprintf("%d", sprint.ID),
			Timestamp: ts,
			Content:   sprint.Name,
			Metadata: map[string]any{
				"state": sprint.State,
			},
			Raw: sprint,
		}

		if sprint.StartDate != nil && *sprint.StartDate != "" {
			result.Metadata["start_date"] = *sprint.StartDate
		}

		if sprint.EndDate != nil && *sprint.EndDate != "" {
			result.Metadata["end_date"] = *sprint.EndDate
		}

		results = append(results, result)
	}

	return results
}

type usersResponse struct {
	jiraAPIResponse
}

func parseUsers(body string) []scraper.Result {
	// Jira users endpoint can return various formats. Try as array first.
	var users []jiraUser
	if err := json.Unmarshal([]byte(body), &users); err != nil {
		// Try single user format.
		var user jiraUser
		if err := json.Unmarshal([]byte(body), &user); err != nil {
			return nil
		}

		users = []jiraUser{user}
	}

	results := make([]scraper.Result, 0, len(users))
	for _, u := range users {
		id := u.AccountID
		if id == "" {
			id = u.Key
		}

		if id == "" {
			id = u.Name
		}

		results = append(results, scraper.Result{
			Type:   scraper.ResultUser,
			Source: "jira",
			ID:     id,
			Author: u.DisplayName,
			Metadata: map[string]any{
				"name":  u.Name,
				"email": u.EmailAddress,
			},
			Raw: u,
		})
	}

	return results
}

type projectsResponse struct {
	jiraAPIResponse

	Values []jiraProject `json:"values,omitempty"`
}

func parseProjects(body string, targetSet map[string]struct{}) []scraper.Result {
	// Try array response first.
	var projects []jiraProject
	if err := json.Unmarshal([]byte(body), &projects); err != nil {
		// Try object response.
		var resp projectsResponse
		if err := json.Unmarshal([]byte(body), &resp); err != nil {
			return nil
		}

		projects = resp.Values
	}

	results := make([]scraper.Result, 0, len(projects))
	for _, p := range projects {
		if targetSet != nil {
			if _, ok := targetSet[p.Key]; !ok {
				continue
			}
		}

		results = append(results, scraper.Result{
			Type:      scraper.ResultProfile,
			Source:    "jira",
			ID:        p.Key,
			Timestamp: time.Now(),
			Content:   p.Name,
			Metadata: map[string]any{
				"project_id": p.ID,
			},
			Raw: p,
		})
	}

	return results
}

// parseJiraTimestamp converts a Jira ISO8601 timestamp string to time.Time.
func parseJiraTimestamp(ts string) time.Time {
	if ts == "" {
		return time.Time{}
	}

	// Jira typically uses ISO8601 format: "2023-01-15T10:30:45.123-0500"
	// Try parsing with timezone.
	t, err := time.Parse(time.RFC3339, ts)
	if err == nil {
		return t
	}

	// Try without timezone offset.
	t, err = time.Parse("2006-01-02T15:04:05.000", ts)
	if err == nil {
		return t
	}

	// Try basic ISO format.
	t, err = time.Parse("2006-01-02T15:04:05", ts)
	if err == nil {
		return t
	}

	return time.Time{}
}

func init() {
	scraper.RegisterMode(&JiraMode{})
}
