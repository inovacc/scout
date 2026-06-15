# Scout — Security Hardening Remediation

**Date:** 2026-06-14
**Audit:** [scout-hardening-audit-2026-06-14.md](scout-hardening-audit-2026-06-14.md) (17 verified findings)
**Status:** all 16 actionable findings remediated (1 info finding — clean `govulncheck` — needed no action). Build, `gofmt`, and `go vet` clean; unit tests + one real-browser integration test pass.

## What changed, by finding

| # | Finding | Fix | Files |
|---|---|---|---|
| 9 | Reports world-readable (0o644) w/ cookies+HAR | `0o600` file / `0o700` dir | `internal/engine/report.go` |
| 10 | Command logs world-readable w/ secret args | `0o600`/`0o700` + `redactArgs()` masks secret flag values | `internal/logger/logger.go` |
| 13 | HAR/screenshot world-readable | `0o600` | `cmd/scout/gather.go` |
| 5 | Plugin install path traversal via manifest `name` | `IsLocal`+separator check in `manifest.validate()` and `safePluginDir()` containment (install + remove) | `pkg/scout/plugin/manifest.go`, `cmd/scout/plugin.go` |
| 6 | Plugin checksum never enforced (TOFU) | Manual install/github **fail closed** without `--checksum` (new `--allow-unverified`); registry `update` now pins the real downloaded SHA in the lock file | `cmd/scout/plugin.go` |
| 1 | MCP SSE unauthenticated, no bind guard | `guardSSEBind()` refuses non-loopback unless `SCOUT_MCP_ALLOW_REMOTE=1` | `pkg/scout/mcp/server.go` |
| 11 | Agent library fails open (guard only in CLI) | `guardBind()` in `ListenAndServe`; `ServerConfig.InsecureAllowRemote` escape hatch | `pkg/scout/agent/server.go` |
| 14 | Proxy: no auth, all interfaces, no SSRF policy | Loopback default; optional `Config.Token`/`SCOUT_PROXY_TOKEN`; `urlpolicy` check on every target | `pkg/scout/proxy/proxy.go`, `cmd/scout/proxy.go` |
| 3 | SSRF policy ignores redirects | CDP `Fetch`-layer request filter re-checks every request | `internal/engine/block.go` (`InstallRequestFilter`) |
| 4 | eval/JS tools bypass SSRF gate | Same request filter covers in-page `fetch()` | `pkg/scout/agent/server.go`, `pkg/scout/mcp/server.go` wire it |
| 2 | SSRF policy DNS-rebind (TOCTOU) | **Partial** — filter re-validates per request (see residuals) | as above |
| 7 | Pairing token+certs over plaintext, all interfaces | Pairing listener defaults to loopback (`--pairing-host` opt-in + plaintext warning); OOB token already required | `cmd/scout/server.go` |
| 8 | gRPC `CreateSession` unbounded session spawn | `maxConcurrentSessions()` cap (default 32, `SCOUT_MAX_SESSIONS`) → `ResourceExhausted` | `grpc/server/server_session.go` |
| 16 | Interactive stream goroutine leak on stall | Forwarder now selects on `stream.Context().Done()` | `grpc/server/server_hijack_stream.go` |
| 12 | gops unauthenticated local control endpoint | Opt-out via `SCOUT_GOPS=0` (discovery via `goprocess.Find` does not need the agent) | `cmd/scout/scout.go` |
| 15 | Unbounded WS frame allocation in CDP client | 256 MiB frame-length cap before allocation | `internal/engine/lib/cdp/websocket.go` |

## New configuration surface

- `--allow-unverified` — opt into installing a plugin with no checksum.
- `--pairing-host` (default `127.0.0.1`) — bind host for the plaintext pairing listener.
- `SCOUT_PROXY_TOKEN` / `Config.auth_token`, `SCOUT_PROXY_ALLOW_REMOTE` — proxy auth + remote opt-in.
- `SCOUT_MCP_ALLOW_REMOTE` — opt into a non-loopback MCP SSE bind.
- `ServerConfig.InsecureAllowRemote` — agent library non-loopback opt-in.
- `SCOUT_MAX_SESSIONS` (default 32) — daemon concurrent-session cap.
- `SCOUT_GOPS=0` — disable the gops diagnostic agent.

## Behavioral changes to note

- `scout proxy start` and the device-pairing listener now bind **loopback by default** (were all interfaces).
- `scout plugin install <url|github:>` now **fails closed** without `--checksum` (use `--allow-unverified` to keep old behavior).

## Residual risk / follow-ups (registry- or platform-side)

- **#2 DNS rebinding** — the request filter re-validates each request, but the browser resolves DNS independently, so a rebind between check and connect is not fully prevented. Full pinning needs Chrome `--host-resolver-rules` or a custom resolver.
- **#6 supply chain** — the registry index publishes no per-release checksums, so registry `update` remains trust-on-first-use (now SHA-pinned in the lock file). Closing fully requires a signed index + published checksums + downgrade protection.
- **#7 pairing** — when an operator opts into a routable `--pairing-host`, the token+certs still transit in plaintext; use a trusted network until an encrypted (ECDH/TLS-bootstrap) handshake is added.
- **#12 gops** — the agent is still on by default (preserves cross-process discovery); `SCOUT_GOPS=0` is the mitigation for multi-user hosts.
