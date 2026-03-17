package sdk

import "context"

// AuthHandler handles auth-related RPC calls from Scout.
type AuthHandler interface {
	// LoginURL returns the URL to start the auth flow.
	LoginURL() string

	// Detect checks if page state indicates a valid authentication.
	Detect(ctx context.Context, state PageState) (bool, error)

	// Capture extracts session data from an authenticated page state.
	Capture(ctx context.Context, state PageState) (*SessionData, error)

	// Validate checks if a session is still valid.
	Validate(ctx context.Context, session SessionData) (bool, string, error)
}

// PageState is the serialized page state received from Scout.
type PageState struct {
	URL            string            `json:"url"`
	Title          string            `json:"title"`
	Cookies        []CookieData      `json:"cookies"`
	LocalStorage   map[string]string `json:"local_storage,omitempty"`
	SessionStorage map[string]string `json:"session_storage,omitempty"`
}

// CookieData is a simplified cookie structure.
type CookieData struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expires"`
	HTTPOnly bool    `json:"httpOnly"`
	Secure   bool    `json:"secure"`
}

// SessionData holds session information returned by auth plugins.
type SessionData struct {
	Provider       string            `json:"provider"`
	Version        string            `json:"version"`
	Timestamp      string            `json:"timestamp"`
	URL            string            `json:"url"`
	Cookies        []CookieData      `json:"cookies"`
	Tokens         map[string]string `json:"tokens,omitempty"`
	LocalStorage   map[string]string `json:"local_storage,omitempty"`
	SessionStorage map[string]string `json:"session_storage,omitempty"`
	Extra          map[string]string `json:"extra,omitempty"`
	ExpiresAt      string            `json:"expires_at,omitempty"`
}
