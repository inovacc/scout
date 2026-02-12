package slack

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/scraper"
)

// CapturedSession holds all browser session data needed to restore a Slack login.
type CapturedSession struct {
	Version        string            `json:"version"`
	Timestamp      time.Time         `json:"timestamp"`
	WorkspaceURL   string            `json:"workspace_url"`
	Token          string            `json:"token"`
	DCookie        string            `json:"d_cookie"`
	Cookies        []scout.Cookie    `json:"cookies"`
	LocalStorage   map[string]string `json:"local_storage"`
	SessionStorage map[string]string `json:"session_storage"`
	UserID         string            `json:"user_id,omitempty"`
	TeamName       string            `json:"team_name,omitempty"`
}

// CaptureFromPage extracts session data from a logged-in Slack page.
// It evaluates tokenExtractionJS to find the xoxc token, reads cookies to find
// the d cookie, and captures localStorage/sessionStorage via SaveSession.
func CaptureFromPage(page *scout.Page) (*CapturedSession, error) {
	// Extract xoxc token
	result, err := page.Eval(tokenExtractionJS)
	if err != nil {
		return nil, fmt.Errorf("slack: capture: eval token: %w", err)
	}

	token := result.String()
	if token == "" || !strings.HasPrefix(token, "xoxc-") {
		return nil, fmt.Errorf("slack: capture: xoxc token not found on page")
	}

	// Get all cookies and find the d cookie
	cookies, err := page.GetCookies()
	if err != nil {
		return nil, fmt.Errorf("slack: capture: get cookies: %w", err)
	}

	dCookie := ""

	for _, c := range cookies {
		if c.Name == "d" {
			dCookie = c.Value
			break
		}
	}

	if dCookie == "" {
		return nil, fmt.Errorf("slack: capture: d cookie not found")
	}

	// Capture full session state (URL, cookies, localStorage, sessionStorage)
	state, err := page.SaveSession()
	if err != nil {
		return nil, fmt.Errorf("slack: capture: save session: %w", err)
	}

	return &CapturedSession{
		Version:        "1.0",
		Timestamp:      time.Now().UTC(),
		WorkspaceURL:   state.URL,
		Token:          token,
		DCookie:        dCookie,
		Cookies:        state.Cookies,
		LocalStorage:   state.LocalStorage,
		SessionStorage: state.SessionStorage,
	}, nil
}

// SaveEncrypted marshals the session to JSON, encrypts it, and writes to the given path.
func SaveEncrypted(session *CapturedSession, path, passphrase string) error {
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("slack: save session: marshal: %w", err)
	}

	encrypted, err := scraper.EncryptData(data, passphrase)
	if err != nil {
		return fmt.Errorf("slack: save session: %w", err)
	}

	if err := os.WriteFile(path, encrypted, 0o600); err != nil {
		return fmt.Errorf("slack: save session: write: %w", err)
	}

	return nil
}

// LoadEncrypted reads an encrypted session file, decrypts it, and unmarshals into CapturedSession.
func LoadEncrypted(path, passphrase string) (*CapturedSession, error) {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("slack: load session: read: %w", err)
	}

	plaintext, err := scraper.DecryptData(data, passphrase)
	if err != nil {
		return nil, fmt.Errorf("slack: load session: %w", err)
	}

	var session CapturedSession
	if err := json.Unmarshal(plaintext, &session); err != nil {
		return nil, fmt.Errorf("slack: load session: unmarshal: %w", err)
	}

	return &session, nil
}
