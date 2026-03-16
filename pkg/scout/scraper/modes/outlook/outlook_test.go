package outlook

import (
	"context"
	"testing"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/scraper"
	"github.com/inovacc/scout/pkg/scout/scraper/auth"
)

// --- Mode metadata tests ---

func TestOutlookMode_Name(t *testing.T) {
	m := &OutlookMode{}
	if got := m.Name(); got != "outlook" {
		t.Errorf("Name() = %q, want %q", got, "outlook")
	}
}

func TestOutlookMode_Description(t *testing.T) {
	m := &OutlookMode{}
	if got := m.Description(); got == "" {
		t.Error("Description() is empty")
	}
}

func TestOutlookMode_AuthProvider(t *testing.T) {
	m := &OutlookMode{}
	p := m.AuthProvider()
	if p == nil {
		t.Fatal("AuthProvider() is nil")
	}
	if p.Name() != "outlook" {
		t.Errorf("AuthProvider().Name() = %q, want %q", p.Name(), "outlook")
	}
}

// --- outlookProvider tests ---

func TestOutlookProvider_Name(t *testing.T) {
	p := &outlookProvider{}
	if got := p.Name(); got != "outlook" {
		t.Errorf("Name() = %q, want %q", got, "outlook")
	}
}

func TestOutlookProvider_LoginURL(t *testing.T) {
	p := &outlookProvider{}
	if got := p.LoginURL(); got != "https://login.microsoftonline.com/" {
		t.Errorf("LoginURL() = %q", got)
	}
}

// --- ValidateSession tests ---

func TestValidateSession_NilSession(t *testing.T) {
	p := &outlookProvider{}
	err := p.ValidateSession(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil session")
	}
}

func TestValidateSession_WrongProvider(t *testing.T) {
	p := &outlookProvider{}
	s := &auth.Session{Provider: "other"}
	err := p.ValidateSession(context.Background(), s)
	if err == nil {
		t.Fatal("expected error for wrong provider")
	}
}

func TestValidateSession_WithTokens(t *testing.T) {
	p := &outlookProvider{}
	s := &auth.Session{
		Provider: "outlook",
		Tokens:   map[string]string{"authToken": "abc"},
	}
	if err := p.ValidateSession(context.Background(), s); err != nil {
		t.Errorf("ValidateSession() error = %v", err)
	}
}

func TestValidateSession_WithLocalStorage(t *testing.T) {
	p := &outlookProvider{}
	s := &auth.Session{
		Provider:     "outlook",
		LocalStorage: map[string]string{"o365SessionInfo": "abc"},
	}
	if err := p.ValidateSession(context.Background(), s); err != nil {
		t.Errorf("ValidateSession() error = %v", err)
	}
}

func TestValidateSession_WithCookies(t *testing.T) {
	p := &outlookProvider{}
	s := &auth.Session{
		Provider: "outlook",
		Cookies:  []scout.Cookie{{Name: "session", Value: "abc"}},
	}
	if err := p.ValidateSession(context.Background(), s); err != nil {
		t.Errorf("ValidateSession() error = %v", err)
	}
}

func TestValidateSession_NoAuth(t *testing.T) {
	p := &outlookProvider{}
	s := &auth.Session{Provider: "outlook"}
	err := p.ValidateSession(context.Background(), s)
	if err == nil {
		t.Fatal("expected error for no auth data")
	}
}

// --- extractArray / stringVal / parseTime helper tests ---

func TestExtractArray_Valid(t *testing.T) {
	data := map[string]any{"value": []any{"a", "b"}}
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
}

func TestParseTime_Invalid(t *testing.T) {
	ts := parseTime("not-a-date")
	if !ts.IsZero() {
		t.Errorf("parseTime(invalid) = %v, want zero", ts)
	}
}

// --- parseFolders tests ---

func TestParseFolders_Valid(t *testing.T) {
	m := &OutlookMode{}
	data := map[string]any{
		"value": []any{
			map[string]any{
				"id":               "f1",
				"displayName":     "Inbox",
				"unreadItemCount": float64(5),
				"totalItemCount":  float64(100),
				"createdDateTime": "2024-01-01T00:00:00Z",
			},
		},
	}
	results := make(chan scraper.Result, 10)
	n := m.parseFolders(data, results)
	close(results)

	if n != 1 {
		t.Fatalf("parseFolders() returned %d, want 1", n)
	}
	r := <-results
	if r.Type != scraper.ResultChannel {
		t.Errorf("Type = %q, want %q", r.Type, scraper.ResultChannel)
	}
	if r.Content != "Inbox" {
		t.Errorf("Content = %q, want %q", r.Content, "Inbox")
	}
}

func TestParseFolders_Empty(t *testing.T) {
	m := &OutlookMode{}
	data := map[string]any{"value": []any{}}
	results := make(chan scraper.Result, 10)
	n := m.parseFolders(data, results)
	close(results)
	if n != 0 {
		t.Errorf("parseFolders(empty) returned %d, want 0", n)
	}
}

// --- parseEmails tests ---

func TestParseEmails_Valid(t *testing.T) {
	m := &OutlookMode{}
	data := map[string]any{
		"value": []any{
			map[string]any{
				"id":      "e1",
				"subject": "Meeting Notes",
				"from": map[string]any{
					"emailAddress": map[string]any{"address": "alice@example.com"},
				},
				"toRecipients": []any{
					map[string]any{
						"emailAddress": map[string]any{"address": "bob@example.com"},
					},
				},
				"body":             map[string]any{"content": "Here are the notes..."},
				"receivedDateTime": "2024-01-15T10:30:00Z",
				"hasAttachments":   true,
				"isRead":           false,
				"importance":       "high",
			},
		},
	}
	results := make(chan scraper.Result, 10)
	n := m.parseEmails(data, "https://example.com/messages", results)
	close(results)

	if n != 1 {
		t.Fatalf("parseEmails() returned %d, want 1", n)
	}
	r := <-results
	if r.Type != scraper.ResultEmail {
		t.Errorf("Type = %q, want %q", r.Type, scraper.ResultEmail)
	}
	if r.Author != "alice@example.com" {
		t.Errorf("Author = %q, want %q", r.Author, "alice@example.com")
	}
	if r.Content != "Meeting Notes" {
		t.Errorf("Content = %q, want %q", r.Content, "Meeting Notes")
	}
	if r.Metadata["body"] != "Here are the notes..." {
		t.Errorf("body = %v", r.Metadata["body"])
	}
	toAddrs, ok := r.Metadata["to_recipients"].([]string)
	if !ok || len(toAddrs) != 1 || toAddrs[0] != "bob@example.com" {
		t.Errorf("to_recipients = %v", r.Metadata["to_recipients"])
	}
}

func TestParseEmails_FallbackMessagesKey(t *testing.T) {
	m := &OutlookMode{}
	data := map[string]any{
		"messages": []any{
			map[string]any{
				"id":      "e2",
				"subject": "Via messages key",
			},
		},
	}
	results := make(chan scraper.Result, 10)
	n := m.parseEmails(data, "", results)
	close(results)
	if n != 1 {
		t.Fatalf("parseEmails() via 'messages' key returned %d, want 1", n)
	}
}

// --- parseContacts tests ---

func TestParseContacts_Valid(t *testing.T) {
	m := &OutlookMode{}
	data := map[string]any{
		"value": []any{
			map[string]any{
				"id":          "c1",
				"displayName": "Charlie",
				"givenName":   "Charlie",
				"surname":     "Brown",
				"mobilePhone": "+1234567890",
				"companyName": "Acme",
				"emailAddresses": []any{
					map[string]any{"address": "charlie@example.com"},
				},
				"createdDateTime": "2024-01-01T00:00:00Z",
			},
		},
	}
	results := make(chan scraper.Result, 10)
	n := m.parseContacts(data, results)
	close(results)

	if n != 1 {
		t.Fatalf("parseContacts() returned %d, want 1", n)
	}
	r := <-results
	if r.Type != scraper.ResultProfile {
		t.Errorf("Type = %q, want %q", r.Type, scraper.ResultProfile)
	}
	if r.Author != "Charlie" {
		t.Errorf("Author = %q, want %q", r.Author, "Charlie")
	}
	emails, ok := r.Metadata["emails"].([]string)
	if !ok || len(emails) != 1 {
		t.Errorf("emails = %v", r.Metadata["emails"])
	}
}

// --- parseMeetings tests ---

func TestParseMeetings_Valid(t *testing.T) {
	m := &OutlookMode{}
	data := map[string]any{
		"value": []any{
			map[string]any{
				"id":      "m1",
				"subject": "Weekly Sync",
				"webLink": "https://outlook/meeting/m1",
				"attendees": []any{
					map[string]any{
						"emailAddress": map[string]any{"address": "alice@example.com"},
					},
				},
				"isOnlineMeeting": true,
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
	if r.Content != "Weekly Sync" {
		t.Errorf("Content = %q, want %q", r.Content, "Weekly Sync")
	}
	attendees, ok := r.Metadata["attendees"].([]string)
	if !ok || len(attendees) != 1 || attendees[0] != "alice@example.com" {
		t.Errorf("attendees = %v", r.Metadata["attendees"])
	}
}

// --- parseGraphEmails tests ---

func TestParseGraphEmails_Valid(t *testing.T) {
	m := &OutlookMode{}
	data := map[string]any{
		"value": []any{
			map[string]any{
				"id":      "ge1",
				"subject": "Graph Email",
				"from": map[string]any{
					"emailAddress": map[string]any{"address": "sender@example.com"},
				},
				"bodyPreview":     "Preview text",
				"receivedDateTime": "2024-01-15T10:30:00Z",
				"hasAttachments":  false,
				"isRead":          true,
			},
		},
	}
	results := make(chan scraper.Result, 10)
	n := m.parseGraphEmails(data, "https://graph.microsoft.com/messages", results)
	close(results)

	if n != 1 {
		t.Fatalf("parseGraphEmails() returned %d, want 1", n)
	}
	r := <-results
	if r.Type != scraper.ResultEmail {
		t.Errorf("Type = %q, want %q", r.Type, scraper.ResultEmail)
	}
	if r.Author != "sender@example.com" {
		t.Errorf("Author = %q, want %q", r.Author, "sender@example.com")
	}
	if r.Metadata["body_preview"] != "Preview text" {
		t.Errorf("body_preview = %v", r.Metadata["body_preview"])
	}
}

// --- parseGraphEvents tests ---

func TestParseGraphEvents_Valid(t *testing.T) {
	m := &OutlookMode{}
	data := map[string]any{
		"value": []any{
			map[string]any{
				"id":            "ev1",
				"subject":       "Standup",
				"webLink":       "https://outlook/event/ev1",
				"isReminderOn":  true,
			},
		},
	}
	results := make(chan scraper.Result, 10)
	n := m.parseGraphEvents(data, results)
	close(results)

	if n != 1 {
		t.Fatalf("parseGraphEvents() returned %d, want 1", n)
	}
	r := <-results
	if r.Type != scraper.ResultMeeting {
		t.Errorf("Type = %q, want %q", r.Type, scraper.ResultMeeting)
	}
}

// --- parseGraphContacts tests ---

func TestParseGraphContacts_Valid(t *testing.T) {
	m := &OutlookMode{}
	data := map[string]any{
		"value": []any{
			map[string]any{
				"id":          "gc1",
				"displayName": "Graph Contact",
				"givenName":   "Test",
				"surname":     "User",
				"mobilePhone": "+1234567890",
				"companyName": "Corp",
				"createdDateTime": "2024-01-01T00:00:00Z",
			},
		},
	}
	results := make(chan scraper.Result, 10)
	n := m.parseGraphContacts(data, results)
	close(results)

	if n != 1 {
		t.Fatalf("parseGraphContacts() returned %d, want 1", n)
	}
	r := <-results
	if r.Type != scraper.ResultProfile {
		t.Errorf("Type = %q, want %q", r.Type, scraper.ResultProfile)
	}
}

// --- parseResponse routing tests ---

func TestParseResponse_Folders(t *testing.T) {
	m := &OutlookMode{}
	resp := &scout.CapturedResponse{
		URL:       "https://outlook.live.com/api/v2/me/mailfolders",
		Body:      `{"value":[{"id":"f1","displayName":"Inbox"}]}`,
		Timestamp: time.Now(),
	}
	results := make(chan scraper.Result, 10)
	n := m.parseResponse(resp, results)
	close(results)
	if n != 1 {
		t.Errorf("parseResponse(folders) returned %d, want 1", n)
	}
}

func TestParseResponse_GraphMessages(t *testing.T) {
	m := &OutlookMode{}
	resp := &scout.CapturedResponse{
		URL:       "https://graph.microsoft.com/v1.0/me/messages",
		Body:      `{"value":[{"id":"m1","subject":"Test","from":{"emailAddress":{"address":"a@b.com"}},"receivedDateTime":"2024-01-15T10:00:00Z"}]}`,
		Timestamp: time.Now(),
	}
	results := make(chan scraper.Result, 10)
	n := m.parseResponse(resp, results)
	close(results)
	if n != 1 {
		t.Errorf("parseResponse(graph messages) returned %d, want 1", n)
	}
}

func TestParseResponse_InvalidJSON(t *testing.T) {
	m := &OutlookMode{}
	resp := &scout.CapturedResponse{
		URL:       "https://outlook.live.com/api/v2/me/messages",
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
	m := &OutlookMode{}
	resp := &scout.CapturedResponse{
		URL:       "https://example.com/api/v2/unknown",
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
