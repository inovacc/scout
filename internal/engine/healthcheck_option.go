package engine

import "time"

// HealthCheckOption configures a health check run.
type HealthCheckOption func(*healthCheckOptions)

type healthCheckOptions struct {
	maxDepth    int
	concurrency int
	timeout     time.Duration
	click       bool
}

func healthCheckDefaults() *healthCheckOptions {
	return &healthCheckOptions{
		maxDepth:    2,
		concurrency: 3,
		timeout:     60 * time.Second,
	}
}

// WithHealthDepth sets maximum crawl depth for health checking. Default: 2.
func WithHealthDepth(n int) HealthCheckOption {
	return func(o *healthCheckOptions) { o.maxDepth = n }
}

// WithHealthConcurrency sets concurrent page limit. Default: 3.
func WithHealthConcurrency(n int) HealthCheckOption {
	return func(o *healthCheckOptions) {
		if n < 1 {
			n = 1
		}
		o.concurrency = n
	}
}

// WithHealthTimeout sets overall health check timeout. Default: 60s.
func WithHealthTimeout(d time.Duration) HealthCheckOption {
	return func(o *healthCheckOptions) { o.timeout = d }
}

// WithHealthClickElements enables clicking interactive elements to discover JS errors. Default: false.
func WithHealthClickElements() HealthCheckOption {
	return func(o *healthCheckOptions) { o.click = true }
}
