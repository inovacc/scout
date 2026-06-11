package confluence

import (
	"testing"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/scraper"
)

// respEvent builds a response-type HijackEvent with the given URL and body.
func respEvent(url, body string) scout.HijackEvent {
	return scout.HijackEvent{
		Type:     scout.HijackEventResponse,
		Response: &scout.CapturedResponse{URL: url, Body: body},
	}
}

// --- parseHijackEvent dispatch branch tests ---
//
// These exercise the switch in parseHijackEvent that routes a captured
// response to the correct parser based on its URL. The existing tests only
// covered the non-response / empty-body / unknown-URL guard branches; here we
// drive each dispatch arm to a non-empty result.
func TestParseHijackEvent_Dispatch(t *testing.T) {
	const (
		spaceBody = `{"results": [{"id": 7, "key": "DEV", "name": "Dev", "type": "global", "description": {"plain": {"value": "d"}}}]}`
		pageBody  = `{"results": [{"id": "10", "type": "page", "title": "P", "space": {"key": "DEV"}, "body": {"storage": {"value": "c"}}, "version": {"number": 1, "by": {"displayName": "A"}}, "_links": {"webui": "/w/10"}}]}`
		v2Body    = `{"results": [{"id": "20", "type": "page", "title": "P2", "spaceId": "s1", "createdBy": {"displayName": "B"}, "body": {"storage": {"value": "c2"}}, "_links": {"webui": "/w/20"}}]}`
		userBody  = `{"results": [{"username": "u", "userKey": "uk", "displayName": "U", "email": "u@x.com", "active": true}]}`
		gqlBody   = `{"data": {"k": "v"}}`
	)

	tests := []struct {
		name     string
		url      string
		body     string
		wantLen  int
		wantType scraper.ResultType
		wantID   string
	}{
		{
			name:     "space",
			url:      "https://acme.atlassian.net/wiki/rest/api/space",
			body:     spaceBody,
			wantLen:  1,
			wantType: scraper.ResultChannel,
			wantID:   "DEV",
		},
		{
			name:     "content_page",
			url:      "https://acme.atlassian.net/wiki/rest/api/content",
			body:     pageBody,
			wantLen:  1,
			wantType: scraper.ResultPost,
			wantID:   "10",
		},
		{
			name:     "pages_v2",
			url:      "https://acme.atlassian.net/wiki/api/v2/pages",
			body:     v2Body,
			wantLen:  1,
			wantType: scraper.ResultPost,
			wantID:   "20",
		},
		{
			name:     "user",
			url:      "https://acme.atlassian.net/wiki/rest/api/user?accountId=uk",
			body:     userBody,
			wantLen:  1,
			wantType: scraper.ResultUser,
			wantID:   "uk",
		},
		{
			name:     "graphql_with_data",
			url:      "https://acme.atlassian.net/cgraphql?q=foo",
			body:     gqlBody,
			wantLen:  1,
			wantType: scraper.ResultPost,
			wantID:   "", // graphql ID is time-based; not asserted
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseHijackEvent(respEvent(tt.url, tt.body), nil)
			if len(got) != tt.wantLen {
				t.Fatalf("parseHijackEvent() len = %d, want %d", len(got), tt.wantLen)
			}
			if tt.wantLen == 0 {
				return
			}
			if got[0].Type != tt.wantType {
				t.Errorf("Type = %q, want %q", got[0].Type, tt.wantType)
			}
			if got[0].Source != "confluence" {
				t.Errorf("Source = %q, want confluence", got[0].Source)
			}
			if tt.wantID != "" && got[0].ID != tt.wantID {
				t.Errorf("ID = %q, want %q", got[0].ID, tt.wantID)
			}
		})
	}
}

// TestParseHijackEvent_GraphQLNoDataKeyword verifies that a graphql URL whose
// body lacks the literal substring "data" falls through to the default branch
// (the switch arm requires both URL match and body containing "data").
func TestParseHijackEvent_GraphQLNoDataKeyword(t *testing.T) {
	ev := respEvent("https://acme.atlassian.net/cgraphql", `{"results":[]}`)
	if got := parseHijackEvent(ev, nil); len(got) != 0 {
		t.Errorf("expected 0 results for graphql body without \"data\", got %d", len(got))
	}
}

// TestParseHijackEvent_SpaceWithTargetFilterMismatch confirms the target-set
// filter is threaded through the dispatch into parseSpaces: a non-matching key
// yields zero results even though the URL and JSON are otherwise valid.
func TestParseHijackEvent_SpaceWithTargetFilterMismatch(t *testing.T) {
	body := `{"results": [{"key": "OPS", "name": "Ops"}]}`
	targetSet := buildTargetSet([]string{"DEV"})
	ev := respEvent("https://acme.atlassian.net/wiki/rest/api/space", body)
	if got := parseHijackEvent(ev, targetSet); len(got) != 0 {
		t.Errorf("expected 0 results for filtered-out space, got %d", len(got))
	}
}
