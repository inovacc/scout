package plugin

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/inovacc/scout/pkg/scout/scraper"
)

func TestModeProxy_Name(t *testing.T) {
	p := &ModeProxy{
		entry: ModeEntry{Name: "test-mode", Description: "Test mode"},
	}

	if p.Name() != "test-mode" {
		t.Errorf("Name() = %q, want %q", p.Name(), "test-mode")
	}

	if p.Description() != "Test mode" {
		t.Errorf("Description() = %q, want %q", p.Description(), "Test mode")
	}

	if p.AuthProvider() != nil {
		t.Error("expected nil AuthProvider")
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"debug", "DEBUG"},
		{"info", "INFO"},
		{"warn", "WARN"},
		{"error", "ERROR"},
		{"unknown", "INFO"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseLevel(tt.input)
			if got.String() != tt.want {
				t.Errorf("parseLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

type mockSession struct{}

func (m mockSession) ProviderName() string { return "test" }

func setupModeProxyWithMock(t *testing.T, handler func(req *Request) *Response) *ModeProxy {
	t.Helper()

	stdoutR, stdoutW := io.Pipe()
	stdinR, stdinW := io.Pipe()

	manifest := &Manifest{Name: "test-plugin", Version: "1.0.0", Command: "./test"}

	client := &Client{
		manifest: manifest,
		encoder:  json.NewEncoder(stdinW),
		scanner:  bufio.NewScanner(stdoutR),
		pending:  make(map[int64]chan *Response),
		notify:   make(chan *Notification, 64),
		done:     make(chan struct{}),
		started:  true,
	}
	client.scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	go client.readLoop()

	mgr := NewManager(nil, nil)
	mgr.clients[manifest.Name] = client

	go mockResponder(t, stdinR, stdoutW, handler)

	t.Cleanup(func() {
		_ = stdoutW.Close()
		_ = stdinW.Close()
	})

	return &ModeProxy{
		entry:    ModeEntry{Name: "test-mode", Description: "Test"},
		manifest: manifest,
		manager:  mgr,
	}
}

func TestModeProxy_Scrape_BatchResults(t *testing.T) {
	proxy := setupModeProxyWithMock(t, func(req *Request) *Response {
		results := []scraper.Result{
			{Source: "test", ID: "1"},
			{Source: "test", ID: "2"},
		}
		data, _ := json.Marshal(results)
		return &Response{JSONRPC: "2.0", ID: req.ID, Result: data}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := proxy.Scrape(ctx, mockSession{}, scraper.ScrapeOptions{Headless: true})
	if err != nil {
		t.Fatalf("Scrape() error: %v", err)
	}

	var results []scraper.Result
	for r := range ch {
		results = append(results, r)
	}

	if len(results) != 2 {
		t.Errorf("got %d results, want 2", len(results))
	}
}

func TestModeProxy_Scrape_SingleResult(t *testing.T) {
	proxy := setupModeProxyWithMock(t, func(req *Request) *Response {
		result := scraper.Result{Source: "test", ID: "single"}
		data, _ := json.Marshal(result)
		return &Response{JSONRPC: "2.0", ID: req.ID, Result: data}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := proxy.Scrape(ctx, mockSession{}, scraper.ScrapeOptions{})
	if err != nil {
		t.Fatalf("Scrape() error: %v", err)
	}

	var results []scraper.Result
	for r := range ch {
		results = append(results, r)
	}

	if len(results) != 1 {
		t.Errorf("got %d results, want 1", len(results))
	}

	if results[0].ID != "single" {
		t.Errorf("ID = %q, want %q", results[0].ID, "single")
	}
}

func TestModeProxy_Scrape_CallError(t *testing.T) {
	proxy := setupModeProxyWithMock(t, func(req *Request) *Response {
		return &Response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &RPCError{Code: CodeInternalError, Message: "scrape failed"},
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := proxy.Scrape(ctx, mockSession{}, scraper.ScrapeOptions{})
	if err != nil {
		t.Fatalf("Scrape() error: %v", err)
	}

	// Channel should close with no results since Call returned error.
	var results []scraper.Result
	for r := range ch {
		results = append(results, r)
	}

	if len(results) != 0 {
		t.Errorf("got %d results, want 0 on error", len(results))
	}
}

func TestModeProxy_Scrape_ClientStartFailed(t *testing.T) {
	manifest := &Manifest{Name: "bad-plugin", Version: "1.0.0", Command: "/nonexistent/binary"}
	mgr := NewManager(nil, nil)

	proxy := &ModeProxy{
		entry:    ModeEntry{Name: "test"},
		manifest: manifest,
		manager:  mgr,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := proxy.Scrape(ctx, mockSession{}, scraper.ScrapeOptions{})
	if err == nil {
		t.Fatal("expected error for failed client start")
	}
}
