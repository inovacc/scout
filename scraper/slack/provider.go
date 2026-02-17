package slack

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/scraper/auth"
)

// SlackProvider implements auth.Provider for Slack workspaces.
type SlackProvider struct {
	workspace string
}

// NewProvider creates a Slack auth provider for the given workspace domain.
func NewProvider(workspace string) *SlackProvider {
	return &SlackProvider{workspace: workspace}
}

// Name returns "slack".
func (p *SlackProvider) Name() string { return "slack" }

// LoginURL returns the Slack workspace login URL.
func (p *SlackProvider) LoginURL() string {
	return normalizeWorkspaceURL(p.workspace)
}

// DetectAuth checks if the page has a valid xoxc token and d cookie.
func (p *SlackProvider) DetectAuth(_ context.Context, page *scout.Page) (bool, error) {
	result, err := page.Eval(tokenExtractionJS)
	if err != nil {
		return false, nil
	}

	token := result.String()
	if token == "" || !strings.HasPrefix(token, "xoxc-") {
		return false, nil
	}

	cookies, err := page.GetCookies()
	if err != nil {
		return false, nil
	}

	for _, c := range cookies {
		if c.Name == "d" && c.Value != "" {
			return true, nil
		}
	}

	return false, nil
}

// CaptureSession extracts the full Slack session from an authenticated page.
func (p *SlackProvider) CaptureSession(_ context.Context, page *scout.Page) (*auth.Session, error) {
	captured, err := CaptureFromPage(page)
	if err != nil {
		return nil, err
	}

	return &auth.Session{
		Provider:       "slack",
		Version:        captured.Version,
		Timestamp:      captured.Timestamp,
		URL:            captured.WorkspaceURL,
		Cookies:        captured.Cookies,
		Tokens:         map[string]string{"xoxc": captured.Token},
		LocalStorage:   captured.LocalStorage,
		SessionStorage: captured.SessionStorage,
		Extra: map[string]string{
			"d_cookie":  captured.DCookie,
			"user_id":   captured.UserID,
			"team_name": captured.TeamName,
		},
	}, nil
}

// ValidateSession checks if a Slack session is still valid by calling auth.test.
func (p *SlackProvider) ValidateSession(ctx context.Context, session *auth.Session) error {
	token := session.Tokens["xoxc"]
	dCookie := session.Extra["d_cookie"]

	if token == "" || dCookie == "" {
		return fmt.Errorf("slack: session missing xoxc token or d cookie")
	}

	api := newAPIClient(token, dCookie, 0)

	if _, err := api.authTest(ctx); err != nil {
		return fmt.Errorf("slack: session invalid: %w", err)
	}

	return nil
}

// FromAuthSession converts a generic auth.Session back into a Slack CapturedSession.
func FromAuthSession(s *auth.Session) *CapturedSession {
	return &CapturedSession{
		Version:        s.Version,
		Timestamp:      s.Timestamp,
		WorkspaceURL:   s.URL,
		Token:          s.Tokens["xoxc"],
		DCookie:        s.Extra["d_cookie"],
		Cookies:        s.Cookies,
		LocalStorage:   s.LocalStorage,
		SessionStorage: s.SessionStorage,
		UserID:         s.Extra["user_id"],
		TeamName:       s.Extra["team_name"],
	}
}

func init() {
	// Register a default Slack provider (workspace can be set later via NewProvider).
	// Individual scraper instances create their own providers with specific workspaces.
	_ = time.Now // avoid unused import
}
