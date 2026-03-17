package plugin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/scraper/auth"
)

// AuthProxy bridges a plugin's auth_provider capability to the auth.Provider interface.
// It forwards RPC calls to the plugin subprocess.
type AuthProxy struct {
	client   *Client
	name     string
	loginURL string
}

// NewAuthProxy creates an AuthProxy for the given plugin client.
func NewAuthProxy(client *Client, name, loginURL string) *AuthProxy {
	return &AuthProxy{client: client, name: name, loginURL: loginURL}
}

// Name returns the provider name.
func (p *AuthProxy) Name() string { return p.name }

// LoginURL returns the URL to start authentication.
func (p *AuthProxy) LoginURL() string { return p.loginURL }

// DetectAuth checks if a page has valid authentication by sending page state to the plugin.
func (p *AuthProxy) DetectAuth(ctx context.Context, page *scout.Page) (bool, error) {
	pageState, err := capturePageState(page)
	if err != nil {
		return false, fmt.Errorf("plugin auth: capture page state: %w", err)
	}

	raw, err := p.client.Call(ctx, "auth/detect", pageState)
	if err != nil {
		return false, fmt.Errorf("plugin auth: detect: %w", err)
	}

	var result struct {
		Detected bool `json:"detected"`
	}

	if err := json.Unmarshal(raw, &result); err != nil {
		return false, fmt.Errorf("plugin auth: detect unmarshal: %w", err)
	}

	return result.Detected, nil
}

// CaptureSession extracts session data from an authenticated page.
func (p *AuthProxy) CaptureSession(ctx context.Context, page *scout.Page) (*auth.Session, error) {
	pageState, err := capturePageState(page)
	if err != nil {
		return nil, fmt.Errorf("plugin auth: capture page state: %w", err)
	}

	raw, err := p.client.Call(ctx, "auth/capture", pageState)
	if err != nil {
		return nil, fmt.Errorf("plugin auth: capture: %w", err)
	}

	var session auth.Session
	if err := json.Unmarshal(raw, &session); err != nil {
		return nil, fmt.Errorf("plugin auth: capture unmarshal: %w", err)
	}

	return &session, nil
}

// ValidateSession checks if a session is still valid.
func (p *AuthProxy) ValidateSession(ctx context.Context, session *auth.Session) error {
	raw, err := p.client.Call(ctx, "auth/validate", session)
	if err != nil {
		return fmt.Errorf("plugin auth: validate: %w", err)
	}

	var result struct {
		Valid   bool   `json:"valid"`
		Reason string `json:"reason,omitempty"`
	}

	if err := json.Unmarshal(raw, &result); err != nil {
		return fmt.Errorf("plugin auth: validate unmarshal: %w", err)
	}

	if !result.Valid {
		reason := result.Reason
		if reason == "" {
			reason = "session expired"
		}

		return &auth.AuthError{Reason: reason}
	}

	return nil
}

// PageState is the serialized page state sent to auth plugins.
// Plugins receive this instead of a live page handle for security isolation.
type PageState struct {
	URL            string            `json:"url"`
	Title          string            `json:"title"`
	Cookies        []scout.Cookie    `json:"cookies"`
	LocalStorage   map[string]string `json:"local_storage,omitempty"`
	SessionStorage map[string]string `json:"session_storage,omitempty"`
}

func capturePageState(page *scout.Page) (*PageState, error) {
	sess, err := page.SaveSession()
	if err != nil {
		return nil, err
	}

	return &PageState{
		URL:            sess.URL,
		Title:          "",
		Cookies:        sess.Cookies,
		LocalStorage:   sess.LocalStorage,
		SessionStorage: sess.SessionStorage,
	}, nil
}
