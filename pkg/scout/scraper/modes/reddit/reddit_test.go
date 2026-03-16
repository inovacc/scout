package reddit

import (
	"testing"
	"time"

	"github.com/inovacc/scout/pkg/scout/scraper"
	"github.com/inovacc/scout/pkg/scout/scraper/auth"
)

// --- Mode metadata tests ---

func TestRedditMode_Name(t *testing.T) {
	m := &RedditMode{}
	if got := m.Name(); got != "reddit" {
		t.Errorf("Name() = %q, want %q", got, "reddit")
	}
}

func TestRedditMode_Description(t *testing.T) {
	m := &RedditMode{}
	if got := m.Description(); got == "" {
		t.Error("Description() is empty")
	}
}

func TestRedditMode_AuthProvider(t *testing.T) {
	m := &RedditMode{}
	p := m.AuthProvider()
	if p == nil {
		t.Fatal("AuthProvider() is nil")
	}
	if p.Name() != "reddit" {
		t.Errorf("AuthProvider().Name() = %q, want %q", p.Name(), "reddit")
	}
}

func TestRedditMode_AuthProvider_LazyInit(t *testing.T) {
	m := &RedditMode{}
	p1 := m.AuthProvider()
	p2 := m.AuthProvider()
	if p1 != p2 {
		t.Error("AuthProvider() should return the same instance")
	}
}

// --- redditProvider tests ---

func TestRedditProvider_LoginURL(t *testing.T) {
	p := &redditProvider{}
	if got := p.LoginURL(); got != "https://www.reddit.com/login" {
		t.Errorf("LoginURL() = %q", got)
	}
}

// --- ValidateSession tests ---

func TestValidateSession_NilSession(t *testing.T) {
	p := &redditProvider{}
	err := p.ValidateSession(nil, nil)
	if err == nil {
		t.Fatal("expected error for nil session")
	}
}

func TestValidateSession_ExpiredSession(t *testing.T) {
	p := &redditProvider{}
	s := &auth.Session{
		Tokens:    map[string]string{"reddit_session": "abc"},
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	err := p.ValidateSession(nil, s)
	if err == nil {
		t.Fatal("expected error for expired session")
	}
}

func TestValidateSession_WithRedditSession(t *testing.T) {
	p := &redditProvider{}
	s := &auth.Session{
		Tokens:    map[string]string{"reddit_session": "abc123"},
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	if err := p.ValidateSession(nil, s); err != nil {
		t.Errorf("ValidateSession() error = %v", err)
	}
}

func TestValidateSession_WithTokenV2(t *testing.T) {
	p := &redditProvider{}
	s := &auth.Session{
		Tokens:    map[string]string{"token_v2": "xyz789"},
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	if err := p.ValidateSession(nil, s); err != nil {
		t.Errorf("ValidateSession() error = %v", err)
	}
}

func TestValidateSession_NoRequiredTokens(t *testing.T) {
	p := &redditProvider{}
	s := &auth.Session{
		Tokens:    map[string]string{"other_key": "value"},
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	err := p.ValidateSession(nil, s)
	if err == nil {
		t.Fatal("expected error when no required tokens")
	}
}

func TestValidateSession_EmptyTokens(t *testing.T) {
	p := &redditProvider{}
	s := &auth.Session{
		Tokens:    map[string]string{},
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	err := p.ValidateSession(nil, s)
	if err == nil {
		t.Fatal("expected error for empty tokens")
	}
}

// --- parseScore tests ---

func TestParseScore(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{"simple number", "500", 500, false},
		{"with commas", "1,234", 1234, false},
		{"k suffix", "1.5k", 1500, false},
		{"K suffix", "2.3K", 2300, false},
		{"m suffix", "1.5m", 1500000, false},
		{"M suffix", "2M", 2000000, false},
		{"with comments suffix", "42 comments", 42, false},
		{"with comment suffix", "1 comment", 1, false},
		{"with points suffix", "100 points", 100, false},
		{"with point suffix", "1 point", 1, false},
		{"bullet character", "\u2022", 0, false},
		{"Vote text", "Vote", 0, false},
		{"empty string", "", 0, false},
		{"whitespace", "  500  ", 500, false},
		{"zero", "0", 0, false},
		{"invalid text", "abc", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseScore(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseScore(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseScore(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

// --- parseJSONArray tests ---

func TestParseJSONArray_Valid(t *testing.T) {
	type item struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	raw := `[{"id":"1","name":"first"},{"id":"2","name":"second"}]`
	var out []item

	err := parseJSONArray(raw, &out)
	if err != nil {
		t.Fatalf("parseJSONArray() error = %v", err)
	}

	if len(out) != 2 {
		t.Fatalf("got %d items, want 2", len(out))
	}
	if out[0].ID != "1" || out[0].Name != "first" {
		t.Errorf("item[0] = %+v", out[0])
	}
}

func TestParseJSONArray_EscapedQuotes(t *testing.T) {
	// Simulates a string that was JSON-stringified (escaped quotes)
	raw := `"[{\"id\":\"1\"}]"`
	type item struct {
		ID string `json:"id"`
	}

	var out []item

	err := parseJSONArray(raw, &out)
	if err != nil {
		t.Fatalf("parseJSONArray() error = %v", err)
	}

	if len(out) != 1 || out[0].ID != "1" {
		t.Errorf("got %+v", out)
	}
}

func TestParseJSONArray_InvalidJSON(t *testing.T) {
	var out []struct{}
	err := parseJSONArray("not json", &out)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseJSONArray_Empty(t *testing.T) {
	var out []struct{}
	err := parseJSONArray("[]", &out)
	if err != nil {
		t.Fatalf("parseJSONArray() error = %v", err)
	}
	if len(out) != 0 {
		t.Errorf("got %d items, want 0", len(out))
	}
}

// --- parseJSONObject tests ---

func TestParseJSONObject_Valid(t *testing.T) {
	type info struct {
		Description string `json:"description"`
		Members     string `json:"members"`
	}

	raw := `{"description":"A cool sub","members":"1500"}`
	var out info

	err := parseJSONObject(raw, &out)
	if err != nil {
		t.Fatalf("parseJSONObject() error = %v", err)
	}

	if out.Description != "A cool sub" {
		t.Errorf("Description = %q", out.Description)
	}
	if out.Members != "1500" {
		t.Errorf("Members = %q", out.Members)
	}
}

func TestParseJSONObject_EscapedQuotes(t *testing.T) {
	type info struct {
		Name string `json:"name"`
	}

	raw := `"{\"name\":\"test\"}"`
	var out info

	err := parseJSONObject(raw, &out)
	if err != nil {
		t.Fatalf("parseJSONObject() error = %v", err)
	}

	if out.Name != "test" {
		t.Errorf("Name = %q", out.Name)
	}
}

func TestParseJSONObject_InvalidJSON(t *testing.T) {
	var out struct{}
	err := parseJSONObject("not json", &out)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// --- emitProgress tests ---

func TestEmitProgress_NilCallback(t *testing.T) {
	m := &RedditMode{}
	// Should not panic
	m.emitProgress(nil, "test", "message")
}

func TestEmitProgress_WithCallback(t *testing.T) {
	m := &RedditMode{}
	var got string
	fn := func(p scraper.Progress) { got = p.Message }

	m.emitProgress(fn, "scraping", "extracting posts")
	if got != "extracting posts" {
		t.Errorf("got %q, want 'extracting posts'", got)
	}
}
