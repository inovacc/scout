package scraper

import (
	"context"
	"time"
)

// AuthProvider is the interface that auth providers must implement.
// This mirrors scraper/auth.Provider to avoid import cycles.
// Concrete types live in scraper/auth and scraper/modes/*.
type AuthProvider interface {
	Name() string
}

// SessionData is an opaque interface for session data passed to scrapers.
// The concrete *auth.Session type satisfies this.
type SessionData interface {
	ProviderName() string
}

// Mode defines the interface that each scraper mode (Slack, Teams, etc.) must implement.
type Mode interface {
	// Name returns the unique mode identifier (e.g. "slack", "discord").
	Name() string

	// Description returns a human-readable description.
	Description() string

	// AuthProvider returns the auth provider for browser-based login.
	AuthProvider() AuthProvider

	// Scrape performs the extraction using an authenticated session.
	// Results are emitted to the channel as they are discovered.
	// The channel is closed when scraping completes or ctx is cancelled.
	Scrape(ctx context.Context, session SessionData, opts ScrapeOptions) (<-chan Result, error)
}

// ScrapeOptions configures a scrape operation.
type ScrapeOptions struct {
	// OutputDir is the base directory for exported data.
	OutputDir string

	// Headless controls browser visibility during scraping.
	Headless bool

	// Stealth enables anti-detection measures.
	Stealth bool

	// Timeout is the maximum total scrape duration.
	Timeout time.Duration

	// Limit caps the number of items to extract (0 = unlimited).
	Limit int

	// Channels/Subreddits/etc. to filter extraction scope.
	Targets []string

	// CaptureBody enables response body capture via session hijacking.
	CaptureBody bool

	// Progress receives status updates during scraping.
	Progress ProgressFunc
}

// DefaultScrapeOptions returns sensible defaults.
func DefaultScrapeOptions() ScrapeOptions {
	return ScrapeOptions{
		Headless: true,
		Stealth:  true,
		Timeout:  10 * time.Minute,
	}
}

// ResultType identifies the kind of scraped item.
type ResultType string

const (
	ResultMessage   ResultType = "message"
	ResultChannel   ResultType = "channel"
	ResultThread    ResultType = "thread"
	ResultUser      ResultType = "user"
	ResultFile      ResultType = "file"
	ResultReaction  ResultType = "reaction"
	ResultPost      ResultType = "post"
	ResultComment   ResultType = "comment"
	ResultSubreddit ResultType = "subreddit"
	ResultMeeting   ResultType = "meeting"
	ResultEmail     ResultType = "email"
	ResultProfile   ResultType = "profile"
	ResultMember    ResultType = "member"
	ResultPin       ResultType = "pin"
)

// Result is a single extracted item from a scraper.
type Result struct {
	Type      ResultType     `json:"type"`
	Source    string         `json:"source"`
	ID        string         `json:"id"`
	Timestamp time.Time      `json:"timestamp"`
	Author    string         `json:"author,omitempty"`
	Content   string         `json:"content,omitempty"`
	URL       string         `json:"url,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	Raw       any            `json:"raw,omitempty"`
}

// ModeRegistry holds registered scraper modes.
type ModeRegistry struct {
	modes map[string]Mode
}

// DefaultModeRegistry is the global mode registry.
var DefaultModeRegistry = &ModeRegistry{
	modes: make(map[string]Mode),
}

// RegisterMode adds a mode to the registry.
func RegisterMode(m Mode) {
	DefaultModeRegistry.modes[m.Name()] = m
}

// GetMode returns a mode by name.
func GetMode(name string) (Mode, error) {
	m, ok := DefaultModeRegistry.modes[name]
	if !ok {
		return nil, &AuthError{Reason: "unknown scraper mode: " + name}
	}

	return m, nil
}

// ListModes returns all registered mode names.
func ListModes() []string {
	names := make([]string, 0, len(DefaultModeRegistry.modes))
	for name := range DefaultModeRegistry.modes {
		names = append(names, name)
	}

	return names
}
