package scout

import (
	"log/slog"

	"github.com/inovacc/scout/internal/engine/swarm"
)

// Swarm types re-exported from internal/engine/swarm for public consumers.

type SwarmConfig = swarm.SwarmConfig
type SwarmCrawlRequest = swarm.CrawlRequest
type SwarmCrawlResult = swarm.CrawlResult
type SwarmWorkerInfo = swarm.WorkerInfo
type SwarmWorkerStatus = swarm.WorkerStatus
type SwarmWorkerOption = swarm.WorkerOption
type SwarmCoordinator = swarm.Coordinator
type SwarmWorker = swarm.Worker

//nolint:gochecknoglobals // facade re-exports
var (
	SwarmWorkerIdle         = swarm.WorkerIdle
	SwarmWorkerBusy         = swarm.WorkerBusy
	SwarmWorkerDisconnected = swarm.WorkerDisconnected
)

// DefaultSwarmConfig returns a SwarmConfig with sensible defaults.
func DefaultSwarmConfig() SwarmConfig { return swarm.DefaultConfig() }

// NewSwarmCoordinator creates a new swarm coordinator.
func NewSwarmCoordinator(cfg SwarmConfig, logger *slog.Logger) *SwarmCoordinator {
	return swarm.NewCoordinator(cfg, logger)
}

// NewSwarmWorker creates a new swarm worker.
func NewSwarmWorker(id, proxy string, batchSize int, logger *slog.Logger, opts ...SwarmWorkerOption) *SwarmWorker {
	return swarm.NewWorker(id, proxy, batchSize, logger, opts...)
}

// WithSwarmWorkerBrowser sets browser options for a swarm worker.
func WithSwarmWorkerBrowser(opts ...Option) SwarmWorkerOption {
	return swarm.WithWorkerBrowser(opts...)
}
