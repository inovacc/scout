# ADR-0003: Add gRPC service layer for remote browser control

## Status

Accepted

## Context

Scout was designed as a library-only package. However, use cases emerged for remote browser control with forensic evidence capture:

- Security testing with full network traffic recording (HAR)
- Compliance auditing with audit trails
- Remote debugging and QA scenario reproduction
- Automated workflows with real-time event monitoring

These require a server component that manages browser sessions and streams events to clients. The question was whether to build this into the core library or as a separate layer.

## Decision

Add a gRPC service layer as an **optional subtree** (`grpc/`, `cmd/`) that imports the core library. The core library (`package scout`) does NOT import gRPC — library-only consumers pull zero gRPC
dependencies.

### Architecture

- `grpc/proto/scout.proto` — Service definition with 25+ RPCs
- `grpc/scoutpb/` — Generated Go code (committed for consumer convenience)
- `grpc/server/` — `ScoutServer` implementing multi-session management, CDP event wiring, and bidirectional streaming
- `cmd/server/` — gRPC server binary with reflection and graceful shutdown
- `cmd/client/` — Interactive CLI client for manual browser control
- `cmd/example-workflow/` — Bidirectional streaming demo

### HAR recording as a library feature

`NetworkRecorder` was added to the core library (`recorder.go`) rather than the gRPC layer, because HAR recording is useful independently of gRPC for any scraping or testing workflow.

## Consequences

### Positive

- Core library stays lightweight — no gRPC dependency for library consumers
- HAR recording available to all library users, not just gRPC users
- Multi-session management via gRPC enables concurrent browser control
- Real-time event streaming (network, console, page lifecycle) for monitoring
- Bidirectional `Interactive` stream enables automated workflows with live feedback
- Server reflection enables grpcurl/grpcui for ad-hoc exploration

### Negative

- gRPC adds significant dependencies to the module (grpc, protobuf, uuid)
- Generated protobuf code must be committed and regenerated when proto changes
- Server layer must track core library API changes (wrapper-of-a-wrapper)
- `mapKey()` must manually map string key names to `input.Key` constants

## Alternatives Considered

### REST/HTTP API

Simpler to implement but cannot support server-push event streaming without WebSockets. gRPC's native streaming and bidirectional streams are a natural fit for real-time browser events.

### WebSocket-based server

Would work for streaming but lacks the structured RPC definitions, code generation, and type safety that protobuf provides.

### Embed server in core library

Would force all library consumers to pull gRPC dependencies even when not needed. The subtree approach keeps the dependency graph clean.
