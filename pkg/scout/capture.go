package scout

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

// CapturedCredentials holds all browser state needed to replicate an authenticated session.
type CapturedCredentials struct {
	URL            string            `json:"url"`
	FinalURL       string            `json:"final_url"`
	CapturedAt     time.Time         `json:"captured_at"`
	Browser        BrowserInfo       `json:"browser"`
	Cookies        []Cookie          `json:"cookies"`
	LocalStorage   map[string]string `json:"local_storage,omitempty"`
	SessionStorage map[string]string `json:"session_storage,omitempty"`
	UserAgent      string            `json:"user_agent"`
}

// BrowserInfo describes the browser used during capture.
type BrowserInfo struct {
	Product  string `json:"product"`
	Platform string `json:"platform"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
}

// CaptureCredentials opens a headed (visible) browser, navigates to the given URL,
// and blocks until the user presses Ctrl+C or the context is cancelled.
// Before returning, it captures all authentication state (cookies, localStorage,
// sessionStorage, user agent, browser version).
//
// Usage: navigate to a login page, log in manually, then press Ctrl+C.
// The returned CapturedCredentials can be saved with SaveCredentials.
func CaptureCredentials(ctx context.Context, url string, opts ...Option) (*CapturedCredentials, error) {
	// Force headed mode for manual interaction.
	captureOpts := []Option{
		WithHeadless(false),
		WithNoSandbox(),
	}
	captureOpts = append(captureOpts, opts...)

	b, err := New(captureOpts...)
	if err != nil {
		return nil, fmt.Errorf("scout: capture credentials: launch browser: %w", err)
	}
	defer func() { _ = b.Close() }()

	page, err := b.NewPage(url)
	if err != nil {
		return nil, fmt.Errorf("scout: capture credentials: create page: %w", err)
	}

	if err := page.WaitLoad(); err != nil {
		return nil, fmt.Errorf("scout: capture credentials: wait load: %w", err)
	}

	// Get browser info early.
	version, _ := b.Version()
	ua, _ := page.Eval(`() => navigator.userAgent`)

	// Wait for signal (Ctrl+C) or context cancellation.
	sigCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	<-sigCtx.Done()

	// Capture all state after user is done authenticating.
	creds := &CapturedCredentials{
		URL:        url,
		CapturedAt: time.Now(),
		Browser: BrowserInfo{
			Product:  version,
			Platform: runtime.GOOS,
			OS:       runtime.GOOS,
			Arch:     runtime.GOARCH,
		},
	}

	if ua != nil {
		creds.UserAgent = ua.String()
	}

	// Capture final URL (may have redirected after login).
	if finalURL, err := page.URL(); err == nil {
		creds.FinalURL = finalURL
	}

	// Capture cookies.
	if cookies, err := page.GetCookies(); err == nil {
		creds.Cookies = cookies
	}

	// Capture localStorage.
	if ls, err := page.LocalStorageGetAll(); err == nil {
		creds.LocalStorage = ls
	}

	// Capture sessionStorage.
	if ss, err := page.SessionStorageGetAll(); err == nil {
		creds.SessionStorage = ss
	}

	return creds, nil
}

// SaveCredentials writes captured credentials to a JSON file.
func SaveCredentials(creds *CapturedCredentials, path string) error {
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("scout: save credentials: marshal: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("scout: save credentials: write: %w", err)
	}

	return nil
}

// LoadCredentials reads captured credentials from a JSON file.
func LoadCredentials(path string) (*CapturedCredentials, error) {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("scout: load credentials: read: %w", err)
	}

	var creds CapturedCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, fmt.Errorf("scout: load credentials: unmarshal: %w", err)
	}

	return &creds, nil
}

// ToSessionState converts captured credentials to a SessionState for use with LoadSession.
func (c *CapturedCredentials) ToSessionState() *SessionState {
	return &SessionState{
		URL:            c.FinalURL,
		Cookies:        c.Cookies,
		LocalStorage:   c.LocalStorage,
		SessionStorage: c.SessionStorage,
	}
}
