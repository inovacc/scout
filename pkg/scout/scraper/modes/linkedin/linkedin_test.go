package linkedin

import (
	"testing"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/scraper"
	"github.com/inovacc/scout/pkg/scout/scraper/auth"
)

// --- Mode metadata tests ---

func TestLinkedInMode_Name(t *testing.T) {
	m := &LinkedInMode{}
	if got := m.Name(); got != "linkedin" {
		t.Errorf("Name() = %q, want %q", got, "linkedin")
	}
}

func TestLinkedInMode_Description(t *testing.T) {
	m := &LinkedInMode{}
	if got := m.Description(); got == "" {
		t.Error("Description() is empty")
	}
}

func TestLinkedInMode_AuthProvider(t *testing.T) {
	m := &LinkedInMode{}
	p := m.AuthProvider()
	if p == nil {
		t.Fatal("AuthProvider() is nil")
	}
	if p.Name() != "linkedin" {
		t.Errorf("AuthProvider().Name() = %q, want %q", p.Name(), "linkedin")
	}
}

// --- linkedinProvider tests ---

func TestLinkedInProvider_LoginURL(t *testing.T) {
	p := &linkedinProvider{}
	if got := p.LoginURL(); got != "https://www.linkedin.com/login" {
		t.Errorf("LoginURL() = %q", got)
	}
}

// --- ValidateSession tests ---

func TestValidateSession_NilSession(t *testing.T) {
	p := &linkedinProvider{}
	err := p.ValidateSession(nil, nil)
	if err == nil {
		t.Fatal("expected error for nil session")
	}
}

func TestValidateSession_ValidLiAt(t *testing.T) {
	p := &linkedinProvider{}
	s := &auth.Session{
		Cookies: []scout.Cookie{{Name: "li_at", Value: "some-token"}},
	}
	if err := p.ValidateSession(nil, s); err != nil {
		t.Errorf("ValidateSession() error = %v", err)
	}
}

func TestValidateSession_ValidJSESSIONID(t *testing.T) {
	p := &linkedinProvider{}
	s := &auth.Session{
		Cookies: []scout.Cookie{{Name: "JSESSIONID", Value: "abc123"}},
	}
	if err := p.ValidateSession(nil, s); err != nil {
		t.Errorf("ValidateSession() error = %v", err)
	}
}

func TestValidateSession_ValidLiMc(t *testing.T) {
	p := &linkedinProvider{}
	s := &auth.Session{
		Cookies: []scout.Cookie{{Name: "li_mc", Value: "token"}},
	}
	if err := p.ValidateSession(nil, s); err != nil {
		t.Errorf("ValidateSession() error = %v", err)
	}
}

func TestValidateSession_NoCookies(t *testing.T) {
	p := &linkedinProvider{}
	s := &auth.Session{
		Cookies: []scout.Cookie{{Name: "other", Value: "val"}},
	}
	err := p.ValidateSession(nil, s)
	if err == nil {
		t.Fatal("expected error for missing linkedin cookies")
	}
	if _, ok := err.(*scraper.AuthError); !ok {
		t.Errorf("expected *scraper.AuthError, got %T", err)
	}
}

func TestValidateSession_EmptyCookies(t *testing.T) {
	p := &linkedinProvider{}
	s := &auth.Session{
		Cookies: []scout.Cookie{},
	}
	err := p.ValidateSession(nil, s)
	if err == nil {
		t.Fatal("expected error for empty cookies")
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

func TestBuildTargetSet_NormalizesURLs(t *testing.T) {
	set := buildTargetSet([]string{"https://johndoe/", "JaneDoe"})
	if _, ok := set["johndoe"]; !ok {
		t.Error("expected 'johndoe' in set after stripping https:// and trailing /")
	}
	if _, ok := set["janedoe"]; !ok {
		t.Error("expected 'janedoe' in set after lowercasing")
	}
}

// --- parseLinkedInTimestamp tests ---

func TestParseLinkedInTimestamp(t *testing.T) {
	tests := []struct {
		name string
		ms   int64
		want time.Time
	}{
		{"zero", 0, time.Time{}},
		{"epoch_ms", 1609459200000, time.Unix(0, 1609459200000*int64(time.Millisecond))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLinkedInTimestamp(tt.ms)
			if !got.Equal(tt.want) {
				t.Errorf("parseLinkedInTimestamp(%d) = %v, want %v", tt.ms, got, tt.want)
			}
		})
	}
}

// --- parseProfileResponse tests ---

func TestParseProfileResponse_Valid(t *testing.T) {
	body := `{
		"data": {
			"firstName": "John",
			"lastName": "Doe",
			"headline": "Software Engineer",
			"publicIdentifier": "johndoe",
			"entityUrn": "urn:li:member:12345",
			"publicProfileUrl": "https://www.linkedin.com/in/johndoe",
			"location": "San Francisco",
			"industry": "Technology",
			"createdAt": 1609459200000,
			"openToWork": true,
			"premiumSubscriber": false
		}
	}`

	results := parseProfileResponse(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}

	r := results[0]
	if r.Type != scraper.ResultProfile {
		t.Errorf("Type = %q, want %q", r.Type, scraper.ResultProfile)
	}
	if r.Source != "linkedin" {
		t.Errorf("Source = %q", r.Source)
	}
	if r.Author != "John Doe" {
		t.Errorf("Author = %q", r.Author)
	}
	if r.Content != "Software Engineer" {
		t.Errorf("Content = %q", r.Content)
	}
	if r.Metadata["public_id"] != "johndoe" {
		t.Errorf("Metadata[public_id] = %v", r.Metadata["public_id"])
	}
}

func TestParseProfileResponse_WithTargetFilter(t *testing.T) {
	body := `{"data": {"firstName": "John", "lastName": "Doe", "publicIdentifier": "johndoe", "entityUrn": "urn:1"}}`
	targetSet := buildTargetSet([]string{"other-person"})
	results := parseProfileResponse(body, targetSet)
	if len(results) != 0 {
		t.Errorf("expected 0 results for filtered target, got %d", len(results))
	}
}

func TestParseProfileResponse_InvalidJSON(t *testing.T) {
	results := parseProfileResponse("not json", nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results for invalid JSON, got %d", len(results))
	}
}

func TestParseProfileResponse_EmptyData(t *testing.T) {
	// null JSON unmarshals to a non-empty RawMessage ("null" = 4 bytes),
	// but the profile fields will be zero-valued.
	results := parseProfileResponse(`{"data": {}}`, nil)
	if len(results) != 1 {
		t.Fatalf("expected 1 result for empty profile, got %d", len(results))
	}
	// Empty profile should have empty author and content.
	if results[0].Author != " " {
		t.Errorf("Author = %q, want single space (empty first+last)", results[0].Author)
	}
}

// --- parseFeedResponse tests ---

func TestParseFeedResponse_Valid(t *testing.T) {
	body := `{
		"elements": [
			{"id": "act-123", "commentary": "Hello LinkedIn!", "createdTime": 1609459200000, "actor": "John Doe", "objectUrn": "urn:li:share:123", "reactionCount": 5},
			{"id": "act-456", "commentary": "Second post", "createdTime": 1609459300000, "actor": "Jane", "objectUrn": "urn:li:share:456", "reactionCount": 0}
		]
	}`

	results := parseFeedResponse(body, nil)
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}

	r := results[0]
	if r.Type != scraper.ResultPost {
		t.Errorf("Type = %q, want %q", r.Type, scraper.ResultPost)
	}
	if r.ID != "act-123" {
		t.Errorf("ID = %q", r.ID)
	}
	if r.Content != "Hello LinkedIn!" {
		t.Errorf("Content = %q", r.Content)
	}
	if r.Metadata["reaction_count"] != 5 {
		t.Errorf("Metadata[reaction_count] = %v", r.Metadata["reaction_count"])
	}
}

func TestParseFeedResponse_SkipsEmptyID(t *testing.T) {
	body := `{"elements": [{"id": "", "commentary": "test"}]}`
	results := parseFeedResponse(body, nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty ID, got %d", len(results))
	}
}

func TestParseFeedResponse_InvalidJSON(t *testing.T) {
	results := parseFeedResponse("bad json", nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// --- parseConnectionsResponse tests ---

func TestParseConnectionsResponse_Valid(t *testing.T) {
	body := `{
		"elements": [
			{"entityUrn": "urn:li:member:1", "publicIdentifier": "conn1", "firstName": "Alice", "lastName": "Smith", "headline": "PM", "location": "NYC", "connectionDegree": "1st", "createdTime": 1609459200000}
		]
	}`
	results := parseConnectionsResponse(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Type != scraper.ResultMember {
		t.Errorf("Type = %q, want %q", results[0].Type, scraper.ResultMember)
	}
	if results[0].Author != "Alice Smith" {
		t.Errorf("Author = %q", results[0].Author)
	}
}

func TestParseConnectionsResponse_SkipsEmptyURN(t *testing.T) {
	body := `{"elements": [{"entityUrn": ""}]}`
	results := parseConnectionsResponse(body, nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// --- parseJobsResponse tests ---

func TestParseJobsResponse_Valid(t *testing.T) {
	body := `{
		"elements": [
			{"entityUrn": "urn:li:job:1", "jobID": "J001", "title": "SRE", "companyName": "Acme", "location": "Remote", "description": "Site reliability", "postedDate": 1609459200000, "applyUrl": "https://apply.example.com", "experienceLevel": "Mid"}
		]
	}`
	results := parseJobsResponse(body, nil)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	r := results[0]
	if r.Author != "Acme" {
		t.Errorf("Author = %q", r.Author)
	}
	if r.URL != "https://apply.example.com" {
		t.Errorf("URL = %q", r.URL)
	}
	if r.Metadata["experience_level"] != "Mid" {
		t.Errorf("Metadata[experience_level] = %v", r.Metadata["experience_level"])
	}
}

// --- parseMessagingResponse tests ---

func TestParseMessagingResponse_Valid(t *testing.T) {
	body := `{
		"elements": [
			{
				"conversationId": "conv-1",
				"participantId": "user-1",
				"subject": "Hello",
				"createdTime": 1609459200000,
				"lastMessageAt": 1609459300000,
				"messages": [
					{"messageId": "msg-1", "content": "Hi there", "createdTime": 1609459200000, "senderId": "user-2"}
				]
			}
		]
	}`
	results := parseMessagingResponse(body, nil)
	// 1 conversation + 1 message = 2 results
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].Type != scraper.ResultMessage {
		t.Errorf("Type = %q, want %q", results[0].Type, scraper.ResultMessage)
	}
	if results[0].Content != "Hello" {
		t.Errorf("Content = %q", results[0].Content)
	}
	if results[1].Content != "Hi there" {
		t.Errorf("nested message Content = %q", results[1].Content)
	}
}

func TestParseMessagingResponse_SkipsEmptyConversationID(t *testing.T) {
	body := `{"elements": [{"conversationId": "", "participantId": "u1"}]}`
	results := parseMessagingResponse(body, nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// --- parseHijackEvent tests ---

func TestParseHijackEvent_NonResponse(t *testing.T) {
	ev := scout.HijackEvent{Type: scout.HijackEventRequest}
	results := parseHijackEvent(ev, nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results for non-response event, got %d", len(results))
	}
}

func TestParseHijackEvent_NilResponse(t *testing.T) {
	ev := scout.HijackEvent{Type: scout.HijackEventResponse, Response: nil}
	results := parseHijackEvent(ev, nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results for nil response, got %d", len(results))
	}
}

func TestParseHijackEvent_EmptyBody(t *testing.T) {
	ev := scout.HijackEvent{
		Type:     scout.HijackEventResponse,
		Response: &scout.CapturedResponse{URL: "https://linkedin.com/voyager/api/identity/profiles/foo", Body: ""},
	}
	results := parseHijackEvent(ev, nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty body, got %d", len(results))
	}
}

func TestParseHijackEvent_UnknownURL(t *testing.T) {
	ev := scout.HijackEvent{
		Type:     scout.HijackEventResponse,
		Response: &scout.CapturedResponse{URL: "https://linkedin.com/voyager/api/unknown", Body: "{}"},
	}
	results := parseHijackEvent(ev, nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results for unknown URL, got %d", len(results))
	}
}

func TestParseHijackEvent_ProfileRoute(t *testing.T) {
	body := `{"data": {"firstName": "A", "lastName": "B", "publicIdentifier": "ab", "entityUrn": "urn:1"}}`
	ev := scout.HijackEvent{
		Type:     scout.HijackEventResponse,
		Response: &scout.CapturedResponse{URL: "https://linkedin.com/voyager/api/identity/profiles/ab", Body: body},
	}
	results := parseHijackEvent(ev, nil)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Type != scraper.ResultProfile {
		t.Errorf("Type = %q", results[0].Type)
	}
}
