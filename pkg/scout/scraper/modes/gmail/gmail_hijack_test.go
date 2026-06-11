package gmail

import (
	"testing"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/scraper"
)

// --- parseHijackEvent routing tests ---

// TestParseHijackEvent_NonResponseType verifies request (and other non-response)
// events are ignored entirely.
func TestParseHijackEvent_NonResponseType(t *testing.T) {
	ev := scout.HijackEvent{
		Type: scout.HijackEventRequest,
		// Response intentionally nil; type guard must short-circuit first.
		Request: &scout.CapturedRequest{URL: "https://mail.google.com/mail/u/0/search?q=x"},
	}

	if results := parseHijackEvent(ev, nil); results != nil {
		t.Errorf("expected nil for request event, got %v", results)
	}
}

// TestParseHijackEvent_NilResponse verifies a response event with a nil
// Response payload is ignored.
func TestParseHijackEvent_NilResponse(t *testing.T) {
	ev := scout.HijackEvent{Type: scout.HijackEventResponse, Response: nil}

	if results := parseHijackEvent(ev, nil); results != nil {
		t.Errorf("expected nil for nil response, got %v", results)
	}
}

// TestParseHijackEvent_EmptyBody verifies a response event with an empty body
// is ignored regardless of URL.
func TestParseHijackEvent_EmptyBody(t *testing.T) {
	ev := scout.HijackEvent{
		Type:     scout.HijackEventResponse,
		Response: &scout.CapturedResponse{URL: "https://mail.google.com/mail/u/0/search?q=x", Body: ""},
	}

	if results := parseHijackEvent(ev, nil); results != nil {
		t.Errorf("expected nil for empty body, got %v", results)
	}
}

// TestParseHijackEvent_Routing exercises every URL-based dispatch branch with a
// minimal valid body so the routed parser yields a deterministic result count.
func TestParseHijackEvent_Routing(t *testing.T) {
	const (
		emailListBody   = `{"response": [["thread-1", "a", "b", "c", ["INBOX"]]]}`
		emailThreadBody = `{"response": [["thread-1", {"from": "alice@example.com", "subject": "Hi", "body": "Hello", "timestamp": 1609459200000}]]}`
		labelsBody      = `[{"id": "INBOX", "name": "Inbox", "type": "system", "count": 3, "unread": 1, "color": ""}]`
		contactsBody    = `[{"id": "C1", "name": "Alice", "email": "alice@example.com", "phone": "", "image": ""}]`
	)

	tests := []struct {
		name      string
		url       string
		body      string
		wantCount int
		wantType  scraper.ResultType
	}{
		{
			name:      "search routes to email list",
			url:       "https://mail.google.com/mail/u/0/?search=1&view=tl",
			body:      emailListBody,
			wantCount: 1,
			wantType:  scraper.ResultMessage,
		},
		{
			name:      "get_thread routes to email thread",
			url:       "https://mail.google.com/mail/u/0/?ui=2&get_thread=t1",
			body:      emailThreadBody,
			wantCount: 1,
			wantType:  scraper.ResultEmail,
		},
		{
			name:      "labels routes to labels list",
			url:       "https://mail.google.com/mail/u/0/?ui=2&labels=1",
			body:      labelsBody,
			wantCount: 1,
			wantType:  scraper.ResultChannel,
		},
		{
			name:      "contacts substring routes to contacts list",
			url:       "https://contacts.google.com/v1/contacts",
			body:      contactsBody,
			wantCount: 1,
			wantType:  scraper.ResultProfile,
		},
		{
			name:      "singular contact substring routes to contacts list",
			url:       "https://mail.google.com/sync/u/0/i/fd?contact=1",
			body:      contactsBody,
			wantCount: 1,
			wantType:  scraper.ResultProfile,
		},
		{
			name:      "unrelated url returns nil",
			url:       "https://mail.google.com/mail/u/0/?ui=2&view=ad",
			body:      emailListBody,
			wantCount: 0,
		},
		{
			name:      "search url with unparsable body returns nil",
			url:       "https://mail.google.com/mail/u/0/?search=1",
			body:      "not json",
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := scout.HijackEvent{
				Type:     scout.HijackEventResponse,
				Response: &scout.CapturedResponse{URL: tt.url, Body: tt.body},
			}

			results := parseHijackEvent(ev, nil)
			if len(results) != tt.wantCount {
				t.Fatalf("got %d results, want %d", len(results), tt.wantCount)
			}

			if tt.wantCount > 0 && results[0].Type != tt.wantType {
				t.Errorf("Type = %q, want %q", results[0].Type, tt.wantType)
			}
		})
	}
}

// TestParseHijackEvent_SearchPrecedence verifies a URL containing both the
// mail/u/0/ prefix and "search" routes to the email-list parser even when the
// "labels" token also appears later in the query string. Branch order in the
// switch must pick search first.
func TestParseHijackEvent_SearchPrecedence(t *testing.T) {
	ev := scout.HijackEvent{
		Type: scout.HijackEventResponse,
		Response: &scout.CapturedResponse{
			URL:  "https://mail.google.com/mail/u/0/?search=1&labels=INBOX",
			Body: `{"response": [["thread-1", "a", "b", "c", ["INBOX"]]]}`,
		},
	}

	results := parseHijackEvent(ev, nil)
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}

	if results[0].Type != scraper.ResultMessage {
		t.Errorf("Type = %q, want %q (search should win over labels)", results[0].Type, scraper.ResultMessage)
	}
}

// TestParseHijackEvent_TargetFilterPassthrough verifies the targetSet argument
// reaches the email-list parser and filters out non-matching labels.
func TestParseHijackEvent_TargetFilterPassthrough(t *testing.T) {
	ev := scout.HijackEvent{
		Type: scout.HijackEventResponse,
		Response: &scout.CapturedResponse{
			URL:  "https://mail.google.com/mail/u/0/?search=1",
			Body: `{"response": [["thread-1", "a", "b", "c", ["INBOX"]], ["thread-2", "a", "b", "c", ["SPAM"]]]}`,
		},
	}

	results := parseHijackEvent(ev, buildTargetSet([]string{"spam"}))
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1 (filtered to spam)", len(results))
	}

	if results[0].ID != "thread-2" {
		t.Errorf("ID = %q, want thread-2", results[0].ID)
	}
}
