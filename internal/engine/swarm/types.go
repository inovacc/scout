package swarm

import "time"

// WorkerStatus represents the current state of a worker.
type WorkerStatus int

const (
	WorkerIdle WorkerStatus = iota
	WorkerBusy
	WorkerDisconnected
)

func (s WorkerStatus) String() string {
	switch s {
	case WorkerIdle:
		return "idle"
	case WorkerBusy:
		return "busy"
	case WorkerDisconnected:
		return "disconnected"
	default:
		return "unknown"
	}
}

// SwarmConfig holds configuration for the swarm coordinator and workers.
type SwarmConfig struct {
	// BatchSize is the number of URLs dispatched per worker request.
	BatchSize int
	// MaxWorkers is the maximum number of concurrent workers.
	MaxWorkers int
	// HeartbeatInterval is how often workers report health.
	HeartbeatInterval time.Duration
	// HeartbeatTimeout is how long before a silent worker is marked disconnected.
	HeartbeatTimeout time.Duration
	// DefaultRateLimit is the per-domain request interval.
	DefaultRateLimit time.Duration
}

// DefaultConfig returns a SwarmConfig with sensible defaults.
func DefaultConfig() SwarmConfig {
	return SwarmConfig{
		BatchSize:         10,
		MaxWorkers:        8,
		HeartbeatInterval: 5 * time.Second,
		HeartbeatTimeout:  15 * time.Second,
		DefaultRateLimit:  time.Second,
	}
}

// CrawlRequest represents a URL to be crawled.
type CrawlRequest struct {
	URL    string
	Depth  int
	Domain string
}

// CrawlResult represents the outcome of crawling a single URL.
type CrawlResult struct {
	URL            string
	StatusCode     int
	Error          string
	DiscoveredURLs []string
	Data           map[string]any
	Duration       time.Duration
}

// WorkerInfo holds metadata about a registered worker.
type WorkerInfo struct {
	ID         string
	Status     WorkerStatus
	Proxy      string
	LastSeen   time.Time
	Processed  int64
	InFlight   []string
}
