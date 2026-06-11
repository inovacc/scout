package grafana

import (
	"testing"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/scraper"
)

// --- parseHijackEvent switch-arm routing tests ---
//
// Each case drives a distinct URL through parseHijackEvent so that every arm of
// the switch in parseHijackEvent is exercised. Bodies are minimal valid JSON for
// the matching parser so the route resolves to at least zero results without a
// browser or network.

func TestParseHijackEvent_Routing(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		body      string
		wantCount int
	}{
		{
			name:      "dashboards_find",
			url:       "https://grafana.example.com/api/dashboards/find?query=foo",
			body:      `{"results": [{"uid": "abc", "title": "D1"}]}`,
			wantCount: 1,
		},
		{
			name:      "dashboards_uid",
			url:       "https://grafana.example.com/api/dashboards/uid/abc",
			body:      `{"dashboard": {"uid": "abc", "title": "Detail", "panels": []}}`,
			wantCount: 1,
		},
		{
			name:      "datasources",
			url:       "https://grafana.example.com/api/datasources",
			body:      `[{"uid": "ds1", "name": "Prometheus", "type": "prometheus"}]`,
			wantCount: 1,
		},
		{
			name:      "alerts",
			url:       "https://grafana.example.com/api/alerts",
			body:      `{"results": [{"id": 1, "name": "A1", "state": "ok", "created": 0}]}`,
			wantCount: 1,
		},
		{
			name:      "search",
			url:       "https://grafana.example.com/api/search?type=dash-db",
			body:      `{"results": [{"uid": "s1", "title": "S1", "type": "dash-db"}]}`,
			wantCount: 1,
		},
		{
			name:      "ds_query",
			url:       "https://grafana.example.com/api/ds/query",
			body:      `{"results": [{"status": 200}]}`,
			wantCount: 1,
		},
		{
			name:      "annotations",
			url:       "https://grafana.example.com/api/annotations",
			body:      `{"results": [{"id": 1, "text": "deploy", "time": 0}]}`,
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := scout.HijackEvent{
				Type: scout.HijackEventResponse,
				Response: &scout.CapturedResponse{
					URL:  tt.url,
					Body: tt.body,
				},
			}

			results := parseHijackEvent(ev, nil)
			if len(results) != tt.wantCount {
				t.Fatalf("parseHijackEvent(%s) returned %d results, want %d", tt.name, len(results), tt.wantCount)
			}

			for i, r := range results {
				if r.Source != "grafana" {
					t.Errorf("result[%d].Source = %q, want %q", i, r.Source, "grafana")
				}
			}
		})
	}
}

// TestParseHijackEvent_NilResponse exercises the guard where Type is a response
// but the Response pointer is nil.
func TestParseHijackEvent_NilResponse(t *testing.T) {
	ev := scout.HijackEvent{
		Type:     scout.HijackEventResponse,
		Response: nil,
	}

	if results := parseHijackEvent(ev, nil); len(results) != 0 {
		t.Errorf("expected 0 results for nil response, got %d", len(results))
	}
}

// --- invalid-JSON branch coverage for each parser ---
//
// Each parser is invoked directly (not through a function value) so that the
// unparam analysis of targetSet in the source is unaffected.

const badJSON = "this-is-not-json"

func TestParseDashboardDetail_InvalidJSON(t *testing.T) {
	if results := parseDashboardDetail(badJSON, nil); len(results) != 0 {
		t.Errorf("parseDashboardDetail(bad JSON) returned %d results, want 0", len(results))
	}
}

func TestParseAlertsList_InvalidJSON(t *testing.T) {
	if results := parseAlertsList(badJSON, nil); len(results) != 0 {
		t.Errorf("parseAlertsList(bad JSON) returned %d results, want 0", len(results))
	}
}

func TestParseSearchResults_InvalidJSON(t *testing.T) {
	if results := parseSearchResults(badJSON, nil); len(results) != 0 {
		t.Errorf("parseSearchResults(bad JSON) returned %d results, want 0", len(results))
	}
}

func TestParsePanelQuery_InvalidJSON(t *testing.T) {
	if results := parsePanelQuery(badJSON, nil); len(results) != 0 {
		t.Errorf("parsePanelQuery(bad JSON) returned %d results, want 0", len(results))
	}
}

func TestParseAnnotations_InvalidJSON(t *testing.T) {
	if results := parseAnnotations(badJSON, nil); len(results) != 0 {
		t.Errorf("parseAnnotations(bad JSON) returned %d results, want 0", len(results))
	}
}

// TestParseSearchResults_FolderNotFiltered verifies that when a target filter is
// active, folder-type items are NOT filtered out (only dash-db items are), so a
// folder passes through even when its UID is not in the target set.
func TestParseSearchResults_FolderNotFiltered(t *testing.T) {
	body := `{"results": [
		{"id": 1, "uid": "f1", "title": "Team Folder", "type": "folder", "url": "/dashboards/f/f1"},
		{"id": 2, "uid": "d-skip", "title": "Skipped Dash", "type": "dash-db", "url": "/d/d-skip"}
	]}`

	// targetSet contains only a different dashboard UID; the folder must still pass.
	targetSet := buildTargetSet([]string{"some-other-uid"})

	results := parseSearchResults(body, targetSet)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1 (folder only)", len(results))
	}

	r := results[0]
	if r.Type != scraper.ResultChannel {
		t.Errorf("folder result Type = %q, want %q", r.Type, scraper.ResultChannel)
	}

	if r.ID != "f1" {
		t.Errorf("result ID = %q, want %q", r.ID, "f1")
	}

	if r.Metadata["type"] != "folder" {
		t.Errorf("metadata type = %v, want folder", r.Metadata["type"])
	}
}
