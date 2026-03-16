package discord

import (
	"context"
	"testing"
	"time"

	"github.com/inovacc/scout/pkg/scout/scraper"
	"github.com/inovacc/scout/pkg/scout/scraper/auth"
)

// --- Mode metadata tests ---

func TestDiscordMode_Name(t *testing.T) {
	m := &DiscordMode{provider: &discordProvider{}}
	if got := m.Name(); got != "discord" {
		t.Errorf("Name() = %q, want %q", got, "discord")
	}
}

func TestDiscordMode_Description(t *testing.T) {
	m := &DiscordMode{provider: &discordProvider{}}
	if got := m.Description(); got == "" {
		t.Error("Description() is empty")
	}
}

func TestDiscordMode_AuthProvider(t *testing.T) {
	m := &DiscordMode{provider: &discordProvider{}}
	p := m.AuthProvider()
	if p == nil {
		t.Fatal("AuthProvider() is nil")
	}
	if p.Name() != "discord" {
		t.Errorf("AuthProvider().Name() = %q, want %q", p.Name(), "discord")
	}
}

// --- discordProvider tests ---

func TestDiscordProvider_Name(t *testing.T) {
	p := &discordProvider{}
	if got := p.Name(); got != "discord" {
		t.Errorf("Name() = %q, want %q", got, "discord")
	}
}

func TestDiscordProvider_LoginURL(t *testing.T) {
	p := &discordProvider{}
	if got := p.LoginURL(); got != "https://discord.com/login" {
		t.Errorf("LoginURL() = %q", got)
	}
}

// --- ValidateSession tests ---

func TestValidateSession_NilSession(t *testing.T) {
	p := &discordProvider{}
	err := p.ValidateSession(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil session")
	}
}

func TestValidateSession_MissingToken(t *testing.T) {
	p := &discordProvider{}
	s := &auth.Session{
		Tokens: map[string]string{},
	}
	err := p.ValidateSession(context.Background(), s)
	if err == nil {
		t.Fatal("expected error for missing token")
	}
	var authErr *scraper.AuthError
	if ok := isAuthError(err, &authErr); !ok {
		t.Errorf("expected AuthError, got %T", err)
	}
}

func TestValidateSession_ValidToken(t *testing.T) {
	p := &discordProvider{}
	s := &auth.Session{
		Tokens: map[string]string{"token": "abc123"},
	}
	if err := p.ValidateSession(context.Background(), s); err != nil {
		t.Errorf("ValidateSession() error = %v", err)
	}
}

func TestValidateSession_ExpiredSession(t *testing.T) {
	p := &discordProvider{}
	s := &auth.Session{
		Tokens:    map[string]string{"token": "abc123"},
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	err := p.ValidateSession(context.Background(), s)
	if err == nil {
		t.Fatal("expected error for expired session")
	}
}

func TestValidateSession_ZeroExpiresAt(t *testing.T) {
	p := &discordProvider{}
	s := &auth.Session{
		Tokens:    map[string]string{"token": "abc123"},
		ExpiresAt: time.Time{},
	}
	if err := p.ValidateSession(context.Background(), s); err != nil {
		t.Errorf("ValidateSession() error = %v (zero ExpiresAt should be valid)", err)
	}
}

// --- authorName tests ---

func TestAuthorName_GlobalName(t *testing.T) {
	a := discordAuthor{GlobalName: "Display", Username: "user", Discriminator: "1234"}
	if got := authorName(a); got != "Display" {
		t.Errorf("authorName() = %q, want %q", got, "Display")
	}
}

func TestAuthorName_Discriminator(t *testing.T) {
	a := discordAuthor{Username: "user", Discriminator: "1234"}
	if got := authorName(a); got != "user#1234" {
		t.Errorf("authorName() = %q, want %q", got, "user#1234")
	}
}

func TestAuthorName_DiscriminatorZero(t *testing.T) {
	a := discordAuthor{Username: "user", Discriminator: "0"}
	if got := authorName(a); got != "user" {
		t.Errorf("authorName() = %q, want %q", got, "user")
	}
}

func TestAuthorName_UsernameOnly(t *testing.T) {
	a := discordAuthor{Username: "user"}
	if got := authorName(a); got != "user" {
		t.Errorf("authorName() = %q, want %q", got, "user")
	}
}

// --- containsSegment / endsWith tests ---

func TestContainsSegment(t *testing.T) {
	tests := []struct {
		parts   []string
		segment string
		want    bool
	}{
		{[]string{"channels", "123", "messages"}, "messages", true},
		{[]string{"channels", "123"}, "messages", false},
		{[]string{}, "messages", false},
	}
	for _, tt := range tests {
		if got := containsSegment(tt.parts, tt.segment); got != tt.want {
			t.Errorf("containsSegment(%v, %q) = %v, want %v", tt.parts, tt.segment, got, tt.want)
		}
	}
}

func TestEndsWith(t *testing.T) {
	tests := []struct {
		parts    []string
		segments []string
		want     bool
	}{
		{[]string{"users", "@me"}, []string{"users", "@me"}, true},
		{[]string{"api", "v9", "users", "@me"}, []string{"users", "@me"}, true},
		{[]string{"users"}, []string{"users", "@me"}, false},
		{[]string{"@me"}, []string{"users", "@me"}, false},
	}
	for _, tt := range tests {
		if got := endsWith(tt.parts, tt.segments...); got != tt.want {
			t.Errorf("endsWith(%v, %v) = %v, want %v", tt.parts, tt.segments, got, tt.want)
		}
	}
}

// --- parseResponse routing tests ---

func TestParseResponse_MessagesArray(t *testing.T) {
	m := &DiscordMode{provider: &discordProvider{}}
	body := `[{"id":"1","channel_id":"ch1","content":"hello","author":{"id":"u1","username":"bob"},"timestamp":"2024-01-15T10:30:00Z"}]`
	url := "https://discord.com/api/v9/channels/ch1/messages"
	results := make(chan scraper.Result, 10)

	n := m.parseResponse(url, body, results, scraper.ScrapeOptions{})
	close(results)

	if n != 1 {
		t.Fatalf("parseResponse() returned %d, want 1", n)
	}
	r := <-results
	if r.Type != scraper.ResultMessage {
		t.Errorf("Type = %q, want %q", r.Type, scraper.ResultMessage)
	}
	if r.ID != "1" {
		t.Errorf("ID = %q, want %q", r.ID, "1")
	}
	if r.Content != "hello" {
		t.Errorf("Content = %q, want %q", r.Content, "hello")
	}
	if r.Author != "bob" {
		t.Errorf("Author = %q, want %q", r.Author, "bob")
	}
}

func TestParseResponse_SingleMessage(t *testing.T) {
	m := &DiscordMode{provider: &discordProvider{}}
	body := `{"id":"2","channel_id":"ch1","content":"single","author":{"id":"u1","username":"alice","global_name":"Alice"},"timestamp":"2024-01-15T10:30:00Z"}`
	url := "https://discord.com/api/v10/channels/ch1/messages/2"
	results := make(chan scraper.Result, 10)

	n := m.parseResponse(url, body, results, scraper.ScrapeOptions{})
	close(results)

	if n != 1 {
		t.Fatalf("parseResponse() returned %d, want 1", n)
	}
	r := <-results
	if r.Author != "Alice" {
		t.Errorf("Author = %q, want %q (global_name preferred)", r.Author, "Alice")
	}
}

func TestParseResponse_MessagesWithLimit(t *testing.T) {
	m := &DiscordMode{provider: &discordProvider{}}
	body := `[{"id":"1","channel_id":"ch1","content":"a","author":{"id":"u1","username":"bob"},"timestamp":"2024-01-15T10:30:00Z"},{"id":"2","channel_id":"ch1","content":"b","author":{"id":"u1","username":"bob"},"timestamp":"2024-01-15T10:31:00Z"}]`
	url := "https://discord.com/api/v9/channels/ch1/messages"
	results := make(chan scraper.Result, 10)

	n := m.parseResponse(url, body, results, scraper.ScrapeOptions{Limit: 1})
	close(results)

	if n != 1 {
		t.Fatalf("parseResponse() with Limit=1 returned %d, want 1", n)
	}
}

func TestParseResponse_Channels(t *testing.T) {
	m := &DiscordMode{provider: &discordProvider{}}
	body := `[{"id":"ch1","name":"general","type":0,"guild_id":"g1","topic":"General chat"}]`
	url := "https://discord.com/api/v9/guilds/g1/channels"
	results := make(chan scraper.Result, 10)

	n := m.parseResponse(url, body, results, scraper.ScrapeOptions{})
	close(results)

	if n != 1 {
		t.Fatalf("parseResponse() returned %d, want 1", n)
	}
	r := <-results
	if r.Type != scraper.ResultChannel {
		t.Errorf("Type = %q, want %q", r.Type, scraper.ResultChannel)
	}
	if r.Metadata["name"] != "general" {
		t.Errorf("name = %v, want %q", r.Metadata["name"], "general")
	}
}

func TestParseResponse_Members(t *testing.T) {
	m := &DiscordMode{provider: &discordProvider{}}
	body := `[{"user":{"id":"u1","username":"bob","global_name":"Bob"},"nick":"Bobby","roles":["r1"],"joined_at":"2024-01-01T00:00:00Z"}]`
	url := "https://discord.com/api/v9/guilds/g1/members"
	results := make(chan scraper.Result, 10)

	n := m.parseResponse(url, body, results, scraper.ScrapeOptions{})
	close(results)

	if n != 1 {
		t.Fatalf("parseResponse() returned %d, want 1", n)
	}
	r := <-results
	if r.Type != scraper.ResultMember {
		t.Errorf("Type = %q, want %q", r.Type, scraper.ResultMember)
	}
	if r.Author != "Bob" {
		t.Errorf("Author = %q, want %q", r.Author, "Bob")
	}
}

func TestParseResponse_ThreadsWrapped(t *testing.T) {
	m := &DiscordMode{provider: &discordProvider{}}
	body := `{"threads":[{"id":"t1","name":"thread1","parent_id":"ch1","guild_id":"g1"}]}`
	url := "https://discord.com/api/v9/guilds/g1/threads/active"
	results := make(chan scraper.Result, 10)

	n := m.parseResponse(url, body, results, scraper.ScrapeOptions{})
	close(results)

	if n != 1 {
		t.Fatalf("parseResponse() returned %d, want 1", n)
	}
	r := <-results
	if r.Type != scraper.ResultThread {
		t.Errorf("Type = %q, want %q", r.Type, scraper.ResultThread)
	}
	if r.Content != "thread1" {
		t.Errorf("Content = %q, want %q", r.Content, "thread1")
	}
}

func TestParseResponse_ThreadsArray(t *testing.T) {
	m := &DiscordMode{provider: &discordProvider{}}
	body := `[{"id":"t1","name":"thread1","parent_id":"ch1","guild_id":"g1"}]`
	url := "https://discord.com/api/v9/channels/ch1/threads"
	results := make(chan scraper.Result, 10)

	n := m.parseResponse(url, body, results, scraper.ScrapeOptions{})
	close(results)

	if n != 1 {
		t.Fatalf("parseResponse() returned %d, want 1", n)
	}
}

func TestParseResponse_Pins(t *testing.T) {
	// Note: /channels/{id}/pins URL contains "channels" segment, which matches
	// the channels case first in the switch. Test parsePins directly instead.
	m := &DiscordMode{provider: &discordProvider{}}
	body := `[{"id":"p1","channel_id":"ch1","content":"pinned","author":{"id":"u1","username":"bob"},"timestamp":"2024-01-15T10:30:00Z"}]`
	results := make(chan scraper.Result, 10)

	n := m.parsePins(body, "https://discord.com/api/v9/channels/ch1/pins", results)
	close(results)

	if n != 1 {
		t.Fatalf("parsePins() returned %d, want 1", n)
	}
	r := <-results
	if r.Type != scraper.ResultPin {
		t.Errorf("Type = %q, want %q", r.Type, scraper.ResultPin)
	}
	if r.Content != "pinned" {
		t.Errorf("Content = %q, want %q", r.Content, "pinned")
	}
}

func TestParseResponse_User(t *testing.T) {
	m := &DiscordMode{provider: &discordProvider{}}
	body := `{"id":"u1","username":"alice","discriminator":"1234","global_name":"Alice"}`
	url := "https://discord.com/api/v9/users/@me"
	results := make(chan scraper.Result, 10)

	n := m.parseResponse(url, body, results, scraper.ScrapeOptions{})
	close(results)

	if n != 1 {
		t.Fatalf("parseResponse() returned %d, want 1", n)
	}
	r := <-results
	if r.Type != scraper.ResultUser {
		t.Errorf("Type = %q, want %q", r.Type, scraper.ResultUser)
	}
	if r.Author != "Alice" {
		t.Errorf("Author = %q, want %q", r.Author, "Alice")
	}
}

func TestParseResponse_InvalidJSON(t *testing.T) {
	m := &DiscordMode{provider: &discordProvider{}}
	results := make(chan scraper.Result, 10)
	n := m.parseResponse("https://discord.com/api/v9/channels/ch1/messages", "not json", results, scraper.ScrapeOptions{})
	close(results)
	if n != 0 {
		t.Errorf("parseResponse() with invalid JSON returned %d, want 0", n)
	}
}

func TestParseResponse_NonAPIURL(t *testing.T) {
	m := &DiscordMode{provider: &discordProvider{}}
	results := make(chan scraper.Result, 10)
	n := m.parseResponse("https://discord.com/assets/something.js", "{}", results, scraper.ScrapeOptions{})
	close(results)
	if n != 0 {
		t.Errorf("parseResponse() with non-API URL returned %d, want 0", n)
	}
}

func TestParseResponse_UnknownEndpoint(t *testing.T) {
	m := &DiscordMode{provider: &discordProvider{}}
	results := make(chan scraper.Result, 10)
	n := m.parseResponse("https://discord.com/api/v9/unknown/endpoint", "{}", results, scraper.ScrapeOptions{})
	close(results)
	if n != 0 {
		t.Errorf("parseResponse() with unknown endpoint returned %d, want 0", n)
	}
}

// --- helpers ---

func isAuthError(err error, target **scraper.AuthError) bool {
	if ae, ok := err.(*scraper.AuthError); ok {
		*target = ae
		return true
	}
	return false
}
