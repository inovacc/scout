package slack

import "time"

// Option configures a Scraper instance.
type Option func(*options)

type options struct {
	workspace      string
	token          string
	dCookie        string
	headless       bool
	stealth        bool
	timeout        time.Duration
	rateLimit      time.Duration
	maxMessages    int
	oldest         string
	latest         string
	includeThreads bool
	progress       progressFunc
	userDataDir    string
}

type progressFunc func(phase string, current, total int, message string)

func defaults() *options {
	return &options{
		headless:  true,
		stealth:   true,
		timeout:   5 * time.Minute,
		rateLimit: 1200 * time.Millisecond, // Slack Tier 3 default
	}
}

// WithWorkspace sets the Slack workspace domain (e.g. "myteam.slack.com").
func WithWorkspace(domain string) Option {
	return func(o *options) { o.workspace = domain }
}

// WithToken sets the xoxc authentication token directly, skipping browser login.
func WithToken(token string) Option {
	return func(o *options) { o.token = token }
}

// WithDCookie sets the Slack "d" cookie required alongside the xoxc token.
func WithDCookie(d string) Option {
	return func(o *options) { o.dCookie = d }
}

// WithHeadless sets whether the browser runs in headless mode during login. Default: true.
func WithHeadless(v bool) Option {
	return func(o *options) { o.headless = v }
}

// WithStealth enables or disables stealth mode for browser login. Default: true.
func WithStealth(v bool) Option {
	return func(o *options) { o.stealth = v }
}

// WithTimeout sets the timeout for browser login. Default: 5 minutes.
func WithTimeout(d time.Duration) Option {
	return func(o *options) { o.timeout = d }
}

// WithRateLimit sets the minimum delay between API calls. Default: 1200ms (Slack Tier 3).
func WithRateLimit(d time.Duration) Option {
	return func(o *options) { o.rateLimit = d }
}

// WithMaxMessages limits the number of messages fetched per channel. 0 means unlimited.
func WithMaxMessages(n int) Option {
	return func(o *options) { o.maxMessages = n }
}

// WithDateRange limits message fetching to a time range.
// Oldest and latest are Slack timestamps (e.g. "1234567890.123456") or Unix epoch strings.
func WithDateRange(oldest, latest string) Option {
	return func(o *options) {
		o.oldest = oldest
		o.latest = latest
	}
}

// WithIncludeThreads enables fetching thread replies for each parent message.
func WithIncludeThreads(v bool) Option {
	return func(o *options) { o.includeThreads = v }
}

// WithProgress sets a callback for receiving progress updates.
func WithProgress(fn func(phase string, current, total int, message string)) Option {
	return func(o *options) { o.progress = fn }
}

// WithUserDataDir sets the browser user data directory for persistent sessions.
func WithUserDataDir(dir string) Option {
	return func(o *options) { o.userDataDir = dir }
}
