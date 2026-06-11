package linkedin

import (
	"testing"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/scraper"
)

// makeResponseEvent is a tiny helper that builds a response-type hijack event
// pointing at the given Voyager URL with the given JSON body.
func makeResponseEvent(url, body string) scout.HijackEvent {
	return scout.HijackEvent{
		Type:     scout.HijackEventResponse,
		Response: &scout.CapturedResponse{URL: url, Body: body},
	}
}

// --- parseHijackEvent: route dispatch for every non-profile branch ---

func TestParseHijackEvent_RouteDispatch(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		body     string
		wantLen  int
		wantType scraper.ResultType
	}{
		{
			name:     "feed route",
			url:      "https://www.linkedin.com/voyager/api/feed/updates?q=feed",
			body:     `{"elements": [{"id": "act-1", "commentary": "hi", "actor": "Ann"}]}`,
			wantLen:  1,
			wantType: scraper.ResultPost,
		},
		{
			name:     "connections route",
			url:      "https://www.linkedin.com/voyager/api/connections?start=0",
			body:     `{"elements": [{"entityUrn": "urn:li:member:1", "firstName": "Ann", "lastName": "Lee"}]}`,
			wantLen:  1,
			wantType: scraper.ResultMember,
		},
		{
			name:     "jobs route",
			url:      "https://www.linkedin.com/voyager/api/jobs/jobPostings/123",
			body:     `{"elements": [{"entityUrn": "urn:li:job:1", "title": "SRE", "companyName": "Acme"}]}`,
			wantLen:  1,
			wantType: scraper.ResultMeeting,
		},
		{
			name:     "messaging route",
			url:      "https://www.linkedin.com/voyager/api/messaging/conversations?q=all",
			body:     `{"elements": [{"conversationId": "conv-1", "participantId": "u1", "subject": "Hey"}]}`,
			wantLen:  1,
			wantType: scraper.ResultMessage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := makeResponseEvent(tt.url, tt.body)
			results := parseHijackEvent(ev, nil)
			if len(results) != tt.wantLen {
				t.Fatalf("parseHijackEvent() len = %d, want %d", len(results), tt.wantLen)
			}
			if results[0].Type != tt.wantType {
				t.Errorf("Type = %q, want %q", results[0].Type, tt.wantType)
			}
			if results[0].Source != "linkedin" {
				t.Errorf("Source = %q, want %q", results[0].Source, "linkedin")
			}
		})
	}
}

// --- parseProfileResponse: data-field Unmarshal-error branch (line 365) ---

func TestParseProfileResponse_DataNotAnObject(t *testing.T) {
	// The top-level wrapper unmarshals fine and data is non-empty, but data is a
	// JSON array, so unmarshaling into profileResponse (a struct) fails -> nil.
	body := `{"data": [1, 2, 3]}`
	results := parseProfileResponse(body, nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results when data is not a profile object, got %d", len(results))
	}
}

func TestParseProfileResponse_DataIsString(t *testing.T) {
	// data is a JSON string literal: valid RawMessage, but cannot decode into a struct.
	body := `{"data": "not-a-profile"}`
	results := parseProfileResponse(body, nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results when data is a string, got %d", len(results))
	}
}

func TestParseProfileResponse_TargetMatch(t *testing.T) {
	// Exercises the positive target-filter branch (target present in set).
	body := `{"data": {"firstName": "John", "lastName": "Doe", "publicIdentifier": "JohnDoe", "entityUrn": "urn:1"}}`
	targetSet := buildTargetSet([]string{"johndoe"})
	results := parseProfileResponse(body, targetSet)
	if len(results) != 1 {
		t.Fatalf("expected 1 result for matching target, got %d", len(results))
	}
	if results[0].Metadata["public_id"] != "JohnDoe" {
		t.Errorf("public_id = %v", results[0].Metadata["public_id"])
	}
}

// --- parseFeedResponse: per-element invalid-JSON continue branch ---

func TestParseFeedResponse_SkipsInvalidElement(t *testing.T) {
	// First element is not an object (number) -> Unmarshal error -> continue.
	// Second element is valid and should be the only result.
	body := `{"elements": [42, {"id": "act-ok", "commentary": "good", "actor": "Ann"}]}`
	results := parseFeedResponse(body, nil)
	if len(results) != 1 {
		t.Fatalf("expected 1 result (invalid element skipped), got %d", len(results))
	}
	if results[0].ID != "act-ok" {
		t.Errorf("ID = %q, want %q", results[0].ID, "act-ok")
	}
}

// --- parseConnectionsResponse: top-level + per-element invalid-JSON branches ---

func TestParseConnectionsResponse_TopLevelInvalidJSON(t *testing.T) {
	results := parseConnectionsResponse("}{ not json", nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results for invalid top-level JSON, got %d", len(results))
	}
}

func TestParseConnectionsResponse_SkipsInvalidElement(t *testing.T) {
	// First element is a string (invalid for struct) -> continue; second is valid.
	body := `{"elements": ["oops", {"entityUrn": "urn:li:member:9", "firstName": "Bo", "lastName": "Li"}]}`
	results := parseConnectionsResponse(body, nil)
	if len(results) != 1 {
		t.Fatalf("expected 1 result (invalid element skipped), got %d", len(results))
	}
	if results[0].ID != "urn:li:member:9" {
		t.Errorf("ID = %q, want %q", results[0].ID, "urn:li:member:9")
	}
}

// --- parseJobsResponse: top-level invalid, per-element invalid, empty-URN skip ---

func TestParseJobsResponse_TopLevelInvalidJSON(t *testing.T) {
	results := parseJobsResponse("not-json-at-all", nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results for invalid top-level JSON, got %d", len(results))
	}
}

func TestParseJobsResponse_SkipsInvalidElement(t *testing.T) {
	body := `{"elements": [true, {"entityUrn": "urn:li:job:7", "title": "Dev", "companyName": "Co"}]}`
	results := parseJobsResponse(body, nil)
	if len(results) != 1 {
		t.Fatalf("expected 1 result (invalid element skipped), got %d", len(results))
	}
	if results[0].ID != "urn:li:job:7" {
		t.Errorf("ID = %q, want %q", results[0].ID, "urn:li:job:7")
	}
}

func TestParseJobsResponse_SkipsEmptyURN(t *testing.T) {
	body := `{"elements": [{"entityUrn": "", "title": "Dev", "companyName": "Co"}]}`
	results := parseJobsResponse(body, nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty entityUrn, got %d", len(results))
	}
}

// --- parseMessagingResponse: top-level + per-element invalid-JSON branches ---

func TestParseMessagingResponse_TopLevelInvalidJSON(t *testing.T) {
	results := parseMessagingResponse("<<<bad", nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results for invalid top-level JSON, got %d", len(results))
	}
}

func TestParseMessagingResponse_SkipsInvalidElement(t *testing.T) {
	// First element is a number (invalid) -> continue; second is a valid conversation.
	body := `{"elements": [99, {"conversationId": "conv-ok", "participantId": "p1", "subject": "Yo"}]}`
	results := parseMessagingResponse(body, nil)
	if len(results) != 1 {
		t.Fatalf("expected 1 result (invalid element skipped, no nested messages), got %d", len(results))
	}
	if results[0].ID != "conv-ok" {
		t.Errorf("ID = %q, want %q", results[0].ID, "conv-ok")
	}
}
