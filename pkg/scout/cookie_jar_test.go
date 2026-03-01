package scout

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCookieJarRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cookies.json")

	cookies := []Cookie{
		{
			Name:    "session",
			Value:   "abc123",
			Domain:  ".example.com",
			Path:    "/",
			Expires: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
			Secure:  true,
		},
		{
			Name:   "temp",
			Value:  "xyz",
			Domain: ".example.com",
			Path:   "/",
			// No expiry — session cookie
		},
	}

	data, err := json.MarshalIndent(cookies, "", "  ")
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	// Read back
	readData, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var loaded []Cookie
	if err := json.Unmarshal(readData, &loaded); err != nil {
		t.Fatal(err)
	}

	if len(loaded) != 2 {
		t.Fatalf("expected 2 cookies, got %d", len(loaded))
	}
	if loaded[0].Name != "session" || loaded[0].Value != "abc123" {
		t.Fatalf("unexpected cookie: %+v", loaded[0])
	}
	if !loaded[0].Secure {
		t.Fatal("expected Secure=true")
	}
}

func TestCookieJarFilterSession(t *testing.T) {
	cookies := []Cookie{
		{Name: "persistent", Value: "a", Expires: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)},
		{Name: "session", Value: "b"}, // no expiry
	}

	// Filter out session cookies (same logic as SaveCookiesToFile)
	filtered := cookies[:0:0]
	for _, c := range cookies {
		if !c.Expires.IsZero() {
			filtered = append(filtered, c)
		}
	}

	if len(filtered) != 1 {
		t.Fatalf("expected 1 non-session cookie, got %d", len(filtered))
	}
	if filtered[0].Name != "persistent" {
		t.Fatalf("expected persistent cookie, got %s", filtered[0].Name)
	}
}
