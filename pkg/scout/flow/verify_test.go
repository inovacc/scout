package flow

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVerifyParity(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	f := &FlowSpec{Version: "1", Steps: []FlowStep{
		{ID: "a", Request: Request{Method: "GET", URL: srv.URL}}}}
	golden := &Capture{Version: "1", Entries: []CaptureEntry{{Method: "GET", URL: srv.URL, Status: http.StatusOK}}}

	rep, err := Verify(context.Background(), f, golden, RunOptions{})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !rep.OK || len(rep.Steps) != 1 || !rep.Steps[0].StatusMatch {
		t.Fatalf("expected parity: %+v", rep)
	}
}

func TestVerifyDetectsDrift(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()
	f := &FlowSpec{Version: "1", Steps: []FlowStep{{ID: "a", Request: Request{Method: "GET", URL: srv.URL}}}}
	golden := &Capture{Version: "1", Entries: []CaptureEntry{{Method: "GET", URL: srv.URL, Status: http.StatusOK}}}
	rep, err := Verify(context.Background(), f, golden, RunOptions{})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if rep.OK || rep.Steps[0].StatusMatch {
		t.Fatalf("expected drift detected: %+v", rep)
	}
}
