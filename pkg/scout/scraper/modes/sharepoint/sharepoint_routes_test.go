package sharepoint

import (
	"testing"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/scraper"
)

// respEvent builds a response HijackEvent for the given URL and body.
func respEvent(url, body string) scout.HijackEvent {
	return scout.HijackEvent{
		Type:     scout.HijackEventResponse,
		Response: &scout.CapturedResponse{URL: url, Body: body},
	}
}

// --- parseHijackEvent routing tests ---
//
// The switch in parseHijackEvent is order-sensitive: the "_api/web/lists"
// case is evaluated before "_api/web/GetFileByServerRelativeUrl" and
// "_api/web/lists(", so URLs containing those longer substrings that also
// contain "_api/web/lists" still route to parseListsResponse. These tests
// assert the actual routing behavior of the dispatcher.

func TestParseHijackEvent_Routes(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		body     string
		wantLen  int
		wantType scraper.ResultType
	}{
		{
			name:     "lists",
			url:      "https://contoso.sharepoint.com/_api/web/lists",
			body:     `{"value":[{"Id":"L1","Title":"Documents","RootFolder":"/sites/team/Documents"}]}`,
			wantLen:  1,
			wantType: scraper.ResultChannel,
		},
		{
			name:     "file_via_GetFileByServerRelativeUrl",
			url:      "https://contoso.sharepoint.com/_api/web/GetFileByServerRelativeUrl('/sites/team/report.docx')",
			body:     `{"Name":"report.docx","ServerRelativeUrl":"/sites/team/report.docx","Length":4096,"TimeLastModified":"2024-02-01T00:00:00Z","ModifiedBy":{"Id":"u1","Title":"Alice"}}`,
			wantLen:  1,
			wantType: scraper.ResultFile,
		},
		{
			name:     "lists_paren_routes_to_lists_case",
			url:      "https://contoso.sharepoint.com/_api/web/lists(guid'abc')/items",
			body:     `{"value":[{"Id":"L9","Title":"Tasks","RootFolder":"/sites/team/Lists/Tasks"}]}`,
			wantLen:  1,
			wantType: scraper.ResultChannel,
		},
		{
			name:     "sitepages_pages",
			url:      "https://contoso.sharepoint.com/_api/sitepages/pages",
			body:     `{"value":[{"id":"p1","title":"Home","description":"Welcome","webUrl":"https://contoso.sharepoint.com/pages/home","lastModifiedDateTime":"2024-01-01T00:00:00Z"}]}`,
			wantLen:  1,
			wantType: scraper.ResultPost,
		},
		{
			name:     "siteusers",
			url:      "https://contoso.sharepoint.com/_api/web/siteusers",
			body:     `{"value":[{"Id":"u1","Title":"Admin","LoginName":"admin@contoso.com","Email":"admin@contoso.com","IsSiteAdmin":true}]}`,
			wantLen:  1,
			wantType: scraper.ResultUser,
		},
		{
			name:     "getsiteusers",
			url:      "https://contoso.sharepoint.com/_api/web/getsiteusers",
			body:     `{"value":[{"Id":"u2","Title":"Member","LoginName":"member@contoso.com","Email":"member@contoso.com","IsSiteAdmin":false}]}`,
			wantLen:  1,
			wantType: scraper.ResultUser,
		},
		{
			name:     "graph_sites",
			url:      "https://graph.microsoft.com/v1.0/sites/root",
			body:     `{"value":[{"id":"s1","displayName":"Team Site","webUrl":"https://contoso.sharepoint.com/sites/team","createdDateTime":"2024-01-01T00:00:00Z"}]}`,
			wantLen:  1,
			wantType: scraper.ResultProfile,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := parseHijackEvent(respEvent(tt.url, tt.body), nil)
			if len(results) != tt.wantLen {
				t.Fatalf("len(results) = %d, want %d", len(results), tt.wantLen)
			}
			if tt.wantLen > 0 && results[0].Type != tt.wantType {
				t.Errorf("Type = %q, want %q", results[0].Type, tt.wantType)
			}
		})
	}
}

func TestParseHijackEvent_NilResponse(t *testing.T) {
	ev := scout.HijackEvent{Type: scout.HijackEventResponse, Response: nil}
	if results := parseHijackEvent(ev, nil); len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

// --- timestamp fallback branches ---

func TestParseFileResponse_TimeCreatedFallback(t *testing.T) {
	// TimeLastModified empty -> falls back to TimeCreated.
	body := `{"Name":"old.txt","ServerRelativeUrl":"/sites/team/old.txt","Length":10,"TimeCreated":"2024-03-15T08:00:00Z","TimeLastModified":""}`
	results := parseFileResponse(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].Timestamp.IsZero() {
		t.Error("Timestamp should fall back to TimeCreated, got zero")
	}
	want := parseISO8601("2024-03-15T08:00:00Z")
	if !results[0].Timestamp.Equal(want) {
		t.Errorf("Timestamp = %v, want %v", results[0].Timestamp, want)
	}
}

func TestParseListItemsResponse_CreatedFallback(t *testing.T) {
	// Modified empty -> falls back to Created.
	body := `{"value":[{"Id":"i1","Title":"Task","Body":"text","Created":"2024-04-10T09:30:00Z","Modified":""}]}`
	results := parseListItemsResponse(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].Type != scraper.ResultPost {
		t.Errorf("Type = %q, want %q", results[0].Type, scraper.ResultPost)
	}
	want := parseISO8601("2024-04-10T09:30:00Z")
	if !results[0].Timestamp.Equal(want) {
		t.Errorf("Timestamp = %v, want %v", results[0].Timestamp, want)
	}
}

func TestParsePagesResponse_CreatedFallbackAndNilCreatedBy(t *testing.T) {
	// lastModifiedDateTime empty -> falls back to createdDateTime.
	// createdBy omitted -> author stays empty.
	body := `{"value":[{"id":"p2","title":"About","description":"desc","webUrl":"https://contoso.sharepoint.com/pages/about","createdDateTime":"2024-05-20T12:00:00Z","lastModifiedDateTime":""}]}`
	results := parsePagesResponse(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].Author != "" {
		t.Errorf("Author = %q, want empty (nil CreatedBy)", results[0].Author)
	}
	want := parseISO8601("2024-05-20T12:00:00Z")
	if !results[0].Timestamp.Equal(want) {
		t.Errorf("Timestamp = %v, want %v", results[0].Timestamp, want)
	}
}

// --- invalid JSON branches ---

func TestParsePagesResponse_InvalidJSON(t *testing.T) {
	if results := parsePagesResponse("not json", nil); results != nil {
		t.Errorf("expected nil, got %v", results)
	}
}

func TestParseListItemsResponse_InvalidJSON(t *testing.T) {
	if results := parseListItemsResponse("{bad", nil); results != nil {
		t.Errorf("expected nil, got %v", results)
	}
}

func TestParseSiteUsersResponse_InvalidJSON(t *testing.T) {
	if results := parseSiteUsersResponse("]["); results != nil {
		t.Errorf("expected nil, got %v", results)
	}
}

func TestParseGraphSitesResponse_InvalidJSON(t *testing.T) {
	if results := parseGraphSitesResponse("<html>"); results != nil {
		t.Errorf("expected nil, got %v", results)
	}
}
