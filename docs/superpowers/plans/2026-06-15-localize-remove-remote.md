# Localize Scout — Remove Remote / Distributed Surface

> **For agentic workers:** execute task-by-task (phases below). Each phase ends with a build/test gate. Work on a branch off `main`; do not push or delete until the gate passes.

**Goal:** Reduce Scout to **explicit browser-automation logic that runs on the same machine**. Remove every remote / distributed / networked / multi-machine subsystem. Keep the local engine, CLI verbs, stdio MCP server, and subprocess plugins.

**Decisions (owner-confirmed 2026-06-15):**
- **No session daemon.** Per-command execution. Multi-step automation = `repl` / stdio MCP / `strategy`+`runbook` (each a single process). The daemon-backed `session` model is removed.
- **Agents:** remove only `browser-automation` (the fan-out offender). Keep `site-tester`, `web-scraper`, `site-mapper`, `session-capture`, `flow-porter`.
- **VPN** (`internal/engine/vpn`, Surfshark) is a *local* browser-routing option → **KEEP** (flag for owner if undesired).

**Architecture after:** `internal/engine` (rod core) → `pkg/scout` facade → entry points = local CLI + **stdio** MCP + subprocess plugins. No gRPC, no HTTP servers, no cloud, no distributed coordination.

**Tech stack:** Go 1.26, Cobra CLI, internalized rod, go-sdk/mcp (stdio), subprocess JSON-RPC plugins.

---

## Phase 0 — Setup & baseline

- [ ] Branch: `git -C <scout> checkout -b refactor/localize-remove-remote`
- [ ] Baseline: `task build` (or `go build ./cmd/scout/ ./pkg/...`) + `go vet ./...` succeed before any change. Record the binary builds.
- [ ] `go build ./cmd/scout/` is the canonical gate used after every phase.

## Phase 1 — Delete fully-isolated remote subsystems (no decoupling needed)

Delete these dirs/files outright (the Explore map confirms no `internal/engine`/`pkg/scout`-core dependents):

```
grpc/                                   # gRPC service: proto, scoutpb/, server/ (mTLS, pairing)
internal/engine/swarm/                  # distributed coordinator/worker
pkg/scout/agent/                        # REST AI ingress (already deprecated)
pkg/scout/discovery/                    # mDNS/zeroconf device pairing
pkg/scout/tools/swarm.go                # swarm tool verb
deploy/helm/  deploy/grafana/           # K8s/Helm/Grafana
Dockerfile.swarm  docker-compose.swarm.yml
agents/browser-automation*              # the fan-out agent (keep the other 5)
```

Delete these `cmd/scout/` command files (whole remote command groups):
```
server.go daemon.go device.go client.go grpc_group.go   # gRPC daemon + pairing
cloud.go                                                  # Helm wrapper
upload.go                                                 # Drive/OneDrive OAuth
agent.go                                                  # HTTP REST API
connect.go                                                # remote-CDP connect
swarm.go                                                  # distributed crawl
mobile.go                                                 # ADB
```

- [ ] **Gate:** `go build ./cmd/scout/` — it will FAIL on dangling references (registration in `scout.go`, gRPC imports in shared verb files). That failure list is the exact Phase 2 worklist. Capture it.

## Phase 2 — Decouple `cmd/scout/` from the gRPC daemon (highest risk)

The ~15 verb files the map flagged (`session.go`, `inspect.go`, `navigate.go`, `screenshot.go`, `har.go`, `interact.go`, `network.go`, `profile.go`, `storage.go`, `window.go`, …) currently RPC the daemon. For **each**, decide and apply one of:
- **Single-shot:** launch `scout.New(baseOpts(cmd)...)`, do the action, `Close()`. Correct for verbs that produce output in one call (`screenshot <url>`, `gather`, `pdf`, `eval --url`).
- **Fold into REPL/strategy:** bare session-mutators with no standalone value (`click`/`type`/`navigate` against a held session) — keep them working only inside `repl` and the strategy/runbook executors (which already hold one browser per process). Remove their daemon-backed standalone variant.
- **Remove** if redundant with the above.

Then in `cmd/scout/scout.go`:
- [ ] Remove `AddCommand` for every deleted group (grpc, cloud, agent, device, swarm, mobile, connect, upload, client, server, daemon).
- [ ] Remove persistent gRPC flags: `--addr`, `--target`, `--insecure`.
- [ ] Keep all local browser flags (`--headless`, `--browser`, `--stealth`, `--no-sandbox`, …).
- [ ] `session.go`: reduce to file-based dir lifecycle only (`list`, `reset`, `destroy` over `~/.scout/sessions/*`), no daemon client. `create`/`use` either become no-ops with a deprecation notice or are removed (owner preference; default = remove).

- [ ] **Gate:** `go build ./cmd/scout/` passes. `scout --help` shows no grpc/cloud/agent/device/swarm/mobile/connect/upload groups.

## Phase 3 — MCP stdio-only

- [ ] `pkg/scout/mcp/`: remove `ServeSSE()` + the HTTP/SSE listener; keep `Serve()` (stdio JSON-RPC).
- [ ] Remove the `swarm_crawl` tool registration (`tools_swarm.go` / wherever registered) and any swarm import.
- [ ] `cmd/scout/mcp.go`: remove `--sse` and `--addr` flags + the `ServeSSE` branch.
- [ ] **Gate:** `go build ./cmd/scout/ ./pkg/scout/mcp/`. `scout mcp` still starts a stdio server.

## Phase 4 — Remove remote browser options

- [ ] Remove `WithRemoteCDP(endpoint)` + `WithMobile()`/mobile touch-over-ADB option plumbing from `internal/engine` (keep desktop `WithTouchEmulation` if local-only).
- [ ] Regenerate the facade if these were re-exported (`gen-facade-full.go`); else hand-remove the aliases.
- [ ] **Gate:** `go build ./pkg/...`.

## Phase 5 — Observability: drop remote exporters

- [ ] `internal/tracing/`: remove the OTLP/stdout exporter setup; keep the `Start`/`Span`/`addTracedTool` API as a **no-op** so the MCP wrapper still compiles (drops the `otel` deps). OR delete tracing entirely and remove `addTracedTool` (more churn).
- [ ] `internal/metrics/`: the `/metrics` HTTP handler was exposed by the (now-deleted) gRPC server; remove the handler. Keep plain counters only if still referenced, else delete.
- [ ] **Gate:** `go build ./...` (root has no main, so `./cmd/scout/ ./pkg/...`).

## Phase 6 — go.mod tidy

- [ ] `go mod tidy`. Expect these to drop: `google.golang.org/grpc`, `google.golang.org/protobuf`, `grandcat/zeroconf`, `golang.org/x/oauth2`, `go.opentelemetry.io/otel*`. **Keep** `google/gops` (session cleanup), `spf13/cobra`, `modelcontextprotocol/go-sdk`.
- [ ] **Gate:** `go build ./cmd/scout/ ./pkg/...` + `go vet ./...`.

## Phase 7 — Docs, plugin manifest, Taskfile

- [ ] `.mcp.json`: remove `swarm_crawl`, `ws_*` if removed; reflect stdio-only.
- [ ] `.claude-plugin/plugin.json`: drop the `browser-automation` agent entry.
- [ ] `README.md` + `CLAUDE.md`: delete the Cloud Deployment, gRPC Service, Swarm, Agent HTTP API, Mobile, Cloud Upload, Monitoring(:9551) sections + the deprecated-`agent serve` heads-up. Update the CLI reference table and the dependency table.
- [ ] `Taskfile.yml`: remove `proto`, `grpc:server`, helm/cloud tasks. Remove `grpc/proto/scout.proto` references.
- [ ] `docs/openapi.yaml` (agent HTTP spec) → delete.

## Phase 8 — Verify the local product

- [ ] `go build ./cmd/scout/ ./pkg/...` clean; `go vet ./...` clean; `task test` (or `go test ./... -short`) green (browser tests skip without Chromium — acceptable).
- [ ] Smoke the local surface: `scout version`, `scout cmdtree` (no remote groups), `scout gather https://example.com --screenshot`, `scout repl https://example.com`, `scout mcp` starts a stdio server, `scout plugin list`.
- [ ] Confirm `pkg/scout` imports neither `grpc/`, `pkg/scout/agent`, nor `pkg/scout/discovery`: `go list -deps ./pkg/scout | grep -E "grpc|discovery|agent"` → empty.

## Risk / rollback
- All work on the branch; `main` untouched. If Phase 2 proves larger than expected (bare verbs deeply daemon-coupled), pause and reassess scope with the owner before continuing.
- Deletion order matters: Phase 1 first surfaces the exact decoupling list (Phase 2) via the build failure.
