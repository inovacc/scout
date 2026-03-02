package scraper

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAuthError(t *testing.T) {
	err := &AuthError{Reason: "invalid token"}
	if err.Error() != "scraper: auth: invalid token" {
		t.Fatalf("unexpected error message: %s", err.Error())
	}

	var wrapped error = err

	var target *AuthError
	if !errors.As(wrapped, &target) {
		t.Fatal("errors.As should match *AuthError")
	}

	if target.Reason != "invalid token" {
		t.Fatalf("unexpected reason: %s", target.Reason)
	}
}

func TestRateLimitError(t *testing.T) {
	tests := []struct {
		name       string
		retryAfter int
		want       string
	}{
		{"with retry", 30, "scraper: rate limited: retry after 30s"},
		{"no retry", 0, "scraper: rate limited"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &RateLimitError{RetryAfter: tt.retryAfter}
			if err.Error() != tt.want {
				t.Fatalf("got %q, want %q", err.Error(), tt.want)
			}

			var wrapped error = err

			var target *RateLimitError
			if !errors.As(wrapped, &target) {
				t.Fatal("errors.As should match *RateLimitError")
			}
		})
	}
}

func TestCredentials(t *testing.T) {
	creds := Credentials{
		Token:   "xoxc-test",
		Cookies: map[string]string{"d": "abc123"},
		Extra:   map[string]string{"workspace": "test.slack.com"},
	}

	b, err := json.Marshal(creds)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got Credentials
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Token != creds.Token {
		t.Fatalf("token mismatch: got %q, want %q", got.Token, creds.Token)
	}

	if got.Cookies["d"] != "abc123" {
		t.Fatalf("cookie mismatch: got %q", got.Cookies["d"])
	}
}

func TestProgress(t *testing.T) {
	p := Progress{
		Phase:   "messages",
		Current: 50,
		Total:   100,
		Message: "fetching messages",
	}

	if p.Phase != "messages" || p.Current != 50 || p.Total != 100 {
		t.Fatalf("unexpected progress: %+v", p)
	}
}

func TestExportJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "output.json")

	data := map[string]string{"key": "value"}
	if err := ExportJSON(data, path); err != nil {
		t.Fatalf("ExportJSON: %v", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	var got map[string]string
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got["key"] != "value" {
		t.Fatalf("unexpected value: %q", got["key"])
	}
}

func TestExportJSON_InvalidData(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "output.json")

	// Channels can't be marshaled to JSON
	err := ExportJSON(make(chan int), path)
	if err == nil {
		t.Fatal("expected error for unmarshalable data")
	}
}
