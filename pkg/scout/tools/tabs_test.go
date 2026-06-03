package tools

import (
	"context"
	"testing"
)

func TestTabs(t *testing.T) {
	b, p := newPageTestBrowser(t)
	srv := newTestServer(t, "<h1>tab</h1>")

	if _, err := Navigate(context.Background(), p, NavigateInput{URL: srv.URL}); err != nil {
		t.Fatal(err)
	}

	out, err := Tabs(context.Background(), b, TabsInput{})
	if err != nil {
		t.Fatalf("Tabs: %v", err)
	}

	if len(out.Tabs) < 1 {
		t.Errorf("Tabs len = %d, want >= 1", len(out.Tabs))
	}

	if _, err := Tabs(context.Background(), nil, TabsInput{}); err == nil {
		t.Error("nil browser should error")
	}
}

func TestNewTab(t *testing.T) {
	b, _ := newPageTestBrowser(t)
	srv := newTestServer(t, "<h1>new</h1>")

	out, err := NewTab(context.Background(), b, NewTabInput{URL: srv.URL})
	if err != nil {
		t.Fatalf("NewTab: %v", err)
	}

	if out.Title != "T" {
		t.Errorf("NewTab Title = %q, want T", out.Title)
	}

	if out.URL == "" {
		t.Errorf("NewTab URL empty")
	}

	blank, err := NewTab(context.Background(), b, NewTabInput{})
	if err != nil {
		t.Fatalf("NewTab blank: %v", err)
	}

	if blank == nil {
		t.Error("blank tab output nil")
	}

	if _, err := NewTab(context.Background(), nil, NewTabInput{}); err == nil {
		t.Error("nil browser should error")
	}
}
