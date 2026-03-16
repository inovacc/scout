package confluence

import (
	"testing"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/scraper"
	"github.com/inovacc/scout/pkg/scout/scraper/auth"
)

// --- Mode metadata tests ---

func TestConfluenceMode_Name(t *testing.T) {
	m := &ConfluenceMode{}
	if got := m.Name(); got != "confluence" {
		t.Errorf("Name() = %q, want %q", got, "confluence")
	}
}

func TestConfluenceMode_Description(t *testing.T) {
	m := &ConfluenceMode{}
	if got := m.Description(); got == "" {
		t.Error("Description() is empty")
	}
}

func TestConfluenceMode_AuthProvider(t *testing.T) {
	m := &ConfluenceMode{}
	p := m.AuthProvider()
	if p == nil {
		t.Fatal("AuthProvider() is nil")
	}
	if p.Name() != "confluence" {
		t.Errorf("AuthProvider().Name() = %q", p.Name())
	}
}

// --- confluenceProvider tests ---

func TestConfluenceProvider_LoginURL(t *testing.T) {
	p := &confluenceProvider{}
	if got := p.LoginURL(); got != "https://id.atlassian.com/login" {
		t.Errorf("LoginURL() = %q", got)
	}
}

// --- ValidateSession tests ---

func TestValidateSession_NilSession(t *testing.T) {
	p := &confluenceProvider{}
	if err := p.ValidateSession(nil, nil); err == nil {
		t.Fatal("expected error for nil session")
	}
}

func TestValidateSession_ValidTokenInTokens(t *testing.T) {
	p := &confluenceProvider{}
	s := &auth.Session{
		Tokens: map[string]string{"cloud.session.token": "abc123"},
	}
	if err := p.ValidateSession(nil, s); err != nil {
		t.Errorf("ValidateSession() error = %v", err)
	}
}

func TestValidateSession_ValidTokenInCookies(t *testing.T) {
	p := &confluenceProvider{}
	s := &auth.Session{
		Tokens:  map[string]string{},
		Cookies: []scout.Cookie{{Name: "cloud.session.token", Value: "tok"}},
	}
	if err := p.ValidateSession(nil, s); err != nil {
		t.Errorf("ValidateSession() error = %v", err)
	}
}

func TestValidateSession_NoToken(t *testing.T) {
	p := &confluenceProvider{}
	s := &auth.Session{
		Tokens:  map[string]string{},
		Cookies: []scout.Cookie{{Name: "other", Value: "val"}},
	}
	err := p.ValidateSession(nil, s)
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(*scraper.AuthError); !ok {
		t.Errorf("expected *scraper.AuthError, got %T", err)
	}
}

// --- buildTargetSet tests ---

func TestBuildTargetSet_Empty(t *testing.T) {
	if set := buildTargetSet(nil); set != nil {
		t.Errorf("expected nil, got %v", set)
	}
}

func TestBuildTargetSet_UppercaseNormalize(t *testing.T) {
	set := buildTargetSet([]string{"  myspace  "})
	if _, ok := set["MYSPACE"]; !ok {
		t.Error("expected uppercased key")
	}
}

// --- parseSpaces tests ---

func TestParseSpaces_Valid(t *testing.T) {
	body := `{
		"size": 1,
		"results": [
			{"id": 1, "key": "DEV", "name": "Development", "type": "global", "description": {"plain": {"value": "Dev space"}}}
		]
	}`
	results := parseSpaces(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	r := results[0]
	if r.Type != scraper.ResultChannel {
		t.Errorf("Type = %q", r.Type)
	}
	if r.ID != "DEV" {
		t.Errorf("ID = %q", r.ID)
	}
	if r.Content != "Dev space" {
		t.Errorf("Content = %q", r.Content)
	}
}

func TestParseSpaces_WithFilter(t *testing.T) {
	body := `{"results": [{"key": "DEV", "name": "Dev"}, {"key": "OPS", "name": "Ops"}]}`
	targetSet := buildTargetSet([]string{"DEV"})
	results := parseSpaces(body, targetSet)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].ID != "DEV" {
		t.Errorf("ID = %q", results[0].ID)
	}
}

func TestParseSpaces_InvalidJSON(t *testing.T) {
	results := parseSpaces("bad", nil)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

// --- parsePages tests ---

func TestParsePages_Valid(t *testing.T) {
	body := `{
		"results": [
			{"id": "123", "type": "page", "title": "My Page", "space": {"key": "DEV"}, "body": {"storage": {"value": "<p>Content</p>"}}, "version": {"number": 3, "by": {"displayName": "Alice"}}, "_links": {"webui": "/wiki/page/123"}}
		]
	}`
	results := parsePages(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	r := results[0]
	if r.Type != scraper.ResultPost {
		t.Errorf("Type = %q", r.Type)
	}
	if r.Author != "Alice" {
		t.Errorf("Author = %q", r.Author)
	}
	if r.Metadata["title"] != "My Page" {
		t.Errorf("Metadata[title] = %v", r.Metadata["title"])
	}
}

func TestParsePages_InvalidJSON(t *testing.T) {
	results := parsePages("bad", nil)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

// --- parsePagesV2 tests ---

func TestParsePagesV2_Valid(t *testing.T) {
	body := `{
		"results": [
			{"id": "456", "type": "page", "title": "V2 Page", "spaceId": "sp1", "createdBy": {"displayName": "Bob"}, "body": {"storage": {"value": "hello"}}, "_links": {"webui": "/wiki/v2/page"}}
		]
	}`
	results := parsePagesV2(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].Author != "Bob" {
		t.Errorf("Author = %q", results[0].Author)
	}
}

// --- parseComments tests ---

func TestParseComments_Valid(t *testing.T) {
	body := `{
		"results": [
			{"id": "c1", "type": "comment", "body": {"storage": {"value": "Nice!"}}, "version": {"number": 1, "by": {"displayName": "Charlie"}}, "container": {"id": "p1", "title": "Page"}}
		]
	}`
	results := parseComments(body)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].Type != scraper.ResultComment {
		t.Errorf("Type = %q", results[0].Type)
	}
	if results[0].Content != "Nice!" {
		t.Errorf("Content = %q", results[0].Content)
	}
}

func TestParseComments_InvalidJSON(t *testing.T) {
	results := parseComments("bad")
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

// --- parseUsers tests ---

func TestParseUsers_Valid(t *testing.T) {
	body := `{
		"results": [
			{"username": "charlie", "userKey": "uk1", "displayName": "Charlie", "email": "c@test.com", "active": true}
		]
	}`
	results := parseUsers(body)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].Type != scraper.ResultUser {
		t.Errorf("Type = %q", results[0].Type)
	}
	if results[0].Metadata["email"] != "c@test.com" {
		t.Errorf("Metadata[email] = %v", results[0].Metadata["email"])
	}
}

// --- parseGraphQL tests ---

func TestParseGraphQL_Valid(t *testing.T) {
	body := `{"data": {"key": "value"}}`
	results := parseGraphQL(body)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].Source != "confluence" {
		t.Errorf("Source = %q", results[0].Source)
	}
}

func TestParseGraphQL_EmptyData(t *testing.T) {
	results := parseGraphQL(`{"data": {}}`)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

func TestParseGraphQL_InvalidJSON(t *testing.T) {
	results := parseGraphQL("bad")
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

// --- parseHijackEvent tests ---

func TestParseHijackEvent_NonResponse(t *testing.T) {
	ev := scout.HijackEvent{Type: scout.HijackEventRequest}
	results := parseHijackEvent(ev, nil)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

func TestParseHijackEvent_EmptyBody(t *testing.T) {
	ev := scout.HijackEvent{
		Type:     scout.HijackEventResponse,
		Response: &scout.CapturedResponse{URL: "https://example.atlassian.net/wiki/rest/api/space", Body: ""},
	}
	results := parseHijackEvent(ev, nil)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

func TestParseHijackEvent_UnknownURL(t *testing.T) {
	ev := scout.HijackEvent{
		Type:     scout.HijackEventResponse,
		Response: &scout.CapturedResponse{URL: "https://example.com/unknown", Body: "{}"},
	}
	results := parseHijackEvent(ev, nil)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}
