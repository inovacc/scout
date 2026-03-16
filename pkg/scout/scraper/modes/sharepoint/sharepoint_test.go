package sharepoint

import (
	"testing"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/scraper"
	"github.com/inovacc/scout/pkg/scout/scraper/auth"
)

// --- Mode metadata tests ---

func TestSharePointMode_Name(t *testing.T) {
	m := &SharePointMode{}
	if got := m.Name(); got != "sharepoint" {
		t.Errorf("Name() = %q, want %q", got, "sharepoint")
	}
}

func TestSharePointMode_Description(t *testing.T) {
	m := &SharePointMode{}
	if got := m.Description(); got == "" {
		t.Error("Description() is empty")
	}
}

func TestSharePointMode_AuthProvider(t *testing.T) {
	m := &SharePointMode{}
	p := m.AuthProvider()
	if p == nil {
		t.Fatal("AuthProvider() is nil")
	}
	if p.Name() != "sharepoint" {
		t.Errorf("AuthProvider().Name() = %q", p.Name())
	}
}

// --- sharepointProvider tests ---

func TestSharePointProvider_LoginURL(t *testing.T) {
	p := &sharepointProvider{}
	if got := p.LoginURL(); got != "https://login.microsoftonline.com/" {
		t.Errorf("LoginURL() = %q", got)
	}
}

// --- ValidateSession tests ---

func TestValidateSession_NilSession(t *testing.T) {
	p := &sharepointProvider{}
	if err := p.ValidateSession(nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateSession_ValidFedAuth(t *testing.T) {
	p := &sharepointProvider{}
	s := &auth.Session{Tokens: map[string]string{"FedAuth": "token123"}}
	if err := p.ValidateSession(nil, s); err != nil {
		t.Errorf("error = %v", err)
	}
}

func TestValidateSession_ValidRtFa(t *testing.T) {
	p := &sharepointProvider{}
	s := &auth.Session{Tokens: map[string]string{"rtFa": "token456"}}
	if err := p.ValidateSession(nil, s); err != nil {
		t.Errorf("error = %v", err)
	}
}

func TestValidateSession_ValidCookieFallback(t *testing.T) {
	p := &sharepointProvider{}
	s := &auth.Session{
		Tokens:  map[string]string{},
		Cookies: []scout.Cookie{{Name: "SPOIDCRL", Value: "val"}},
	}
	if err := p.ValidateSession(nil, s); err != nil {
		t.Errorf("error = %v", err)
	}
}

func TestValidateSession_NoToken(t *testing.T) {
	p := &sharepointProvider{}
	s := &auth.Session{
		Tokens:  map[string]string{},
		Cookies: []scout.Cookie{{Name: "other", Value: "v"}},
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

func TestBuildTargetSet_Lowered(t *testing.T) {
	set := buildTargetSet([]string{"Documents"})
	if _, ok := set["documents"]; !ok {
		t.Error("expected lowered key")
	}
}

// --- parseISO8601 tests ---

func TestParseISO8601(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		isZero bool
	}{
		{"empty", "", true},
		{"rfc3339", "2024-02-28T10:30:45Z", false},
		{"no_timezone", "2024-02-28T10:30:45", false},
		{"invalid", "not-a-date", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseISO8601(tt.input)
			if got.IsZero() != tt.isZero {
				t.Errorf("parseISO8601(%q).IsZero() = %v, want %v", tt.input, got.IsZero(), tt.isZero)
			}
		})
	}
}

func TestParseISO8601_CorrectValue(t *testing.T) {
	got := parseISO8601("2024-02-28T10:30:45Z")
	want := time.Date(2024, 2, 28, 10, 30, 45, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// --- parseListsResponse tests ---

func TestParseListsResponse_Valid(t *testing.T) {
	body := `{"value": [{"Id": "L1", "Title": "Documents", "RootFolder": "/sites/team/Documents"}]}`
	results := parseListsResponse(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].Type != scraper.ResultChannel {
		t.Errorf("Type = %q", results[0].Type)
	}
	if results[0].Content != "Documents" {
		t.Errorf("Content = %q", results[0].Content)
	}
}

func TestParseListsResponse_WithFilter(t *testing.T) {
	body := `{"value": [{"Id": "L1", "Title": "Documents"}, {"Id": "L2", "Title": "Pages"}]}`
	targetSet := buildTargetSet([]string{"documents"})
	results := parseListsResponse(body, targetSet)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
}

func TestParseListsResponse_InvalidJSON(t *testing.T) {
	results := parseListsResponse("bad", nil)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

// --- parseFileResponse tests ---

func TestParseFileResponse_Valid(t *testing.T) {
	body := `{"Name": "report.docx", "ServerRelativeUrl": "/sites/team/report.docx", "Length": 4096, "TimeCreated": "2024-01-01T00:00:00Z", "TimeLastModified": "2024-02-01T00:00:00Z", "ModifiedBy": {"Id": "u1", "Title": "Alice"}}`
	results := parseFileResponse(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].Type != scraper.ResultFile {
		t.Errorf("Type = %q", results[0].Type)
	}
	if results[0].Author != "Alice" {
		t.Errorf("Author = %q", results[0].Author)
	}
}

func TestParseFileResponse_EmptyName(t *testing.T) {
	results := parseFileResponse(`{"Name": ""}`, nil)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

// --- parseListItemsResponse tests ---

func TestParseListItemsResponse_Valid(t *testing.T) {
	body := `{"value": [{"Id": "i1", "Title": "Task 1", "Body": "Do something", "Modified": "2024-01-01T00:00:00Z"}]}`
	results := parseListItemsResponse(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].Type != scraper.ResultPost {
		t.Errorf("Type = %q", results[0].Type)
	}
}

// --- parsePagesResponse tests ---

func TestParsePagesResponse_Valid(t *testing.T) {
	body := `{"value": [{"id": "p1", "title": "Home", "description": "Welcome page", "webUrl": "https://contoso.sharepoint.com/pages/home", "createdBy": {"user": {"displayName": "Bob"}}, "lastModifiedDateTime": "2024-01-01T00:00:00Z"}]}`
	results := parsePagesResponse(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].Author != "Bob" {
		t.Errorf("Author = %q", results[0].Author)
	}
}

// --- parseSiteUsersResponse tests ---

func TestParseSiteUsersResponse_Valid(t *testing.T) {
	body := `{"value": [{"Id": "u1", "Title": "Admin", "LoginName": "admin@contoso.com", "Email": "admin@contoso.com", "IsSiteAdmin": true}]}`
	results := parseSiteUsersResponse(body)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].Type != scraper.ResultUser {
		t.Errorf("Type = %q", results[0].Type)
	}
	if results[0].Metadata["is_admin"] != true {
		t.Errorf("Metadata[is_admin] = %v", results[0].Metadata["is_admin"])
	}
}

// --- parseGraphSitesResponse tests ---

func TestParseGraphSitesResponse_Valid(t *testing.T) {
	body := `{"value": [{"id": "s1", "displayName": "Team Site", "webUrl": "https://contoso.sharepoint.com/sites/team", "createdDateTime": "2024-01-01T00:00:00Z"}]}`
	results := parseGraphSitesResponse(body)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].Type != scraper.ResultProfile {
		t.Errorf("Type = %q", results[0].Type)
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
		Response: &scout.CapturedResponse{URL: "https://contoso.sharepoint.com/_api/web/lists", Body: ""},
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
