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
	// MaxURLs caps the total unique URLs tracked (the dedup seen-set, which
	// also bounds the queue) so a hostile or sprawling site cannot grow the
	// coordinator's memory without limit. A value <= 0 is replaced with a safe
	// default in NewCoordinator; Enqueue stops admitting new URLs at the cap.
	MaxURLs int
}

// DefaultConfig returns a SwarmConfig with sensible defaults.
func DefaultConfig() SwarmConfig {
	return SwarmConfig{
		BatchSize:         10,
		MaxWorkers:        8,
		HeartbeatInterval: 5 * time.Second,
		HeartbeatTimeout:  15 * time.Second,
		DefaultRateLimit:  time.Second,
		MaxURLs:           10000,
	}
}

// defaultMaxURLs bounds the coordinator seen-set when a caller leaves MaxURLs
// unset (e.g. a SwarmConfig literal); it is applied in NewCoordinator so the
// memory cap is fail-closed even on the gRPC daemon path.
const defaultMaxURLs = 100_000

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
	ID        string
	Status    WorkerStatus
	Proxy     string
	LastSeen  time.Time
	Processed int64
	InFlight  []string
}
