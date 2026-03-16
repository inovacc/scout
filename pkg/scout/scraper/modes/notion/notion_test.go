package notion

import (
	"testing"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/scraper"
	"github.com/inovacc/scout/pkg/scout/scraper/auth"
)

// --- Mode metadata tests ---

func TestNotionMode_Name(t *testing.T) {
	m := &NotionMode{}
	if got := m.Name(); got != "notion" {
		t.Errorf("Name() = %q, want %q", got, "notion")
	}
}

func TestNotionMode_Description(t *testing.T) {
	m := &NotionMode{}
	if got := m.Description(); got == "" {
		t.Error("Description() is empty")
	}
}

func TestNotionMode_AuthProvider(t *testing.T) {
	m := &NotionMode{}
	p := m.AuthProvider()
	if p == nil {
		t.Fatal("AuthProvider() is nil")
	}
	if p.Name() != "notion" {
		t.Errorf("AuthProvider().Name() = %q", p.Name())
	}
}

// --- notionProvider tests ---

func TestNotionProvider_LoginURL(t *testing.T) {
	p := &notionProvider{}
	if got := p.LoginURL(); got != "https://www.notion.so/login" {
		t.Errorf("LoginURL() = %q", got)
	}
}

// --- ValidateSession tests ---

func TestValidateSession_NilSession(t *testing.T) {
	p := &notionProvider{}
	if err := p.ValidateSession(nil, nil); err == nil {
		t.Fatal("expected error for nil session")
	}
}

func TestValidateSession_ValidTokenV2InTokens(t *testing.T) {
	p := &notionProvider{}
	s := &auth.Session{
		Tokens: map[string]string{"token_v2": "abc123"},
	}
	if err := p.ValidateSession(nil, s); err != nil {
		t.Errorf("ValidateSession() error = %v", err)
	}
}

func TestValidateSession_ValidTokenV2LS(t *testing.T) {
	p := &notionProvider{}
	s := &auth.Session{
		Tokens: map[string]string{"token_v2_ls": "from-localstorage"},
	}
	if err := p.ValidateSession(nil, s); err != nil {
		t.Errorf("ValidateSession() error = %v", err)
	}
}

func TestValidateSession_ValidTokenV2InCookies(t *testing.T) {
	p := &notionProvider{}
	s := &auth.Session{
		Tokens:  map[string]string{},
		Cookies: []scout.Cookie{{Name: "token_v2", Value: "cookie-token"}},
	}
	if err := p.ValidateSession(nil, s); err != nil {
		t.Errorf("ValidateSession() error = %v", err)
	}
}

func TestValidateSession_NoToken(t *testing.T) {
	p := &notionProvider{}
	s := &auth.Session{
		Tokens:  map[string]string{},
		Cookies: []scout.Cookie{{Name: "other", Value: "val"}},
	}
	err := p.ValidateSession(nil, s)
	if err == nil {
		t.Fatal("expected error for missing token_v2")
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

func TestBuildTargetSet_NormalizesCase(t *testing.T) {
	set := buildTargetSet([]string{"  PageID123  "})
	if _, ok := set["pageid123"]; !ok {
		t.Error("expected lowercased and trimmed key")
	}
}

// --- blockToResult tests ---

func TestBlockToResult_ValidPage(t *testing.T) {
	blockData := map[string]any{
		"value": map[string]any{
			"type":         "page",
			"created_time": float64(1609459200000),
			"properties": map[string]any{
				"title": []any{[]any{"My Page Title"}},
			},
		},
	}

	result := blockToResult("block-id-123", blockData)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Type != scraper.ResultPost {
		t.Errorf("Type = %q, want %q", result.Type, scraper.ResultPost)
	}
	if result.ID != "block-id-123" {
		t.Errorf("ID = %q", result.ID)
	}
	if result.Content != "My Page Title" {
		t.Errorf("Content = %q", result.Content)
	}
	if result.Metadata["type"] != "page" {
		t.Errorf("Metadata[type] = %v", result.Metadata["type"])
	}
}

func TestBlockToResult_Database(t *testing.T) {
	blockData := map[string]any{
		"value": map[string]any{"type": "database"},
	}
	result := blockToResult("db-1", blockData)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Type != scraper.ResultChannel {
		t.Errorf("Type = %q, want %q", result.Type, scraper.ResultChannel)
	}
}

func TestBlockToResult_Comment(t *testing.T) {
	blockData := map[string]any{
		"value": map[string]any{"type": "comment"},
	}
	result := blockToResult("c-1", blockData)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Type != scraper.ResultComment {
		t.Errorf("Type = %q, want %q", result.Type, scraper.ResultComment)
	}
}

func TestBlockToResult_NilInput(t *testing.T) {
	result := blockToResult("id", nil)
	if result != nil {
		t.Errorf("expected nil for nil input, got %v", result)
	}
}

func TestBlockToResult_NoValue(t *testing.T) {
	result := blockToResult("id", map[string]any{})
	if result != nil {
		t.Errorf("expected nil for missing value, got %v", result)
	}
}

// --- userToResult tests ---

func TestUserToResult_Valid(t *testing.T) {
	userData := map[string]any{
		"value": map[string]any{
			"name":  "Alice",
			"email": "alice@example.com",
		},
	}
	result := userToResult("user-1", userData)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Type != scraper.ResultUser {
		t.Errorf("Type = %q", result.Type)
	}
	if result.Author != "Alice" {
		t.Errorf("Author = %q", result.Author)
	}
	if result.Metadata["email"] != "alice@example.com" {
		t.Errorf("Metadata[email] = %v", result.Metadata["email"])
	}
}

func TestUserToResult_NilInput(t *testing.T) {
	if result := userToResult("id", nil); result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

// --- parseGetPageAsNested tests ---

func TestParseGetPageAsNested_Valid(t *testing.T) {
	body := `{"recordMap": {"block": {"page-1": {"value": {"type": "page", "properties": {"title": [["Test Page"]]}}}}}}`
	results := parseGetPageAsNested(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
}

func TestParseGetPageAsNested_InvalidJSON(t *testing.T) {
	results := parseGetPageAsNested("bad", nil)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

func TestParseGetPageAsNested_NilRecordMap(t *testing.T) {
	results := parseGetPageAsNested(`{}`, nil)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

func TestParseGetPageAsNested_WithTargetFilter(t *testing.T) {
	body := `{"recordMap": {"block": {"page-1": {"value": {"type": "page"}}}}}`
	targetSet := buildTargetSet([]string{"other-page"})
	results := parseGetPageAsNested(body, targetSet)
	if len(results) != 0 {
		t.Errorf("expected 0 for filtered target, got %d", len(results))
	}
}

// --- parseQueryCollection tests ---

func TestParseQueryCollection_Valid(t *testing.T) {
	body := `{"recordMap": {"block": {"b1": {"value": {"type": "page"}}}}}`
	results := parseQueryCollection(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
}

func TestParseQueryCollection_InvalidJSON(t *testing.T) {
	results := parseQueryCollection("bad", nil)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

// --- parseGetRecordValues tests ---

func TestParseGetRecordValues_BlockAndUser(t *testing.T) {
	body := `{"results": [
		{"block": {"b1": {"value": {"type": "page"}}}},
		{"user": {"u1": {"value": {"name": "Bob", "email": "bob@test.com"}}}}
	]}`
	results := parseGetRecordValues(body, nil)
	if len(results) != 2 {
		t.Fatalf("got %d, want 2", len(results))
	}
}

func TestParseGetRecordValues_InvalidJSON(t *testing.T) {
	results := parseGetRecordValues("bad", nil)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

// --- parseLoadPageChunk tests ---

func TestParseLoadPageChunk_Valid(t *testing.T) {
	body := `{"recordMap": {"block": {"b1": {"value": {"type": "synced_block"}}}}}`
	results := parseLoadPageChunk(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
	if results[0].Type != scraper.ResultMessage {
		t.Errorf("Type = %q", results[0].Type)
	}
}

func TestParseLoadPageChunk_NilRecordMap(t *testing.T) {
	results := parseLoadPageChunk(`{}`, nil)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

// --- parseQueryCollectionPages tests ---

func TestParseQueryCollectionPages_Valid(t *testing.T) {
	body := `{"results": [{"id": "p1", "value": {"type": "page"}}]}`
	results := parseQueryCollectionPages(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d, want 1", len(results))
	}
}

func TestParseQueryCollectionPages_InvalidJSON(t *testing.T) {
	results := parseQueryCollectionPages("bad", nil)
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
		Response: &scout.CapturedResponse{URL: "https://notion.so/api/v3/getPageAsNested", Body: ""},
	}
	results := parseHijackEvent(ev, nil)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}

func TestParseHijackEvent_UnknownURL(t *testing.T) {
	ev := scout.HijackEvent{
		Type:     scout.HijackEventResponse,
		Response: &scout.CapturedResponse{URL: "https://notion.so/api/v3/unknown", Body: "{}"},
	}
	results := parseHijackEvent(ev, nil)
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}
