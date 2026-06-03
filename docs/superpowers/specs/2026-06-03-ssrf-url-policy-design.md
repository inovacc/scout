# SSRF URL-Policy for Untrusted Ingress — Design

**Date:** 2026-06-03
**Status:** Approved (brainstorm)
**Source:** HARDENING-AUDIT-2026-06-03 finding C2 (HIGH) — MCP `navigate`/`open`/`gather`/`form_detect`/`swarm_crawl` and the agent REST API accept `file://`, `chrome://`, and internal/metadata URLs unchecked, enabling SSRF + arbitrary local-file read.

## Goal

Prevent an **untrusted principal** (an AI model driving the MCP server, or a remote caller of the agent REST API) from steering Scout's browser at internal/metadata endpoints or local files — **without** restricting Scout used as a library or via the CLI, where navigating to `localhost`/`file://`/internal services is a legitimate primary use case.

## Key decisions (locked during brainstorm)

1. **Enforcement layer:** untrusted ingress **only** — the MCP server and the agent REST API. The core `internal/engine` `Page.Navigate`, the public `pkg/scout` library API, and the CLI are left **unrestricted**.
2. **Default posture:** block-by-default at those ingresses, with an opt-in to re-enable local/internal targets.
3. **Block scope:** all non-`http(s)` schemes; for `http(s)`, **resolve the hostname** and block if it lands on an internal IP range. DNS resolution closes the `attacker.com → 127.0.0.1` bypass.
4. **Depth (v1):** validate the **top-level target URL pre-navigation** only.
5. **Opt-in shape:** a blanket boolean opt-out **plus** a granular host/CIDR allowlist.

## Architecture

A new pure, dependency-light package decides policy; the two untrusted ingresses build a `Policy` from their config and call it before any URL-taking tool runs.

```
pkg/scout/urlpolicy/      NEW — pure decision unit (no browser, no engine deps)
  policy.go               Policy type + Check()
  policy_test.go          table-driven unit tests with an injected resolver

pkg/scout/mcp/            build Policy from flags/env; guard each URL-taking tool
pkg/scout/agent/          build Policy from flags/env; guard the navigate path
cmd/scout/                --allow-local-targets / --allow-target flags on mcp+agent serve
```

### `pkg/scout/urlpolicy` — the decision unit

```go
package urlpolicy

// Resolver looks a host up to IPs. Injectable so tests need no network.
type Resolver interface {
    LookupIP(ctx context.Context, host string) ([]net.IP, error)
}

type Policy struct {
    AllowLocal bool          // blanket opt-out: allow everything, incl. non-http schemes
    AllowCIDRs []*net.IPNet  // granular allowlist of IP ranges
    AllowHosts []string      // granular allowlist of exact hostnames (case-insensitive)
    Resolver   Resolver      // nil → net.DefaultResolver
}

// Check reports whether rawURL may be navigated to. It returns nil when allowed
// and a *BlockedError when not.
func (p Policy) Check(ctx context.Context, rawURL string) error

type BlockedError struct {
    Reason string // "scheme" | "internal-ip" | "parse"
    Detail string // offending scheme / IP / parse message
    URL    string
}
func (e *BlockedError) Error() string // includes the opt-in hint
```

**What it does:** given a raw URL string, returns nil or a `*BlockedError`.
**How you use it:** build a `Policy` once from config; call `Check` per request.
**What it depends on:** `net`, `net/url`, `context`, and an injectable `Resolver`. Nothing in `internal/engine` or the browser.

## Decision logic (`Policy.Check`)

1. If `AllowLocal` → **allow** (full opt-out, including non-http schemes).
2. Parse `rawURL` with `net/url`. On error → **block** (`parse`).
3. If scheme ∉ {`http`, `https`} → **block** (`scheme`). Covers `file`, `chrome`, `data`, `view-source`, `ftp`, `blob`, etc.
4. Extract host. If host (case-insensitive) ∈ `AllowHosts` → **allow**.
5. Resolve host to IPs:
   - If host is an IP literal, use it directly (no DNS).
   - Else `Resolver.LookupIP`. On resolution error → **block** (`internal-ip`, detail = "unresolved") — fail closed.
6. For the resolved IP set:
   - An IP in any `AllowCIDRs` range is treated as allowed.
   - An IP is **internal** if any of: `IsLoopback()`, `IsPrivate()` (10/8, 172.16/12, 192.168/16, fc00::/7), `IsLinkLocalUnicast()` (169.254/16 incl. `169.254.169.254`, fe80::/10), `IsLinkLocalMulticast()`, `IsInterfaceLocalMulticast()`, `IsMulticast()`, `IsUnspecified()`.
   - **Block (`internal-ip`) if ANY resolved IP is internal and not in `AllowCIDRs`.** Blocking on *any* internal IP (rather than all) defeats mixed-record DNS tricks.
7. Otherwise → **allow**.

## Integration & data flow

- **Config → Policy.** `cmd/scout` adds to both `mcp` (serve) and `agent serve`:
  - `--allow-local-targets` (bool) → `Policy.AllowLocal`; env `SCOUT_ALLOW_LOCAL_TARGETS` (`1`/`true`).
  - `--allow-target <host|CIDR>` (repeatable) → parsed into `AllowHosts` (plain host) or `AllowCIDRs` (parses as CIDR); env `SCOUT_ALLOW_TARGETS` (comma-separated).
- **MCP.** A shared `func (s *mcpState) checkURL(ctx, url string) error` holds the built `Policy`. It is called at the top of every URL-taking tool handler: `navigate`, `open`, `gather`, `swarm_crawl`, `form_detect`. On block it returns the tool result with `IsError = true` and the `BlockedError` message.
- **Agent.** The agent `Provider` (or server) holds a `Policy`; the `navigate` tool path calls `Check` before navigating and returns the block message as a `ToolResult` error.

## Error handling

A blocked request never reaches the browser. The `BlockedError.Error()` string is returned verbatim to the model/caller and names how to opt in, e.g.:

```
blocked: refusing to navigate to an internal address 127.0.0.1 (loopback).
This MCP server denies internal/local targets by default. To allow it, restart
with --allow-target 127.0.0.1 (or a CIDR), or --allow-local-targets to allow all.
```

## Testing

- **Unit (no network, no browser):** table-driven `Policy.Check` tests with an injected fake `Resolver`:
  - schemes: `file://`, `data:…`, `chrome://…`, `view-source:…` → blocked (`scheme`).
  - IP literals: `http://127.0.0.1`, `http://169.254.169.254`, `http://10.0.0.1`, `http://192.168.0.5`, `http://[::1]` → blocked (`internal-ip`).
  - public: `https://example.com` (resolver → `93.184.x.x`) → allowed.
  - DNS bypass: `http://evil.test` (fake resolver → `127.0.0.1`) → blocked.
  - allowlist: same internal IP/host present in `AllowCIDRs`/`AllowHosts` → allowed.
  - opt-out: `AllowLocal=true` → everything (incl. `file://`) allowed.
  - resolution failure → blocked (fail-closed).
- **Integration (one test):** an MCP `navigate` call to `http://127.0.0.1:1/` (or `file://`) returns an error result without launching a navigation.

## Scope — explicit v1 limitations (out of scope; v2 candidates)

These are deliberately **not** covered in v1 (per the chosen "top-level target, pre-navigation" depth) and will be documented in `docs/BACKLOG.md`:

- **Redirect-to-internal:** a permitted public URL that 3xx-redirects to an internal IP. (v2: re-validate the landed URL, or disable redirect-following for guarded navigations.)
- **In-page subresources:** a page that issues `fetch`/`XHR`/`img` requests to internal IPs. (v2: CDP request interception via Scout's existing hijack/block infra.)
- **Crawler-discovered links:** `swarm_crawl`/`gather` following a discovered link to an internal IP. Only the user-supplied seed URL is checked in v1.
- **DNS rebinding:** the policy resolves once at check time; the browser resolves again at navigation time. (v2: pin the resolved IP, or intercept at CDP.)

## Non-goals

- No change to `internal/engine` `Page.Navigate`, the `pkg/scout` library API, or CLI navigation behavior.
- No allowlist persistence/config file — flags + env only.
