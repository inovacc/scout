package notion

import (
	"testing"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/scraper"
)

// --- parseHijackEvent routing tests ---
//
// These cover the URL-based dispatch switch in parseHijackEvent that the
// existing tests don't exercise (only the non-response, empty-body, and
// unknown-URL paths were covered before).

func TestParseHijackEvent_NilResponse(t *testing.T) {
	// Type says response but Response pointer is nil -> early return.
	ev := scout.HijackEvent{Type: scout.HijackEventResponse, Response: nil}
	if results := parseHijackEvent(ev, nil); len(results) != 0 {
		t.Errorf("expected 0 results for nil response, got %d", len(results))
	}
}

func TestParseHijackEvent_Routing(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		body     string
		wantLen  int
		wantType scraper.ResultType
	}{
		{
			name:     "getPageAsNested",
			url:      "https://www.notion.so/api/v3/getPageAsNested",
			body:     `{"recordMap":{"block":{"p1":{"value":{"type":"page"}}}}}`,
			wantLen:  1,
			wantType: scraper.ResultPost,
		},
		{
			name:     "queryCollection",
			url:      "https://www.notion.so/api/v3/queryCollection",
			body:     `{"recordMap":{"block":{"b1":{"value":{"type":"database"}}}}}`,
			wantLen:  1,
			wantType: scraper.ResultChannel,
		},
		{
			name:     "getRecordValues",
			url:      "https://www.notion.so/api/v3/getRecordValues",
			body:     `{"results":[{"block":{"b1":{"value":{"type":"page"}}}}]}`,
			wantLen:  1,
			wantType: scraper.ResultPost,
		},
		{
			name:     "loadPageChunk",
			url:      "https://www.notion.so/api/v3/loadPageChunk",
			body:     `{"recordMap":{"block":{"b1":{"value":{"type":"callout"}}}}}`,
			wantLen:  1,
			wantType: scraper.ResultComment,
		},
		{
			// queryCollectionPages is matched ahead of queryCollection only
			// because Contains("/queryCollection") would also match it; the
			// switch checks queryCollection first, so a queryCollectionPages
			// URL actually routes to parseQueryCollection (recordMap-based).
			// Use a recordMap body so the assertion is meaningful.
			name:     "queryCollectionPages routes via queryCollection match",
			url:      "https://www.notion.so/api/v3/queryCollectionPages",
			body:     `{"recordMap":{"block":{"b1":{"value":{"type":"page"}}}}}`,
			wantLen:  1,
			wantType: scraper.ResultPost,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := scout.HijackEvent{
				Type:     scout.HijackEventResponse,
				Response: &scout.CapturedResponse{URL: tt.url, Body: tt.body},
			}
			results := parseHijackEvent(ev, nil)
			if len(results) != tt.wantLen {
				t.Fatalf("got %d results, want %d", len(results), tt.wantLen)
			}
			if tt.wantLen > 0 && results[0].Type != tt.wantType {
				t.Errorf("Type = %q, want %q", results[0].Type, tt.wantType)
			}
		})
	}
}

// --- parseQueryCollection edge paths ---

func TestParseQueryCollection_NonBlockRecordType(t *testing.T) {
	// recordMap present but contains only non-block record types -> skipped.
	body := `{"recordMap":{"collection":{"c1":{"value":{"name":"x"}}}}}`
	if results := parseQueryCollection(body, nil); len(results) != 0 {
		t.Errorf("expected 0 results for non-block record type, got %d", len(results))
	}
}

func TestParseQueryCollection_BlockNotMap(t *testing.T) {
	// "block" key present but value is not an object -> type assertion fails.
	body := `{"recordMap":{"block":"not-an-object"}}`
	if results := parseQueryCollection(body, nil); len(results) != 0 {
		t.Errorf("expected 0 results when block is not a map, got %d", len(results))
	}
}

func TestParseQueryCollection_NilRecordMap(t *testing.T) {
	if results := parseQueryCollection(`{}`, nil); len(results) != 0 {
		t.Errorf("expected 0 results for nil recordMap, got %d", len(results))
	}
}

func TestParseQueryCollection_TargetFilterMatch(t *testing.T) {
	// Target filter that matches the page id (lowercased) should keep the result.
	body := `{"recordMap":{"block":{"PageABC":{"value":{"type":"page"}}}}}`
	set := buildTargetSet([]string{"pageabc"})
	results := parseQueryCollection(body, set)
	if len(results) != 1 {
		t.Fatalf("expected 1 result for matching target, got %d", len(results))
	}
	if results[0].ID != "PageABC" {
		t.Errorf("ID = %q, want %q", results[0].ID, "PageABC")
	}
}

// --- parseGetPageAsNested edge paths ---

func TestParseGetPageAsNested_NonBlockRecordType(t *testing.T) {
	body := `{"recordMap":{"space":{"s1":{"value":{"name":"ws"}}}}}`
	if results := parseGetPageAsNested(body, nil); len(results) != 0 {
		t.Errorf("expected 0 results for non-block record type, got %d", len(results))
	}
}

func TestParseGetPageAsNested_TargetFilterMatch(t *testing.T) {
	body := `{"recordMap":{"block":{"Page-1":{"value":{"type":"page","properties":{"title":[["Hi"]]}}}}}}`
	set := buildTargetSet([]string{"page-1"})
	results := parseGetPageAsNested(body, set)
	if len(results) != 1 {
		t.Fatalf("expected 1 result for matching target, got %d", len(results))
	}
	if results[0].Content != "Hi" {
		t.Errorf("Content = %q, want %q", results[0].Content, "Hi")
	}
}

// --- parseLoadPageChunk edge paths ---

func TestParseLoadPageChunk_InvalidJSON(t *testing.T) {
	if results := parseLoadPageChunk("not json", nil); len(results) != 0 {
		t.Errorf("expected 0 results for invalid JSON, got %d", len(results))
	}
}

func TestParseLoadPageChunk_NonBlockRecordType(t *testing.T) {
	body := `{"recordMap":{"notion_user":{"u1":{"value":{"name":"x"}}}}}`
	if results := parseLoadPageChunk(body, nil); len(results) != 0 {
		t.Errorf("expected 0 results for non-block record type, got %d", len(results))
	}
}

func TestParseLoadPageChunk_TargetFilterSkipAndMatch(t *testing.T) {
	body := `{"recordMap":{"block":{"keep":{"value":{"type":"page"}},"drop":{"value":{"type":"page"}}}}}`
	set := buildTargetSet([]string{"keep"})
	results := parseLoadPageChunk(body, set)
	if len(results) != 1 {
		t.Fatalf("expected 1 result after filter, got %d", len(results))
	}
	if results[0].ID != "keep" {
		t.Errorf("ID = %q, want %q", results[0].ID, "keep")
	}
}

// --- parseGetRecordValues edge paths ---

func TestParseGetRecordValues_BlockTargetFilter(t *testing.T) {
	// Two blocks, only one in target set -> single result.
	body := `{"results":[{"block":{"a":{"value":{"type":"page"}},"b":{"value":{"type":"page"}}}}]}`
	set := buildTargetSet([]string{"a"})
	results := parseGetRecordValues(body, set)
	if len(results) != 1 {
		t.Fatalf("expected 1 result after block filter, got %d", len(results))
	}
	if results[0].ID != "a" {
		t.Errorf("ID = %q, want %q", results[0].ID, "a")
	}
}

func TestParseGetRecordValues_UserOnly(t *testing.T) {
	// User records are not filtered by targetSet; both should appear.
	body := `{"results":[{"user":{"u1":{"value":{"name":"A","email":"a@x.io"}}}},{"user":{"u2":{"value":{"name":"B"}}}}]}`
	results := parseGetRecordValues(body, buildTargetSet([]string{"unrelated"}))
	if len(results) != 2 {
		t.Fatalf("expected 2 user results, got %d", len(results))
	}
	for _, r := range results {
		if r.Type != scraper.ResultUser {
			t.Errorf("Type = %q, want %q", r.Type, scraper.ResultUser)
		}
	}
}

func TestParseGetRecordValues_EmptyResults(t *testing.T) {
	if results := parseGetRecordValues(`{"results":[]}`, nil); len(results) != 0 {
		t.Errorf("expected 0 results for empty results array, got %d", len(results))
	}
}

func TestParseGetRecordValues_ItemWithoutBlockOrUser(t *testing.T) {
	// Item has neither "block" nor "user" -> contributes nothing.
	body := `{"results":[{"space":{"s1":{"value":{"name":"ws"}}}}]}`
	if results := parseGetRecordValues(body, nil); len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// --- parseQueryCollectionPages edge paths ---

func TestParseQueryCollectionPages_ItemWithoutID(t *testing.T) {
	// Result item lacking a string "id" is ignored.
	body := `{"results":[{"value":{"type":"page"}}]}`
	if results := parseQueryCollectionPages(body, nil); len(results) != 0 {
		t.Errorf("expected 0 results for item without id, got %d", len(results))
	}
}

func TestParseQueryCollectionPages_TargetFilterSkipAndMatch(t *testing.T) {
	body := `{"results":[{"id":"Keep","value":{"type":"page"}},{"id":"Drop","value":{"type":"page"}}]}`
	set := buildTargetSet([]string{"keep"})
	results := parseQueryCollectionPages(body, set)
	if len(results) != 1 {
		t.Fatalf("expected 1 result after filter, got %d", len(results))
	}
	if results[0].ID != "Keep" {
		t.Errorf("ID = %q, want %q", results[0].ID, "Keep")
	}
}

func TestParseQueryCollectionPages_EmptyResults(t *testing.T) {
	if results := parseQueryCollectionPages(`{"results":[]}`, nil); len(results) != 0 {
		t.Errorf("expected 0 results for empty array, got %d", len(results))
	}
}

// --- blockToResult additional behavior ---

func TestBlockToResult_Timestamp(t *testing.T) {
	const createdMs = int64(1609459200000) // 2021-01-01T00:00:00Z
	blockData := map[string]any{
		"value": map[string]any{
			"type":         "page",
			"created_time": float64(createdMs),
		},
	}
	result := blockToResult("ts-1", blockData)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if got := result.Timestamp.UTC(); !got.Equal(time.UnixMilli(createdMs).UTC()) {
		t.Errorf("Timestamp = %v, want %v", got, time.UnixMilli(createdMs).UTC())
	}
}

func TestBlockToResult_SyncedBlockIsMessage(t *testing.T) {
	result := blockToResult("sb-1", map[string]any{"value": map[string]any{"type": "synced_block"}})
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Type != scraper.ResultMessage {
		t.Errorf("Type = %q, want %q", result.Type, scraper.ResultMessage)
	}
}

func TestBlockToResult_CalloutIsComment(t *testing.T) {
	result := blockToResult("co-1", map[string]any{"value": map[string]any{"type": "callout"}})
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Type != scraper.ResultComment {
		t.Errorf("Type = %q, want %q", result.Type, scraper.ResultComment)
	}
}

func TestBlockToResult_UnknownTypeDefaultsToMessage(t *testing.T) {
	// An unrecognised block type falls through to the default ResultMessage.
	result := blockToResult("x-1", map[string]any{"value": map[string]any{"type": "to_do"}})
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Type != scraper.ResultMessage {
		t.Errorf("Type = %q, want %q", result.Type, scraper.ResultMessage)
	}
}

func TestBlockToResult_MissingTitleAndType(t *testing.T) {
	// value present but no properties/title/type -> empty content, message type.
	result := blockToResult("empty-1", map[string]any{"value": map[string]any{}})
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Content != "" {
		t.Errorf("Content = %q, want empty", result.Content)
	}
	if result.Type != scraper.ResultMessage {
		t.Errorf("Type = %q, want %q", result.Type, scraper.ResultMessage)
	}
	if result.Metadata["type"] != "" {
		t.Errorf("Metadata[type] = %v, want empty", result.Metadata["type"])
	}
}

func TestBlockToResult_MalformedTitle(t *testing.T) {
	// title is present but the inner shape is wrong -> title stays empty, no panic.
	blockData := map[string]any{
		"value": map[string]any{
			"type":       "page",
			"properties": map[string]any{"title": []any{"not-an-inner-array"}},
		},
	}
	result := blockToResult("mt-1", blockData)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Content != "" {
		t.Errorf("Content = %q, want empty for malformed title", result.Content)
	}
}

// --- userToResult additional behavior ---

func TestUserToResult_NoValueKey(t *testing.T) {
	// userMap is a map but lacks the "value" object -> nil result.
	if result := userToResult("u-1", map[string]any{"other": 1}); result != nil {
		t.Errorf("expected nil for missing value key, got %v", result)
	}
}

func TestUserToResult_MissingNameAndEmail(t *testing.T) {
	// value present but no name/email -> result with empty author and email.
	result := userToResult("u-2", map[string]any{"value": map[string]any{}})
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Author != "" {
		t.Errorf("Author = %q, want empty", result.Author)
	}
	if result.Metadata["email"] != "" {
		t.Errorf("Metadata[email] = %v, want empty", result.Metadata["email"])
	}
	if result.ID != "u-2" {
		t.Errorf("ID = %q, want %q", result.ID, "u-2")
	}
}

// --- buildTargetSet additional behavior ---

func TestBuildTargetSet_MultipleEntries(t *testing.T) {
	set := buildTargetSet([]string{" One ", "TWO", "three"})
	for _, want := range []string{"one", "two", "three"} {
		if _, ok := set[want]; !ok {
			t.Errorf("missing normalized key %q in %v", want, set)
		}
	}
	if len(set) != 3 {
		t.Errorf("len(set) = %d, want 3", len(set))
	}
}
