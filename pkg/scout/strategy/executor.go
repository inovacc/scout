package strategy

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/scraper"
	"github.com/inovacc/scout/pkg/scout/scraper/auth"
)

// ProgressFunc receives status updates during execution.
type ProgressFunc func(step string, phase string, msg string)

// ExecuteOptions configures strategy execution.
type ExecuteOptions struct {
	// DryRun validates steps and auth without actually scraping.
	DryRun bool

	// Progress receives status updates.
	Progress ProgressFunc

	// Logger for structured logging.
	Logger *slog.Logger

	// ModeResolver provides custom mode lookup (e.g. plugins).
	// If nil, uses scraper.GetMode.
	ModeResolver func(name string) (scraper.Mode, error)
}

// Execute runs a strategy end-to-end: browser setup, auth, steps, sinks.
func Execute(ctx context.Context, s *Strategy, opts ExecuteOptions) error {
	if err := Validate(s); err != nil {
		return err
	}

	logger := opts.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}

	report := func(step, phase, msg string) {
		logger.Info(msg, "step", step, "phase", phase)
		if opts.Progress != nil {
			opts.Progress(step, phase, msg)
		}
	}

	// 1. Create output sinks.
	sinks, err := createSinks(s.Output.Sinks)
	if err != nil {
		return err
	}

	defer closeSinks(sinks)

	// 2. Set up browser options.
	browserOpts := buildBrowserOpts(s.Browser)

	// 3. Authenticate if needed.
	var session *auth.Session

	if s.Auth != nil {
		report("", "auth", "authenticating with "+s.Auth.Provider)

		session, err = resolveAuth(ctx, s.Auth, opts.DryRun)
		if err != nil {
			return fmt.Errorf("strategy: auth: %w", err)
		}

		if session != nil {
			report("", "auth", "session loaded for "+session.Provider)
		}
	}

	if opts.DryRun {
		report("", "dry-run", "validation passed — would execute "+fmt.Sprintf("%d steps", len(s.Steps)))
		return nil
	}

	// 4. Launch browser.
	browser, err := scout.New(browserOpts...)
	if err != nil {
		return fmt.Errorf("strategy: launch browser: %w", err)
	}

	defer func() { _ = browser.Close() }()

	_ = browser // browser is available for URL-based steps

	// 5. Execute steps sequentially.
	for i, step := range s.Steps {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("strategy: cancelled at step %d (%s): %w", i, step.Name, err)
		}

		// Evaluate conditions.
		if !evaluateWhen(step.When, session) {
			report(step.Name, "skip", "condition not met")
			continue
		}

		report(step.Name, "start", fmt.Sprintf("step %d/%d: %s", i+1, len(s.Steps), step.Name))

		if err := executeStep(ctx, step, session, sinks, opts); err != nil {
			return fmt.Errorf("strategy: step %q: %w", step.Name, err)
		}

		report(step.Name, "done", "completed")
	}

	return nil
}

func buildBrowserOpts(cfg BrowserConfig) []scout.Option {
	var opts []scout.Option

	opts = append(opts, scout.WithHeadless(cfg.IsHeadless()))

	if cfg.Type != "" {
		opts = append(opts, scout.WithBrowser(scout.BrowserType(cfg.Type)))
	}

	if cfg.Stealth {
		opts = append(opts, scout.WithStealth())
	}

	if cfg.Proxy != "" {
		opts = append(opts, scout.WithProxy(cfg.Proxy))
	}

	if cfg.UserAgent != "" {
		opts = append(opts, scout.WithUserAgent(cfg.UserAgent))
	}

	if len(cfg.WindowSize) == 2 {
		opts = append(opts, scout.WithWindowSize(cfg.WindowSize[0], cfg.WindowSize[1]))
	}

	return opts
}

func resolveAuth(ctx context.Context, cfg *AuthConfig, dryRun bool) (*auth.Session, error) {
	// Try loading existing session file.
	if cfg.Session != "" {
		passphrase := cfg.Passphrase
		if passphrase == "" {
			passphrase = os.Getenv("SCOUT_PASSPHRASE")
		}

		if passphrase != "" {
			session, err := auth.LoadEncrypted(cfg.Session, passphrase)
			if err == nil {
				return session, nil
			}
			// Fall through to interactive auth.
		}

		// Try loading as plain JSON.
		session, err := loadPlainSession(cfg.Session)
		if err == nil {
			return session, nil
		}
	}

	if dryRun {
		return nil, nil
	}

	// Interactive auth required — get provider from registry.
	provider, err := auth.Get(cfg.Provider)
	if err != nil {
		return nil, fmt.Errorf("unknown auth provider %q", cfg.Provider)
	}

	timeout := ParseTimeout(cfg.Timeout, 5*time.Minute)

	authCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	opts := auth.DefaultBrowserAuthOptions()
	opts.Timeout = timeout
	opts.CaptureOnClose = cfg.CaptureOnClose

	return auth.BrowserAuth(authCtx, provider, opts)
}

func loadPlainSession(path string) (*auth.Session, error) {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil, err
	}

	var s auth.Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}

	return &s, nil
}

func executeStep(ctx context.Context, step Step, session *auth.Session, sinks []Sink, opts ExecuteOptions) error {
	if step.Mode != "" {
		return executeModeScrape(ctx, step, session, sinks, opts)
	}

	// URL-based steps are a future extension point (gather, extract, etc.)
	return fmt.Errorf("URL-based steps not yet implemented (use mode)")
}

func executeModeScrape(ctx context.Context, step Step, session *auth.Session, sinks []Sink, opts ExecuteOptions) error {
	resolver := opts.ModeResolver
	if resolver == nil {
		resolver = scraper.GetMode
	}

	mode, err := resolver(step.Mode)
	if err != nil {
		return fmt.Errorf("unknown mode %q: %w", step.Mode, err)
	}

	timeout := ParseTimeout(step.Timeout, 10*time.Minute)

	stepCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	scrapeOpts := scraper.DefaultScrapeOptions()
	scrapeOpts.Targets = step.Targets
	scrapeOpts.Limit = step.Limit
	scrapeOpts.Timeout = timeout

	results, err := mode.Scrape(stepCtx, session, scrapeOpts)
	if err != nil {
		return fmt.Errorf("scrape: %w", err)
	}

	count := 0

	for r := range results {
		for _, sink := range sinks {
			if err := sink.Write(r); err != nil {
				return fmt.Errorf("sink %s: %w", sink.Name(), err)
			}
		}

		count++
	}

	if opts.Progress != nil {
		opts.Progress(step.Name, "results", fmt.Sprintf("collected %d items", count))
	}

	return nil
}

func evaluateWhen(conditions map[string]any, session *auth.Session) bool {
	if len(conditions) == 0 {
		return true
	}

	for key, val := range conditions {
		switch key {
		case "has_auth":
			want, ok := val.(bool)
			if ok && want && session == nil {
				return false
			}

			if ok && !want && session != nil {
				return false
			}
		case "env":
			envName, ok := val.(string)
			if ok && os.Getenv(envName) == "" {
				return false
			}
		}
	}

	return true
}

func createSinks(configs []SinkConfig) ([]Sink, error) {
	sinks := make([]Sink, 0, len(configs))

	for _, cfg := range configs {
		s, err := NewSink(cfg)
		if err != nil {
			closeSinks(sinks)
			return nil, err
		}

		sinks = append(sinks, s)
	}

	return sinks, nil
}

func closeSinks(sinks []Sink) {
	for _, s := range sinks {
		_ = s.Close()
	}
}
