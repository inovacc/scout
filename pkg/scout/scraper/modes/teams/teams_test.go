package teams

import (
	"context"
	"testing"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/scraper"
	"github.com/inovacc/scout/pkg/scout/scraper/auth"
)

// --- Mode metadata tests ---

func TestTeamsMode_Name(t *testing.T) {
	m := &TeamsMode{}
	if got := m.Name(); got != "teams" {
		t.Errorf("Name() = %q, want %q", got, "teams")
	}
}

func TestTeamsMode_Description(t *testing.T) {
	m := &TeamsMode{}
	if got := m.Description(); got == "" {
		t.Error("Description() is empty")
	}
}

func TestTeamsMode_AuthProvider(t *testing.T) {
	m := &TeamsMode{}
	p := m.AuthProvider()
	if p == nil {
		t.Fatal("AuthProvider() is nil")
	}
	if p.Name() != "teams" {
		t.Errorf("AuthProvider().Name() = %q, want %q", p.Name(), "teams")
	}
}

// --- teamsProvider tests ---

func TestTeamsProvider_Name(t *testing.T) {
	p := &teamsProvider{}
	if got := p.Name(); got != "teams" {
		t.Errorf("Name() = %q, want %q", got, "teams")
	}
}

func TestTeamsProvider_LoginURL(t *testing.T) {
	p := &teamsProvider{}
	if got := p.LoginURL(); got != "https://teams.microsoft.com" {
		t.Errorf("LoginURL() = %q", got)
	}
}

// --- ValidateSession tests ---

func TestValidateSession_NilSession(t *testing.T) {
	p := &teamsProvider{}
	err := p.ValidateSession(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil session")
	}
}

func TestValidateSession_WrongProvider(t *testing.T) {
	p := &teamsProvider{}
	s := &auth.Session{Provider: "other", Tokens: map[string]string{"skypeToken": "abc"}}
	err := p.ValidateSession(context.Background(), s)
	if err == nil {
		t.Fatal("expected error for wrong provider")
	}
}

func TestValidateSession_WithSkypeToken(t *testing.T) {
	p := &teamsProvider{}
	s := &auth.Session{Provider: "teams", Tokens: map[string]string{"skypeToken": "abc"}}
	if err := p.ValidateSession(context.Background(), s); err != nil {
		t.Errorf("ValidateSession() error = %v", err)
	}
}

func TestValidateSession_WithAuthToken(t *testing.T) {
	p := &teamsProvider{}
	s := &auth.Session{Provider: "teams", Tokens: map[string]string{"authToken": "xyz"}}
	if err := p.ValidateSession(context.Background(), s); err != nil {
		t.Errorf("ValidateSession() error = %v", err)
	}
}

func TestValidateSession_WithLatestToken(t *testing.T) {
	p := &teamsProvider{}
	s := &auth.Session{Provider: "teams", Tokens: map[string]string{"latestToken": "xyz"}}
	if err := p.ValidateSession(context.Background(), s); err != nil {
		t.Errorf("ValidateSession() error = %v", err)
	}
}

func TestValidateSession_MissingTokens(t *testing.T) {
	p := &teamsProvider{}
	s := &auth.Session{Provider: "teams", Tokens: map[string]string{"chatToken": "abc"}}
	err := p.ValidateSession(context.Background(), s)
	if err == nil {
		t.Fatal("expected error for missing required tokens")
	}
}

func TestValidateSession_EmptyTokenValue(t *testing.T) {
	p := &teamsProvider{}
	s := &auth.Session{Provider: "teams", Tokens: map[string]string{"skypeToken": ""}}
	err := p.ValidateSession(context.Background(), s)
	if err == nil {
		t.Fatal("expected error for empty token value")
	}
}

// --- extractArray / stringVal / parseTime helper tests ---

func TestExtractArray_Valid(t *testing.T) {
	data := map[string]any{
		"value": []any{"a", "b"},
	}
	arr := extractArray(data, "value")
	if len(arr) != 2 {
		t.Errorf("extractArray() len = %d, want 2", len(arr))
	}
}

func TestExtractArray_Missing(t *testing.T) {
	data := map[string]any{}
	arr := extractArray(data, "value")
	if arr != nil {
		t.Errorf("extractArray() = %v, want nil", arr)
	}
}

func TestExtractArray_WrongType(t *testing.T) {
	data := map[string]any{"value": "not-an-array"}
	arr := extractArray(data, "value")
	if arr != nil {
		t.Errorf("extractArray() = %v, want nil", arr)
	}
}

func TestStringVal_Valid(t *testing.T) {
	m := map[string]any{"key": "value"}
	if got := stringVal(m, "key"); got != "value" {
		t.Errorf("stringVal() = %q, want %q", got, "value")
	}
}

func TestStringVal_Missing(t *testing.T) {
	m := map[string]any{}
	if got := stringVal(m, "key"); got != "" {
		t.Errorf("stringVal() = %q, want empty", got)
	}
}

func TestStringVal_WrongType(t *testing.T) {
	m := map[string]any{"key": 42}
	if got := stringVal(m, "key"); got != "" {
		t.Errorf("stringVal() = %q, want empty", got)
	}
}

func TestParseTime_Empty(t *testing.T) {
	ts := parseTime("")
	if !ts.IsZero() {
		t.Errorf("parseTime('') = %v, want zero", ts)
	}
}

func TestParseTime_RFC3339(t *testing.T) {
	ts := parseTime("2024-01-15T10:30:00Z")
	if ts.IsZero() {
		t.Error("parseTime(RFC3339) returned zero time")
	}
	if ts.Year() != 2024 {
		t.Errorf("year = %d, want 2024", ts.Year())
	}
}

func TestParseTime_RFC3339Nano(t *testing.T) {
	ts := parseTime("2024-01-15T10:30:00.123456789Z")
	if ts.IsZero() {
		t.Error("parseTime(RFC3339Nano) returned zero time")
	}
}

func TestParseTime_ShortZ(t *testing.T) {
	ts := parseTime("2024-01-15T10:30:45Z")
	if ts.IsZero() {
		t.Error("parseTime(short Z) returned zero time")
	}
}

func TestParseTime_Invalid(t *testing.T) {
	ts := parseTime("not-a-date")
	if !ts.IsZero() {
		t.Errorf("parseTime(invalid) = %v, want zero", ts)
	}
}

// --- parseChats tests ---

func TestParseChats_Valid(t *testing.T) {
	m := &TeamsMode{}
	data := map[string]any{
		"value": []any{
			map[string]any{
				"id":                  "chat1",
				"topic":              "Team Chat",
				"chatType":           "group",
				"lastUpdatedDateTime": "2024-01-15T10:30:00Z",
			},
		},
	}
	results := make(chan scraper.Result, 10)
	n := m.parseChats(data, results)
	close(results)

	if n != 1 {
		t.Fatalf("parseChats() returned %d, want 1", n)
	}
	r := <-results
	if r.Type != scraper.ResultThread {
		t.Errorf("Type = %q, want %q", r.Type, scraper.ResultThread)
	}
	if r.ID != "chat1" {
		t.Errorf("ID = %q, want %q", r.ID, "chat1")
	}
	if r.Content != "Team Chat" {
		t.Errorf("Content = %q, want %q", r.Content, "Team Chat")
	}
}

func TestParseChats_Empty(t *testing.T) {
	m := &TeamsMode{}
	data := map[string]any{"value": []any{}}
	results := make(chan scraper.Result, 10)
	n := m.parseChats(data, results)
	close(results)
	if n != 0 {
		t.Errorf("parseChats(empty) returned %d, want 0", n)
	}
}

// --- parseMessages tests ---

func TestParseMessages_WithValueKey(t *testing.T) {
	m := &TeamsMode{}
	data := map[string]any{
		"value": []any{
			map[string]any{
				"id":              "msg1",
				"createdDateTime": "2024-01-15T10:30:00Z",
				"messageType":    "message",
				"importance":     "normal",
				"body":           map[string]any{"content": "Hello Teams"},
				"from": map[string]any{
					"user": map[string]any{"displayName": "Alice"},
				},
			},
		},
	}
	results := make(chan scraper.Result, 10)
	n := m.parseMessages(data, "https://example.com/messages", results)
	close(results)

	if n != 1 {
		t.Fatalf("parseMessages() returned %d, want 1", n)
	}
	r := <-results
	if r.Type != scraper.ResultMessage {
		t.Errorf("Type = %q, want %q", r.Type, scraper.ResultMessage)
	}
	if r.Author != "Alice" {
		t.Errorf("Author = %q, want %q", r.Author, "Alice")
	}
	if r.Content != "Hello Teams" {
		t.Errorf("Content = %q, want %q", r.Content, "Hello Teams")
	}
}

func TestParseMessages_WithMessagesKey(t *testing.T) {
	m := &TeamsMode{}
	data := map[string]any{
		"messages": []any{
			map[string]any{
				"id":   "msg2",
				"body": map[string]any{"content": "Via messages key"},
			},
		},
	}
	results := make(chan scraper.Result, 10)
	n := m.parseMessages(data, "", results)
	close(results)
	if n != 1 {
		t.Fatalf("parseMessages() via 'messages' key returned %d, want 1", n)
	}
}

// --- parseMembers tests ---

func TestParseMembers_Valid(t *testing.T) {
	m := &TeamsMode{}
	data := map[string]any{
		"value": []any{
			map[string]any{
				"id":          "user1",
				"displayName": "Bob",
				"email":       "bob@example.com",
				"roles":       []any{"owner"},
			},
		},
	}
	results := make(chan scraper.Result, 10)
	n := m.parseMembers(data, "https://example.com", results)
	close(results)

	if n != 1 {
		t.Fatalf("parseMembers() returned %d, want 1", n)
	}
	r := <-results
	if r.Type != scraper.ResultUser {
		t.Errorf("Type = %q, want %q", r.Type, scraper.ResultUser)
	}
	if r.Author != "Bob" {
		t.Errorf("Author = %q, want %q", r.Author, "Bob")
	}
}

// --- parseChannels tests ---

func TestParseChannels_Valid(t *testing.T) {
	m := &TeamsMode{}
	data := map[string]any{
		"value": []any{
			map[string]any{
				"id":              "ch1",
				"displayName":    "General",
				"description":    "Main channel",
				"membershipType": "standard",
				"webUrl":         "https://teams.microsoft.com/channel/ch1",
				"createdDateTime": "2024-01-01T00:00:00Z",
			},
		},
	}
	results := make(chan scraper.Result, 10)
	n := m.parseChannels(data, results)
	close(results)

	if n != 1 {
		t.Fatalf("parseChannels() returned %d, want 1", n)
	}
	r := <-results
	if r.Type != scraper.ResultChannel {
		t.Errorf("Type = %q, want %q", r.Type, scraper.ResultChannel)
	}
	if r.Content != "General" {
		t.Errorf("Content = %q, want %q", r.Content, "General")
	}
}

// --- parseFiles tests ---

func TestParseFiles_Array(t *testing.T) {
	m := &TeamsMode{}
	data := map[string]any{
		"value": []any{
			map[string]any{
				"id":                   "f1",
				"name":                 "report.docx",
				"webUrl":              "https://example.com/files/f1",
				"size":                float64(1024),
				"lastModifiedDateTime": "2024-01-15T10:30:00Z",
			},
		},
	}
	results := make(chan scraper.Result, 10)
	n := m.parseFiles(data, "https://example.com/files", results)
	close(results)

	if n != 1 {
		t.Fatalf("parseFiles() returned %d, want 1", n)
	}
	r := <-results
	if r.Type != scraper.ResultFile {
		t.Errorf("Type = %q, want %q", r.Type, scraper.ResultFile)
	}
	if r.Content != "report.docx" {
		t.Errorf("Content = %q, want %q", r.Content, "report.docx")
	}
}

func TestParseFiles_SingleFile(t *testing.T) {
	m := &TeamsMode{}
	data := map[string]any{
		"id":                   "f2",
		"name":                 "single.pdf",
		"webUrl":              "https://example.com/files/f2",
		"lastModifiedDateTime": "2024-01-15T10:30:00Z",
	}
	results := make(chan scraper.Result, 10)
	n := m.parseFiles(data, "https://example.com/files/f2", results)
	close(results)

	if n != 1 {
		t.Fatalf("parseFiles() single file returned %d, want 1", n)
	}
	r := <-results
	if r.Content != "single.pdf" {
		t.Errorf("Content = %q, want %q", r.Content, "single.pdf")
	}
}

// --- parseMeetings tests ---

func TestParseMeetings_Valid(t *testing.T) {
	m := &TeamsMode{}
	data := map[string]any{
		"value": []any{
			map[string]any{
				"id":            "meet1",
				"subject":       "Weekly Standup",
				"joinWebUrl":    "https://teams.microsoft.com/meet/meet1",
				"startDateTime": "2024-01-15T09:00:00Z",
				"endDateTime":   "2024-01-15T09:30:00Z",
			},
		},
	}
	results := make(chan scraper.Result, 10)
	n := m.parseMeetings(data, results)
	close(results)

	if n != 1 {
		t.Fatalf("parseMeetings() returned %d, want 1", n)
	}
	r := <-results
	if r.Type != scraper.ResultMeeting {
		t.Errorf("Type = %q, want %q", r.Type, scraper.ResultMeeting)
	}
	if r.Content != "Weekly Standup" {
		t.Errorf("Content = %q, want %q", r.Content, "Weekly Standup")
	}
}

// --- parseResponse routing tests ---

func TestParseResponse_Chats(t *testing.T) {
	m := &TeamsMode{}
	resp := &scout.CapturedResponse{
		URL:       "https://graph.microsoft.com/beta/me/chats",
		Body:      `{"value":[{"id":"c1","topic":"Chat","chatType":"group","lastUpdatedDateTime":"2024-01-15T10:00:00Z"}]}`,
		Timestamp: time.Now(),
	}
	results := make(chan scraper.Result, 10)
	n := m.parseResponse(resp, results)
	close(results)
	if n != 1 {
		t.Errorf("parseResponse(chats) returned %d, want 1", n)
	}
}

func TestParseResponse_InvalidJSON(t *testing.T) {
	m := &TeamsMode{}
	resp := &scout.CapturedResponse{
		URL:       "https://example.com/api/v1/messages",
		Body:      "not json",
		Timestamp: time.Now(),
	}
	results := make(chan scraper.Result, 10)
	n := m.parseResponse(resp, results)
	close(results)
	if n != 0 {
		t.Errorf("parseResponse(invalid JSON) returned %d, want 0", n)
	}
}

func TestParseResponse_UnknownEndpoint(t *testing.T) {
	m := &TeamsMode{}
	resp := &scout.CapturedResponse{
		URL:       "https://example.com/api/v1/unknown",
		Body:      `{"data": "test"}`,
		Timestamp: time.Now(),
		RequestID: "req1",
	}
	results := make(chan scraper.Result, 10)
	n := m.parseResponse(resp, results)
	close(results)
	// Unknown endpoints emit raw metadata
	if n != 1 {
		t.Errorf("parseResponse(unknown) returned %d, want 1", n)
	}
	r := <-results
	if r.Metadata["raw_endpoint"] != true {
		t.Error("expected raw_endpoint metadata")
	}
}
