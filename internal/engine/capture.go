package engine

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

// CaptureOption configures the CaptureOnClose workflow.
type CaptureOption func(*captureConfig)

type captureConfig struct {
	savePath string
	persist  bool
}

// WithCaptureSavePath sets the file path where captured credentials are saved.
// If empty, credentials are returned but not written to disk.
func WithCaptureSavePath(path string) CaptureOption {
	return func(o *captureConfig) { o.savePath = path }
}

// WithCapturePersist keeps the browser session directory after capture.
// By default the session is deleted after credentials are saved.
func WithCapturePersist() CaptureOption {
	return func(o *captureConfig) { o.persist = true }
}

// CaptureOnClose opens a headed browser to the given URL, continuously
// snapshots authentication state while the user interacts, then returns the
// last good snapshot when the browser window is closed. If a save path is
// configured via WithCaptureSavePath, credentials are written to disk. Unless
// WithCapturePersist is set, the session directory is deleted after capture.
//
//nolint:contextcheck,cyclop // Browser API doesn't accept context at construction time
func CaptureOnClose(ctx context.Context, url string, browserOpts []Option, opts ...CaptureOption) (*CapturedCredentials, error) {
	cfg := &captureConfig{}
	for _, fn := range opts {
		fn(cfg)
	}

	// Force headed mode for manual interaction.
	launchOpts := []Option{
		WithHeadless(false),
		WithNoSandbox(),
	}
	launchOpts = append(launchOpts, browserOpts...)

	b, err := New(launchOpts...)
	if err != nil {
		return nil, fmt.Errorf("scout: capture on close: launch browser: %w", err)
	}

	sessionID := b.SessionID()

	page, err := b.NewPage(url)
	if err != nil {
		_ = b.Close()
		return nil, fmt.Errorf("scout: capture on close: create page: %w", err)
	}

	if err := page.WaitLoad(); err != nil {
		_ = b.Close()
		return nil, fmt.Errorf("scout: capture on close: wait load: %w", err)
	}

	// Get browser info early (before user closes anything).
	version, _ := b.Version()
	ua, _ := page.Eval(`() => navigator.userAgent`)

	baseCreds := &CapturedCredentials{
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
		baseCreds.UserAgent = ua.String()
	}

	// Snapshot credentials from the live page. Returns nil on failure (page gone).
	snapshot := func() *CapturedCredentials {
		c := *baseCreds // shallow copy
		c.CapturedAt = time.Now()

		if finalURL, err := page.URL(); err == nil {
			c.FinalURL = finalURL
		}

		if cookies, err := page.GetCookies(); err == nil {
			c.Cookies = cookies
		}

		if ls, err := page.LocalStorageGetAll(); err == nil {
			c.LocalStorage = ls
		} else {
			return nil
		}

		if ss, err := page.SessionStorageGetAll(); err == nil {
			c.SessionStorage = ss
		}

		return &c
	}

	// Take an initial snapshot.
	last := snapshot()
	if last == nil {
		last = baseCreds
	}

	// Prepare close channels.
	pageClosed := page.WaitClose()
	browserDone := b.Done()

	// Poll every 2s to keep a fresh snapshot while the browser is alive.
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

loop:
	for {
		select {
		case <-pageClosed:
			break loop
		case <-browserDone:
			break loop
		case <-ctx.Done():
			// Context cancelled — take one final snapshot before exiting.
			if s := snapshot(); s != nil {
				last = s
			}

			break loop
		case <-ticker.C:
			if s := snapshot(); s != nil {
				last = s
			}
		}
	}

	_ = b.Close()

	// Save to disk if path configured.
	if cfg.savePath != "" {
		if err := SaveCredentials(last, cfg.savePath); err != nil {
			return last, fmt.Errorf("scout: capture on close: save: %w", err)
		}
	}

	// Delete session directory unless persistent.
	if !cfg.persist && sessionID != "" {
		_ = ResetSession(sessionID)
	}

	return last, nil
}

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
//nolint:contextcheck // Browser API doesn't accept context at construction time
func CaptureCredentials(ctx context.Context, url string, opts ...Option) (*CapturedCredentials, error) {
	// Force headed mode for manual interaction.
	captureOpts := []Option{ //nolint:prealloc // complex init with conditional appends
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
