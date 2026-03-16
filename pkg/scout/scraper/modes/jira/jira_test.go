package jira

import (
	"context"
	"testing"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/scraper"
	"github.com/inovacc/scout/pkg/scout/scraper/auth"
)

// --- Mode metadata tests ---

func TestJiraMode_Name(t *testing.T) {
	m := &JiraMode{}
	if got := m.Name(); got != "jira" {
		t.Errorf("Name() = %q, want %q", got, "jira")
	}
}

func TestJiraMode_Description(t *testing.T) {
	m := &JiraMode{}
	if got := m.Description(); got == "" {
		t.Error("Description() is empty")
	}
}

func TestJiraMode_AuthProvider(t *testing.T) {
	m := &JiraMode{}
	p := m.AuthProvider()
	if p == nil {
		t.Fatal("AuthProvider() is nil")
	}
	if p.Name() != "jira" {
		t.Errorf("AuthProvider().Name() = %q, want %q", p.Name(), "jira")
	}
}

// --- jiraProvider tests ---

func TestJiraProvider_Name(t *testing.T) {
	p := &jiraProvider{}
	if got := p.Name(); got != "jira" {
		t.Errorf("Name() = %q, want %q", got, "jira")
	}
}

func TestJiraProvider_LoginURL(t *testing.T) {
	p := &jiraProvider{}
	if got := p.LoginURL(); got != "https://id.atlassian.com/login" {
		t.Errorf("LoginURL() = %q", got)
	}
}

// --- ValidateSession tests ---

func TestValidateSession_NilSession(t *testing.T) {
	p := &jiraProvider{}
	err := p.ValidateSession(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil session")
	}
}

func TestValidateSession_WithTokenInTokens(t *testing.T) {
	p := &jiraProvider{}
	s := &auth.Session{
		Tokens: map[string]string{"cloud.session.token": "abc123"},
	}
	if err := p.ValidateSession(context.Background(), s); err != nil {
		t.Errorf("ValidateSession() error = %v", err)
	}
}

func TestValidateSession_WithTokenInCookies(t *testing.T) {
	p := &jiraProvider{}
	s := &auth.Session{
		Tokens:  map[string]string{},
		Cookies: []scout.Cookie{{Name: "cloud.session.token", Value: "abc"}},
	}
	if err := p.ValidateSession(context.Background(), s); err != nil {
		t.Errorf("ValidateSession() error = %v", err)
	}
}

func TestValidateSession_MissingToken(t *testing.T) {
	p := &jiraProvider{}
	s := &auth.Session{
		Tokens: map[string]string{"other": "val"},
	}
	err := p.ValidateSession(context.Background(), s)
	if err == nil {
		t.Fatal("expected error for missing session token")
	}
}

// --- buildTargetSet tests ---

func TestBuildTargetSet_Empty(t *testing.T) {
	set := buildTargetSet(nil)
	if set != nil {
		t.Errorf("buildTargetSet(nil) = %v, want nil", set)
	}
}

func TestBuildTargetSet_NormalizesCase(t *testing.T) {
	set := buildTargetSet([]string{"MyProject"})
	if _, ok := set["MYPROJECT"]; !ok {
		t.Error("expected uppercase key in set")
	}
	if _, ok := set["myproject"]; !ok {
		t.Error("expected lowercase key in set")
	}
}

// --- parseJiraTimestamp tests ---

func TestParseJiraTimestamp_Empty(t *testing.T) {
	ts := parseJiraTimestamp("")
	if !ts.IsZero() {
		t.Errorf("parseJiraTimestamp('') = %v, want zero", ts)
	}
}

func TestParseJiraTimestamp_RFC3339(t *testing.T) {
	ts := parseJiraTimestamp("2024-01-15T10:30:00Z")
	if ts.IsZero() {
		t.Error("parseJiraTimestamp(RFC3339) returned zero time")
	}
	if ts.Year() != 2024 || ts.Month() != 1 || ts.Day() != 15 {
		t.Errorf("parseJiraTimestamp() = %v", ts)
	}
}

func TestParseJiraTimestamp_WithMillis(t *testing.T) {
	ts := parseJiraTimestamp("2024-01-15T10:30:45.123")
	if ts.IsZero() {
		t.Error("parseJiraTimestamp(millis) returned zero time")
	}
}

func TestParseJiraTimestamp_BasicISO(t *testing.T) {
	ts := parseJiraTimestamp("2024-01-15T10:30:45")
	if ts.IsZero() {
		t.Error("parseJiraTimestamp(basic ISO) returned zero time")
	}
}

func TestParseJiraTimestamp_Invalid(t *testing.T) {
	ts := parseJiraTimestamp("not-a-date")
	if !ts.IsZero() {
		t.Errorf("parseJiraTimestamp(invalid) = %v, want zero", ts)
	}
}

// --- parseIssuesSearch tests ---

func TestParseIssuesSearch_Valid(t *testing.T) {
	body := `{
		"issues": [{
			"key": "PROJ-1",
			"id": "10001",
			"fields": {
				"summary": "Fix the bug",
				"status": {"name": "Open", "id": "1"},
				"priority": {"name": "High", "id": "2"},
				"project": {"key": "PROJ", "id": "100", "name": "Project"},
				"reporter": {"displayName": "Alice"},
				"created": "2024-01-15T10:30:00Z",
				"updated": "2024-01-16T10:30:00Z",
				"labels": ["bug"],
				"comment": {"comments": [], "total": 0},
				"attachment": []
			}
		}],
		"total": 1,
		"maxResults": 50
	}`

	results := parseIssuesSearch(body, nil)
	if len(results) != 1 {
		t.Fatalf("parseIssuesSearch() returned %d results, want 1", len(results))
	}
	r := results[0]
	if r.ID != "PROJ-1" {
		t.Errorf("ID = %q, want %q", r.ID, "PROJ-1")
	}
	if r.Author != "Alice" {
		t.Errorf("Author = %q, want %q", r.Author, "Alice")
	}
	if r.Content != "Fix the bug" {
		t.Errorf("Content = %q, want %q", r.Content, "Fix the bug")
	}
	if r.Metadata["status"] != "Open" {
		t.Errorf("status = %v, want %q", r.Metadata["status"], "Open")
	}
}

func TestParseIssuesSearch_WithTargetFilter(t *testing.T) {
	body := `{
		"issues": [{
			"key": "PROJ-1",
			"id": "10001",
			"fields": {
				"summary": "Match",
				"status": {"name": "Open"},
				"priority": {"name": "High"},
				"project": {"key": "PROJ"},
				"created": "2024-01-15T10:30:00Z",
				"comment": {"comments": []},
				"attachment": []
			}
		},{
			"key": "OTHER-1",
			"id": "10002",
			"fields": {
				"summary": "No match",
				"status": {"name": "Open"},
				"priority": {"name": "High"},
				"project": {"key": "OTHER"},
				"created": "2024-01-15T10:30:00Z",
				"comment": {"comments": []},
				"attachment": []
			}
		}],
		"total": 2
	}`

	targetSet := map[string]struct{}{"PROJ": {}, "proj": {}}
	results := parseIssuesSearch(body, targetSet)
	if len(results) != 1 {
		t.Fatalf("parseIssuesSearch() with target filter returned %d results, want 1", len(results))
	}
	if results[0].ID != "PROJ-1" {
		t.Errorf("ID = %q, want %q", results[0].ID, "PROJ-1")
	}
}

func TestParseIssuesSearch_WithCommentsAndAttachments(t *testing.T) {
	body := `{
		"issues": [{
			"key": "PROJ-1",
			"id": "10001",
			"fields": {
				"summary": "Bug",
				"status": {"name": "Open"},
				"priority": {"name": "High"},
				"project": {"key": "PROJ"},
				"reporter": {"displayName": "Alice"},
				"assignee": {"displayName": "Bob"},
				"created": "2024-01-15T10:30:00Z",
				"comment": {
					"comments": [{"id": "c1", "author": {"displayName": "Charlie"}, "body": "Comment text", "created": "2024-01-16T10:00:00Z"}],
					"total": 1
				},
				"attachment": [{"id": "a1", "filename": "file.txt", "mimeType": "text/plain", "size": 100, "created": "2024-01-16T11:00:00Z", "author": {"displayName": "Dave"}, "content": "https://jira/attachment/a1"}]
			}
		}],
		"total": 1
	}`

	results := parseIssuesSearch(body, nil)
	// 1 issue + 1 comment + 1 attachment = 3
	if len(results) != 3 {
		t.Fatalf("parseIssuesSearch() returned %d results, want 3", len(results))
	}
	if results[0].Metadata["assignee"] != "Bob" {
		t.Errorf("assignee = %v, want %q", results[0].Metadata["assignee"], "Bob")
	}
	if results[1].Type != scraper.ResultComment {
		t.Errorf("comment Type = %q, want %q", results[1].Type, scraper.ResultComment)
	}
	if results[2].Type != scraper.ResultFile {
		t.Errorf("attachment Type = %q, want %q", results[2].Type, scraper.ResultFile)
	}
}

func TestParseIssuesSearch_InvalidJSON(t *testing.T) {
	results := parseIssuesSearch("not json", nil)
	if results != nil {
		t.Errorf("parseIssuesSearch(invalid) = %v, want nil", results)
	}
}

// --- parseIssueDetail tests ---

func TestParseIssueDetail_Valid(t *testing.T) {
	body := `{
		"key": "PROJ-5",
		"id": "10005",
		"fields": {
			"summary": "Single issue",
			"status": {"name": "Done"},
			"priority": {"name": "Low"},
			"project": {"key": "PROJ"},
			"reporter": {"displayName": "Zoe"},
			"created": "2024-02-01T08:00:00Z",
			"comment": {"comments": []},
			"attachment": []
		}
	}`

	results := parseIssueDetail(body, nil)
	if len(results) != 1 {
		t.Fatalf("parseIssueDetail() returned %d results, want 1", len(results))
	}
	if results[0].Content != "Single issue" {
		t.Errorf("Content = %q, want %q", results[0].Content, "Single issue")
	}
}

func TestParseIssueDetail_Filtered(t *testing.T) {
	body := `{
		"key": "OTHER-1",
		"id": "10001",
		"fields": {
			"summary": "Filtered out",
			"status": {"name": "Open"},
			"priority": {"name": "High"},
			"project": {"key": "OTHER"},
			"created": "2024-01-01T00:00:00Z",
			"comment": {"comments": []},
			"attachment": []
		}
	}`

	targetSet := map[string]struct{}{"PROJ": {}}
	results := parseIssueDetail(body, targetSet)
	if results != nil {
		t.Errorf("parseIssueDetail() with filter should return nil, got %d", len(results))
	}
}

// --- parseBoards tests ---

func TestParseBoards_Valid(t *testing.T) {
	body := `{"values": [{"id": 1, "name": "Sprint Board", "type": "scrum"}], "total": 1}`
	results := parseBoards(body, nil)
	if len(results) != 1 {
		t.Fatalf("parseBoards() returned %d results, want 1", len(results))
	}
	if results[0].Content != "Sprint Board" {
		t.Errorf("Content = %q, want %q", results[0].Content, "Sprint Board")
	}
	if results[0].Type != scraper.ResultChannel {
		t.Errorf("Type = %q, want %q", results[0].Type, scraper.ResultChannel)
	}
}

func TestParseBoards_WithFilter(t *testing.T) {
	body := `{"values": [{"id": 1, "name": "Board 1", "type": "scrum"}, {"id": 2, "name": "Board 2", "type": "kanban"}], "total": 2}`
	targetSet := map[string]struct{}{"1": {}}
	results := parseBoards(body, targetSet)
	if len(results) != 1 {
		t.Fatalf("parseBoards() with filter returned %d, want 1", len(results))
	}
}

// --- parseSprints tests ---

func TestParseSprints_Valid(t *testing.T) {
	startDate := "2024-01-01T00:00:00Z"
	endDate := "2024-01-14T00:00:00Z"
	body := `{"values": [{"id": 10, "name": "Sprint 1", "state": "active", "startDate": "` + startDate + `", "endDate": "` + endDate + `", "createdDate": "2023-12-15T00:00:00Z"}], "total": 1}`

	results := parseSprints(body, nil)
	if len(results) != 1 {
		t.Fatalf("parseSprints() returned %d results, want 1", len(results))
	}
	if results[0].Content != "Sprint 1" {
		t.Errorf("Content = %q, want %q", results[0].Content, "Sprint 1")
	}
	if results[0].Metadata["state"] != "active" {
		t.Errorf("state = %v, want %q", results[0].Metadata["state"], "active")
	}
	if results[0].Metadata["start_date"] != startDate {
		t.Errorf("start_date = %v, want %q", results[0].Metadata["start_date"], startDate)
	}
}

// --- parseUsers tests ---

func TestParseUsers_Array(t *testing.T) {
	body := `[{"accountId":"acc1","displayName":"Alice","emailAddress":"alice@example.com"}]`
	results := parseUsers(body)
	if len(results) != 1 {
		t.Fatalf("parseUsers() returned %d results, want 1", len(results))
	}
	if results[0].Author != "Alice" {
		t.Errorf("Author = %q, want %q", results[0].Author, "Alice")
	}
	if results[0].ID != "acc1" {
		t.Errorf("ID = %q, want %q", results[0].ID, "acc1")
	}
}

func TestParseUsers_SingleUser(t *testing.T) {
	body := `{"name":"bob","key":"bob-key","displayName":"Bob"}`
	results := parseUsers(body)
	if len(results) != 1 {
		t.Fatalf("parseUsers() returned %d results, want 1", len(results))
	}
	if results[0].ID != "bob-key" {
		t.Errorf("ID = %q, want %q (fallback to key)", results[0].ID, "bob-key")
	}
}

func TestParseUsers_FallbackToName(t *testing.T) {
	body := `[{"name":"charlie","displayName":"Charlie"}]`
	results := parseUsers(body)
	if len(results) != 1 {
		t.Fatalf("parseUsers() returned %d results, want 1", len(results))
	}
	if results[0].ID != "charlie" {
		t.Errorf("ID = %q, want %q (fallback to name)", results[0].ID, "charlie")
	}
}

func TestParseUsers_InvalidJSON(t *testing.T) {
	results := parseUsers("not json")
	if results != nil {
		t.Errorf("parseUsers(invalid) = %v, want nil", results)
	}
}

// --- parseProjects tests ---

func TestParseProjects_ArrayResponse(t *testing.T) {
	body := `[{"key":"PROJ","id":"100","name":"My Project"}]`
	results := parseProjects(body, nil)
	if len(results) != 1 {
		t.Fatalf("parseProjects() returned %d results, want 1", len(results))
	}
	if results[0].ID != "PROJ" {
		t.Errorf("ID = %q, want %q", results[0].ID, "PROJ")
	}
	if results[0].Type != scraper.ResultProfile {
		t.Errorf("Type = %q, want %q", results[0].Type, scraper.ResultProfile)
	}
}

func TestParseProjects_ObjectResponse(t *testing.T) {
	body := `{"values":[{"key":"TEAM","id":"200","name":"Team Project"}]}`
	results := parseProjects(body, nil)
	if len(results) != 1 {
		t.Fatalf("parseProjects() returned %d results, want 1", len(results))
	}
}

func TestParseProjects_WithFilter(t *testing.T) {
	body := `[{"key":"PROJ","id":"100","name":"Match"},{"key":"OTHER","id":"200","name":"No match"}]`
	targetSet := map[string]struct{}{"PROJ": {}}
	results := parseProjects(body, targetSet)
	if len(results) != 1 {
		t.Fatalf("parseProjects() with filter returned %d, want 1", len(results))
	}
}

// --- parseHijackEvent routing tests ---

func TestParseHijackEvent_NonResponse(t *testing.T) {
	ev := scout.HijackEvent{Type: scout.HijackEventRequest}
	results := parseHijackEvent(ev, nil)
	if results != nil {
		t.Errorf("expected nil for non-response event, got %d", len(results))
	}
}

func TestParseHijackEvent_EmptyBody(t *testing.T) {
	ev := scout.HijackEvent{
		Type:     scout.HijackEventResponse,
		Response: &scout.CapturedResponse{URL: "https://jira/rest/api/2/search", Body: ""},
	}
	results := parseHijackEvent(ev, nil)
	if results != nil {
		t.Errorf("expected nil for empty body, got %d", len(results))
	}
}

func TestParseHijackEvent_UnknownEndpoint(t *testing.T) {
	ev := scout.HijackEvent{
		Type:     scout.HijackEventResponse,
		Response: &scout.CapturedResponse{URL: "https://jira/rest/unknown/endpoint", Body: `{}`},
	}
	results := parseHijackEvent(ev, nil)
	if results != nil {
		t.Errorf("expected nil for unknown endpoint, got %d", len(results))
	}
}

func TestParseHijackEvent_IssuesSearch(t *testing.T) {
	ev := scout.HijackEvent{
		Type: scout.HijackEventResponse,
		Response: &scout.CapturedResponse{
			URL:       "https://jira.example.com/rest/api/2/search",
			Body:      `{"issues":[{"key":"T-1","id":"1","fields":{"summary":"Test","status":{"name":"Open"},"priority":{"name":"High"},"project":{"key":"T"},"created":"2024-01-01T00:00:00Z","comment":{"comments":[]},"attachment":[]}}],"total":1}`,
			Timestamp: time.Now(),
		},
	}
	results := parseHijackEvent(ev, nil)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}
