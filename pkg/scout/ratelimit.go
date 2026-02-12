package scout

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"time"

	"golang.org/x/time/rate"
)

// RateLimitOption configures a RateLimiter.
type RateLimitOption func(*rateLimitOptions)

type rateLimitOptions struct {
	rps            float64
	burst          int
	maxConcurrent  int
	maxRetries     int
	initialBackoff time.Duration
	maxBackoff     time.Duration
}

func rateLimitDefaults() *rateLimitOptions {
	return &rateLimitOptions{
		rps:            2.0,
		burst:          5,
		maxConcurrent:  0,
		maxRetries:     3,
		initialBackoff: 1 * time.Second,
		maxBackoff:     30 * time.Second,
	}
}

// WithRateLimit sets the requests per second. Default: 2.
func WithRateLimit(rps float64) RateLimitOption {
	return func(o *rateLimitOptions) { o.rps = rps }
}

// WithBurstSize sets the burst size for the token bucket. Default: 5.
func WithBurstSize(n int) RateLimitOption {
	return func(o *rateLimitOptions) { o.burst = n }
}

// WithMaxConcurrent limits concurrent executions. 0 means unlimited. Default: 0.
func WithMaxConcurrent(n int) RateLimitOption {
	return func(o *rateLimitOptions) { o.maxConcurrent = n }
}

// WithMaxRetries sets the maximum number of retries on failure. Default: 3.
func WithMaxRetries(n int) RateLimitOption {
	return func(o *rateLimitOptions) { o.maxRetries = n }
}

// WithBackoff sets the initial backoff duration. Default: 1s.
func WithBackoff(d time.Duration) RateLimitOption {
	return func(o *rateLimitOptions) { o.initialBackoff = d }
}

// WithMaxBackoff sets the maximum backoff duration. Default: 30s.
func WithMaxBackoff(d time.Duration) RateLimitOption {
	return func(o *rateLimitOptions) { o.maxBackoff = d }
}

// RateLimiter provides rate limiting with retry and backoff for browser operations.
type RateLimiter struct {
	limiter *rate.Limiter
	opts    *rateLimitOptions
	sem     chan struct{}
}

// NewRateLimiter creates a new rate limiter with the given options.
func NewRateLimiter(opts ...RateLimitOption) *RateLimiter {
	o := rateLimitDefaults()
	for _, fn := range opts {
		fn(o)
	}

	rl := &RateLimiter{
		limiter: rate.NewLimiter(rate.Limit(o.rps), o.burst),
		opts:    o,
	}

	if o.maxConcurrent > 0 {
		rl.sem = make(chan struct{}, o.maxConcurrent)
	}

	return rl
}

// Wait blocks until a rate limit token is available.
func (rl *RateLimiter) Wait() {
	_ = rl.limiter.Wait(context.Background())
}

// Do executes fn with rate limiting and retry with exponential backoff.
func (rl *RateLimiter) Do(fn func() error) error {
	if rl.sem != nil {
		rl.sem <- struct{}{}

		defer func() { <-rl.sem }()
	}

	var lastErr error

	for attempt := 0; attempt <= rl.opts.maxRetries; attempt++ {
		_ = rl.limiter.Wait(context.Background())

		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		if attempt < rl.opts.maxRetries {
			backoff := rl.calculateBackoff(attempt)
			time.Sleep(backoff)
		}
	}

	return fmt.Errorf("scout: rate limiter: max retries (%d) exceeded: %w", rl.opts.maxRetries, lastErr)
}

// NavigateWithRetry navigates to the URL using the rate limiter for pacing and retry.
func (p *Page) NavigateWithRetry(url string, rl *RateLimiter) error {
	return rl.Do(func() error {
		return p.Navigate(url)
	})
}

func (rl *RateLimiter) calculateBackoff(attempt int) time.Duration {
	backoff := float64(rl.opts.initialBackoff) * math.Pow(2, float64(attempt))
	if backoff > float64(rl.opts.maxBackoff) {
		backoff = float64(rl.opts.maxBackoff)
	}
	// Add jitter: +/- 25%
	jitter := backoff * 0.25
	backoff = backoff - jitter + rand.Float64()*2*jitter //nolint:gosec // jitter doesn't need crypto rand

	return time.Duration(backoff)
}
