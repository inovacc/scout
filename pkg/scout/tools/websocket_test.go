package tools

import (
	"context"
	"testing"
)

func TestWSListen(t *testing.T) {
	_, p := newPageTestBrowser(t)
	srv := newTestServer(t, "<p>no websockets here</p>")

	if _, err := Navigate(context.Background(), p, NavigateInput{URL: srv.URL}); err != nil {
		t.Fatal(err)
	}

	// A page with no WebSocket activity must return an empty-but-no-error result
	// once the (short) duration elapses.
	out, err := WSListen(context.Background(), p, WSListenInput{Duration: 1})
	if err != nil {
		t.Fatalf("WSListen: %v", err)
	}

	if out == nil {
		t.Fatal("WSListen returned nil output")
	}

	if len(out.Messages) != 0 {
		t.Errorf("Messages = %d, want 0", len(out.Messages))
	}

	if _, err := WSListen(context.Background(), nil, WSListenInput{}); err == nil {
		t.Error("nil page WSListen should error")
	}
}

func TestWSConnections(t *testing.T) {
	_, p := newPageTestBrowser(t)
	srv := newTestServer(t, "<p>x</p>")

	if _, err := Navigate(context.Background(), p, NavigateInput{URL: srv.URL}); err != nil {
		t.Fatal(err)
	}

	// No connections registered: result is a JSON array string ("[]"), no error.
	out, err := WSConnections(context.Background(), p, WSConnectionsInput{})
	if err != nil {
		t.Fatalf("WSConnections: %v", err)
	}

	if out == nil || out.Result == "" {
		t.Errorf("WSConnections result empty")
	}

	if _, err := WSConnections(context.Background(), nil, WSConnectionsInput{}); err == nil {
		t.Error("nil page WSConnections should error")
	}
}

func TestWSSend(t *testing.T) {
	_, p := newPageTestBrowser(t)
	srv := newTestServer(t, "<p>x</p>")

	if _, err := Navigate(context.Background(), p, NavigateInput{URL: srv.URL}); err != nil {
		t.Fatal(err)
	}

	// Valid JS that does not actually open a socket — asserts no error on a
	// well-formed expression and a stringified result.
	out, err := WSSend(context.Background(), p, WSSendInput{Script: "() => 'sent'"})
	if err != nil {
		t.Fatalf("WSSend: %v", err)
	}

	if out.Result != "sent" {
		t.Errorf("Result = %q, want sent", out.Result)
	}

	if _, err := WSSend(context.Background(), nil, WSSendInput{Script: "1"}); err == nil {
		t.Error("nil page WSSend should error")
	}

	if _, err := WSSend(context.Background(), p, WSSendInput{}); err == nil {
		t.Error("empty script WSSend should error")
	}
}
