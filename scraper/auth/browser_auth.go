package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/scraper"
)

// BrowserAuthOptions configures the browser-based auth flow.
type BrowserAuthOptions struct {
	// Headless controls whether the browser runs in headless mode.
	// For interactive login this should be false.
	Headless bool

	// Stealth enables stealth mode to avoid bot detection.
	Stealth bool

	// Timeout is the maximum time to wait for the user to complete login.
	Timeout time.Duration

	// PollInterval is how often to check if auth is complete.
	PollInterval time.Duration

	// UserDataDir sets a persistent browser profile directory.
	UserDataDir string

	// CaptureOnClose captures all session data (cookies, localStorage,
	// sessionStorage) when the browser closes, regardless of provider-specific
	// auth detection. Use this for generic "launch browser, do anything, capture
	// everything" workflows.
	CaptureOnClose bool

	// Progress receives status updates during the auth flow.
	Progress scraper.ProgressFunc
}

// DefaultBrowserAuthOptions returns sensible defaults.
func DefaultBrowserAuthOptions() BrowserAuthOptions {
	return BrowserAuthOptions{
		Headless:     false,
		Stealth:      true,
		Timeout:      5 * time.Minute,
		PollInterval: 2 * time.Second,
	}
}

// BrowserAuth launches a browser for interactive authentication.
// It opens the provider's login URL, polls for auth completion using the
// provider's DetectAuth method, then captures the session before closing.
func BrowserAuth(ctx context.Context, provider Provider, opts BrowserAuthOptions) (*Session, error) {
	browserOpts := []scout.Option{
		scout.WithHeadless(opts.Headless),
		scout.WithTimeout(opts.Timeout),
		scout.WithNoSandbox(),
	}

	if opts.Stealth {
		browserOpts = append(browserOpts, scout.WithStealth())
	}

	if opts.UserDataDir != "" {
		browserOpts = append(browserOpts, scout.WithUserDataDir(opts.UserDataDir))
	}

	browser, err := scout.New(browserOpts...)
	if err != nil {
		return nil, fmt.Errorf("auth: launch browser: %w", err)
	}

	defer func() { _ = browser.Close() }()

	loginURL := provider.LoginURL()

	page, err := browser.NewPage(loginURL)
	if err != nil {
		return nil, fmt.Errorf("auth: open login page: %w", err)
	}

	reportProgress(opts.Progress, "auth", 0, 0, "waiting for login at "+loginURL)

	deadline := time.Now().Add(opts.Timeout)
	pollInterval := opts.PollInterval
	if pollInterval == 0 {
		pollInterval = 2 * time.Second
	}

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			if opts.CaptureOnClose {
				return captureGenericSession(provider.Name(), page)
			}

			return nil, fmt.Errorf("auth: %w", ctx.Err())
		default:
		}

		detected, err := provider.DetectAuth(ctx, page)
		if err != nil {
			// Non-fatal: keep polling
			time.Sleep(pollInterval)

			continue
		}

		if detected {
			reportProgress(opts.Progress, "auth", 1, 1, "login detected, capturing session...")

			return provider.CaptureSession(ctx, page)
		}

		time.Sleep(pollInterval)
	}

	// Timeout: if CaptureOnClose is set, capture whatever state we have
	if opts.CaptureOnClose {
		reportProgress(opts.Progress, "auth", 0, 0, "timeout reached, capturing current state...")

		return captureGenericSession(provider.Name(), page)
	}

	return nil, &scraper.AuthError{Reason: "login timeout: auth not detected within " + opts.Timeout.String()}
}

// BrowserCapture launches a browser to a URL and captures all session data
// when the context is cancelled or timeout expires. This is the generic
// "launch browser, capture all data before close" flow that doesn't require
// any provider-specific auth detection.
func BrowserCapture(ctx context.Context, targetURL string, opts BrowserAuthOptions) (*Session, error) {
	browserOpts := []scout.Option{
		scout.WithHeadless(opts.Headless),
		scout.WithTimeout(opts.Timeout),
		scout.WithNoSandbox(),
	}

	if opts.Stealth {
		browserOpts = append(browserOpts, scout.WithStealth())
	}

	if opts.UserDataDir != "" {
		browserOpts = append(browserOpts, scout.WithUserDataDir(opts.UserDataDir))
	}

	browser, err := scout.New(browserOpts...)
	if err != nil {
		return nil, fmt.Errorf("auth: launch browser: %w", err)
	}

	defer func() { _ = browser.Close() }()

	page, err := browser.NewPage(targetURL)
	if err != nil {
		return nil, fmt.Errorf("auth: open page: %w", err)
	}

	reportProgress(opts.Progress, "capture", 0, 0, "browser open — interact freely, session captured on close")

	// Wait for context cancellation (user presses Ctrl+C) or timeout
	deadline := time.Now().Add(opts.Timeout)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			reportProgress(opts.Progress, "capture", 1, 1, "capturing session data...")

			return captureGenericSession("generic", page)
		default:
		}

		time.Sleep(1 * time.Second)
	}

	reportProgress(opts.Progress, "capture", 1, 1, "timeout reached, capturing session data...")

	return captureGenericSession("generic", page)
}

// captureGenericSession captures all available browser state from the current page.
func captureGenericSession(providerName string, page *scout.Page) (*Session, error) {
	state, err := page.SaveSession()
	if err != nil {
		return nil, fmt.Errorf("auth: capture session: %w", err)
	}

	return &Session{
		Provider:       providerName,
		Version:        "1.0",
		Timestamp:      time.Now().UTC(),
		URL:            state.URL,
		Cookies:        state.Cookies,
		LocalStorage:   state.LocalStorage,
		SessionStorage: state.SessionStorage,
	}, nil
}

func reportProgress(fn scraper.ProgressFunc, phase string, current, total int, message string) {
	if fn != nil {
		fn(scraper.Progress{
			Phase:   phase,
			Current: current,
			Total:   total,
			Message: message,
		})
	}
}
