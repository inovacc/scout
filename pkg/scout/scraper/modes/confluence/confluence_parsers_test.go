package confluence

import "testing"

// --- parsePages target-filter mismatch branch ---
//
// When a target set is supplied and a page's space key is not in it, the page
// is skipped. The existing tests only covered the nil-filter and invalid-JSON
// paths, so this drives the continue branch inside parsePages.
func TestParsePages_FilterMismatch(t *testing.T) {
	body := `{
		"results": [
			{"id": "1", "type": "page", "title": "Keep", "space": {"key": "DEV"}, "body": {"storage": {"value": "x"}}, "version": {"number": 1, "by": {"displayName": "A"}}, "_links": {"webui": "/w/1"}},
			{"id": "2", "type": "page", "title": "Drop", "space": {"key": "OPS"}, "body": {"storage": {"value": "y"}}, "version": {"number": 1, "by": {"displayName": "B"}}, "_links": {"webui": "/w/2"}}
		]
	}`
	targetSet := buildTargetSet([]string{"dev"}) // lowercase: filter normalizes to DEV
	results := parsePages(body, targetSet)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1 (only DEV page)", len(results))
	}
	if results[0].ID != "1" {
		t.Errorf("ID = %q, want %q", results[0].ID, "1")
	}
	if results[0].Metadata["space_key"] != "DEV" {
		t.Errorf("Metadata[space_key] = %v, want DEV", results[0].Metadata["space_key"])
	}
}

// TestParsePages_FilterAllExcluded confirms that when no page matches the
// target set, parsePages returns an empty (nil) slice.
func TestParsePages_FilterAllExcluded(t *testing.T) {
	body := `{"results": [{"id": "9", "space": {"key": "OPS"}}]}`
	targetSet := buildTargetSet([]string{"DEV"})
	if results := parsePages(body, targetSet); len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// --- parsePagesV2 empty-results + invalid-JSON paths ---

func TestParsePagesV2_EmptyResults(t *testing.T) {
	if results := parsePagesV2(`{"results": []}`, nil); len(results) != 0 {
		t.Errorf("expected 0 results for empty results array, got %d", len(results))
	}
}

func TestParsePagesV2_InvalidJSON(t *testing.T) {
	if results := parsePagesV2("not-json", nil); len(results) != 0 {
		t.Errorf("expected 0 results for invalid JSON, got %d", len(results))
	}
}

// TestParsePagesV2_WebuiLink verifies the v2 webui link is read from the
// per-page _links map and mapped onto Result.URL.
func TestParsePagesV2_WebuiLink(t *testing.T) {
	body := `{"results": [{"id": "30", "title": "T", "spaceId": "s9", "createdBy": {"displayName": "C"}, "body": {"storage": {"value": "v"}}, "_links": {"webui": "/wiki/x/30"}}]}`
	results := parsePagesV2(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].URL != "/wiki/x/30" {
		t.Errorf("URL = %q, want %q", results[0].URL, "/wiki/x/30")
	}
	if results[0].Metadata["space_id"] != "s9" {
		t.Errorf("Metadata[space_id] = %v, want s9", results[0].Metadata["space_id"])
	}
}

// --- parseUsers invalid-JSON path ---

func TestParseUsers_InvalidJSON(t *testing.T) {
	if results := parseUsers("{bad json"); len(results) != 0 {
		t.Errorf("expected 0 results for invalid JSON, got %d", len(results))
	}
}

func TestParseUsers_EmptyResults(t *testing.T) {
	if results := parseUsers(`{"results": []}`); len(results) != 0 {
		t.Errorf("expected 0 results for empty results array, got %d", len(results))
	}
}
