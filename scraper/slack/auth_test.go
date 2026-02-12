package slack

import (
	"context"
	"errors"
	"testing"

	"github.com/inovacc/scout/scraper"
)

func TestAuthenticateWithToken_Valid(t *testing.T) {
	srv := newMockSlackAPI()
	defer srv.Close()

	s := New(
		WithToken("xoxc-valid-token"),
		WithDCookie("xoxd-test"),
		WithRateLimit(0),
	)

	err := s.authenticateWithTokenTestable(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("authenticateWithToken: %v", err)
	}

	ws := s.GetWorkspace()

	if ws.Name != "Test Team" {
		t.Fatalf("workspace name = %q, want %q", ws.Name, "Test Team")
	}

	if ws.ID != "T01TEST" {
		t.Fatalf("workspace ID = %q, want %q", ws.ID, "T01TEST")
	}

	creds := s.GetCredentials()

	if creds.Token != "xoxc-valid-token" {
		t.Fatalf("token = %q, want %q", creds.Token, "xoxc-valid-token")
	}

	if creds.Cookies["d"] != "xoxd-test" {
		t.Fatalf("d cookie = %q, want %q", creds.Cookies["d"], "xoxd-test")
	}
}

func TestAuthenticateWithToken_Invalid(t *testing.T) {
	srv := newMockSlackAPI()
	defer srv.Close()

	s := New(
		WithToken("xoxc-bad-token"),
		WithDCookie("xoxd-test"),
		WithRateLimit(0),
	)

	err := s.authenticateWithTokenTestable(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("expected error for invalid token")
	}

	var authErr *scraper.AuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected AuthError, got %T: %v", err, err)
	}

	if s.api != nil {
		t.Fatal("api should be nil after failed auth")
	}
}

func TestAuthenticateWithBrowser_NoWorkspace(t *testing.T) {
	s := New(WithRateLimit(0))

	err := s.authenticateWithBrowser(context.Background())
	if err == nil {
		t.Fatal("expected error when workspace is empty")
	}

	var authErr *scraper.AuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected AuthError, got %T: %v", err, err)
	}
}

func TestNormalizeWorkspaceURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"myteam", "https://myteam.slack.com"},
		{"myteam.slack.com", "https://myteam.slack.com"},
		{"https://myteam.slack.com", "https://myteam.slack.com"},
		{"http://myteam.slack.com", "http://myteam.slack.com"},
		{"myteam.enterprise.slack.com", "https://myteam.enterprise.slack.com"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeWorkspaceURL(tt.input)
			if got != tt.want {
				t.Fatalf("normalizeWorkspaceURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractDomain(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://myteam.slack.com/", "myteam.slack.com"},
		{"https://myteam.slack.com", "myteam.slack.com"},
		{"http://myteam.slack.com/", "myteam.slack.com"},
		{"myteam.slack.com", "myteam.slack.com"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := extractDomain(tt.input)
			if got != tt.want {
				t.Fatalf("extractDomain(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// authenticateWithTokenTestable is a test helper that allows overriding the base URL.
func (s *Scraper) authenticateWithTokenTestable(ctx context.Context, baseURL string) error {
	s.api = newAPIClient(s.opts.token, s.opts.dCookie, s.opts.rateLimit)
	s.api.baseURL = baseURL

	resp, err := s.api.authTest(ctx)
	if err != nil {
		s.api = nil
		return err
	}

	s.workspace = Workspace{
		ID:     resp.TeamID,
		Name:   resp.Team,
		Domain: extractDomain(resp.URL),
		URL:    resp.URL,
	}

	s.creds = scraper.Credentials{
		Token:   s.opts.token,
		Cookies: map[string]string{"d": s.opts.dCookie},
		Extra: map[string]string{
			"workspace": s.workspace.Domain,
			"user":      resp.User,
			"user_id":   resp.UserID,
		},
	}

	return nil
}
