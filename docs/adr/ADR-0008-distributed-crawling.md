# ADR-0008: Distributed Crawling (Swarm Mode)

## Status

Proposed

## Context

Scout currently runs all browser automation on a single machine with a single browser instance (or a small pool via `BatchScrape`). For large-scale crawling (10k+ pages), this is bottlenecked by:

- Single IP rate limiting and blocking
- Memory pressure from concurrent Chrome tabs
- Network bandwidth on one machine
- Total crawl time scaling linearly with page count

Users need the ability to distribute crawl workloads across multiple browser instances running on different machines, IPs, or proxies.

## Decision

Implement a distributed crawling system ("Swarm Mode") with the following architecture:

### Coordinator / Worker Model

```
┌─────────────┐       gRPC        ┌──────────────┐
│ Coordinator  │◄────────────────►│  Worker 1     │
│              │                   │  (browser+IP) │
│  - BFS queue │       gRPC        ├──────────────┤
│  - Dedup set │◄────────────────►│  Worker 2     │
│  - Results   │                   │  (browser+IP) │
│  - Rate ctrl │       gRPC        ├──────────────┤
│              │◄────────────────►│  Worker N     │
└─────────────┘                   │  (browser+IP) │
                                  └──────────────┘
```

### Components

1. **Coordinator** (`internal/engine/swarm/coordinator.go`)
   - Manages shared BFS queue (domain-partitioned for politeness)
   - URL deduplication via bloom filter or hash set
   - Dispatches URL batches to workers
   - Aggregates results (extracted data, HAR, errors)
   - Global rate limiting per domain
   - Fault tolerance: re-queues URLs from dead workers

2. **Worker** (`internal/engine/swarm/worker.go`)
   - Runs a `scout.Browser` instance with optional proxy
   - Accepts URL batches, returns extraction results
   - Reports health/progress to coordinator
   - Supports `WithVPN()` or `WithProxy()` for IP rotation

3. **Transport** (extend existing gRPC service)
   - New RPCs: `JoinSwarm`, `LeaveSwarm`, `FetchBatch`, `SubmitResults`, `Heartbeat`
   - Workers auto-discover coordinator via mDNS (existing `pkg/scout/discovery/`)
   - mTLS for worker authentication (existing `grpc/server/tls.go`)

4. **CLI**
   - `scout swarm start --workers=N --proxy-list=proxies.txt` — start coordinator + local workers
   - `scout swarm join --coordinator=host:port` — join as remote worker
   - `scout swarm status` — show queue depth, worker count, progress
   - `scout crawl <url> --swarm` — run crawl in swarm mode

### Queue Strategy

- **Domain-partitioned**: Each worker is assigned domains to avoid multiple workers hitting the same domain simultaneously
- **Priority queue**: Depth-first within a domain, breadth-first across domains
- **Backpressure**: Workers request batches (pull model), coordinator controls batch size based on worker throughput

### Data Flow

1. User submits seed URLs via `scout crawl --swarm`
2. Coordinator enqueues seeds, partitions by domain
3. Workers pull batches, navigate + extract, return results
4. Coordinator deduplicates discovered URLs, enqueues new ones
5. Results streamed to output (JSON lines, database, or cloud upload)

## Alternatives Considered

1. **Message queue (Redis/NATS)**: More infrastructure to deploy. gRPC is already in the project and sufficient for coordinator-worker communication.
2. **Peer-to-peer**: No coordinator. More complex, harder to control rate limiting and deduplication.
3. **Kubernetes Job-based**: Too heavy for local/small deployments. Could be added later as a deployment mode.

## Consequences

- New `internal/engine/swarm/` package with coordinator and worker
- Extends gRPC proto with swarm-specific RPCs
- Leverages existing mDNS discovery and mTLS auth
- Workers can be local processes, remote machines, or Docker containers
- First implementation targets local multi-process mode (single machine, multiple browser instances with different proxies)
