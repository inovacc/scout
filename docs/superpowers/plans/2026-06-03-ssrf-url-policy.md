# SSRF URL-Policy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Block untrusted MCP/agent callers from steering Scout's browser at internal/metadata endpoints or local files, while leaving the library and CLI unrestricted.

**Architecture:** A pure `pkg/scout/urlpolicy` package decides allow/deny for a URL (scheme + DNS-resolved IP ranges). The MCP server and agent server build a `*urlpolicy.Policy` from environment variables inside their constructors and call `Check` at the top of every URL-taking tool. CLI `--allow-local-targets` / `--allow-target` flags set those env vars. No changes to `internal/engine`, `pkg/scout` library navigation, or the CLI navigate behavior.

**Tech Stack:** Go 1.26, stdlib `net` / `net/url`, existing `pkg/scout/mcp` + `pkg/scout/agent` servers, Cobra CLI.

**Spec:** `docs/superpowers/specs/2026-06-03-ssrf-url-policy-design.md`

---

## File Structure

- `pkg/scout/urlpolicy/policy.go` — NEW. `Policy`, `Check`, `BlockedError`, `Resolver`, internal-IP test.
- `pkg/scout/urlpolicy/config.go` — NEW. `FromEnv`, `ParseAllowTargets`, env helpers.
- `pkg/scout/urlpolicy/policy_test.go` — NEW. Table-driven `Check` tests with a fake resolver.
- `pkg/scout/urlpolicy/config_test.go` — NEW. `ParseAllowTargets` / `FromEnv` tests.
- `pkg/scout/mcp/server.go` — MODIFY. Add `policy` field to `mcpState`; build it in `NewServer`; add `checkURL`.
- `pkg/scout/mcp/tools_browser.go`, `tools_session.go`, `tools_gather.go`, `tools_form.go`, `tools_swarm.go` — MODIFY. Insert the guard in the URL-taking handlers.
- `pkg/scout/agent/provider.go` — MODIFY. Add policy + guard the navigate tool.
- `cmd/scout/mcp.go`, `cmd/scout/agent.go` — MODIFY. Add flags → env bridge.
- `docs/BACKLOG.md` — MODIFY. Record v2 SSRF items.

---

## Task 1: urlpolicy core — `Policy.Check`

**Files:**
- Create: `pkg/scout/urlpolicy/policy.go`
- Test: `pkg/scout/urlpolicy/policy_test.go`

- [ ] **Step 1: Write the failing test**

```go
package urlpolicy

import (
	"context"
	"net"
	"testing"
)

// fakeResolver maps hostnames to fixed IPs so tests need no real DNS.
type fakeResolver map[string][]net.IP

func (f fakeResolver) LookupIP(_ context.Context, host string) ([]net.IP, error) {
	if ips, ok := f[host]; ok {
		return ips, nil
	}
	return nil, &net.DNSError{Err: "not found", Name: host}
}

func TestCheck(t *testing.T) {
	res := fakeResolver{
		"example.com": {net.ParseIP("93.184.216.34")},
		"evil.test":   {net.ParseIP("127.0.0.1")},
		"mixed.test":  {net.ParseIP("93.184.216.34"), net.ParseIP("10.0.0.5")},
	}
	base := Policy{Resolver: res}

	allowed := []string{
		"https://example.com/path?q=1",
		"http://example.com",
	}
	for _, u := range allowed {
		if err := base.Check(context.Background(), u); err != nil {
			t.Errorf("Check(%q) = %v, want allowed", u, err)
		}
	}

	blocked := []string{
		"file:///etc/passwd",
		"data:text/html,<h1>x</h1>",
		"chrome://settings",
		"http://127.0.0.1",
		"http://169.254.169.254/latest/meta-data/",
		"http://10.0.0.1",
		"http://192.168.1.5:8080",
		"http://[::1]/",
		"http://evil.test",   // DNS bypass: name → loopback
		"http://mixed.test",  // any internal IP in the set blocks
		"not a url ::::",
	}
	for _, u := range blocked {
		var be *BlockedError
		err := base.Check(context.Background(), u)
		if err == nil {
			t.Errorf("Check(%q) = nil, want blocked", u)
			continue
		}
		if !asBlocked(err, &be) {
			t.Errorf("Check(%q) = %v, want *BlockedError", u, err)
		}
	}
}

func TestCheckAllowLocalBypass(t *testing.T) {
	p := Policy{AllowLocal: true}
	for _, u := range []string{"file:///etc/passwd", "http://127.0.0.1", "http://169.254.169.254"} {
		if err := p.Check(context.Background(), u); err != nil {
			t.Errorf("AllowLocal Check(%q) = %v, want allowed", u, err)
		}
	}
}

func TestCheckAllowlist(t *testing.T) {
	res := fakeResolver{"box.local": {net.ParseIP("192.168.1.50")}}
	_, cidr, _ := net.ParseCIDR("192.168.1.0/24")
	p := Policy{Resolver: res, AllowCIDRs: []*net.IPNet{cidr}}
	if err := p.Check(context.Background(), "http://box.local"); err != nil {
		t.Errorf("allowlisted CIDR Check = %v, want allowed", err)
	}

	p2 := Policy{Resolver: res, AllowHosts: []string{"box.local"}}
	if err := p2.Check(context.Background(), "http://box.local"); err != nil {
		t.Errorf("allowlisted host Check = %v, want allowed", err)
	}
}

// asBlocked is a tiny errors.As helper kept local to avoid an import in the test body.
func asBlocked(err error, target **BlockedError) bool {
	be, ok := err.(*BlockedError)
	if ok {
		*target = be
	}
	return ok
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/scout/urlpolicy/`
Expected: FAIL — `undefined: Policy`, `undefined: BlockedError`.

- [ ] **Step 3: Write minimal implementation**

```go
// Package urlpolicy decides whether an untrusted-supplied URL may be navigated
// to. It is enforced at untrusted ingress (the MCP server and agent REST API)
// only; the Scout library and CLI are unaffected.
package urlpolicy

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
)

// Resolver looks a host up to IPs. Injecting it lets tests avoid real DNS.
type Resolver interface {
	LookupIP(ctx context.Context, host string) ([]net.IP, error)
}

type defaultResolver struct{}

func (defaultResolver) LookupIP(ctx context.Context, host string) ([]net.IP, error) {
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	ips := make([]net.IP, 0, len(addrs))
	for _, a := range addrs {
		ips = append(ips, a.IP)
	}
	return ips, nil
}

// Policy decides whether an untrusted-supplied URL may be navigated to.
type Policy struct {
	AllowLocal bool         // blanket opt-out: allow everything, including non-http schemes
	AllowCIDRs []*net.IPNet // granular allowlist of IP ranges
	AllowHosts []string     // granular allowlist of exact hostnames (case-insensitive)
	Resolver   Resolver     // nil → net.DefaultResolver
}

// BlockedError reports why a URL was denied.
type BlockedError struct {
	Reason string // "scheme" | "internal-ip" | "parse"
	Detail string
	URL    string
}

func (e *BlockedError) Error() string {
	return fmt.Sprintf("blocked %q: %s (%s). This endpoint denies internal/local targets by default; "+
		"restart with --allow-target <host|CIDR> or --allow-local-targets to permit it.", e.URL, e.Reason, e.Detail)
}

// Check reports whether rawURL may be navigated to. nil means allowed.
func (p Policy) Check(ctx context.Context, rawURL string) error {
	if p.AllowLocal {
		return nil
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return &BlockedError{Reason: "parse", Detail: err.Error(), URL: rawURL}
	}

	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return &BlockedError{Reason: "scheme", Detail: schemeOrEmpty(scheme), URL: rawURL}
	}

	host := u.Hostname()
	if host == "" {
		return &BlockedError{Reason: "parse", Detail: "empty host", URL: rawURL}
	}
	for _, h := range p.AllowHosts {
		if strings.EqualFold(h, host) {
			return nil
		}
	}

	ips, err := p.resolve(ctx, host)
	if err != nil {
		return &BlockedError{Reason: "internal-ip", Detail: "unresolved: " + err.Error(), URL: rawURL}
	}

	for _, ip := range ips {
		if p.allowedIP(ip) {
			continue
		}
		if isInternalIP(ip) {
			return &BlockedError{Reason: "internal-ip", Detail: ip.String(), URL: rawURL}
		}
	}

	return nil
}

func schemeOrEmpty(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}

func (p Policy) resolve(ctx context.Context, host string) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, nil
	}
	r := p.Resolver
	if r == nil {
		r = defaultResolver{}
	}
	return r.LookupIP(ctx, host)
}

func (p Policy) allowedIP(ip net.IP) bool {
	for _, n := range p.AllowCIDRs {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// isInternalIP reports whether ip is in a range an untrusted caller must not reach.
func isInternalIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified()
}
```

NOTE: `url.Parse` is lenient, so most malformed inputs parse with an empty scheme and are caught by the scheme check (`(none)` ≠ http/https → blocked) rather than the parse guard. Both paths return a `*BlockedError`, which is all the test asserts.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/scout/urlpolicy/`
Expected: PASS.

- [ ] **Step 5: Run golangci-lint and fix any new issues**

Run: `golangci-lint run ./pkg/scout/urlpolicy/`
Expected: 0 issues. (Likely linters: `errorlint` is disabled project-wide; `revive`/`gocritic` may flag — fix per message. Do NOT use `==` on errors.)

- [ ] **Step 6: Commit**

```bash
git add pkg/scout/urlpolicy/policy.go pkg/scout/urlpolicy/policy_test.go
git commit -m "feat(urlpolicy): URL allow/deny decision unit for SSRF defense"
```

---

## Task 2: urlpolicy config — `FromEnv` + `ParseAllowTargets`

**Files:**
- Create: `pkg/scout/urlpolicy/config.go`
- Test: `pkg/scout/urlpolicy/config_test.go`

- [ ] **Step 1: Write the failing test**

```go
package urlpolicy

import (
	"net"
	"testing"
)

func TestParseAllowTargets(t *testing.T) {
	hosts, cidrs := ParseAllowTargets([]string{
		"192.168.1.0/24",  // CIDR
		"127.0.0.1",       // bare IPv4 → /32
		"::1",             // bare IPv6 → /128
		"box.local",       // hostname
		"  ",              // ignored
	})

	if len(hosts) != 1 || hosts[0] != "box.local" {
		t.Errorf("hosts = %v, want [box.local]", hosts)
	}
	if len(cidrs) != 3 {
		t.Fatalf("cidrs = %d, want 3", len(cidrs))
	}
	if !cidrs[1].Contains(net.ParseIP("127.0.0.1")) {
		t.Errorf("bare IPv4 not parsed to containing CIDR")
	}
}

func TestFromEnv(t *testing.T) {
	t.Setenv("SCOUT_ALLOW_LOCAL_TARGETS", "1")
	t.Setenv("SCOUT_ALLOW_TARGETS", "10.0.0.0/8, host.example")
	p := FromEnv()
	if !p.AllowLocal {
		t.Error("AllowLocal = false, want true")
	}
	if len(p.AllowCIDRs) != 1 || len(p.AllowHosts) != 1 {
		t.Errorf("got %d cidrs %d hosts, want 1/1", len(p.AllowCIDRs), len(p.AllowHosts))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -run 'TestParseAllowTargets|TestFromEnv' ./pkg/scout/urlpolicy/`
Expected: FAIL — `undefined: ParseAllowTargets`, `undefined: FromEnv`.

- [ ] **Step 3: Write minimal implementation**

```go
package urlpolicy

import (
	"net"
	"os"
	"strings"
)

// FromEnv builds a Policy from SCOUT_ALLOW_LOCAL_TARGETS (bool) and
// SCOUT_ALLOW_TARGETS (comma-separated host/CIDR list).
func FromEnv() *Policy {
	hosts, cidrs := ParseAllowTargets(splitList(os.Getenv("SCOUT_ALLOW_TARGETS")))
	return &Policy{
		AllowLocal: envTrue(os.Getenv("SCOUT_ALLOW_LOCAL_TARGETS")),
		AllowHosts: hosts,
		AllowCIDRs: cidrs,
	}
}

// ParseAllowTargets splits entries into exact hostnames and CIDR ranges. A bare
// IP becomes a single-address CIDR; anything else is treated as a hostname.
func ParseAllowTargets(entries []string) (hosts []string, cidrs []*net.IPNet) {
	for _, e := range entries {
		if _, n, err := net.ParseCIDR(e); err == nil {
			cidrs = append(cidrs, n)
			continue
		}
		if ip := net.ParseIP(e); ip != nil {
			bits := 32
			if ip.To4() == nil {
				bits = 128
			}
			cidrs = append(cidrs, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
			continue
		}
		hosts = append(hosts, e)
	}
	return hosts, cidrs
}

func envTrue(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func splitList(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/scout/urlpolicy/`
Expected: PASS (all of Task 1 + Task 2).

- [ ] **Step 5: Commit**

```bash
git add pkg/scout/urlpolicy/config.go pkg/scout/urlpolicy/config_test.go
git commit -m "feat(urlpolicy): build Policy from env + parse allow-target entries"
```

---

## Task 3: MCP integration — guard URL-taking tools

**Files:**
- Modify: `pkg/scout/mcp/server.go` (`mcpState` struct ~line 34; `NewServer` ~line 190)
- Modify: `pkg/scout/mcp/tools_browser.go` (`navigate` ~line 32), `tools_session.go` (`open`), `tools_gather.go` (`gather`), `tools_form.go` (`form_detect`), `tools_swarm.go` (`swarm_crawl`)
- Test: `pkg/scout/mcp/urlpolicy_test.go` (new)

- [ ] **Step 1: Add the policy field + helper to `mcpState`**

In `pkg/scout/mcp/server.go`, add the import `"github.com/inovacc/scout/pkg/scout/urlpolicy"`. Add a field to `mcpState`:

```go
type mcpState struct {
	mu        sync.Mutex
	browser   *scout.Browser
	page      *scout.Page
	config    ServerConfig
	idle      *idle.Timer
	ariaStore *aria.Store
	hooks     *hookRegistry
	policy    *urlpolicy.Policy
}
```

In `NewServer` (~line 191), set the policy when building state:

```go
	state := &mcpState{config: cfg, ariaStore: aria.NewStore(), hooks: newHookRegistry(), policy: urlpolicy.FromEnv()}
```

Add this method to `server.go` (next to `ensurePage`):

```go
// checkURL enforces the SSRF URL-policy for untrusted MCP callers. A nil policy
// (should not happen) allows everything.
func (s *mcpState) checkURL(ctx context.Context, rawURL string) error {
	if s.policy == nil {
		return nil
	}
	return s.policy.Check(ctx, rawURL)
}
```

- [ ] **Step 2: Insert the guard in each URL-taking handler**

In `tools_browser.go` `navigate` handler, immediately AFTER the `json.Unmarshal` of args and BEFORE `state.ensurePage`:

```go
		if err := state.checkURL(ctx, args.URL); err != nil {
			return errResult(err.Error())
		}
```

Repeat the same insertion (matching the local arg field name — it may be `args.URL`) in the handlers for `open` (`tools_session.go`), `gather` (`tools_gather.go`), `form_detect` (`tools_form.go`), and `swarm_crawl` (`tools_swarm.go`). For each: read the handler, find where it unmarshals the URL argument, and insert the guard right after, before any browser/page call. If a tool's URL field is named differently (e.g. `args.StartURL` for swarm), use that field.

- [ ] **Step 3: Write the integration test**

```go
package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/inovacc/scout/pkg/scout/urlpolicy"
)

func TestCheckURLBlocksInternalByDefault(t *testing.T) {
	s := &mcpState{policy: urlpolicy.FromEnv()} // no env → block-by-default

	for _, u := range []string{"file:///etc/passwd", "http://127.0.0.1", "http://169.254.169.254"} {
		if err := s.checkURL(context.Background(), u); err == nil {
			t.Errorf("checkURL(%q) = nil, want blocked", u)
		} else if !strings.HasPrefix(err.Error(), "blocked") {
			t.Errorf("checkURL(%q) error = %q, want blocked-prefixed", u, err)
		}
	}

	if err := s.checkURL(context.Background(), "https://example.com"); err != nil {
		t.Errorf("checkURL(public) = %v, want allowed", err)
	}
}

func TestCheckURLAllowLocalEnv(t *testing.T) {
	t.Setenv("SCOUT_ALLOW_LOCAL_TARGETS", "1")
	s := &mcpState{policy: urlpolicy.FromEnv()}
	if err := s.checkURL(context.Background(), "http://127.0.0.1"); err != nil {
		t.Errorf("with AllowLocal, checkURL(loopback) = %v, want allowed", err)
	}
}
```

- [ ] **Step 4: Build + test**

Run: `go build ./pkg/scout/mcp/ && go test -run 'TestCheckURL' ./pkg/scout/mcp/`
Expected: PASS. Then `go vet ./pkg/scout/mcp/` and `golangci-lint run ./pkg/scout/mcp/` — fix any new issues (0 new).

- [ ] **Step 5: Commit**

```bash
git add pkg/scout/mcp/
git commit -m "feat(mcp): enforce SSRF URL-policy on navigate/open/gather/form_detect/swarm_crawl"
```

---

## Task 4: Agent integration — guard navigate

**Files:**
- Modify: `pkg/scout/agent/provider.go` (the `navigate` tool handler)

- [ ] **Step 1: Add a policy to the Provider**

Read `pkg/scout/agent/provider.go`. Add the import `"github.com/inovacc/scout/pkg/scout/urlpolicy"`. Add a `policy *urlpolicy.Policy` field to the `Provider` struct, and initialize it in `NewProvider` with `urlpolicy.FromEnv()`:

```go
// in the Provider struct:
	policy *urlpolicy.Policy

// in NewProvider(...), when building the Provider value:
	policy: urlpolicy.FromEnv(),
```

- [ ] **Step 2: Guard the navigate tool**

In the `navigate` tool implementation (the `case "navigate":` in `Call`, or the dedicated navigate method), BEFORE the call to `page.Navigate`/`browser` navigation, add:

```go
	if p.policy != nil {
		if err := p.policy.Check(ctx, url); err != nil {
			return ToolResult{Content: err.Error(), IsError: true}, nil
		}
	}
```

Match the actual `url` variable name and the `ToolResult` construction already used in that handler (read the surrounding code; the project returns errors as `ToolResult{... IsError: true}`).

- [ ] **Step 3: Build + smoke test**

Run: `go build ./pkg/scout/agent/ && go test -short ./pkg/scout/agent/`
Expected: PASS (browser-dependent tests skip). `golangci-lint run ./pkg/scout/agent/` — 0 new issues.

- [ ] **Step 4: Commit**

```bash
git add pkg/scout/agent/provider.go
git commit -m "feat(agent): enforce SSRF URL-policy on the navigate tool"
```

---

## Task 5: CLI flags + env bridge + BACKLOG

**Files:**
- Modify: `cmd/scout/mcp.go` (`init` ~line 117; `RunE` before the `Serve`/`ServeSSE` calls ~line 108)
- Modify: `cmd/scout/agent.go` (the agent `serve` flag registration + `RunE`)
- Modify: `docs/BACKLOG.md`

- [ ] **Step 1: Add flags to `mcp` (in `cmd/scout/mcp.go` `init`)**

```go
	mcpCmd.Flags().Bool("allow-local-targets", false, "allow MCP tools to navigate to local/internal addresses (off by default)")
	mcpCmd.Flags().StringSlice("allow-target", nil, "allow a specific host or CIDR as an MCP navigation target (repeatable)")
```

- [ ] **Step 2: Bridge the flags to env in `mcp` `RunE`**

Add `"strings"` to the imports of `cmd/scout/mcp.go`. In `RunE`, AFTER reading the existing flags (after the `idleTimeout` line, before the `if useSSE` block), add:

```go
		if v, _ := cmd.Flags().GetBool("allow-local-targets"); v {
			_ = os.Setenv("SCOUT_ALLOW_LOCAL_TARGETS", "1")
		}
		if v, _ := cmd.Flags().GetStringSlice("allow-target"); len(v) > 0 {
			_ = os.Setenv("SCOUT_ALLOW_TARGETS", strings.Join(v, ","))
		}
```

(`os` is already imported. The env vars are read by `urlpolicy.FromEnv()` inside `scoutmcp.Serve` / `ServeSSE` → `NewServer`.)

- [ ] **Step 3: Add the same flags + bridge to the agent `serve` command**

In `cmd/scout/agent.go`, find where `agentServeCmd` flags are registered (an `init` or `agentServeCmd.Flags()...` block) and add the identical two flags. In its `RunE`, after the existing flag reads and after the existing non-loopback auth guard, add the same env-bridge block (the agent's `urlpolicy.FromEnv()` in `NewProvider` reads them). Ensure `"strings"` is imported in `agent.go` (add if missing).

- [ ] **Step 4: Build the CLI**

Run: `go build ./cmd/scout/`
Expected: success. Run `go run ./cmd/scout/ mcp --help` and confirm `--allow-local-targets` and `--allow-target` appear.

- [ ] **Step 5: Document v2 follow-ups in `docs/BACKLOG.md`**

Add an entry:

```markdown
### SSRF-V2 — deeper SSRF coverage (v1 shipped 2026-06-03)

v1 (`pkg/scout/urlpolicy`) validates only the top-level target URL pre-navigation
at MCP + agent ingress. Out of scope, tracked here:
- Redirect-to-internal: re-validate the landed URL or disable redirect-following for guarded navigations.
- In-page subresources (fetch/XHR/img to internal IPs): enforce via CDP request interception (reuse the hijack/block infra).
- Crawler-discovered links: apply the policy to each URL swarm_crawl/gather fetches, not just the seed.
- DNS rebinding: pin the resolved IP through to navigation, or intercept at CDP.
```

- [ ] **Step 6: Final verification + commit**

```bash
go build ./cmd/scout/ ./pkg/... && go test ./pkg/scout/urlpolicy/
git add cmd/scout/mcp.go cmd/scout/agent.go docs/BACKLOG.md
git commit -m "feat(cli): --allow-local-targets / --allow-target flags for MCP + agent; doc SSRF v2"
```

---

## Final review (whole feature)

After all tasks: dispatch a code review, then `superpowers:finishing-a-development-branch` to merge `feat/ssrf-url-policy` into `main`.

- Confirm: library/CLI navigation unchanged (no edits to `internal/engine` or `pkg/scout/*.go` navigation).
- Confirm: `go test ./pkg/scout/urlpolicy/ ./pkg/scout/mcp/ ./pkg/scout/agent/ ./cmd/scout/` green.
- Confirm: default (no flags/env) blocks `file://` + internal IPs at MCP; public URLs still work.
