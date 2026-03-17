package tiktok

import (
	"context"
	"testing"
	"time"

	"github.com/inovacc/scout/pkg/scout/scraper"
)

func TestTikTokMode_Name(t *testing.T) {
	m := &TikTokMode{}
	if m.Name() != "tiktok" {
		t.Errorf("Name() = %q, want tiktok", m.Name())
	}
}

func TestTikTokMode_Description(t *testing.T) {
	m := &TikTokMode{}
	if m.Description() == "" {
		t.Error("Description() is empty")
	}
}

func TestTikTokMode_AuthProvider(t *testing.T) {
	m := &TikTokMode{}
	p := m.AuthProvider()

	if p == nil {
		t.Fatal("AuthProvider() is nil")
	}

	if p.Name() != "tiktok" {
		t.Errorf("AuthProvider().Name() = %q, want tiktok", p.Name())
	}
}

func TestTikTokProvider_LoginURL(t *testing.T) {
	p := &tiktokProvider{}
	url := p.LoginURL()

	if url != "https://www.tiktok.com/login" {
		t.Errorf("LoginURL() = %q, want https://www.tiktok.com/login", url)
	}
}

func TestTikTokProvider_ValidateSession_Nil(t *testing.T) {
	p := &tiktokProvider{}

	if err := p.ValidateSession(context.Background(), nil); err == nil {
		t.Error("expected error for nil session")
	}
}

func TestTikTokProvider_ValidateSession_MissingTokens(t *testing.T) {
	p := &tiktokProvider{}

	// ValidateSession takes *auth.Session — test nil path only here.
	// Full token validation tested via integration tests.
	if err := p.ValidateSession(context.Background(), nil); err == nil {
		t.Error("expected error for nil session")
	}
}

func TestResolveTarget(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://www.tiktok.com/@user", "https://www.tiktok.com/@user"},
		{"@cooluser", "https://www.tiktok.com/@cooluser"},
		{"#dance", "https://www.tiktok.com/tag/dance"},
		{"someuser", "https://www.tiktok.com/@someuser"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := resolveTarget(tt.input)
			if got != tt.want {
				t.Errorf("resolveTarget(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractVideoID(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://www.tiktok.com/@user/video/1234567890", "1234567890"},
		{"https://www.tiktok.com/@user/video/1234567890?is_copy_url=1", "1234567890"},
		{"https://www.tiktok.com/some/other/path", "https://www.tiktok.com/some/other/path"},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := extractVideoID(tt.url)
			if got != tt.want {
				t.Errorf("extractVideoID(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestTikTokMode_Registered(t *testing.T) {
	mode, err := scraper.GetMode("tiktok")
	if err != nil {
		t.Fatalf("tiktok mode not registered: %v", err)
	}

	if mode.Name() != "tiktok" {
		t.Errorf("registered mode name = %q, want tiktok", mode.Name())
	}
}

func TestTikTokMode_Scrape_CancelledContext(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping browser test in -short mode")
	}

	m := &TikTokMode{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	opts := scraper.ScrapeOptions{
		Headless: true,
		Stealth:  true,
		Timeout:  10 * time.Second,
		Targets:  []string{"@tiktok"},
		Limit:    1,
	}

	results, err := m.Scrape(ctx, nil, opts)
	if err != nil {
		t.Fatalf("Scrape: %v", err)
	}

	count := 0
	for range results {
		count++
	}

	// Cancelled context should yield 0 results.
	if count != 0 {
		t.Errorf("expected 0 results on cancelled context, got %d", count)
	}
}
