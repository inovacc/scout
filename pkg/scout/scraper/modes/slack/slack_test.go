package slack

import (
	"testing"
	"time"

	"github.com/inovacc/scout/pkg/scout/scraper"
	"github.com/inovacc/scout/pkg/scout/scraper/auth"
)

// --- Mode metadata tests ---

func TestSlackMode_Name(t *testing.T) {
	m := &SlackMode{}
	if got := m.Name(); got != "slack" {
		t.Errorf("Name() = %q, want %q", got, "slack")
	}
}

func TestSlackMode_Description(t *testing.T) {
	m := &SlackMode{}
	if got := m.Description(); got == "" {
		t.Error("Description() is empty")
	}
}

func TestSlackMode_AuthProvider(t *testing.T) {
	m := &SlackMode{}
	p := m.AuthProvider()
	if p == nil {
		t.Fatal("AuthProvider() is nil")
	}
	if p.Name() != "slack" {
		t.Errorf("AuthProvider().Name() = %q, want %q", p.Name(), "slack")
	}
}

// --- slackProvider tests ---

func TestSlackProvider_LoginURL(t *testing.T) {
	p := &slackProvider{}
	if got := p.LoginURL(); got != "https://slack.com/signin" {
		t.Errorf("LoginURL() = %q", got)
	}
}

// --- ValidateSession tests ---

func TestValidateSession_NilSession(t *testing.T) {
	p := &slackProvider{}
	err := p.ValidateSession(nil, nil)
	if err == nil {
		t.Fatal("expected error for nil session")
	}
}

func TestValidateSession_ValidXoxc(t *testing.T) {
	p := &slackProvider{}
	s := &auth.Session{
		Tokens: map[string]string{"api_token": "xoxc-1234567890"},
	}
	if err := p.ValidateSession(nil, s); err != nil {
		t.Errorf("ValidateSession() error = %v", err)
	}
}

func TestValidateSession_ValidXoxs(t *testing.T) {
	p := &slackProvider{}
	s := &auth.Session{
		Tokens: map[string]string{"session": "xoxs-abcdef"},
	}
	if err := p.ValidateSession(nil, s); err != nil {
		t.Errorf("ValidateSession() error = %v", err)
	}
}

func TestValidateSession_NoValidToken(t *testing.T) {
	p := &slackProvider{}
	s := &auth.Session{
		Tokens: map[string]string{"api_token": "invalid-token"},
	}
	err := p.ValidateSession(nil, s)
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
	if _, ok := err.(*scraper.AuthError); !ok {
		t.Errorf("expected *scraper.AuthError, got %T", err)
	}
}

func TestValidateSession_EmptyTokens(t *testing.T) {
	p := &slackProvider{}
	s := &auth.Session{
		Tokens: map[string]string{},
	}
	err := p.ValidateSession(nil, s)
	if err == nil {
		t.Fatal("expected error for empty tokens")
	}
}

// --- extractTokens tests ---

func TestExtractTokens_FlatMap(t *testing.T) {
	tokens := make(map[string]string)
	m := map[string]any{
		"api_token": "xoxc-12345",
		"session":   "xoxs-67890",
		"name":      "not-a-token",
	}

	extractTokens(m, tokens)

	if tokens["api_token"] != "xoxc-12345" {
		t.Errorf("api_token = %q", tokens["api_token"])
	}
	if tokens["session"] != "xoxs-67890" {
		t.Errorf("session = %q", tokens["session"])
	}
	if _, ok := tokens["name"]; ok {
		t.Error("non-token key should not be extracted")
	}
}

func TestExtractTokens_NestedMap(t *testing.T) {
	tokens := make(map[string]string)
	m := map[string]any{
		"config": map[string]any{
			"deep": map[string]any{
				"token": "xoxc-deep-token",
			},
		},
	}

	extractTokens(m, tokens)

	if tokens["token"] != "xoxc-deep-token" {
		t.Errorf("token = %q, want 'xoxc-deep-token'", tokens["token"])
	}
}

func TestExtractTokens_EmptyMap(t *testing.T) {
	tokens := make(map[string]string)
	extractTokens(map[string]any{}, tokens)

	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens, got %d", len(tokens))
	}
}

func TestExtractTokens_NonStringValues(t *testing.T) {
	tokens := make(map[string]string)
	m := map[string]any{
		"count":   42,
		"enabled": true,
		"list":    []any{"xoxc-in-array"}, // arrays are not traversed
	}

	extractTokens(m, tokens)

	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens, got %d", len(tokens))
	}
}

// --- buildTargetSet tests ---

func TestBuildTargetSet_Empty(t *testing.T) {
	set := buildTargetSet(nil)
	if set != nil {
		t.Errorf("expected nil for empty targets, got %v", set)
	}

	set = buildTargetSet([]string{})
	if set != nil {
		t.Errorf("expected nil for empty slice, got %v", set)
	}
}

func TestBuildTargetSet_WithTargets(t *testing.T) {
	set := buildTargetSet([]string{"#General", "random", "#Dev-Team"})

	tests := []struct {
		key  string
		want bool
	}{
		{"general", true},
		{"random", true},
		{"dev-team", true},
		{"#general", false}, // hash prefix is stripped
		{"other", false},
	}

	for _, tt := range tests {
		_, ok := set[tt.key]
		if ok != tt.want {
			t.Errorf("set[%q] = %v, want %v", tt.key, ok, tt.want)
		}
	}
}

// --- parseSlackTimestamp tests ---

func TestParseSlackTimestamp(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantSec int64
	}{
		{"empty", "", 0},
		{"integer", "1234567890", 1234567890},
		{"with fractional", "1234567890.123456", 1234567890},
		{"zero", "0", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSlackTimestamp(tt.input)
			if tt.input == "" {
				if !got.IsZero() {
					t.Errorf("expected zero time for empty input")
				}
				return
			}
			if got.Unix() != tt.wantSec {
				t.Errorf("parseSlackTimestamp(%q).Unix() = %d, want %d", tt.input, got.Unix(), tt.wantSec)
			}
		})
	}
}

func TestParseSlackTimestamp_FractionalNanoseconds(t *testing.T) {
	ts := parseSlackTimestamp("1234567890.123456")
	// "123456" padded to 9 digits = "123456000" = 123456000 ns
	if ns := ts.Nanosecond(); ns != 123456000 {
		t.Errorf("Nanosecond() = %d, want 123456000", ns)
	}
}

func TestParseSlackTimestamp_InvalidInput(t *testing.T) {
	got := parseSlackTimestamp("not-a-number")
	if !got.IsZero() {
		t.Errorf("expected zero time for invalid input, got %v", got)
	}
}

// --- parseChannelsList tests ---

func TestParseChannelsList_ValidResponse(t *testing.T) {
	body := `{
		"ok": true,
		"channels": [
			{"id": "C001", "name": "general", "topic": {"value": "General chat"}, "purpose": {"value": "Main channel"}, "num_members": 42, "created": "1609459200"},
			{"id": "C002", "name": "random", "topic": {"value": ""}, "purpose": {"value": ""}, "num_members": 10, "created": "1609459300"}
		]
	}`

	results := parseChannelsList(body, nil)
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}

	r := results[0]
	if r.Type != scraper.ResultChannel {
		t.Errorf("Type = %q, want %q", r.Type, scraper.ResultChannel)
	}
	if r.ID != "C001" {
		t.Errorf("ID = %q, want %q", r.ID, "C001")
	}
	if r.Content != "General chat" {
		t.Errorf("Content = %q", r.Content)
	}
	if r.Metadata["name"] != "general" {
		t.Errorf("Metadata[name] = %v", r.Metadata["name"])
	}
	if r.Metadata["num_members"] != 42 {
		t.Errorf("Metadata[num_members] = %v", r.Metadata["num_members"])
	}
}

func TestParseChannelsList_WithTargetFilter(t *testing.T) {
	body := `{
		"ok": true,
		"channels": [
			{"id": "C001", "name": "general", "topic": {"value": ""}, "purpose": {"value": ""}, "num_members": 1, "created": "0"},
			{"id": "C002", "name": "random", "topic": {"value": ""}, "purpose": {"value": ""}, "num_members": 1, "created": "0"}
		]
	}`

	targetSet := buildTargetSet([]string{"general"})
	results := parseChannelsList(body, targetSet)

	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].ID != "C001" {
		t.Errorf("ID = %q, want C001", results[0].ID)
	}
}

func TestParseChannelsList_NotOK(t *testing.T) {
	body := `{"ok": false, "error": "not_authed"}`
	results := parseChannelsList(body, nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results for not-ok response, got %d", len(results))
	}
}

func TestParseChannelsList_InvalidJSON(t *testing.T) {
	results := parseChannelsList("not json", nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results for invalid JSON, got %d", len(results))
	}
}

// --- parseUsersList tests ---

func TestParseUsersList_ValidResponse(t *testing.T) {
	body := `{
		"ok": true,
		"members": [
			{
				"id": "U001",
				"name": "jdoe",
				"real_name": "John Doe",
				"deleted": false,
				"is_bot": false,
				"profile": {"display_name": "JD", "email": "jdoe@example.com", "image_72": "https://img.example.com/72.png"}
			},
			{
				"id": "U002",
				"name": "botuser",
				"real_name": "Bot",
				"deleted": false,
				"is_bot": true,
				"profile": {"display_name": "Bot", "email": "", "image_72": ""}
			}
		]
	}`

	results := parseUsersList(body)
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}

	r := results[0]
	if r.Type != scraper.ResultUser {
		t.Errorf("Type = %q, want %q", r.Type, scraper.ResultUser)
	}
	if r.ID != "U001" {
		t.Errorf("ID = %q", r.ID)
	}
	if r.Author != "John Doe" {
		t.Errorf("Author = %q", r.Author)
	}
	if r.Metadata["email"] != "jdoe@example.com" {
		t.Errorf("Metadata[email] = %v", r.Metadata["email"])
	}
	if r.Metadata["is_bot"] != false {
		t.Errorf("Metadata[is_bot] = %v", r.Metadata["is_bot"])
	}
}

func TestParseUsersList_NotOK(t *testing.T) {
	results := parseUsersList(`{"ok": false}`)
	if len(results) != 0 {
		t.Errorf("expected 0 results for not-ok, got %d", len(results))
	}
}

// --- parseFilesList tests ---

func TestParseFilesList_ValidResponse(t *testing.T) {
	body := `{
		"ok": true,
		"files": [
			{
				"id": "F001",
				"name": "report.pdf",
				"title": "Q4 Report",
				"mimetype": "application/pdf",
				"size": 1024,
				"url_private_download": "https://files.slack.com/files/report.pdf",
				"user": "U001",
				"timestamp": "1609459200"
			}
		]
	}`

	results := parseFilesList(body)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}

	r := results[0]
	if r.Type != scraper.ResultFile {
		t.Errorf("Type = %q, want %q", r.Type, scraper.ResultFile)
	}
	if r.ID != "F001" {
		t.Errorf("ID = %q", r.ID)
	}
	if r.Content != "Q4 Report" {
		t.Errorf("Content = %q", r.Content)
	}
	if r.Author != "U001" {
		t.Errorf("Author = %q", r.Author)
	}
	if r.URL != "https://files.slack.com/files/report.pdf" {
		t.Errorf("URL = %q", r.URL)
	}
	if r.Metadata["mimetype"] != "application/pdf" {
		t.Errorf("Metadata[mimetype] = %v", r.Metadata["mimetype"])
	}
	if r.Metadata["size"] != int64(1024) {
		t.Errorf("Metadata[size] = %v (type %T)", r.Metadata["size"], r.Metadata["size"])
	}
}

// --- parseConversationHistory tests ---

func TestParseConversationHistory_ValidResponse(t *testing.T) {
	body := `{
		"ok": true,
		"messages": [
			{
				"type": "message",
				"user": "U001",
				"text": "Hello world",
				"ts": "1609459200.000100",
				"channel": "C001"
			},
			{
				"type": "message",
				"user": "U002",
				"text": "Threaded reply",
				"ts": "1609459300.000200",
				"thread_ts": "1609459200.000100",
				"channel": "C001"
			}
		]
	}`

	results := parseConversationHistory(body, nil)
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}

	// First message
	r := results[0]
	if r.Type != scraper.ResultMessage {
		t.Errorf("Type = %q, want %q", r.Type, scraper.ResultMessage)
	}
	if r.Author != "U001" {
		t.Errorf("Author = %q", r.Author)
	}
	if r.Content != "Hello world" {
		t.Errorf("Content = %q", r.Content)
	}

	// Threaded reply
	r2 := results[1]
	if r2.Metadata["thread_ts"] != "1609459200.000100" {
		t.Errorf("thread_ts = %v", r2.Metadata["thread_ts"])
	}
}

func TestParseConversationHistory_WithReactions(t *testing.T) {
	body := `{
		"ok": true,
		"messages": [
			{
				"type": "message",
				"user": "U001",
				"text": "React to this",
				"ts": "1609459200.000100",
				"reactions": [
					{"name": "thumbsup", "users": ["U002", "U003"], "count": 2}
				]
			}
		]
	}`

	results := parseConversationHistory(body, nil)
	// 1 message + 1 reaction = 2 results
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}

	reaction := results[1]
	if reaction.Type != scraper.ResultReaction {
		t.Errorf("Type = %q, want %q", reaction.Type, scraper.ResultReaction)
	}
	if reaction.Content != "thumbsup" {
		t.Errorf("Content = %q", reaction.Content)
	}
	if reaction.Metadata["count"] != 2 {
		t.Errorf("Metadata[count] = %v", reaction.Metadata["count"])
	}
}

func TestParseConversationHistory_WithFiles(t *testing.T) {
	body := `{
		"ok": true,
		"messages": [
			{
				"type": "message",
				"user": "U001",
				"text": "See attached",
				"ts": "1609459200.000100",
				"files": [
					{"id": "F001", "name": "doc.txt", "title": "Document", "mimetype": "text/plain", "size": 256, "url_private_download": "https://example.com/doc.txt", "user": "U001", "timestamp": "1609459200"}
				]
			}
		]
	}`

	results := parseConversationHistory(body, nil)
	// 1 message + 1 file = 2 results
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}

	file := results[1]
	if file.Type != scraper.ResultFile {
		t.Errorf("Type = %q, want %q", file.Type, scraper.ResultFile)
	}
	if file.ID != "F001" {
		t.Errorf("ID = %q", file.ID)
	}
}

func TestParseConversationHistory_SkipsNonMessage(t *testing.T) {
	body := `{
		"ok": true,
		"messages": [
			{"type": "channel_join", "user": "U001", "text": "joined", "ts": "1000"}
		]
	}`

	results := parseConversationHistory(body, nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results for non-message type, got %d", len(results))
	}
}

// --- fileToResult tests ---

func TestFileToResult(t *testing.T) {
	f := slackFile{
		ID:                 "F999",
		Name:               "photo.jpg",
		Title:              "My Photo",
		Mimetype:           "image/jpeg",
		Size:               2048,
		URLPrivateDownload: "https://files.slack.com/photo.jpg",
		User:               "U001",
		Timestamp:          "1609459200",
	}

	r := fileToResult(f)
	if r.Type != scraper.ResultFile {
		t.Errorf("Type = %q", r.Type)
	}
	if r.Source != "slack" {
		t.Errorf("Source = %q", r.Source)
	}
	if r.ID != "F999" {
		t.Errorf("ID = %q", r.ID)
	}
	if r.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
	if r.Timestamp != time.Unix(1609459200, 0) {
		t.Errorf("Timestamp = %v", r.Timestamp)
	}
}

// --- parseHijackEvent tests (URL routing) ---
// These require scout.HijackEvent which depends on the scout package.
// We test the underlying parsers instead above.

func TestParseChannelsList_EmptyChannels(t *testing.T) {
	body := `{"ok": true, "channels": []}`
	results := parseChannelsList(body, nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}
