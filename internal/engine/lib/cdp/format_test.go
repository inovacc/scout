package cdp

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFSessionID(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "empty becomes zero placeholder",
			in:   "",
			want: "@00000000",
		},
		{
			name: "exactly eight chars preserved",
			in:   "ABCDEFGH",
			want: "@ABCDEFGH",
		},
		{
			name: "longer than eight is truncated",
			in:   "0123456789ABCDEF",
			want: "@01234567",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fSessionID(tt.in); got != tt.want {
				t.Fatalf("fSessionID(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestDump(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want string
	}{
		{name: "nil renders null", in: nil, want: "null"},
		{name: "map", in: map[string]int{"a": 1}, want: `{"a":1}`},
		{name: "string", in: "hi", want: `"hi"`},
		{name: "html not escaped", in: "<b>", want: `"<b>"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dump(tt.in); got != tt.want {
				t.Fatalf("dump(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestRequestString(t *testing.T) {
	tests := []struct {
		name string
		req  Request
		want string
	}{
		{
			name: "with session and params",
			req:  Request{ID: 7, SessionID: "ABCDEFGH1234", Method: "Page.navigate", Params: map[string]string{"url": "x"}},
			want: `=> #7 @ABCDEFGH Page.navigate {"url":"x"}`,
		},
		{
			name: "empty session uses placeholder, nil params render null",
			req:  Request{ID: 1, Method: "Page.enable"},
			want: `=> #1 @00000000 Page.enable null`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.req.String(); got != tt.want {
				t.Fatalf("Request.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResponseString(t *testing.T) {
	tests := []struct {
		name string
		res  Response
		want string
	}{
		{
			name: "success path dumps result",
			res:  Response{ID: 3, Result: json.RawMessage(`{"ok":true}`)},
			want: `<= #3 {"ok":true}`,
		},
		{
			name: "nil result renders null",
			res:  Response{ID: 4},
			want: `<= #4 null`,
		},
		{
			name: "error path takes precedence",
			res:  Response{ID: 5, Error: &Error{Code: -32000, Message: "boom"}},
			want: `<= #5 error: {"code":-32000,"message":"boom","data":""}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.res.String()
			if got != tt.want {
				t.Fatalf("Response.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEventString(t *testing.T) {
	ev := Event{
		SessionID: "SESSIONID9",
		Method:    "Target.attachedToTarget",
		Params:    json.RawMessage(`{"k":1}`),
	}
	got := ev.String()
	want := `<- @SESSIONI Target.attachedToTarget {"k":1}`
	if got != want {
		t.Fatalf("Event.String() = %q, want %q", got, want)
	}

	// Empty session id falls back to the zero placeholder; nil params -> null.
	got = Event{Method: "Page.loadEventFired"}.String()
	want = `<- @00000000 Page.loadEventFired null`
	if got != want {
		t.Fatalf("Event.String() empty = %q, want %q", got, want)
	}
}

// TestEventStringRoundTrip ensures String reflects the decoded params verbatim
// rather than re-escaping, which would corrupt embedded JSON.
func TestEventStringRoundTrip(t *testing.T) {
	raw := `{"selector":"a > b","html":"<div>&</div>"}`
	ev := Event{Method: "X", Params: json.RawMessage(raw)}
	if !strings.Contains(ev.String(), raw) {
		t.Fatalf("Event.String() = %q, want it to contain raw params %q", ev.String(), raw)
	}
}
