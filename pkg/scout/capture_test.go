package scout

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestSaveLoadCredentials(t *testing.T) {
	creds := &CapturedCredentials{
		URL:        "https://example.com/login",
		FinalURL:   "https://example.com/dashboard",
		CapturedAt: time.Date(2026, 2, 24, 12, 0, 0, 0, time.UTC),
		Browser: BrowserInfo{
			Product:  "Chrome/120.0",
			Platform: "linux",
			OS:       "linux",
			Arch:     "amd64",
		},
		Cookies: []Cookie{
			{Name: "session", Value: "abc123", Domain: ".example.com", Path: "/", Secure: true, HTTPOnly: true},
			{Name: "csrf", Value: "xyz789", Domain: ".example.com", Path: "/"},
		},
		LocalStorage:   map[string]string{"token": "bearer-abc", "theme": "dark"},
		SessionStorage: map[string]string{"tab_id": "42"},
		UserAgent:      "Mozilla/5.0 TestAgent",
	}

	path := filepath.Join(t.TempDir(), "creds.json")

	if err := SaveCredentials(creds, path); err != nil {
		t.Fatalf("SaveCredentials: %v", err)
	}

	loaded, err := LoadCredentials(path)
	if err != nil {
		t.Fatalf("LoadCredentials: %v", err)
	}

	if loaded.URL != creds.URL {
		t.Errorf("URL = %q, want %q", loaded.URL, creds.URL)
	}

	if loaded.FinalURL != creds.FinalURL {
		t.Errorf("FinalURL = %q, want %q", loaded.FinalURL, creds.FinalURL)
	}

	if !loaded.CapturedAt.Equal(creds.CapturedAt) {
		t.Errorf("CapturedAt = %v, want %v", loaded.CapturedAt, creds.CapturedAt)
	}

	if loaded.Browser.Product != creds.Browser.Product {
		t.Errorf("Browser.Product = %q, want %q", loaded.Browser.Product, creds.Browser.Product)
	}

	if loaded.Browser.Platform != creds.Browser.Platform {
		t.Errorf("Browser.Platform = %q, want %q", loaded.Browser.Platform, creds.Browser.Platform)
	}

	if loaded.Browser.Arch != creds.Browser.Arch {
		t.Errorf("Browser.Arch = %q, want %q", loaded.Browser.Arch, creds.Browser.Arch)
	}

	if loaded.UserAgent != creds.UserAgent {
		t.Errorf("UserAgent = %q, want %q", loaded.UserAgent, creds.UserAgent)
	}

	if len(loaded.Cookies) != len(creds.Cookies) {
		t.Fatalf("Cookies len = %d, want %d", len(loaded.Cookies), len(creds.Cookies))
	}

	for i, c := range loaded.Cookies {
		if c.Name != creds.Cookies[i].Name || c.Value != creds.Cookies[i].Value {
			t.Errorf("Cookie[%d] = %s=%s, want %s=%s", i, c.Name, c.Value, creds.Cookies[i].Name, creds.Cookies[i].Value)
		}
	}

	if loaded.LocalStorage["token"] != "bearer-abc" {
		t.Errorf("LocalStorage[token] = %q, want %q", loaded.LocalStorage["token"], "bearer-abc")
	}

	if loaded.SessionStorage["tab_id"] != "42" {
		t.Errorf("SessionStorage[tab_id] = %q, want %q", loaded.SessionStorage["tab_id"], "42")
	}
}

func TestToSessionState(t *testing.T) {
	creds := &CapturedCredentials{
		URL:      "https://example.com/login",
		FinalURL: "https://example.com/dashboard",
		Cookies: []Cookie{
			{Name: "session", Value: "abc123", Domain: ".example.com"},
		},
		LocalStorage:   map[string]string{"key1": "val1"},
		SessionStorage: map[string]string{"key2": "val2"},
	}

	state := creds.ToSessionState()

	if state.URL != creds.FinalURL {
		t.Errorf("URL = %q, want %q (FinalURL)", state.URL, creds.FinalURL)
	}

	if len(state.Cookies) != 1 || state.Cookies[0].Name != "session" {
		t.Errorf("Cookies = %+v, want single cookie named 'session'", state.Cookies)
	}

	if state.LocalStorage["key1"] != "val1" {
		t.Errorf("LocalStorage[key1] = %q, want %q", state.LocalStorage["key1"], "val1")
	}

	if state.SessionStorage["key2"] != "val2" {
		t.Errorf("SessionStorage[key2] = %q, want %q", state.SessionStorage["key2"], "val2")
	}
}

func TestSaveCredentials_Permissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permission check not reliable on Windows")
	}

	creds := &CapturedCredentials{URL: "https://example.com"}
	path := filepath.Join(t.TempDir(), "creds.json")

	if err := SaveCredentials(creds, path); err != nil {
		t.Fatalf("SaveCredentials: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}
}

func TestLoadCredentials_NotFound(t *testing.T) {
	_, err := LoadCredentials(filepath.Join(t.TempDir(), "nonexistent.json"))
	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}
}

func TestLoadCredentials_InvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("{not valid json!!!"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := LoadCredentials(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}
