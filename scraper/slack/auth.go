package slack

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/inovacc/scout"
	"github.com/inovacc/scout/scraper"
)

// tokenExtractionJS tries multiple Slack storage locations to find the xoxc token.
const tokenExtractionJS = `() => {
	// Try localConfig_v2
	try {
		const keys = Object.keys(localStorage);
		for (const key of keys) {
			if (key.startsWith('localConfig_v2')) {
				const val = localStorage.getItem(key);
				if (val) {
					const parsed = JSON.parse(val);
					const teams = parsed.teams || {};
					for (const teamId of Object.keys(teams)) {
						const token = teams[teamId].token;
						if (token && token.startsWith('xoxc-')) {
							return token;
						}
					}
				}
			}
		}
	} catch(e) {}

	// Try window.boot_data
	try {
		if (window.boot_data && window.boot_data.api_token) {
			const token = window.boot_data.api_token;
			if (token.startsWith('xoxc-')) {
				return token;
			}
		}
	} catch(e) {}

	// Try all localStorage keys for xoxc pattern
	try {
		for (let i = 0; i < localStorage.length; i++) {
			const key = localStorage.key(i);
			const val = localStorage.getItem(key);
			if (val && val.includes('xoxc-')) {
				const match = val.match(/(xoxc-[a-zA-Z0-9-]+)/);
				if (match) return match[1];
			}
		}
	} catch(e) {}

	return "";
}`

// authenticateWithToken validates an existing xoxc token and d cookie.
func (s *Scraper) authenticateWithToken(ctx context.Context) error {
	s.api = newAPIClient(s.opts.token, s.opts.dCookie, s.opts.rateLimit)

	resp, err := s.api.authTest(ctx)
	if err != nil {
		s.api = nil
		return fmt.Errorf("slack: authenticate: %w", err)
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

// authenticateWithBrowser opens a Slack workspace in a browser and extracts the xoxc token.
func (s *Scraper) authenticateWithBrowser(ctx context.Context) error {
	if s.opts.workspace == "" {
		return &scraper.AuthError{Reason: "workspace domain is required for browser login"}
	}

	browserOpts := []scout.Option{
		scout.WithHeadless(s.opts.headless),
		scout.WithTimeout(s.opts.timeout),
		scout.WithNoSandbox(),
	}

	if s.opts.stealth {
		browserOpts = append(browserOpts, scout.WithStealth())
	}

	if s.opts.userDataDir != "" {
		browserOpts = append(browserOpts, scout.WithUserDataDir(s.opts.userDataDir))
	}

	browser, err := scout.New(browserOpts...)
	if err != nil {
		return fmt.Errorf("slack: launch browser: %w", err)
	}

	defer func() { _ = browser.Close() }()

	workspaceURL := normalizeWorkspaceURL(s.opts.workspace)

	page, err := browser.NewPage(workspaceURL)
	if err != nil {
		return fmt.Errorf("slack: open workspace: %w", err)
	}

	s.reportProgress("auth", 0, 0, "waiting for login...")

	// Poll for token extraction
	deadline := time.Now().Add(s.opts.timeout)
	pollInterval := 2 * time.Second

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return fmt.Errorf("slack: authenticate: %w", ctx.Err())
		default:
		}

		// Try to extract xoxc token
		result, err := page.Eval(tokenExtractionJS)
		if err == nil {
			token := result.String()
			if token != "" && strings.HasPrefix(token, "xoxc-") {
				// Extract d cookie
				cookies, err := page.GetCookies()
				if err != nil {
					return fmt.Errorf("slack: get cookies: %w", err)
				}

				dCookie := ""

				for _, c := range cookies {
					if c.Name == "d" {
						dCookie = c.Value
						break
					}
				}

				if dCookie == "" {
					return &scraper.AuthError{Reason: "xoxc token found but d cookie missing"}
				}

				// Validate the extracted credentials
				s.opts.token = token
				s.opts.dCookie = dCookie

				return s.authenticateWithToken(ctx)
			}
		}

		time.Sleep(pollInterval)
	}

	return &scraper.AuthError{Reason: "login timeout: could not extract xoxc token"}
}

// normalizeWorkspaceURL ensures the workspace URL is fully qualified.
func normalizeWorkspaceURL(workspace string) string {
	if strings.HasPrefix(workspace, "http://") || strings.HasPrefix(workspace, "https://") {
		return workspace
	}

	if !strings.Contains(workspace, ".") {
		workspace += ".slack.com"
	}

	return "https://" + workspace
}

// extractDomain extracts the domain from a URL like "https://team.slack.com/".
func extractDomain(u string) string {
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	u = strings.TrimSuffix(u, "/")

	return u
}
