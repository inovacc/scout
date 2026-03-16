package gdrive

import (
	"testing"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/scraper"
	"github.com/inovacc/scout/pkg/scout/scraper/auth"
)

// --- Mode metadata tests ---

func TestGDriveMode_Name(t *testing.T) {
	m := &GDriveMode{}
	if got := m.Name(); got != "gdrive" {
		t.Errorf("Name() = %q, want %q", got, "gdrive")
	}
}

func TestGDriveMode_Description(t *testing.T) {
	m := &GDriveMode{}
	if got := m.Description(); got == "" {
		t.Error("Description() is empty")
	}
}

func TestGDriveMode_AuthProvider(t *testing.T) {
	m := &GDriveMode{}
	p := m.AuthProvider()
	if p == nil {
		t.Fatal("AuthProvider() is nil")
	}
	if p.Name() != "gdrive" {
		t.Errorf("AuthProvider().Name() = %q", p.Name())
	}
}

// --- gdriveProvider tests ---

func TestGDriveProvider_LoginURL(t *testing.T) {
	p := &gdriveProvider{}
	if got := p.LoginURL(); got != "https://accounts.google.com/" {
		t.Errorf("LoginURL() = %q", got)
	}
}

// --- ValidateSession tests ---

func TestValidateSession_NilSession(t *testing.T) {
	p := &gdriveProvider{}
	if err := p.ValidateSession(nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateSession_ValidSID(t *testing.T) {
	p := &gdriveProvider{}
	s := &auth.Session{Cookies: []scout.Cookie{{Name: "SID", Value: "abc"}}}
	if err := p.ValidateSession(nil, s); err != nil {
		t.Errorf("error = %v", err)
	}
}

func TestValidateSession_ValidSSID(t *testing.T) {
	p := &gdriveProvider{}
	s := &auth.Session{Cookies: []scout.Cookie{{Name: "SSID", Value: "xyz"}}}
	if err := p.ValidateSession(nil, s); err != nil {
		t.Errorf("error = %v", err)
	}
}

func TestValidateSession_NoCookies(t *testing.T) {
	p := &gdriveProvider{}
	s := &auth.Session{Cookies: []scout.Cookie{{Name: "other", Value: "val"}}}
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
	set := buildTargetSet([]string{"FolderID"})
	if _, ok := set["folderid"]; !ok {
		t.Error("expected lowered key")
	}
}

// --- parseGoogleTimestamp tests ---

func TestParseGoogleTimestamp(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		isZero bool
	}{
		{"empty", "", true},
		{"valid", "2024-02-28T10:30:45Z", false},
		{"invalid", "not-a-date", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseGoogleTimestamp(tt.input)
			if got.IsZero() != tt.isZero {
				t.Errorf("parseGoogleTimestamp(%q).IsZero() = %v, want %v", tt.input, got.IsZero(), tt.isZero)
			}
		})
	}
}

func TestParseGoogleTimestamp_CorrectValue(t *testing.T) {
	got := parseGoogleTimestamp("2024-02-28T10:30:45Z")
	want := time.Date(2024, 2, 28, 10, 30, 45, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// --- parseFilesList tests ---

func TestParseFilesList_Valid(t *testing.T) {
	body := `{
		"files": [
			{"id": "f1", "name": "doc.pdf", "mimeType": "application/pdf", "size": "2048", "owners": ["alice"], "createdTime": "2024-01-01T00:00:00Z", "modifiedTime": "2024-01-02T00:00:00Z", "webViewLink": "https://drive.google.com/file/d/f1"}
		]
	}`
	results := parseFilesList(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	r := results[0]
	if r.Type != scraper.ResultFile {
		t.Errorf("Type = %q", r.Type)
	}
	if r.ID != "f1" {
		t.Errorf("ID = %q", r.ID)
	}
	if r.Author != "alice" {
		t.Errorf("Author = %q", r.Author)
	}
	if r.Metadata["size"] != int64(2048) {
		t.Errorf("Metadata[size] = %v (%T)", r.Metadata["size"], r.Metadata["size"])
	}
}

func TestParseFilesList_InvalidJSON(t *testing.T) {
	results := parseFilesList("bad", nil)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

func TestParseFilesList_WithTargetFilter(t *testing.T) {
	body := `{"files": [{"id": "f1", "name": "doc", "parents": ["folder-a"]}, {"id": "f2", "name": "doc2", "parents": ["folder-b"]}]}`
	targetSet := buildTargetSet([]string{"folder-a"})
	results := parseFilesList(body, targetSet)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].ID != "f1" {
		t.Errorf("ID = %q", results[0].ID)
	}
}

// --- parseFileGet tests ---

func TestParseFileGet_Valid(t *testing.T) {
	body := `{"id": "f1", "name": "report.xlsx", "mimeType": "application/xlsx", "size": "1024", "owners": ["bob"], "createdTime": "2024-01-01T00:00:00Z"}`
	results := parseFileGet(body)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].Content != "report.xlsx" {
		t.Errorf("Content = %q", results[0].Content)
	}
}

func TestParseFileGet_EmptyID(t *testing.T) {
	results := parseFileGet(`{"id": ""}`)
	if len(results) != 0 {
		t.Errorf("expected 0 for empty ID, got %d", len(results))
	}
}

// --- parseFolderList tests ---

func TestParseFolderList_Valid(t *testing.T) {
	body := `{"teamDrives": [{"id": "td1", "name": "TeamDrive1", "createdTime": "2024-01-01T00:00:00Z"}], "drives": [{"id": "d1", "name": "Drive1"}]}`
	results := parseFolderList(body, nil)
	if len(results) != 2 {
		t.Fatalf("got %d, want 2", len(results))
	}
	if results[0].Type != scraper.ResultChannel {
		t.Errorf("Type = %q", results[0].Type)
	}
}

// --- parsePermissions tests ---

func TestParsePermissions_Valid(t *testing.T) {
	body := `{"permissions": [{"id": "p1", "type": "user", "role": "writer", "emailAddress": "alice@test.com", "displayName": "Alice"}]}`
	results := parsePermissions(body)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].Type != scraper.ResultMember {
		t.Errorf("Type = %q", results[0].Type)
	}
	if results[0].Author != "alice@test.com" {
		t.Errorf("Author = %q", results[0].Author)
	}
}

// --- parseComments tests ---

func TestParseComments_Valid(t *testing.T) {
	body := `{"comments": [{"id": "c1", "fileId": "f1", "content": "Great work!", "createdTime": "2024-01-01T00:00:00Z", "author": {"displayName": "Bob", "emailAddress": "bob@test.com"}}]}`
	results := parseComments(body)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].Type != scraper.ResultComment {
		t.Errorf("Type = %q", results[0].Type)
	}
	if results[0].Author != "Bob" {
		t.Errorf("Author = %q", results[0].Author)
	}
}

func TestParseComments_AuthorFallbackEmail(t *testing.T) {
	body := `{"comments": [{"id": "c1", "content": "note", "author": {"emailAddress": "anon@test.com"}}]}`
	results := parseComments(body)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].Author != "anon@test.com" {
		t.Errorf("Author = %q, want email fallback", results[0].Author)
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
		Response: &scout.CapturedResponse{URL: "https://drive.google.com/files/list", Body: ""},
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
