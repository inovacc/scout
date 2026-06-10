package slack

import (
	"context"
	"testing"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/scraper"
	"github.com/inovacc/scout/pkg/scout/scraper/auth"
)

// --- parseHijackEvent dispatch tests ---
//
// parseHijackEvent is pure logic: it inspects a scout.HijackEvent (a plain
// struct, constructible without a browser) and routes the captured response
// body to the correct parser based on the URL. These tests exercise the
// routing table plus the guard clauses, none of which require a live page.

func respEvent(url, body string) scout.HijackEvent {
	return scout.HijackEvent{
		Type: scout.HijackEventResponse,
		Response: &scout.CapturedResponse{
			URL:  url,
			Body: body,
		},
	}
}

func TestParseHijackEvent_RoutesConversationsList(t *testing.T) {
	body := `{"ok": true, "channels": [{"id": "C001", "name": "general", "topic": {"value": "hi"}, "purpose": {"value": ""}, "num_members": 3, "created": "1609459200"}]}`
	ev := respEvent("https://app.slack.com/api/conversations.list?x=1", body)

	results := parseHijackEvent(ev, nil)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Type != scraper.ResultChannel {
		t.Errorf("Type = %q, want %q", results[0].Type, scraper.ResultChannel)
	}
	if results[0].ID != "C001" {
		t.Errorf("ID = %q, want C001", results[0].ID)
	}
}

func TestParseHijackEvent_RoutesConversationsHistory(t *testing.T) {
	body := `{"ok": true, "messages": [{"type": "message", "user": "U001", "text": "hello", "ts": "1609459200.000100", "channel": "C001"}]}`
	ev := respEvent("https://app.slack.com/api/conversations.history", body)

	results := parseHijackEvent(ev, nil)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Type != scraper.ResultMessage {
		t.Errorf("Type = %q, want %q", results[0].Type, scraper.ResultMessage)
	}
}

func TestParseHijackEvent_RoutesUsersList(t *testing.T) {
	body := `{"ok": true, "members": [{"id": "U001", "name": "jdoe", "real_name": "John Doe", "profile": {"display_name": "JD", "email": "j@x.com"}}]}`
	ev := respEvent("https://app.slack.com/api/users.list", body)

	results := parseHijackEvent(ev, nil)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Type != scraper.ResultUser {
		t.Errorf("Type = %q, want %q", results[0].Type, scraper.ResultUser)
	}
}

func TestParseHijackEvent_RoutesFilesList(t *testing.T) {
	body := `{"ok": true, "files": [{"id": "F001", "name": "a.pdf", "title": "A", "mimetype": "application/pdf", "size": 10, "user": "U001", "timestamp": "1609459200"}]}`
	ev := respEvent("https://app.slack.com/api/files.list", body)

	results := parseHijackEvent(ev, nil)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Type != scraper.ResultFile {
		t.Errorf("Type = %q, want %q", results[0].Type, scraper.ResultFile)
	}
}

func TestParseHijackEvent_PassesTargetSetThrough(t *testing.T) {
	body := `{"ok": true, "channels": [
		{"id": "C001", "name": "general", "topic": {"value": ""}, "purpose": {"value": ""}, "num_members": 1, "created": "0"},
		{"id": "C002", "name": "random", "topic": {"value": ""}, "purpose": {"value": ""}, "num_members": 1, "created": "0"}
	]}`
	ev := respEvent("https://app.slack.com/api/conversations.list", body)

	results := parseHijackEvent(ev, buildTargetSet([]string{"random"}))
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1 (target filtered)", len(results))
	}
	if results[0].ID != "C002" {
		t.Errorf("ID = %q, want C002", results[0].ID)
	}
}

func TestParseHijackEvent_Guards(t *testing.T) {
	tests := []struct {
		name string
		ev   scout.HijackEvent
	}{
		{
			name: "request event ignored",
			ev: scout.HijackEvent{
				Type:    scout.HijackEventRequest,
				Request: &scout.CapturedRequest{URL: "https://app.slack.com/api/users.list"},
			},
		},
		{
			name: "response type but nil response struct",
			ev:   scout.HijackEvent{Type: scout.HijackEventResponse, Response: nil},
		},
		{
			name: "empty body",
			ev:   respEvent("https://app.slack.com/api/users.list", ""),
		},
		{
			name: "unrelated url",
			ev:   respEvent("https://app.slack.com/api/auth.test", `{"ok": true}`),
		},
		{
			name: "non-api url with slack-ish path",
			ev:   respEvent("https://app.slack.com/client/T01/C01", `{"ok": true}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseHijackEvent(tt.ev, nil); got != nil {
				t.Errorf("expected nil results, got %d", len(got))
			}
		})
	}
}

func TestParseHijackEvent_RoutedButNotOK(t *testing.T) {
	// Routing succeeds (URL matches) but the parser rejects a not-ok payload.
	ev := respEvent("https://app.slack.com/api/users.list", `{"ok": false, "error": "invalid_auth"}`)
	if got := parseHijackEvent(ev, nil); len(got) != 0 {
		t.Errorf("expected 0 results for not-ok payload, got %d", len(got))
	}
}

// --- parseFilesList edge cases ---

func TestParseFilesList_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
	}{
		{"not ok", `{"ok": false, "error": "x"}`, 0},
		{"invalid json", `{not json`, 0},
		{"empty files", `{"ok": true, "files": []}`, 0},
		{"missing files key", `{"ok": true}`, 0},
		{"two files", `{"ok": true, "files": [
			{"id": "F1", "name": "a", "title": "A", "user": "U1", "timestamp": "0"},
			{"id": "F2", "name": "b", "title": "B", "user": "U2", "timestamp": "0"}
		]}`, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseFilesList(tt.body); len(got) != tt.want {
				t.Errorf("got %d results, want %d", len(got), tt.want)
			}
		})
	}
}

// --- parseUsersList edge cases ---

func TestParseUsersList_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
	}{
		{"invalid json", `}{`, 0},
		{"empty members", `{"ok": true, "members": []}`, 0},
		{"missing members key", `{"ok": true}`, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseUsersList(tt.body); len(got) != tt.want {
				t.Errorf("got %d results, want %d", len(got), tt.want)
			}
		})
	}
}

func TestParseUsersList_DeletedAndBotFlags(t *testing.T) {
	body := `{"ok": true, "members": [
		{"id": "U001", "name": "ghost", "real_name": "Ghost", "deleted": true, "is_bot": false, "profile": {"display_name": "G", "email": "g@x.com"}}
	]}`

	results := parseUsersList(body)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	r := results[0]
	if r.Metadata["deleted"] != true {
		t.Errorf("Metadata[deleted] = %v, want true", r.Metadata["deleted"])
	}
	if r.Metadata["is_bot"] != false {
		t.Errorf("Metadata[is_bot] = %v, want false", r.Metadata["is_bot"])
	}
	if r.Metadata["display_name"] != "G" {
		t.Errorf("Metadata[display_name] = %v", r.Metadata["display_name"])
	}
	if r.Source != "slack" {
		t.Errorf("Source = %q, want slack", r.Source)
	}
}

// --- parseConversationHistory edge cases ---

func TestParseConversationHistory_EdgeCases(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
	}{
		{"not ok", `{"ok": false}`, 0},
		{"invalid json", `nope`, 0},
		{"empty messages", `{"ok": true, "messages": []}`, 0},
		{"only non-message types", `{"ok": true, "messages": [
			{"type": "channel_join", "user": "U1", "ts": "1"},
			{"type": "channel_leave", "user": "U2", "ts": "2"}
		]}`, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseConversationHistory(tt.body, nil); len(got) != tt.want {
				t.Errorf("got %d results, want %d", len(got), tt.want)
			}
		})
	}
}

func TestParseConversationHistory_TargetSetIgnored(t *testing.T) {
	// parseConversationHistory accepts a targetSet parameter but (by current
	// design) does not filter on it — every message is emitted regardless.
	// This test documents that behavior so a future change is intentional.
	body := `{"ok": true, "messages": [
		{"type": "message", "user": "U1", "text": "a", "ts": "1609459200.000100", "channel": "C001"}
	]}`

	withFilter := parseConversationHistory(body, buildTargetSet([]string{"nonexistent-channel"}))
	if len(withFilter) != 1 {
		t.Fatalf("targetSet should not filter messages; got %d, want 1", len(withFilter))
	}
}

func TestParseConversationHistory_ReactionsAndFilesOrdering(t *testing.T) {
	// A single message with two reactions and one file should emit, in order:
	// the message, then each reaction, then the file.
	body := `{"ok": true, "messages": [
		{
			"type": "message", "user": "U1", "text": "hi", "ts": "1609459200.000100", "channel": "C001",
			"reactions": [
				{"name": "tada", "users": ["U2"], "count": 1},
				{"name": "eyes", "users": ["U3", "U4"], "count": 2}
			],
			"files": [
				{"id": "F1", "name": "x.txt", "title": "X", "mimetype": "text/plain", "size": 5, "user": "U1", "timestamp": "1609459200"}
			]
		}
	]}`

	results := parseConversationHistory(body, nil)
	if len(results) != 4 {
		t.Fatalf("got %d results, want 4 (1 msg + 2 reactions + 1 file)", len(results))
	}

	if results[0].Type != scraper.ResultMessage {
		t.Errorf("results[0].Type = %q, want message", results[0].Type)
	}
	if results[1].Type != scraper.ResultReaction || results[1].Content != "tada" {
		t.Errorf("results[1] = {%q, %q}, want reaction/tada", results[1].Type, results[1].Content)
	}
	if results[1].ID != "1609459200.000100:tada" {
		t.Errorf("reaction ID = %q, want composite message_ts:name", results[1].ID)
	}
	if results[2].Type != scraper.ResultReaction || results[2].Content != "eyes" {
		t.Errorf("results[2] = {%q, %q}, want reaction/eyes", results[2].Type, results[2].Content)
	}
	if results[2].Metadata["count"] != 2 {
		t.Errorf("results[2].Metadata[count] = %v, want 2", results[2].Metadata["count"])
	}
	if results[3].Type != scraper.ResultFile || results[3].ID != "F1" {
		t.Errorf("results[3] = {%q, %q}, want file/F1", results[3].Type, results[3].ID)
	}
	// Reaction inherits the parent message timestamp and channel source.
	if results[1].Source != "C001" {
		t.Errorf("reaction Source = %q, want C001", results[1].Source)
	}
	if results[1].Timestamp != results[0].Timestamp {
		t.Errorf("reaction Timestamp = %v, want parent %v", results[1].Timestamp, results[0].Timestamp)
	}
}

func TestParseConversationHistory_NoThreadTSMetadataAbsent(t *testing.T) {
	body := `{"ok": true, "messages": [
		{"type": "message", "user": "U1", "text": "root", "ts": "1609459200.000100", "channel": "C001"}
	]}`

	results := parseConversationHistory(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if _, ok := results[0].Metadata["thread_ts"]; ok {
		t.Error("thread_ts metadata should be absent for non-threaded message")
	}
}

// --- DetectAuth / CaptureSession browser-free error paths ---
//
// Only the nil-page guard clauses are browser-free; the happy paths require a
// live page and are intentionally NOT tested here.

func TestDetectAuth_NilPage(t *testing.T) {
	p := &slackProvider{}
	ok, err := p.DetectAuth(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil page")
	}
	if ok {
		t.Error("expected ok=false for nil page")
	}
}

func TestCaptureSession_NilPage(t *testing.T) {
	p := &slackProvider{}
	s, err := p.CaptureSession(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil page")
	}
	if s != nil {
		t.Errorf("expected nil session, got %+v", s)
	}
}

// --- Scrape browser-free error paths ---
//
// Scrape returns early (before scout.New) for invalid sessions, so these
// paths never launch a browser.

// fakeSession is a SessionData that is NOT *auth.Session, to exercise the
// type-assertion failure branch of Scrape.
type fakeSession struct{}

func (fakeSession) ProviderName() string { return "fake" }

func TestScrape_InvalidSessionType(t *testing.T) {
	m := &SlackMode{}
	ch, err := m.Scrape(context.Background(), fakeSession{}, scraper.ScrapeOptions{})
	if err == nil {
		t.Fatal("expected error for non-*auth.Session session")
	}
	if ch != nil {
		t.Error("expected nil channel on error")
	}
}

func TestScrape_NilSession(t *testing.T) {
	m := &SlackMode{}
	// A typed-nil *auth.Session passes the type assertion but is caught by the
	// explicit nil check.
	var s *auth.Session
	ch, err := m.Scrape(context.Background(), s, scraper.ScrapeOptions{})
	if err == nil {
		t.Fatal("expected error for nil *auth.Session")
	}
	if ch != nil {
		t.Error("expected nil channel on error")
	}
}

func TestScrape_SessionFailsValidation(t *testing.T) {
	m := &SlackMode{}
	// Valid type, non-nil, but no usable xoxc-/xoxs- token -> ValidateSession
	// fails and Scrape returns before launching a browser.
	s := &auth.Session{
		Provider: "slack",
		Tokens:   map[string]string{"api_token": "not-a-slack-token"},
	}
	ch, err := m.Scrape(context.Background(), s, scraper.ScrapeOptions{})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if ch != nil {
		t.Error("expected nil channel on validation error")
	}
}
