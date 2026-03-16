package gmail

import (
	"testing"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/scraper"
	"github.com/inovacc/scout/pkg/scout/scraper/auth"
)

// --- Mode metadata tests ---

func TestGmailMode_Name(t *testing.T) {
	m := &GmailMode{}
	if got := m.Name(); got != "gmail" {
		t.Errorf("Name() = %q, want %q", got, "gmail")
	}
}

func TestGmailMode_Description(t *testing.T) {
	m := &GmailMode{}
	if got := m.Description(); got == "" {
		t.Error("Description() is empty")
	}
}

func TestGmailMode_AuthProvider(t *testing.T) {
	m := &GmailMode{}
	p := m.AuthProvider()
	if p == nil {
		t.Fatal("AuthProvider() is nil")
	}
	if p.Name() != "gmail" {
		t.Errorf("AuthProvider().Name() = %q, want %q", p.Name(), "gmail")
	}
}

// --- gmailProvider tests ---

func TestGmailProvider_LoginURL(t *testing.T) {
	p := &gmailProvider{}
	if got := p.LoginURL(); got != "https://accounts.google.com/" {
		t.Errorf("LoginURL() = %q", got)
	}
}

// --- ValidateSession tests ---

func TestValidateSession_NilSession(t *testing.T) {
	p := &gmailProvider{}
	err := p.ValidateSession(nil, nil)
	if err == nil {
		t.Fatal("expected error for nil session")
	}
}

func TestValidateSession_WithSSID(t *testing.T) {
	p := &gmailProvider{}
	s := &auth.Session{
		Cookies: []scout.Cookie{{Name: "SSID", Value: "abc"}},
	}
	if err := p.ValidateSession(nil, s); err != nil {
		t.Errorf("ValidateSession() error = %v", err)
	}
}

func TestValidateSession_WithSID(t *testing.T) {
	p := &gmailProvider{}
	s := &auth.Session{
		Cookies: []scout.Cookie{{Name: "SID", Value: "xyz"}},
	}
	if err := p.ValidateSession(nil, s); err != nil {
		t.Errorf("ValidateSession() error = %v", err)
	}
}

func TestValidateSession_WithHSID(t *testing.T) {
	p := &gmailProvider{}
	s := &auth.Session{
		Cookies: []scout.Cookie{{Name: "HSID", Value: "123"}},
	}
	if err := p.ValidateSession(nil, s); err != nil {
		t.Errorf("ValidateSession() error = %v", err)
	}
}

func TestValidateSession_NoCookies(t *testing.T) {
	p := &gmailProvider{}
	s := &auth.Session{
		Cookies: []scout.Cookie{},
	}
	err := p.ValidateSession(nil, s)
	if err == nil {
		t.Fatal("expected error for missing session cookies")
	}
	if _, ok := err.(*scraper.AuthError); !ok {
		t.Errorf("expected *scraper.AuthError, got %T", err)
	}
}

func TestValidateSession_WrongCookies(t *testing.T) {
	p := &gmailProvider{}
	s := &auth.Session{
		Cookies: []scout.Cookie{
			{Name: "other", Value: "val"},
			{Name: "APISID", Value: "notvalid"},
		},
	}
	err := p.ValidateSession(nil, s)
	if err == nil {
		t.Fatal("expected error for wrong cookies")
	}
}

// --- buildTargetSet tests ---

func TestBuildTargetSet_Empty(t *testing.T) {
	set := buildTargetSet(nil)
	if set != nil {
		t.Errorf("expected nil for empty targets, got %v", set)
	}
}

func TestBuildTargetSet_WithTargets(t *testing.T) {
	set := buildTargetSet([]string{"INBOX", "Sent", "SPAM"})

	tests := []struct {
		key  string
		want bool
	}{
		{"inbox", true},
		{"sent", true},
		{"spam", true},
		{"INBOX", false}, // stored lowercase
		{"drafts", false},
	}

	for _, tt := range tests {
		_, ok := set[tt.key]
		if ok != tt.want {
			t.Errorf("set[%q] = %v, want %v", tt.key, ok, tt.want)
		}
	}
}

// --- extractStringField tests ---

func TestExtractStringField(t *testing.T) {
	m := map[string]any{
		"name":  "John",
		"count": 42,
		"nil":   nil,
	}

	if got := extractStringField(m, "name"); got != "John" {
		t.Errorf("extractStringField(name) = %q, want %q", got, "John")
	}
	if got := extractStringField(m, "count"); got != "" {
		t.Errorf("extractStringField(count) = %q, want empty", got)
	}
	if got := extractStringField(m, "missing"); got != "" {
		t.Errorf("extractStringField(missing) = %q, want empty", got)
	}
	if got := extractStringField(m, "nil"); got != "" {
		t.Errorf("extractStringField(nil) = %q, want empty", got)
	}
}

// --- extractInt64Field tests ---

func TestExtractInt64Field(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]any
		key  string
		want int64
	}{
		{"float64", map[string]any{"ts": float64(1609459200000)}, "ts", 1609459200000},
		{"int64", map[string]any{"ts": int64(1609459200000)}, "ts", 1609459200000},
		{"int", map[string]any{"ts": int(42)}, "ts", 42},
		{"string value", map[string]any{"ts": "not-a-number"}, "ts", 0},
		{"missing key", map[string]any{}, "ts", 0},
		{"nil value", map[string]any{"ts": nil}, "ts", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractInt64Field(tt.m, tt.key); got != tt.want {
				t.Errorf("extractInt64Field() = %d, want %d", got, tt.want)
			}
		})
	}
}

// --- parseLabelsList tests ---

func TestParseLabelsList_ValidResponse(t *testing.T) {
	body := `[
		{"id": "INBOX", "name": "Inbox", "type": "system", "count": 100, "unread": 5, "color": ""},
		{"id": "SENT", "name": "Sent", "type": "system", "count": 50, "unread": 0, "color": "blue"}
	]`

	results := parseLabelsList(body)
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}

	r := results[0]
	if r.Type != scraper.ResultChannel {
		t.Errorf("Type = %q, want %q", r.Type, scraper.ResultChannel)
	}
	if r.ID != "INBOX" {
		t.Errorf("ID = %q", r.ID)
	}
	if r.Content != "Inbox" {
		t.Errorf("Content = %q", r.Content)
	}
	if r.Source != "gmail" {
		t.Errorf("Source = %q", r.Source)
	}
	if r.Metadata["label_type"] != "system" {
		t.Errorf("Metadata[label_type] = %v", r.Metadata["label_type"])
	}
	if r.Metadata["count"] != 100 {
		t.Errorf("Metadata[count] = %v", r.Metadata["count"])
	}
	if r.Metadata["unread"] != 5 {
		t.Errorf("Metadata[unread] = %v", r.Metadata["unread"])
	}
}

func TestParseLabelsList_InvalidJSON(t *testing.T) {
	results := parseLabelsList("not json")
	if len(results) != 0 {
		t.Errorf("expected 0 results for invalid JSON, got %d", len(results))
	}
}

func TestParseLabelsList_EmptyArray(t *testing.T) {
	results := parseLabelsList("[]")
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty array, got %d", len(results))
	}
}

// --- parseContactsList tests ---

func TestParseContactsList_ValidResponse(t *testing.T) {
	body := `[
		{"id": "C001", "name": "Alice Smith", "email": "alice@example.com", "phone": "+1234567890", "image": "https://img.example.com/alice.jpg"},
		{"id": "C002", "name": "Bob Jones", "email": "bob@example.com", "phone": "", "image": ""}
	]`

	results := parseContactsList(body)
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}

	r := results[0]
	if r.Type != scraper.ResultProfile {
		t.Errorf("Type = %q, want %q", r.Type, scraper.ResultProfile)
	}
	if r.ID != "C001" {
		t.Errorf("ID = %q", r.ID)
	}
	if r.Author != "Alice Smith" {
		t.Errorf("Author = %q", r.Author)
	}
	if r.Source != "gmail" {
		t.Errorf("Source = %q", r.Source)
	}
	if r.Metadata["email"] != "alice@example.com" {
		t.Errorf("Metadata[email] = %v", r.Metadata["email"])
	}
	if r.Metadata["phone"] != "+1234567890" {
		t.Errorf("Metadata[phone] = %v", r.Metadata["phone"])
	}
}

func TestParseContactsList_InvalidJSON(t *testing.T) {
	results := parseContactsList("not json")
	if len(results) != 0 {
		t.Errorf("expected 0 results for invalid JSON, got %d", len(results))
	}
}

func TestParseContactsList_EmptyArray(t *testing.T) {
	results := parseContactsList("[]")
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty array, got %d", len(results))
	}
}

// --- parseEmailList tests ---

func TestParseEmailList_ValidResponse(t *testing.T) {
	body := `{
		"response": [
			["thread-1", "data1", "data2", "data3", ["INBOX", "IMPORTANT"]],
			["thread-2", "data1", "data2", "data3", ["SPAM"]]
		]
	}`

	results := parseEmailList(body, nil)
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}

	r := results[0]
	if r.Type != scraper.ResultMessage {
		t.Errorf("Type = %q, want %q", r.Type, scraper.ResultMessage)
	}
	if r.Source != "gmail" {
		t.Errorf("Source = %q", r.Source)
	}
	if r.ID != "thread-1" {
		t.Errorf("ID = %q", r.ID)
	}

	labels, ok := r.Metadata["labels"].([]string)
	if !ok {
		t.Fatalf("labels type = %T", r.Metadata["labels"])
	}
	if len(labels) != 2 || labels[0] != "INBOX" {
		t.Errorf("labels = %v", labels)
	}
}

func TestParseEmailList_WithTargetFilter(t *testing.T) {
	body := `{
		"response": [
			["thread-1", "a", "b", "c", ["INBOX"]],
			["thread-2", "a", "b", "c", ["SPAM"]]
		]
	}`

	targetSet := buildTargetSet([]string{"inbox"})
	results := parseEmailList(body, targetSet)

	if len(results) != 1 {
		t.Fatalf("got %d results, want 1 (filtered)", len(results))
	}
	if results[0].ID != "thread-1" {
		t.Errorf("ID = %q, want thread-1", results[0].ID)
	}
}

func TestParseEmailList_ShortItems(t *testing.T) {
	body := `{"response": [["short"], ["too", "short"]]}`
	results := parseEmailList(body, nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results for short items, got %d", len(results))
	}
}

func TestParseEmailList_InvalidJSON(t *testing.T) {
	results := parseEmailList("not json", nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results for invalid JSON, got %d", len(results))
	}
}

func TestParseEmailList_EmptyResponse(t *testing.T) {
	body := `{"response": []}`
	results := parseEmailList(body, nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty response, got %d", len(results))
	}
}

// --- parseEmailThread tests ---

func TestParseEmailThread_ValidResponse(t *testing.T) {
	body := `{
		"response": [
			["thread-1", {"from": "alice@example.com", "subject": "Hello", "body": "Hi there", "timestamp": 1609459200000}]
		]
	}`

	results := parseEmailThread(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}

	r := results[0]
	if r.Type != scraper.ResultEmail {
		t.Errorf("Type = %q, want %q", r.Type, scraper.ResultEmail)
	}
	if r.Author != "alice@example.com" {
		t.Errorf("Author = %q", r.Author)
	}
	if r.Content != "Hi there" {
		t.Errorf("Content = %q", r.Content)
	}
	if r.Metadata["subject"] != "Hello" {
		t.Errorf("Metadata[subject] = %v", r.Metadata["subject"])
	}
}

func TestParseEmailThread_ShortItems(t *testing.T) {
	body := `{"response": [["only-id"]]}`
	results := parseEmailThread(body, nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results for short items, got %d", len(results))
	}
}

func TestParseEmailThread_NonObjectData(t *testing.T) {
	body := `{"response": [["thread-1", "not-an-object"]]}`
	results := parseEmailThread(body, nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results for non-object data, got %d", len(results))
	}
}

func TestParseEmailThread_InvalidJSON(t *testing.T) {
	results := parseEmailThread("not json", nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results for invalid JSON, got %d", len(results))
	}
}
