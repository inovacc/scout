# Scout — Security Hardening Audit

**Date:** 2026-06-14  
**Method:** Multi-agent adversarial audit — 13 security dimensions; every candidate finding re-verified by an independent auditor (false positives dropped). Report-first; no code changed.  
**Overall risk rating:** high  
**Verified findings:** 17 ranked (after dedupe); 19 pre-dedupe candidates that survived verification.

## Executive summary

Scout's hardening review surfaced 16 verified findings; after deduplicating the two overlapping plugin-integrity findings into one, 15 distinct issues remain. The dependency/toolchain surface is clean (govulncheck: zero known-vulnerable modules), so risk is concentrated in two structural areas. First, Scout's network-exposed control planes lack defense-in-depth at the LIBRARY boundary even where the shipped CLI is safe: the MCP SSE server exposes full browser control (including arbitrary JS eval) with NO authentication and no loopback guard once an operator sets a non-loopback --addr (authnz-1, high), and the gRPC device-pairing handshake transmits its trust-bootstrap token and certificates in cleartext on all interfaces (tls-1, high). Second, Scout's purpose-built SSRF egress policy (urlpolicy) is enforced only at the navigate API surface and only on the seed URL, so it is defeated three independent ways — DNS rebinding (ssrf-egress-1), HTTP redirect-to-internal (ssrf-egress-2), and the eval/JS tools that bypass the gate entirely (ssrf-egress-3), all high/plausible — letting an authorized-but-confined caller reach cloud-metadata and internal services. The plugin supply chain is the third high-risk theme: plugin names are not validated, enabling arbitrary file write outside the plugins dir (path traversal -> persistence/RCE, plugin-supply-chain-1), and a fully-built checksum/integrity API is never enforced at install or launch, so a tampered remote/registry artifact runs as the local user (plugin-supply-chain-2, high). Lower-tier issues cluster around secret-bearing artifacts written world-readable (0o644) inconsistent with the project's own 0o600 standard, an unauthenticated gops diagnostic endpoint on every process, and resource-exhaustion gaps. The single highest-leverage fixes are: enforce urlpolicy at the CDP Fetch.requestPaused layer (closes all three SSRF bypasses at once), require auth for non-loopback MCP/agent binds in the library (not just the CLI), validate plugin names + fail closed on checksum mismatch at launch, and encrypt the pairing channel.

## Cross-cutting themes

- Network-exposed services lack auth-by-default and a fail-closed bind guard: MCP SSE has no auth at all and no loopback guard (authnz-1); the agent library fails open at the package boundary while only the CLI is safe (jseval-injection-1); the proxy binds all interfaces with no auth (ssrf-egress-4). Security guards live in CLI wrappers instead of the reusable library/server constructors.
- Scout's SSRF egress policy is enforced at the wrong layer: it validates the seed URL at the navigate API surface only, so it is defeated by DNS rebinding, redirect-to-internal, and the eval/JS tools that never call it. The correct fix is one architectural change — enforce urlpolicy on every outbound request at the CDP Fetch.requestPaused layer and pin the validated IP.
- Plugin supply chain lacks integrity enforcement and input validation: manifest 'name' is used as a path with no containment (zip-slip-style escape after extraction), and a complete checksum/signature verification API is never called at install or launch (optional --checksum, empty pinned lock checksum, unsigned registry index, no downgrade protection).
- Secret-bearing artifacts are persisted world-readable (0o644) inconsistent with the project's own established 0o600 standard: command logs (vault secrets in args), HAR captures (Cookie/Authorization headers), and persisted reports (session cookies + HAR). A centralized secret-aware file-write helper plus redaction would close all three.
- Sensitive transports run in cleartext: device-pairing token + certificates over plaintext gRPC on all interfaces (tls-1, the trust-bootstrap for the whole mTLS control plane). Encrypt and pin before sending the bearer token.
- Resource consumption is unbounded for authenticated remote peers: gRPC CreateSession has no concurrent-session cap and the Interactive stream leaks goroutines on stall; mTLS gates WHO can call but never bounds HOW MUCH. Add per-peer quotas and lifecycle-bound goroutines.
- Untrusted local principals are under-isolated: the gops control endpoint is unauthenticated on loopback for every process, and several secret files rely on POSIX mode bits that are weak/cosmetic on the documented primary Windows platform — local cross-user boundaries are an under-considered threat model.

## Quick wins

- Change three world-readable writes to 0o600 (and their dirs to 0o700): internal/logger/logger.go:177, internal/engine/report.go:88/81, cmd/scout/gather.go:91 and internal/engine/knowledge_writer.go:68 — aligns with the project's existing 0o600 standard and closes secret-leakage-1/2/3 with a one-line change each.
- Validate plugin manifest Name in Manifest.validate(): require filepath.IsLocal(m.Name) plus charset ^[A-Za-z0-9._-]+$ (no separators/..) — closes the plugin path-traversal arbitrary-write/RCE (plugin-supply-chain-1) at a single chokepoint.
- Add a bounds check before make([]byte, size) in internal/engine/lib/cdp/websocket.go:175 (reject size < 0 || size > maxFrameBytes; cap the handshake io.ReadAll at :239) — eliminates the remote-CDP OOM DoS with a few lines.
- Move the loopback-without-API-key refusal from the CLI into agent.NewServer/ListenAndServe and add the same guard + an --api-key flag to the MCP SSE path (cmd/scout/mcp.go), reusing isLoopbackHost and the existing constant-time Bearer authMiddleware — makes both servers fail closed by default.
- Fail closed in plugin Client.Start: re-hash CommandPath() against the lock-file checksum and refuse to exec on mismatch/missing, and stop writing an empty checksum in pluginUpdateCmd (plugin.go:515) — activates the already-built, already-tested verification API.
- Tie the Interactive forwarding goroutine to stream.Context().Done() and call unsubscribe on Send error (grpc/server/server_hijack_stream.go:299) — mirrors the correct pattern already used by StreamHijack/StreamEvents in the same file.
- Gate the gops agent behind an opt-in env (SCOUT_DEBUG_GOPS=1) and skip it for daemon/MCP/agent-serve in cmd/scout/scout.go:149 — orphan detection via goprocess.Find() does not need the TCP control endpoint.
- Correct the stale CLAUDE.md tech-stack notes (remove ollama/gin/sqlite/bubbletea, which are absent from the module graph) — documentation hygiene flagged by the clean govulncheck result.

## Findings by severity

| Severity | Count |
|---|---|
| high | 7 |
| medium | 3 |
| low | 6 |
| info | 1 |

## Ranked findings

| # | Severity | Exploitability | Finding | Location | CWE |
|---|---|---|---|---|---|
| 1 | high | plausible | MCP SSE server exposes full browser control (eval/navigate) over HTTP with no authentication and no bind-safety guard | `pkg/scout/mcp/server.go:346` | CWE-306: Missing Authentication for Critical Function |
| 2 | high | plausible | SSRF URL-policy is bypassable via DNS rebinding (validate-then-reresolve TOCTOU) | `pkg/scout/urlpolicy/policy.go:86` | CWE-918: Server-Side Request Forgery (SSRF) / CWE-367: Time-of-check Time-of-use Race Condition |
| 3 | high | plausible | SSRF URL-policy only checks the initial URL; HTTP redirects to internal targets are not re-validated | `pkg/scout/urlpolicy/policy.go:54` | CWE-918: Server-Side Request Forgery (SSRF) |
| 4 | high | plausible | eval / JS-execution tools bypass the SSRF navigate gate entirely (agent + MCP) | `pkg/scout/agent/agent.go:317` | CWE-918: Server-Side Request Forgery (SSRF) |
| 5 | high | plausible | Plugin install: attacker-controlled manifest "name" enables arbitrary file write outside the plugins directory (path traversal) | `cmd/scout/plugin.go:297` | CWE-22: Improper Limitation of a Pathname to a Restricted Directory ('Path Traversal') |
| 6 | high | plausible | Plugin checksum/integrity is never enforced at install or launch (TOFU, empty pinned checksum, no signed index, no downgrade protection) | `cmd/scout/plugin.go:224` | CWE-494: Download of Code Without Integrity Check / CWE-345: Insufficient Verification of Data Authenticity |
| 7 | high | plausible | Device pairing exchanges secret token and certificates over plaintext gRPC on all network interfaces | `cmd/scout/server.go:178` | CWE-319: Cleartext Transmission of Sensitive Information |
| 8 | medium | plausible | gRPC CreateSession launches unbounded browser processes (no concurrent-session cap) | `grpc/server/server_session.go:205` | CWE-770: Allocation of Resources Without Limits or Throttling |
| 9 | medium | plausible | Persisted reports embed raw session cookies and HAR into a world-readable (0o644) file | `internal/engine/report.go:88` | CWE-200: Exposure of Sensitive Information to an Unauthorized Actor |
| 10 | medium | plausible | Command logger writes secret CLI arguments and full stdout/stderr to world-readable JSON logs | `internal/logger/logger.go:177` | CWE-532: Insertion of Sensitive Information into Log File |
| 11 | low | difficult | Agent library API exposes unauthenticated browser JS-eval with no bind-address guard (insecure default at the package boundary) | `pkg/scout/agent/server.go:376` | CWE-306: Missing Authentication for Critical Function / CWE-1188: Insecure Default Initialization |
| 12 | low | plausible | gops diagnostic agent exposes an unauthenticated local control endpoint on every Scout process | `cmd/scout/scout.go:149` | CWE-419: Unprotected Primary Channel |
| 13 | low | plausible | HAR network recordings written world-readable (0o644) despite containing Cookie/Authorization headers | `cmd/scout/gather.go:91` | CWE-312: Cleartext Storage of Sensitive Information |
| 14 | low | plausible | API proxy server has no SSRF policy, binds all interfaces with no auth, and injects caller-controlled params into the target URL | `pkg/scout/proxy/proxy.go:176` | CWE-918: Server-Side Request Forgery (SSRF) |
| 15 | low | plausible | Unbounded memory allocation from attacker-controlled WebSocket frame length in CDP client | `internal/engine/lib/cdp/websocket.go:175` | CWE-789: Memory Allocation with Excessive Size Value / CWE-130: Improper Handling of Length Parameter Inconsistency |
| 16 | low | plausible | gRPC Interactive bidirectional stream leaks an event-forwarding goroutine on stall | `grpc/server/server_hijack_stream.go:299` | CWE-404: Improper Resource Shutdown or Release |
| 17 | info | not-exploitable | govulncheck reports no known-vulnerable dependencies (clean toolchain + module graph) | `go.mod:1` | CWE-1395: Dependency on Vulnerable Third-Party Component |

## Finding details

### 1. [HIGH] MCP SSE server exposes full browser control (eval/navigate) over HTTP with no authentication and no bind-safety guard

- **Location:** `pkg/scout/mcp/server.go:346`
- **CWE:** CWE-306: Missing Authentication for Critical Function
- **Exploitability:** plausible
- **Category:** AuthN/AuthZ on exposed services

**Why it matters:** Once an operator sets a non-loopback --addr, any TCP-reachable client gets unauthenticated arbitrary in-browser JS execution and SSRF with zero credentials — the sibling agent server blocks exactly this, but MCP has no auth, no api-key flag, and no loopback guard.

**Exploit scenario:** Operator runs `scout mcp --sse --addr 0.0.0.0:8080`; any attacker reaching TCP 8080 connects an MCP SSE client and calls eval with arbitrary JS (e.g. fetch('http://169.254.169.254/...')), achieving in-browser RCE and SSRF in the host's security context.

**Recommendation:** Add an APIKey to mcp.ServerConfig and wrap the SSE handler in a constant-time Bearer middleware (reuse pkg/scout/agent/server.go:373-403); add --api-key/SCOUT_AGENT_API_KEY to cmd/scout/mcp.go and refuse non-loopback SSE binds without a key (reuse isLoopbackHost as cmd/scout/agent.go:79-87 does); add an Origin/Host allowlist to defeat DNS-rebinding even on loopback.

### 2. [HIGH] SSRF URL-policy is bypassable via DNS rebinding (validate-then-reresolve TOCTOU)

- **Location:** `pkg/scout/urlpolicy/policy.go:86`
- **CWE:** CWE-918: Server-Side Request Forgery (SSRF) / CWE-367: Time-of-check Time-of-use Race Condition
- **Exploitability:** plausible
- **Category:** SSRF & egress on behalf of host

**Why it matters:** The single SSRF control resolves and validates the hostname, then discards the validated IP and hands the raw hostname to Chrome, which re-resolves independently — an attacker controlling authoritative DNS flips the answer to 169.254.169.254 for the fetch, defeating default-deny for any attacker-owned domain.

**Exploit scenario:** A token-holding agent or MCP model calls navigate with url=http://rebind.attacker.com/; the attacker's DNS returns a public IP to the policy's resolver (passing the check) then 169.254.169.254 with 0 TTL for Chrome's fetch, and the metadata response is read back via extract/markdown, exfiltrating cloud credentials.

**Recommendation:** Pin the validated IP into the fetch via CDP Network.setHostResolverRules or a pinned DialContext, re-validate every IP the connection actually uses, and reject non-deterministic resolutions; cache the validated answer for the request lifetime.

### 3. [HIGH] SSRF URL-policy only checks the initial URL; HTTP redirects to internal targets are not re-validated

- **Location:** `pkg/scout/urlpolicy/policy.go:54`
- **CWE:** CWE-918: Server-Side Request Forgery (SSRF)
- **Exploitability:** plausible
- **Category:** SSRF & egress on behalf of host

**Why it matters:** Policy.Check validates exactly one URL; the browser then transparently follows 3xx redirects to internal/loopback/metadata hosts with no second policy evaluation, so an external URL that 302-redirects internally fully defeats the default-deny control (acknowledged as deferred SSRF-V2 in the project's own BACKLOG).

**Exploit scenario:** Attacker hosts https://evil.example/r returning 302 Location: http://169.254.169.254/latest/meta-data/iam/security-credentials/; the remote/AI caller invokes navigate on the public URL, the browser follows the redirect to metadata, and credentials are read back via markdown/extract/screenshot.

**Recommendation:** Intercept navigations/redirects via CDP Fetch.requestPaused (or Network.requestWillBeSent) and run urlpolicy.Check on every resolved request URL, aborting internal targets; or disable automatic redirect-following on ingress paths and re-validate each Location.

### 4. [HIGH] eval / JS-execution tools bypass the SSRF navigate gate entirely (agent + MCP)

- **Location:** `pkg/scout/agent/agent.go:317`
- **CWE:** CWE-918: Server-Side Request Forgery (SSRF)
- **Exploitability:** plausible
- **Category:** SSRF & egress on behalf of host

**Why it matters:** The SSRF gate is wired only into navigate-family tools; eval (and click/extract) run attacker-supplied JS with no policy.Check, so a caller meant to be confined to public navigation can issue fetch() to internal/metadata hosts directly and read the body back — voiding the default-deny boundary with no DNS tricks, and on MCP with no extra credential.

**Exploit scenario:** A token-holding agent client (or the MCP model) POSTs /call {name:'eval', script:"await fetch('http://169.254.169.254/latest/meta-data/iam/security-credentials/role').then(r=>r.text())"}, reading internal/metadata content back in the tool response despite default-deny.

**Recommendation:** Enforce urlpolicy.Check at the CDP request layer (Fetch.requestPaused) for ALL outbound requests regardless of initiating tool; and/or gate eval/JS-click behind an explicit opt-in flag that documents it voids the SSRF policy, and disable it on non-loopback agent binds.

### 5. [HIGH] Plugin install: attacker-controlled manifest "name" enables arbitrary file write outside the plugins directory (path traversal)

- **Location:** `cmd/scout/plugin.go:297`
- **CWE:** CWE-22: Improper Limitation of a Pathname to a Restricted Directory ('Path Traversal')
- **Exploitability:** plausible
- **Category:** Path traversal / unsafe file ops

**Why it matters:** The plugin install destination is taken verbatim from the remote manifest's name field with no containment check (scouthome.Sub is a bare filepath.Join), so a name like ../../../.config/autostart escapes the sandboxed plugins dir and writes attacker-chosen files to an attacker-chosen host path — yielding persistence/RCE on next login; --checksum is optional so this is unauthenticated TOFU.

**Exploit scenario:** Victim runs `scout plugin install github:attacker/scout-helper` without --checksum; the archive's plugin.json sets name to ../../../.config/autostart and ships a top-level .desktop/script entry, so install copies the attacker's file into the autostart directory, achieving code execution on next login.

**Recommendation:** In Manifest.validate() require filepath.IsLocal(m.Name) plus a strict charset (^[A-Za-z0-9._-]+$, no separators/..); defensively verify the resolved destDir is still within the plugins root via filepath.Rel before the copy loop; consider making --checksum mandatory for url/github installs.

### 6. [HIGH] Plugin checksum/integrity is never enforced at install or launch (TOFU, empty pinned checksum, no signed index, no downgrade protection)

- **Location:** `cmd/scout/plugin.go:224`
- **CWE:** CWE-494: Download of Code Without Integrity Check / CWE-345: Insufficient Verification of Data Authenticity
- **Exploitability:** plausible
- **Category:** Plugin supply chain / integrity verification

**Why it matters:** A fully-built, unit-tested checksum API (registry.VerifyChecksum/FileChecksum, LockFile.Checksum) has zero live callers: --checksum is optional and returns nil when empty, github:/update flows pass no checksum, pluginUpdateCmd writes an EMPTY lock checksum, the registry index carries no sha256/signature, and Client.Start execs the binary with no verification — so a tampered remote/registry/release artifact runs as the local user with no detection possible.

**Exploit scenario:** A compromised CDN/mirror, hijacked GitHub release asset, or attacker-controlled registry index serves a malicious plugin; the victim runs `scout plugin install`/`scout plugin update`, Scout installs it after only a stderr warning and records an empty checksum, and the unverified binary is executed on next use — arbitrary code execution as the local user.

**Recommendation:** Fail closed at launch: re-hash CommandPath() against a populated lock entry in Client.Start and refuse to exec on mismatch/missing. Require a trust anchor (registry-supplied sha256 and ideally a signed/detached signature over the index) for all remote installs instead of warning; persist the real verified checksum (never "") into the lock file; reject lower-semver updates absent --allow-downgrade.

### 7. [HIGH] Device pairing exchanges secret token and certificates over plaintext gRPC on all network interfaces

- **Location:** `cmd/scout/server.go:178`
- **CWE:** CWE-319: Cleartext Transmission of Sensitive Information
- **Exploitability:** plausible
- **Category:** TLS / transport configuration

**Why it matters:** The pairing gRPC server runs with no transport credentials bound to :port+1 (all interfaces); the out-of-band pairing token — the sole gate for the entire mTLS trust enrollment — and both certificates travel in cleartext, so an on-path/same-LAN attacker sniffs the token and enrolls their own cert, gaining full Eval/Navigate control of the host browser. (Finding header cites grpc/server/server.go; the actual code is cmd/scout/server.go:171/178.)

**Exploit scenario:** An attacker on the same Wi-Fi/LAN passively sniffs the plaintext Pair RPC on TCP port+1, recovers the x-pairing-token, then makes their own Pair call with an attacker-generated cert; the server trusts it, and the attacker opens an mTLS session to call Eval/InjectJS and Navigate (SSRF).

**Recommendation:** Run the pairing service over TLS using the daemon's own certificate (tls.Config MinVersion TLS13) and have the client pin the server by expected device ID in VerifyPeerCertificate before sending the token; bind the pairing listener to a specific operator-chosen interface rather than all interfaces; rate-limit/one-shot the pairing window.

### 8. [MEDIUM] gRPC CreateSession launches unbounded browser processes (no concurrent-session cap)

- **Location:** `grpc/server/server_session.go:205`
- **CWE:** CWE-770: Allocation of Resources Without Limits or Throttling
- **Exploitability:** plausible
- **Category:** Resource exhaustion / DoS

**Why it matters:** Each CreateSession RPC forks a real Chrome process and stores it with no per-peer or global cap; the only reaper is the daemon-wide idle timer (never fires under load and cannot reap individual sessions), so a paired-but-compromised mTLS peer can loop CreateSession until host RAM/PIDs/FDs are exhausted, taking down the daemon and host.

**Exploit scenario:** An attacker controlling a paired mTLS device invokes CreateSession in a tight loop; each call forks a real Chrome with no cap, so a few hundred calls exhaust the host's memory and process table and crash the daemon.

**Recommendation:** Enforce a configurable max concurrent sessions and a per-peer/per-deviceID quota in CreateSession, returning codes.ResourceExhausted at the cap; add an idle/TTL reaper that closes individual leaked sessions; default cap (e.g. 16) tunable via config.

### 9. [MEDIUM] Persisted reports embed raw session cookies and HAR into a world-readable (0o644) file

- **Location:** `internal/engine/report.go:88`
- **CWE:** CWE-200: Exposure of Sensitive Information to an Unauthorized Actor
- **Exploitability:** plausible
- **Category:** Secret handling & leakage

**Why it matters:** SaveReport serializes the entire Report struct (including GatherResult.Cookies and HAR with Cookie/Authorization headers) as raw JSON into a 0o644 file in a 0o755 dir, so on a multi-user host another local account reads live session cookies and hijacks the session — inconsistent with the project's own 0o600 standard and with reports being designed for external AI ingestion.

**Exploit scenario:** An analyst runs `scout gather https://portal.example --cookies --har --report`; the generated ~/.scout/reports/<uuid>.txt is world-readable and its Raw Data JSON contains the portal's session cookies, so another local account reads it and hijacks the session.

**Recommendation:** Write report files 0o600 and the reports dir 0o700; strip or mask Cookies and HAR auth headers from the embedded Raw Data JSON before persisting.

### 10. [MEDIUM] Command logger writes secret CLI arguments and full stdout/stderr to world-readable JSON logs

- **Location:** `internal/logger/logger.go:177`
- **CWE:** CWE-532: Insertion of Sensitive Information into Log File
- **Exploitability:** plausible
- **Category:** Secret handling & leakage

**Why it matters:** When logging is enabled, raw positional args and captured stdout/stderr are written to a 0o644 file in a 0o755 dir with no redaction, and the ignored-commands allowlist excludes only `logger`; `scout vault set KEY=VALUE` passes the secret as a positional arg, so the plaintext key lands in a world-readable log on a multi-user host, defeating the encrypted vault's 0o600 protection.

**Exploit scenario:** On a shared host an operator runs `scout logger --path /var/log/scout` then `scout vault set STRIPE_KEY=sk_live_...`; the raw key is written into a 0o644 file under /var/log/scout that any local user can read.

**Recommendation:** Open log files 0o600 and the log dir 0o700; redact KEY=VALUE/secret-flag patterns and skip stdout/stderr capture for secret-bearing commands (vault/scrape/auth/capture-host), or add them to ignoredCommands; prefer stdin/prompt for secrets as vault already does for the passphrase.

### 11. [LOW] Agent library API exposes unauthenticated browser JS-eval with no bind-address guard (insecure default at the package boundary)

- **Location:** `pkg/scout/agent/server.go:376`
- **CWE:** CWE-306: Missing Authentication for Critical Function / CWE-1188: Insecure Default Initialization
- **Exploitability:** difficult
- **Category:** AuthN/AuthZ on exposed services

**Why it matters:** authMiddleware short-circuits when APIKey is empty and agent.NewServer/ListenAndServe perform NO bind-address validation, so a downstream integrator binding a routable interface with no key gets an unauthenticated remote JS-eval/SSRF endpoint; the loopback-without-key refusal lives only in the CLI wrapper, so the library fails OPEN. No first-party path reaches the unsafe state (the shipped CLI is fail-closed), making this a defense-in-depth hardening item.

**Exploit scenario:** A downstream integrator embeds the agent package and calls agent.NewServer(ServerConfig{Addr:"0.0.0.0:9000"}) with no APIKey, assuming it is safe by default; any host on the network then POSTs an eval tool call and runs arbitrary JavaScript in the host's browser.

**Recommendation:** Move the isLoopbackHost-without-APIKey refusal into agent.NewServer/ListenAndServe so the library fails closed (treat empty host/0.0.0.0/[::] as non-loopback and return an error), or make APIKey mandatory for any non-loopback bind; independently, document/gate eval as a privileged tool since it bypasses the navigate SSRF policy.

### 12. [LOW] gops diagnostic agent exposes an unauthenticated local control endpoint on every Scout process

- **Location:** `cmd/scout/scout.go:149`
- **CWE:** CWE-419: Unprotected Primary Channel
- **Exploitability:** plausible
- **Category:** Sensitive data exposure / local privilege boundary

**Why it matters:** agent.Listen is called unconditionally for every subcommand (including the long-lived secret-handling daemon/MCP/agent processes), binding an unauthenticated gops TCP control endpoint on a random loopback port; loopback is not uid-isolated, so any other local user can BinaryDump the executable, force CPU/Trace/GC stalls (DoS), and read goroutine introspection — though HeapProfile emits allocation metadata, not raw secret buffers.

**Exploit scenario:** On a shared host running `scout daemon`, a second unprivileged user scans 127.0.0.1, finds the gops port, sends BinaryDump to exfiltrate the binary and repeatedly sends CPUProfile/Trace/GC signals to stall the daemon — no credentials required.

**Recommendation:** Do not start the gops agent by default; gate it behind an opt-in (e.g. SCOUT_DEBUG_GOPS=1) and never enable it in daemon/MCP/agent-serve. Orphan detection via goprocess.Find() works without the TCP endpoint; alternatively bind a uid-restricted Unix socket.

### 13. [LOW] HAR network recordings written world-readable (0o644) despite containing Cookie/Authorization headers

- **Location:** `cmd/scout/gather.go:91`
- **CWE:** CWE-312: Cleartext Storage of Sensitive Information
- **Exploitability:** plausible
- **Category:** Secret handling & leakage

**Why it matters:** HAR exports capture every request/response header (Cookie, Authorization, Set-Cookie) with no redaction and are persisted 0o644 in at least gather.go:91 and knowledge_writer.go:68, while the project's own session-flush and flow-capture paths use 0o600 — on a multi-user POSIX host another local user copies the .har and replays the session (Windows ACLs largely neutralize this).

**Exploit scenario:** A user runs `scout gather https://app.internal --har --save-har` against an authenticated app; the resulting world-readable .har in the working directory contains the live Cookie/Authorization header, letting another local user copy and replay the session.

**Recommendation:** Write all HAR artifacts 0o600 (gather.go:91, knowledge_writer.go:68, and sibling snapshot/manifest writes); optionally add a Cookie/Authorization/Set-Cookie redaction/allowlist pass to the HAR exporter.

### 14. [LOW] API proxy server has no SSRF policy, binds all interfaces with no auth, and injects caller-controlled params into the target URL

- **Location:** `pkg/scout/proxy/proxy.go:176`
- **CWE:** CWE-918: Server-Side Request Forgery (SSRF)
- **Exploitability:** plausible
- **Category:** SSRF & egress on behalf of host

**Why it matters:** The proxy navigates a browser per request, applies NO urlpolicy check, defaults to binding all interfaces (":8080") with no auth, and substitutes caller-supplied query params into the operator's target template via raw strings.ReplaceAll — so a route that templates a param into the host/scheme position yields unauthenticated remote SSRF to internal/metadata hosts (path-position params do not re-point the authority, contrary to the finding's @169.254 example).

**Exploit scenario:** An operator deploys `scout proxy start` with a host-position route (e.g. target https://{{.host}}/data) on the default :8080; an unauthenticated remote attacker sets host=169.254.169.254 and reads cloud-metadata/internal content back as scraped JSON.

**Recommendation:** Apply urlpolicy.Check to the fully-resolved targetURL before browser.NewPage in scrapeRoute (reuse urlpolicy.FromEnv as agent does); URL-encode substituted params and validate they cannot alter scheme/authority; default the bind to loopback and mirror agent.go's isLoopbackHost guard requiring an explicit non-loopback opt-in plus auth; document the host-position-template route pattern as dangerous.

### 15. [LOW] Unbounded memory allocation from attacker-controlled WebSocket frame length in CDP client

- **Location:** `internal/engine/lib/cdp/websocket.go:175`
- **CWE:** CWE-789: Memory Allocation with Excessive Size Value / CWE-130: Improper Handling of Length Parameter Inconsistency
- **Exploitability:** plausible
- **Category:** Deserialization & input validation

**Why it matters:** The CDP WebSocket reader allocates make([]byte, size) using a frame-declared 64-bit length before reading any payload, with no cap and no >=0 check; a malicious remote/MITM'd CDP endpoint reached via WithRemoteCDP / `scout connect --cdp ws://...` sends one frame declaring ~2^63 bytes to OOM-kill the Scout process. Impact is a self-inflicted single-process DoS against a peer that, by speaking CDP, already controls the driven browser.

**Exploit scenario:** A user runs `scout connect --cdp ws://attacker-cdp-endpoint`; after the handshake the hostile endpoint sends a frame whose 8-byte length field declares ~0x7FFFFFFFFFFFFFFF bytes, and Scout immediately executes make([]byte, hugeSize), exhausting memory and crashing the host process.

**Recommendation:** Add a maxFrameBytes constant and return a protocol error when size < 0 || size > maxFrameBytes before allocating; prefer bounded reads via io.LimitReader/io.CopyN; treat length-accumulation overflow as a protocol error; apply the same cap to the handshake io.ReadAll at websocket.go:239.

### 16. [LOW] gRPC Interactive bidirectional stream leaks an event-forwarding goroutine on stall

- **Location:** `grpc/server/server_hijack_stream.go:299`
- **CWE:** CWE-404: Improper Resource Shutdown or Release
- **Exploitability:** plausible
- **Category:** Resource exhaustion / DoS

**Why it matters:** The Interactive forwarding goroutine exits only on channel close or Send error and does not select on stream.Context().Done() (unlike the sibling StreamHijack/StreamEvents handlers), and unsubscribe runs only when Interactive returns; a paired peer that sends one command then stalls parks a goroutine plus a 256-slot channel per stream, slowly growing daemon memory/goroutine count. Requires an already-trusted mTLS peer and is a slow creep, not a crash primitive.

**Exploit scenario:** A paired device opens many Interactive streams, sends a single command on each to trigger subscription, then stops reading and never closes; each stream parks a forwarding goroutine plus a 256-entry channel, accumulating until goroutines/memory on the daemon are exhausted.

**Recommendation:** Tie the forwarding goroutine's lifetime to stream.Context().Done() (select on ctx.Done() alongside the channel read), call unsubscribe as soon as Send fails, and bound concurrent Interactive/streaming subscriptions per peer.

### 17. [INFO] govulncheck reports no known-vulnerable dependencies (clean toolchain + module graph)

- **Location:** `go.mod:1`
- **CWE:** CWE-1395: Dependency on Vulnerable Third-Party Component
- **Exploitability:** not-exploitable
- **Category:** Dependency CVEs & toolchain

**Why it matters:** Independently reproduced: govulncheck v1.3.0 (DB 2026-06-02) returns zero findings in both source-reachability and module modes across 158 OSV entries; all security-sensitive deps are at patched versions and there are no replace directives masking a vulnerable fork — so there is no dependency-CVE attack path. The only action is correcting stale CLAUDE.md tech-stack notes (ollama/gin/sqlite/bubbletea are absent from the build graph).

**Exploit scenario:** No exploit: govulncheck confirms no reachable or at-version known-vulnerable dependency exists in the build graph across any trust boundary.

**Recommendation:** No CVE action required; keep govulncheck ./... wired into CI (already run via inovacc/workflows) to catch regressions, and correct the stale CLAUDE.md dependency notes.

---

## Appendix — verified findings (raw evidence & verifier verdicts)

### [HIGH] Plugin install: attacker-controlled manifest "name" enables arbitrary file write outside the plugins directory (path traversal)

- **Dimension:** path_traversal | **Location:** `cmd/scout/plugin.go:297` | **CWE:** CWE-22: Improper Limitation of a Pathname to a Restricted Directory ('Path Traversal')
- **Original severity:** high -> **adjusted:** high | **Exploitability:** plausible | **Confidence:** high

**Description:** `scout plugin install <url|github:owner/plugin>` downloads an archive, extracts it into a temp dir, finds the bundled plugin.json, and then copies the extracted files into a destination directory whose name is taken verbatim from the manifest's `name` field. `pluginDestDir(name)` (cmd/scout/plugin.go:297-308) computes the destination as `scouthome.Sub(filepath.Join("plugins", name))`, and `scouthome.Sub` (internal/engine/scouthome/scouthome.go:125-132) performs a bare `filepath.Join(root, subdir)` with no containment check. `plugin.Manifest.validate()` (pkg/scout/plugin/manifest.go:140-182) enforces `filepath.IsLocal` on the `Command` field but performs NO validation on `Name` beyond non-empty. A manifest with `"name": "../../../../home/user/.config/autostart"` therefore makes `destDir` resolve outside the plugins root, after which `installPluginFromDir` copies each archive top-level file via `copyFile(s, filepath.Join(destDir, entry.Name()))` (cmd/scout/plugin.go:179-184). Both the destination directory (via Name) and the written filenames (archive entry names) are attacker-controlled, yielding a write of attacker-chosen content to an attacker-chosen path on the host. The archive extractor itself is zip-slip-protected (pkg/scout/archive: pathSlipCheck + symlinkTargetWithinDest), but that protection only constrains extraction into the temp dir; the subsequent name-driven copy step is entirely unguarded. A `--checksum` is optional (the code only prints a warning when absent, cmd/scout/plugin.go:228-230), so the realistic trust-on-first-use install of a hostile archive is unauthenticated.

**Evidence:**

```
cmd/scout/plugin.go:297-308:
  func pluginDestDir(name string) (string, error) {
      destDir, err := scouthome.Sub(filepath.Join("plugins", name))
      ...
      if err := os.MkdirAll(destDir, 0o755); err != nil { ... }
      return destDir, nil
  }
cmd/scout/plugin.go:179-184 (copy uses escaped destDir):
  s := filepath.Join(srcDir, entry.Name())
  d := filepath.Join(destDir, entry.Name())
  if err := copyFile(s, d); err != nil { ... }
scouthome.go:125-132 (no traversal check):
  return filepath.Join(root, subdir), nil
manifest.go:140-182 validate() guards Command with filepath.IsLocal but never validates Name.
Demonstrated: name "../../../../tmp/evil" with root "/home/user/.local/share/scout" resolves to "/home/user/tmp/evil".
```

**Exploit scenario:** A victim runs `scout plugin install github:attacker/scout-helper` (or any attacker-hosted archive URL) without supplying `--checksum`; the archive's plugin.json sets `name` to `../../../.config/autostart` (Linux) or a startup/Run-key-adjacent path and includes a top-level `evil.desktop`/script entry, so the install copies the attacker's file into the victim's autostart directory, achieving code execution on next login outside the sandboxed plugins folder.

**Recommendation:** Reject non-local plugin names before using them as a path component: in `Manifest.validate()` require `filepath.IsLocal(m.Name)` and additionally restrict Name to a safe charset (e.g. `^[a-zA-Z0-9._-]+$`, no separators, no `..`). Defensively, also harden `pluginDestDir`/`scouthome.Sub` to verify the resolved path is still within the intended parent via `filepath.Rel`/prefix containment, and re-validate the name immediately before the copy loop in `installPluginFromDir`. Consider making `--checksum` mandatory for URL/github installs.

**Verifier reasoning:** Verified the full chain end-to-end by reading the cited code.

SINK (cmd/scout/plugin.go): installPluginFromDir (158-190) calls pluginDestDir(manifest.Name) (164) then copies every top-level archive file via copyFile(filepath.Join(srcDir, entry.Name()), filepath.Join(destDir, entry.Name())) (179-184). pluginDestDir (297-308) = scouthome.Sub(filepath.Join("plugins", name)), and Sub (scouthome.go:125-132) is a bare filepath.Join(root, subdir) with NO containment check. filepath.Join cleans embedded ".." segments, so a manifest name like "../../../.config/autostart" resolves outside the plugins root. copyFile (556-579) opens dst with O_CREATE|O_WRONLY|O_TRUNC and io.Copy — an unconditional content write of attacker bytes. Confirmed: both the destination directory (Name) and the filenames (archive entry names) are attacker-controlled.

VALIDATION GAP (manifest.go): validate() (140-182) enforces filepath.IsLocal only on Command (156); Name is checked solely for non-empty (141-143). Grep confirmed no IsLocal/Rel/Clean/prefix-containment/charset check on Name anywhere in plugin.go or manifest.go. The archive extractor's zip-slip guard only constrains extraction into the temp dir; the subsequent name-driven copy is unguarded — accurate.

TRUST BOUNDARY: The manifest is remote-attacker content, not local-user content. installPluginFromURL (194-253) downloads an arbitrary URL, and installPluginFromGitHub (312-343) builds an attacker repo's release URL from github:owner/plugin, then both feed the REMOTE plugin.json to installPluginFromDir. The victim only chooses which plugin to install; the name field is authored by the remote archive. --checksum is optional — verifyPluginChecksum returns nil on empty (262-265) and the code merely prints an UNVERIFIED warning (228-230) — so remote content is unauthenticated TOFU by default. The user's reasonable expectation is "install into the sandboxed ~/.scout/plugins dir," but the write escapes that boundary to an arbitrary host path. This is a genuine cross-boundary flow, not a local-only intended capability.

IMPACT: Arbitrary file write of attacker-chosen content to an attacker-chosen path at the invoking user's privilege → persistence/RCE via autostart/.bashrc/cron/Run-key on next login.

Not refuted. Severity stays High: outcome is host code execution and the remote content is unverified by default. Exploitability is "plausible" rather than "trivial" only because it requires the victim to run `scout plugin install` against an attacker source — but that is the documented, headline marketplace/github install flow and is the expected supply-chain action, so the bar is low. Recommended fix is correct: require filepath.IsLocal(m.Name) plus a strict charset (^[A-Za-z0-9._-]+$, reject separators/..) in validate(), and defensively verify the resolved destDir is still within the plugins root via filepath.Rel before the copy loop.

**Verifier notes:** Live secondary instance of the same primitive: pluginRemoveCmd (350-374) and pluginRunCmd also pass user/manifest names straight into scouthome.Sub without containment, though remove is local-CLI-arg-controlled (lower priority). The single highest-leverage fix is centralizing name validation in Manifest.validate() so every install path inherits it, plus a Rel/prefix check in pluginDestDir as defense-in-depth. Consider making --checksum mandatory for url/github installs.

---

### [HIGH] SSRF URL-policy is bypassable via DNS rebinding (validate-then-reresolve TOCTOU)

- **Dimension:** ssrf | **Location:** `pkg/scout/urlpolicy/policy.go:86` | **CWE:** CWE-918: Server-Side Request Forgery (SSRF) / CWE-367: Time-of-check Time-of-use Race Condition
- **Original severity:** high -> **adjusted:** high | **Exploitability:** plausible | **Confidence:** high

**Description:** urlpolicy.Policy.Check is the single SSRF control protecting the network-exposed agent REST server and the MCP server from being abused to reach localhost / 169.254.169.254 (cloud metadata) / RFC1918 hosts. Check() resolves the hostname itself (p.resolve -> net.DefaultResolver) and rejects the request if any resolved IP is internal. It then returns nil and the caller hands the ORIGINAL hostname string to the browser (tools.Navigate -> p.Navigate(in.URL); agent ensurePage -> browser.NewPage(url)). Chrome performs its OWN, independent DNS resolution when it actually fetches the URL. No validated IP is pinned and passed to the fetch. An attacker who controls the authoritative DNS for their hostname returns a public IP on the policy's lookup and a private/metadata IP (e.g. 169.254.169.254) on the browser's lookup a fraction of a second later. The default-deny posture is therefore defeated for any attacker-controlled domain.

**Evidence:**

```
policy.go:86 `ips, err := p.resolve(ctx, host)` then policy.go:91-98 validates those ips, returns nil; pkg/scout/tools/page.go:43 `if err := p.Navigate(in.URL); err != nil` re-uses the raw URL string (and Chrome re-resolves). agent.go:216 `p.policy.Check(ctx, url)` then agent.go:221 `p.getPage(ctx, url)` -> ensurePage -> `p.browser.NewPage(url)` (agent.go:194). There is no IP pinning between check and fetch.
```

**Exploit scenario:** An operator runs `scout agent serve --addr 0.0.0.0:9000 --api-key K` (or MCP behind an AI host). The attacker (token holder / the AI model) calls navigate with url=http://rebind.attacker.com/. attacker.com's DNS returns 93.184.x.x for the policy's resolver, passing the internal-IP check, then flips to 169.254.169.254 with a 0-second TTL for Chrome's fetch; the agent then reads the AWS/GCP metadata response via the extract/markdown/page_url tools, exfiltrating cloud credentials.

**Recommendation:** Pin the resolution: resolve the host once in Check(), and force the browser/HTTP client to connect to the exact validated IP (e.g. inject a host->IP override into Chrome via CDP Network.setHostResolverRules, or dial via a custom net.Resolver/DialContext that reuses the already-validated address). Re-validate every IP the connection actually uses. Reject hostnames whose resolution is non-deterministic. Set a minimum DNS TTL and cache the validated answer for the lifetime of the request.

**Verifier reasoning:** Confirmed by reading the cited code. urlpolicy.Policy.Check (pkg/scout/urlpolicy/policy.go:54-101) resolves the hostname via net.DefaultResolver (line 86), validates each resolved IP with isInternalIP (lines 91-98), then returns nil and DISCARDS the validated IPs. The caller hands the original hostname string to Chrome, which re-resolves independently at fetch time: agent path agent.go:216 Check -> agent.go:221 getPage -> ensurePage agent.go:194 browser.NewPage(url); MCP path server.go:115; tools/page.go:43 p.Navigate(in.URL). I grepped the navigation path for setHostResolverRules / host-resolver-rules / IP-pinning DialContext and found NONE (the only DialContext hits are the CDP websocket to Chrome itself), so no validated IP is ever pinned to the connection. This is a textbook validate-then-reresolve TOCTOU / DNS-rebinding bypass.\n\nThis crosses a genuine trust boundary: the policy docstring (policy.go:1-3) states it exists to gate untrusted ingress at the MCP server and the network-exposed agent REST API. The agent server can be bound non-loopback; the CLI permits --addr 0.0.0.0:9000 provided --api-key is set (cmd/scout/agent.go:77-85). The attacker is a remote token-holder (agent) or the connected AI model (MCP) — exactly the untrusted party the default-deny control is meant to contain. With attacker-controlled low/zero-TTL authoritative DNS returning a public IP to the policy and 169.254.169.254 / 127.0.0.1 / RFC1918 to Chrome, the internal-IP check passes and Chrome fetches the internal/metadata target; the agent then returns the body via extract/markdown/html/page_url, yielding cloud-credential exfiltration.\n\nI keep High severity (cloud metadata / internal service access). Exploitability is plausible rather than trivial: it requires (a) already holding the API key or being the MCP model, and (b) winning a DNS-rebinding race against Chrome's internal DNS cache and any resolver pinning — well-demonstrated in the wild but not push-button. Requiring auth does not negate the finding, because the SSRF policy's explicit purpose is to stop even authorized callers from reaching internal targets. The recommended fix (pin the validated IP into the fetch via Network.setHostResolverRules / a pinned DialContext and re-validate the connected IP) is correct.

**Verifier notes:** No IP pinning exists on any navigation path. Same gap affects MCP (server.go:115) and the internal crawl/sitemap path (internal/engine/crawl.go:348), not just the agent server. The recent CHANGELOG hardening only fixed the AllowLocal scheme-gate ordering, not the rebinding TOCTOU. Fix must pin the resolved IP for the actual fetch and re-validate it.

---

### [HIGH] SSRF URL-policy only checks the initial URL; HTTP redirects to internal targets are not re-validated

- **Dimension:** ssrf | **Location:** `pkg/scout/urlpolicy/policy.go:54` | **CWE:** CWE-918: Server-Side Request Forgery (SSRF)
- **Original severity:** high -> **adjusted:** high | **Exploitability:** plausible | **Confidence:** high

**Description:** Policy.Check validates exactly one URL string. Once it passes, the browser navigates and transparently follows HTTP 3xx redirects to wherever the server points, including internal/loopback/metadata hosts, with no second policy evaluation. The check is performed in the agent/MCP handler before navigation; nothing re-inspects the post-redirect location, and Scout has no redirect interception hooked to urlpolicy on these ingress paths.

**Evidence:**

```
policy.go:54 `func (p Policy) Check(ctx context.Context, rawURL string) error` operates on a single rawURL. tools_browser.go:27 calls state.checkURL once, then tools.Navigate (pkg/scout/tools/page.go:43 `p.Navigate(in.URL)`) lets Chrome follow redirects unchecked. agent.go:215-221 likewise checks once then NewPage navigates.
```

**Exploit scenario:** Attacker hosts https://evil.example/r which returns `302 Location: http://169.254.169.254/latest/meta-data/iam/security-credentials/`. The remote/AI caller invokes navigate(url=https://evil.example/r); the public host passes the policy, the browser follows the redirect to the cloud metadata endpoint, and the caller reads the credentials back via the markdown/extract/screenshot tools.

**Recommendation:** Intercept navigations/redirects (e.g. via a HijackRequests/Fetch.requestPaused router or CDP Network.requestWillBeSent) and run urlpolicy.Check on every resolved request URL, aborting any that target internal ranges. Alternatively disable automatic redirect following on these ingress paths and re-validate each Location before continuing.

**Verifier reasoning:** Verified every link in the cited chain against the source. urlpolicy.Policy.Check (policy.go:54) validates exactly one URL string by resolving its host and blocking internal IP ranges — it has no redirect awareness by construction. MCP navigate (tools_browser.go:27) calls state.checkURL once, then tools.Navigate -> p.Navigate(in.URL) (page.go:43); the agent handleNavigate (agent.go:215-219) calls p.policy.Check once, then getPage navigates. In both ingress paths Chrome transparently follows HTTP 3xx, and nothing re-runs urlpolicy on the post-redirect (server-chosen) Location, nor on Chrome's fresh DNS resolution of the redirect target. No CDP request interception (Fetch.requestPaused / Network.requestWillBeSent) is wired to urlpolicy on these paths.

This crosses a real trust boundary: urlpolicy's package comment states it gates untrusted MCP/agent ingress (remote AI/network callers via SSE MCP transport or the agent REST API), block-by-default for internal/loopback/link-local ranges. An attacker who controls only an external URL (which passes the single pre-check) can return 302 Location: http://169.254.169.254/... and steer the browser at cloud metadata / loopback / RFC1918 services; the result is read back via markdown/extract/screenshot/html tools. The attacker never has to name the internal IP, so the control is fully defeated.

Refutations fail: this is not merely a local-CLI capability (the library/CLI navigate path is intentionally unguarded, but the MCP/agent paths are the protected untrusted surface); it is not already mitigated (no second check, no redirect-disable, no interception); and the agent API key is optional, with these endpoints being the intended untrusted ingress the policy must protect. The codebase's own docs corroborate it as an accepted, deferred gap: BACKLOG.md:24-31 "SSRF-V2" lists 'Redirect-to-internal: re-validate the landed URL or disable redirect-following for guarded navigations,' and the design spec (docs/superpowers/specs/2026-06-03-ssrf-url-policy-design.md) and CHANGELOG note v1 validates only the top-level target.

Severity: the metadata-credential-theft impact is severe but conditional on (a) the operator exposing MCP-SSE/agent to an untrusted caller and (b) an environment with a sensitive internal/metadata endpoint — both are the intended deployment model of these servers, so high is justified, though it sits near the medium/high boundary. Exploitability is plausible rather than trivial: it needs an attacker-controlled redirect server plus an exposed ingress plus a worthwhile internal target. Recommended fix (CDP request interception re-running urlpolicy.Check on every resolved request/redirect URL, or disabling automatic redirect-follow and re-validating each Location) is correct.

Relevant files: D:\weaver-sync\development\personal\projects\scout\pkg\scout\urlpolicy\policy.go (Check, line 54); D:\weaver-sync\development\personal\projects\scout\pkg\scout\tools\page.go (Navigate, line 43); D:\weaver-sync\development\personal\projects\scout\pkg\scout\mcp\tools_browser.go (line 27); D:\weaver-sync\development\personal\projects\scout\pkg\scout\agent\agent.go (lines 215-219); D:\weaver-sync\development\personal\projects\scout\docs\BACKLOG.md (lines 24-31, documented as deferred SSRF-V2).

**Verifier notes:** Author-described impact and fix are accurate. The gap is explicitly acknowledged in the project's own SSRF-V2 backlog as deferred-out-of-scope for urlpolicy v1, which strengthens (not weakens) the finding — it is a known unmitigated control bypass. Same class of issue also applies to in-page subresources (fetch/XHR to internal IPs) and crawler-discovered links per BACKLOG.md:29-30, and to DNS rebinding (BACKLOG.md:31), all of which share the root cause that the policy only inspects the seed URL.

---

### [HIGH] eval / JS-execution tools bypass the SSRF navigate gate entirely (agent + MCP)

- **Dimension:** ssrf | **Location:** `pkg/scout/agent/agent.go:317` | **CWE:** CWE-918: Server-Side Request Forgery (SSRF)
- **Original severity:** high -> **adjusted:** high | **Exploitability:** plausible | **Confidence:** high

**Description:** The SSRF URL-policy is enforced only on the navigate-style tools. The eval tool (agent handleEval, MCP eval) runs caller-supplied JavaScript in the page with no checkURL/policy.Check at all. A remote token holder (agent) or AI driver (MCP) can drive the host browser to internal targets without ever calling navigate: e.g. set `location.href='http://169.254.169.254/...'`, open an iframe/window to an internal URL, or issue fetch()/XHR. The result is then exfiltrated through the eval return value or via subsequent extract/markdown/page_url/page_title/screenshot tools. This makes the urlpolicy gate (findings 1-2) easy to sidestep even without DNS tricks. The agent eval handler is reachable by any caller holding the API key on a non-loopback bind, and by the AI model on every MCP deployment.

**Evidence:**

```
agent.go:317-331 handleEval: `script, _ := args["script"].(string)` then `pg.Eval(script)` with no policy.Check. mcp/tools_browser.go:122-145 the `eval` tool calls tools.Eval with no state.checkURL (compare tools_browser.go:27 where navigate does call checkURL). agent.go:269 handleClick and agent.go:253 handleExtractText similarly Eval arbitrary JS with no policy.
```

**Exploit scenario:** Operator runs the agent with an API key on a shared host. The token-holding client POSTs /call {name:'eval', arguments:{script:"await fetch('http://169.254.169.254/latest/meta-data/iam/security-credentials/role').then(r=>r.text())"}} (or sets location.href to an internal admin panel and then calls markdown), reading internal/metadata content back in the tool response despite the default-deny SSRF policy.

**Recommendation:** Treat eval as a privileged capability: gate it behind an explicit opt-in flag separate from navigation, and document that enabling it voids the SSRF policy. Where the SSRF boundary must hold, intercept all network requests at the CDP layer (Fetch.requestPaused) and apply urlpolicy.Check to every outbound request regardless of which tool initiated it, rather than only at the navigate API. Consider disabling eval/click-via-JS on network-exposed (non-loopback) agent deployments.

**Verifier reasoning:** Verified against source. The codebase deliberately builds a default-deny SSRF egress boundary (`pkg/scout/urlpolicy`), whose own package doc states it is "enforced at untrusted ingress (the MCP server and agent REST API)." The gate (`state.checkURL` / `p.policy.Check`) is wired into navigate-family tools only: tools_browser.go:27 (navigate), tools_form.go:33, tools_crawl.go:36, tools_gather.go:45, tools_session.go:70, tools_sitemap.go:40, tools_swarm.go:38, and agent.go:215 (handleNavigate). It is NOT applied to eval, click, extract, type, html, or markdown.

The eval path runs fully attacker-controlled JavaScript with no policy check: agent.go:317-331 (`handleEval` -> pg.Eval(script)) and tools_browser.go:122-145 -> tools.Eval -> page.go:223 `p.Eval(in.Expression)`. There is no CDP-layer request interception (no Fetch.requestPaused) applying the policy to outbound requests, so the gate exists only at the navigate API surface. A caller can therefore issue `eval` with `fetch('http://169.254.169.254/latest/meta-data/...')` (or set location.href to an internal host) and read the response back through the eval return value or a follow-up markdown/extract/page_url call, completely sidestepping the default-deny policy that findings 1-2 rely on. handleClick (agent.go:269) and handleExtractText (agent.go:253) likewise inject arbitrary JS, though eval is the cleanest vector.

Cross-boundary reachability holds: (1) MCP — the AI model is exactly the untrusted driver the urlpolicy is designed to confine; it can call eval with no additional credential, so this is a clean bypass of the intended boundary on every MCP deployment. (2) Agent HTTP — on non-loopback binds an API key is mandatory (cmd/scout/agent.go:79-87 refuses non-loopback without auth; authMiddleware enforces a constant-time Bearer check), so a network attacker without the key cannot reach eval. But a token holder who is meant to be confined to public navigation by the default-deny policy still escapes that confinement via eval, which is the finding's scoped claim.

Not refuted: the input genuinely crosses a trust boundary (MCP model / API-key client) and the impact (internal-network and cloud-metadata SSRF + exfil) matches the claim. The eval tool being "arbitrary JS by design" does not excuse it, because the codebase simultaneously asserts an SSRF egress boundary at this exact ingress; eval silently voids that boundary with no opt-in flag or documentation. Severity stays high (defeats a purpose-built security control; MCP path needs no extra auth). Exploitability set to plausible rather than trivial because the realistic network-exposed agent case is gated behind the API key, and the MCP case presupposes a malicious/prompt-injected model — the documented threat the policy guards against. Recommended fix is correct: enforce urlpolicy.Check at the CDP request layer (Fetch.requestPaused) regardless of initiating tool, and/or gate eval/JS-click behind an explicit opt-in that documents it voids the SSRF policy.</parameter>
<parameter name="notes">Key files: pkg/scout/agent/agent.go:317 (handleEval, no policy), agent.go:215 (navigate gates), pkg/scout/mcp/tools_browser.go:122-145 (eval, no checkURL) vs :27 (navigate gates), pkg/scout/tools/page.go:223 (tools.Eval -> p.Eval), pkg/scout/urlpolicy/policy.go (boundary doc + default-deny), cmd/scout/agent.go:79-87 (non-loopback requires API key), pkg/scout/agent/server.go:373-403 (authMiddleware). Gate present on form/crawl/gather/session/sitemap/swarm but absent on eval/click/extract/type/html/markdown. Fix: apply urlpolicy at CDP Fetch.requestPaused layer rather than only the navigate API.</parameter>
</invoke>


---

### [HIGH] MCP SSE server exposes full browser control (eval/navigate) over HTTP with no authentication and no bind-safety guard

- **Dimension:** authnz | **Location:** `pkg/scout/mcp/server.go:346` | **CWE:** CWE-306: Missing Authentication for Critical Function
- **Original severity:** high -> **adjusted:** high | **Exploitability:** plausible | **Confidence:** high

**Description:** ServeSSE() stands up an HTTP server whose handler is the raw mcp.NewSSEHandler with no authentication layer of any kind: no API key, no Bearer token, no Origin allowlist, no DNS-rebinding/Host check. The SDK's NewSSEHandler is a pure transport and enforces no auth. Every MCP tool — including `eval` (arbitrary JavaScript via CDP Runtime.evaluate), `navigate`/`browse_url` (SSRF on behalf of the host), `click`, `type`, `screenshot`, `pdf`, `session_reset`, `swarm_crawl` — is therefore reachable by any client that can open a TCP connection to the listen address. This is the same network-exposed browser-control surface that the sibling agent HTTP server protects with both an authMiddleware (Bearer token, constant-time compare) and a CLI guard that refuses to bind a non-loopback address without an API key (cmd/scout/agent.go:79-87). The MCP SSE path has neither. The default --addr is `localhost:8080` (loopback), which is safe in the default configuration, but `--addr` is a documented, supported flag (cmd/scout/mcp.go:130) and binding `0.0.0.0:8080`/`:8080` produces a fully unauthenticated remote-code-execution-via-eval endpoint with no warning and no opt-in auth available.

**Evidence:**

```
pkg/scout/mcp/server.go:346-353:
  handler := mcp.NewSSEHandler(func(r *http.Request) *mcp.Server {
      return NewServer(cfg, cancel)
  }, nil)
  srv := &http.Server{
      Addr:    addr,
      Handler: handler,   // <-- no auth/origin middleware wrapping the handler
  }
Contrast pkg/scout/agent/server.go:422 (agent server wraps every route): Handler: securityHeaders(s.corsMiddleware(s.authMiddleware(s.rateLimitMiddleware(s.mux)))). The MCP CLI (cmd/scout/mcp.go) exposes no --api-key flag and performs no isLoopbackHost() check on --addr, unlike cmd/scout/agent.go:79-87 which returns an error when binding a non-loopback address with no API key.
```

**Exploit scenario:** An operator runs `scout mcp --sse --addr 0.0.0.0:8080` (or in a container/host with the port published) to let a remote AI host reach it. Any attacker who can reach TCP 8080 connects an MCP SSE client and calls the `eval` tool with `fetch('http://169.254.169.254/latest/meta-data/...')` or arbitrary JS, and the `navigate` tool against internal services — achieving SSRF and arbitrary in-browser code execution in the host's security context with zero credentials.

**Recommendation:** Add authentication to ServeSSE equivalent to the agent server: accept an APIKey in ServerConfig and wrap the SSE handler in an http.Handler that enforces a constant-time Bearer-token check (reuse the pattern in pkg/scout/agent/server.go:373-403). In cmd/scout/mcp.go, add a --api-key flag (and SCOUT_AGENT_API_KEY-style env fallback) and refuse to start the SSE transport on a non-loopback address when no API key is set, mirroring cmd/scout/agent.go:79-87 (isLoopbackHost guard). Additionally set an Origin/Host allowlist on the SSE handler to defeat DNS-rebinding from a victim browser even on loopback binds.

**Verifier reasoning:** Verified directly in source. pkg/scout/mcp/server.go:346-353: ServeSSE wraps the raw mcp.NewSSEHandler with no middleware — no API key, no Bearer check, no Origin/Host allowlist; ServerConfig has no APIKey field. cmd/scout/mcp.go: grep for api-key/Authorization/isLoopbackHost/Origin returned NO matches, and --addr (line 130) is passed straight to ServeSSE (line 118) with no loopback guard. This is the codebase's own security bar inverted: the sibling agent server enforces both a constant-time Bearer authMiddleware (pkg/scout/agent/server.go:373-403, wired at line 422) AND a CLI guard refusing non-loopback binds without an API key (cmd/scout/agent.go:79-87), whose inline comment explicitly cites browser eval/navigation/SSRF exposure. The MCP SSE path exposes the same eval tool (tools_browser.go:123-145 -> tools/page.go:232 p.Eval, arbitrary JS via CDP) with none of those protections. Trust boundary is crossed when an operator uses the documented --addr 0.0.0.0:8080 flag or publishes the container port; any TCP-reachable client then gets unauthenticated arbitrary in-browser JS execution. One refinement: the navigate-based SSRF is partially mitigated by a block-by-default URL policy (SCOUT_ALLOW_LOCAL_TARGETS off by default, confirmed via ssrf_tools_test.go), but eval runs arbitrary JS in the renderer where fetch() to 169.254.169.254 is NOT subject to Scout's Go-side policy, so the RCE-in-browser-context and SSRF-via-eval claims hold. Not Critical because the default bind is loopback (safe out of the box) and exploitation requires the operator to choose a non-loopback --addr — but the tool offers no warning, no opt-in auth, and no guard, exactly the condition the project deemed worth blocking elsewhere. Exploitability is plausible (not trivial) because it depends on that one operator misconfiguration; once present, exploitation is trivial and credential-free.

**Verifier notes:** Fix is concrete and mirrors existing code: add APIKey to mcp.ServerConfig, wrap the SSE handler in a constant-time Bearer middleware reusing pkg/scout/agent/server.go:373-403, add --api-key/SCOUT_AGENT_API_KEY to cmd/scout/mcp.go, and reuse cmd/scout/server.go:35 isLoopbackHost to refuse non-loopback SSE binds without a key (as cmd/scout/agent.go:79-87 already does). Also add an Origin/Host allowlist on the SSE handler to defeat DNS-rebinding even on loopback. Note for reporter: the navigate SSRF leg is dampened by the default-on URL policy, but eval-based SSRF/RCE is not — keep the eval emphasis.

---

### [HIGH] Device pairing exchanges secret token and certificates over plaintext gRPC on all network interfaces

- **Dimension:** tls | **Location:** `grpc/server/server.go:178` | **CWE:** CWE-319: Cleartext Transmission of Sensitive Information
- **Original severity:** high -> **adjusted:** high | **Exploitability:** plausible | **Confidence:** high

**Description:** When the gRPC daemon is started in the default (mTLS) mode, a second "pairing" gRPC server is created with `grpc.NewServer()` and NO transport credentials (plaintext), then bound to `pairingAddr = fmt.Sprintf(":%d", port+1)` — an empty host, i.e. ALL interfaces, reachable from the LAN. The out-of-band pairing token is the sole secret that gates trust enrollment (grpc/server/pairing.go:96-99), yet it is transmitted as cleartext gRPC metadata over this unencrypted channel: the client dials with `insecure.NewCredentials()` and attaches the token via `metadata.AppendToOutgoingContext(ctx, server.PairingTokenMetadataKey, pairingToken)` (cmd/scout/device.go:186-201). Because the channel has no TLS, the token, the client's certificate DER, and the server's certificate DER all travel in plaintext. Pairing is the single trust-bootstrap step for the entire mTLS control plane (Eval/InjectJS = arbitrary host JS execution, Navigate = SSRF on behalf of the host).

**Evidence:**

```
grpc/server/server.go:178 `pairingGRPC = grpc.NewServer()` (no grpc.Creds); server.go:171 `pairingAddr = fmt.Sprintf(":%d", port+1)` (binds all interfaces); cmd/scout/device.go:187 `grpc.WithTransportCredentials(insecure.NewCredentials())` and device.go:201 `metadata.AppendToOutgoingContext(ctx, server.PairingTokenMetadataKey, pairingToken)`; pairing.go:97 the token is the only gate before `trustStore.Trust(derivedID, req.GetCertDer())`.
```

**Exploit scenario:** An attacker on the same LAN segment (or any on-path position between two devices being paired, e.g. shared Wi-Fi or a compromised switch) passively sniffs the plaintext pairing RPC on TCP port+1, recovers the cleartext `x-pairing-token`, then immediately makes their own `Pair` call presenting that token plus an attacker-generated certificate; the server stores the attacker cert in its trust store, and the attacker can now open an mTLS session to the main daemon and call Eval/InjectJS to execute arbitrary JavaScript in the host's browser and Navigate to internal URLs (SSRF). The 160-bit token's entropy is irrelevant once it is observed in transit.

**Recommendation:** Run the pairing service over TLS using the daemon's own certificate (e.g. a server-only `tls.Config` with `MinVersion: tls.VersionTLS13` so the token and cert exchange are encrypted), and have the client pin the server by its expected device ID (the `--server-id` value) in a `VerifyPeerCertificate` callback before sending the token — mirroring `ClientTLSCredentials` in tls.go. Additionally bind the pairing listener to a specific operator-chosen interface rather than all interfaces (`:port+1`), and rate-limit / one-shot the pairing window. This converts the token into a true second factor rather than a sniffable bearer secret.

**Verifier reasoning:** Confirmed in code (actual location is cmd/scout/server.go, not grpc/server/server.go as the header states; all cited line numbers/constructs match there). In default mTLS mode the daemon starts a SECOND gRPC server with grpc.NewServer() and NO grpc.Creds (plaintext) bound to pairingAddr=":port+1" — empty host = all interfaces, LAN-reachable (cmd/scout/server.go:171,178). Unlike --insecure mode, which is forced to 127.0.0.1 (line 58-59), the pairing listener is intentionally exposed to the LAN. The client dials with insecure.NewCredentials() and sends the secret as x-pairing-token metadata in cleartext (cmd/scout/device.go:187,201). On the server, that token is the sole gate before trust enrollment: after a constant-time compare, the server parses the client-supplied cert DER, checks only that DeviceIDFromCert(cert)==req.DeviceId (a self-consistency check any attacker satisfies with their own cert), then trustStore.Trust() (pairing.go:92-127). An enrolled cert grants full mTLS control plane (Eval/InjectJS = arbitrary JS in the host browser, Navigate = SSRF). CWE-319 is correct: token, client cert DER, and server cert DER all traverse plaintext. The 160-bit entropy and constant-time compare do nothing against a passive on-path observer. Additionally, the client sends the token BEFORE validating the server's identity against --server-id (device.go:203 then :214), so an active rogue pairing endpoint also harvests the token. I could not refute it: the path is not gated by any other auth (pairing IS the bootstrap), the listener is not loopback-restricted, and the token is reusable with no one-shot/rate-limit/pairing-window. Severity held at High rather than Critical because exploitation requires an on-path/same-LAN position during the brief, operator-initiated pairing window (an occasional deliberate action, not continuous traffic); an active attacker on shared Wi-Fi/a switch can ARP-spoof to guarantee position, so it is well within reach. Recommendation (server-side TLS using the daemon cert + client-side VerifyPeerCertificate pinning the expected device ID before sending the token, plus binding to a specific interface and one-shot/rate-limiting) is sound.

**Verifier notes:** Finding's file path is wrong (says grpc/server/server.go:178; real code is cmd/scout/server.go:171/178). Pairing server lives in pkg/grpc and is wired only when !insecureMode. Worth flagging to the author so the report cites the correct file. Everything else (line numbers within the wrong file are coincidentally close to the right file's, and all code constructs) checks out.

---

### [HIGH] GitHub/registry plugin install and update have no integrity anchor (TOFU with empty checksum, version downgrade)

- **Dimension:** plugin_supply | **Location:** `cmd/scout/plugin.go:515` | **CWE:** CWE-345: Insufficient Verification of Data Authenticity
- **Original severity:** high -> **adjusted:** high | **Exploitability:** plausible | **Confidence:** high

**Description:** The registry-driven install/update flows provide no way to verify a downloaded plugin binary. `installPluginFromGitHub` (plugin.go:312-343) builds a `releases/latest/download/...` URL and calls `installPluginFromURL` with no checksum, so the `--checksum` flag is effectively unusable for `github:` installs (the user cannot know the SHA of a `latest` asset that changes over time). `pluginUpdateCmd` is worse: it downloads `registry.LatestReleaseURL(info.Repo, p.Name)` and then records the lock entry with an EMPTY checksum string: `lf.Lock(p.Name, info.Latest, "", info.Repo)` (plugin.go:515). The registry `Index`/`PluginInfo` structs (registry.go:20-34) carry no `checksum` or `signature` field at all, and `FetchIndex` (registry.go:54-85) fetches the index JSON over plain `http.Get` with no signature/integrity check on the index content. Consequently the entire registry->release->install pipeline is trust-on-first-use against whatever the registry index and GitHub return, and `info.Latest` is trusted blindly with no downgrade protection (an attacker who controls the index response can name any tag/repo, and `LatestReleaseURL`/update will fetch+run it unverified).

**Evidence:**

```
plugin.go:509-515: `url := registry.LatestReleaseURL(info.Repo, p.Name); if err := installPluginFromURL(cmd, url); err != nil { ...; continue }; lf.Lock(p.Name, info.Latest, "", info.Repo)` — note the empty `""` checksum argument.  |  plugin.go:337-342: github install builds `https://github.com/%s/%s/releases/latest/download/...` and calls `installPluginFromURL` with no checksum source.  |  registry.go:27-34: `PluginInfo` has Name/Description/Author/Repo/Latest/Tags — no Checksum/Signature.  |  registry.go:59: `resp, err := http.Get(url) //nolint:gosec,noctx` for the index, no signature verification.
```

**Exploit scenario:** An attacker compromises (or MITMs without cert pinning, or wins a path collision on) the GitHub-hosted `registry.json` or a plugin's release pipeline (boundary 4). When the victim runs `scout subplugin update` or `scout subplugin install github:owner/plugin`, Scout downloads the attacker-supplied release asset, extracts it, records a lock entry with an empty checksum, and on next use spawns the malicious binary — full code execution as the local user. Because no prior good checksum is pinned (empty string) and the index carries no signed digest, neither TOFU pinning nor downgrade detection can ever trigger.

**Recommendation:** Add a signed/per-asset checksum to the registry index: extend `PluginInfo` (or a per-release entry) with a `sha256` (and ideally a detached signature / minisign / cosign signature over the index). After download in `installPluginFromURL`, verify the archive bytes against the registry-supplied digest; for `github:`/update flows pass that digest through instead of an empty string. Persist the verified binary checksum into the lock file (never an empty string) and reject updates whose semver is lower than the installed version unless `--allow-downgrade` is given. Pin or verify the index transport (HTTPS is default but add signature verification so a compromised host cannot serve a malicious index).

**Verifier reasoning:** I re-read all cited code and confirmed every claim. cmd/scout/plugin.go:515 does record an EMPTY checksum: `lf.Lock(p.Name, info.Latest, "", info.Repo)`. installPluginFromGitHub (plugin.go:312-343) builds a `releases/latest/download/...` URL and calls installPluginFromURL with no checksum, and because `latest` is a moving target the `--checksum` flag (plugin.go:40) is effectively unusable for `github:`/update flows — installPluginFromURL only verifies when a checksum is passed, otherwise prints a warning and proceeds (plugin.go:222-230). registry.PluginInfo/Index (registry.go:20-34) carry NO checksum/signature field; FetchIndex (registry.go:59) does a plain http.Get of the index with no signature/integrity check on content. There is no signature/minisign/cosign verification anywhere in the registry package (grep returned nothing). Update freshness is plain string inequality `entry.Latest != installed.Version` (update.go:62) — no semver, so any attacker-named tag (including a downgrade to a known-vulnerable version) is treated as an update and installed. The downloaded plugin's Command is then executed as a subprocess via exec.CommandContext (client.go:107), so a tampered asset yields code execution as the local user.

Trust boundary: this is boundary-4 (registry/release pipeline). The bytes that get executed originate from remote, third-party-controllable GitHub content (raw.githubusercontent.com index + github.com release assets) with NO end-to-end authenticity anchor — unlike a pure local-CLI capability. A compromise of the registry repo, a maintainer/CI token, or a release asset lets an attacker serve a malicious binary that every `scout plugin update` user pulls and runs unverified; the empty-checksum lock entry means TOFU pinning can never fire and downgrade detection can never trigger. The gap is real and as described.

I adjust two things from the finding's framing: (1) The MITM/cert-pinning sub-claim is weak — both transports are HTTPS (DefaultIndexURL and the GitHub release URLs), and Go's http.Get validates server certificates, so a network MITM cannot forge those hosts without a CA compromise; installPluginFromURL also refuses plaintext HTTP unless --insecure-http. This is a partial existing mitigation. (2) Consequently the realistic attack vector is repo/release-pipeline/maintainer-account compromise rather than trivial network MITM, so exploitability is "plausible," not "trivial." Severity stays High because the blast radius from a single index/release compromise is mass RCE across all auto-updating users with no possible detection (no signed index, empty pinned checksum), and the remediation (signed index digest + non-empty verified+pinned checksum + semver downgrade guard) is entirely absent.

**Verifier notes:** Real supply-chain integrity finding, accurately described at the code level. Recommended fixes are sound: add per-asset sha256 (and ideally a signed/detached signature over the index) to PluginInfo, verify archive bytes after download for github:/update flows, persist the real verified checksum into the lock file (never ""), and reject lower-semver updates absent --allow-downgrade. Key files: D:\weaver-sync\development\personal\projects\scout\cmd\scout\plugin.go (lines 194-253, 312-343, 448-527) and D:\weaver-sync\development\personal\projects\scout\pkg\scout\plugin\registry\registry.go (lines 19-85, 199-218) and update.go (line 62). Mitigations already present: HTTPS-only transport (plaintext HTTP refused without --insecure-http), 128MB download cap, and a printed SHA256 warning on unverified installs — these reduce but do not close the integrity gap.

---

### [HIGH] Plugin checksum is recorded but never verified before the plugin binary is executed

- **Dimension:** plugin_supply | **Location:** `cmd/scout/plugin.go:224` | **CWE:** CWE-494: Download of Code Without Integrity Check
- **Original severity:** high -> **adjusted:** high | **Exploitability:** plausible | **Confidence:** high

**Description:** The registry package exposes a full checksum verification API (`registry.FileChecksum`, `registry.VerifyChecksum`, `LockFile.Checksum`) and it is unit-tested, but NONE of these functions are called anywhere in the install, update, discovery, or subprocess-launch code paths. The only integrity gate that actually runs is `verifyPluginChecksum(data, expected)` in `installPluginFromURL`, where `expected` comes from the OPTIONAL `--checksum` flag. When the flag is omitted the function returns nil immediately (`if expected == "" { return nil }`, plugin.go:263) and the binary is installed after only a stderr warning (plugin.go:228-230). At launch time, `Manager.getClient` -> `Client.Start` -> `exec.CommandContext(ctx, manifest.CommandPath())` (client.go:107) executes the installed binary with no checksum comparison against the lock file at all. The lock file's SHA256 field is therefore dead metadata: it is written only by `pluginUpdateCmd` (and with an empty string at that, see plugin-supply-chain-2) and never read back to gate execution.

**Evidence:**

```
plugin.go:224-230: `expected, _ := cmd.Flags().GetString("checksum"); if err := verifyPluginChecksum(data, expected); err != nil { return err } if expected == "" { _,_ = fmt.Fprintf(cmd.OutOrStderr(), "warning: installing UNVERIFIED plugin (no --checksum)...") }`  |  plugin.go:262-265: `func verifyPluginChecksum(data []byte, expected string) error { if expected == "" { return nil } ... }`  |  client.go:107: `cmd := exec.CommandContext(ctx, c.manifest.CommandPath())` with no preceding checksum check. `registry.VerifyChecksum` (registry.go:250) has zero non-test callers.
```

**Exploit scenario:** A user runs `scout subplugin install https://cdn.example.com/scout-diag-linux-amd64.tar.gz` (boundary 4: third-party archive from a URL). Because `--checksum` is optional and absent, Scout prints a one-line warning and installs whatever bytes the server (or a MITM who controls the CDN / a compromised mirror / a hijacked GitHub release asset) returns. On the next `scout subplugin run` / first mode/tool/extractor call, the manager spawns the unverified binary, achieving arbitrary code execution in the local user's security context. The recorded lock-file checksum provides no protection because it is never compared at launch.

**Recommendation:** Make integrity verification mandatory and enforced at execution time. (1) On install, after extraction, compute the SHA256 of the resolved plugin command binary (`registry.FileChecksum(manifest.CommandPath())`) and store it in the lock file. (2) Require a trust anchor for remote installs: refuse `installPluginFromURL`/`installPluginFromGitHub` unless a `--checksum` (or registry-supplied checksum/signature) is provided, instead of merely warning. (3) In `Manager.getClient`/`Client.Start`, before `exec.CommandContext`, look up the plugin in the lock file and call `registry.VerifyChecksum(manifest.CommandPath(), locked.Checksum)`, failing closed (do not launch) on mismatch or missing entry. (4) Treat the stderr warning path as an error, not a soft warning.

**Verifier reasoning:** I re-read every cited path and the finding holds up.

Confirmed facts:
1. plugin.go:224-230 — `--checksum` is optional. `verifyPluginChecksum` returns nil immediately when `expected==""` (plugin.go:263-265), and an absent flag yields only a stderr warning before the install proceeds unconditionally.
2. The registry/GitHub install paths provide no trust anchor either: `installPluginFromGitHub` (plugin.go:312-342) builds a URL and calls `installPluginFromURL` with no checksum, and the registry index struct `PluginInfo` (registry.go:27-34) has NO checksum/sha256 field, so even a "trusted" registry-driven install has nothing to verify against.
3. `pluginUpdateCmd` writes an EMPTY checksum to the lock file (plugin.go:515: `lf.Lock(p.Name, info.Latest, "", info.Repo)`), confirming the lock metadata is hollow.
4. The launch path never re-verifies: grep of manager.go/client.go for LoadLockFile/Get/Checksum/VerifyChecksum returns nothing; `Client.Start` goes straight to `exec.CommandContext(ctx, manifest.CommandPath())` (client.go:107).
5. `registry.VerifyChecksum`/`FileChecksum` have zero non-test callers in the live tree (the only other hits are duplicate `.claude/worktrees/` copies).

Trust-boundary judgment: Installing a plugin is an intended local-user capability, but the untrusted input here is the artifact BYTES returned by a remote server/CDN/GitHub release — not the user's intent. The user authorizes "install plugin X," not "execute whatever a MITM or compromised mirror substitutes." HTTPS (and the http:// guard at plugin.go:195-199) authenticates the transport endpoint but does not bind the downloaded artifact to an expected hash, so a hijacked GitHub release asset, a malicious/compromised registry entry, or a TLS-MITM/compromised CDN leads directly to local arbitrary code execution at next launch. This is textbook CWE-494. The in-code comment ("A checksum is the only trust anchor... Verify when provided, warn otherwise") and the fully unit-tested-but-uncalled verification API confirm integrity verification was an intended control that is not enforced. This is NOT a case where the input is purely CLI-local, already gated by auth, or a documented deliberate feature.

Severity/exploitability calibration: This is a genuine supply-chain integrity gap warranting high design severity. I keep severity at high but set exploitability to "plausible" rather than "trivial" because a concrete attack requires the adversary to occupy an artifact-tampering position (CDN/MITM, hijacked release, or malicious registry/mirror) AND the user to install from a remote source without --checksum; it is not exploitable against an untampered TLS-protected GitHub asset.

**Verifier notes:** Relevant files (absolute): D:\weaver-sync\development\personal\projects\scout\cmd\scout\plugin.go (install paths 194-253, GitHub 310-342, update 462-526), D:\weaver-sync\development\personal\projects\scout\pkg\scout\plugin\client.go:107 (unverified exec), D:\weaver-sync\development\personal\projects\scout\pkg\scout\plugin\registry\registry.go (PluginInfo lacks checksum field 27-34; VerifyChecksum/FileChecksum 231-258 uncalled; Lock writes empty checksum). The cross-referenced plugin-supply-chain-2 (empty checksum written by update) is corroborated at plugin.go:515. Strongest fix is to fail closed at launch by re-hashing CommandPath() against a populated lock entry, add a checksum field to the registry index, and require a checksum/signature for all remote installs rather than warning.

---

### [MEDIUM] Persisted reports embed raw session cookies and HAR into a world-readable (0o644) file

- **Dimension:** secrets | **Location:** `internal/engine/report.go:88` | **CWE:** CWE-200: Exposure of Sensitive Information to an Unauthorized Actor
- **Original severity:** medium -> **adjusted:** medium | **Exploitability:** plausible | **Confidence:** high

**Description:** `SaveReport` (report.go:66) serializes the entire `Report` struct as raw JSON appended to the rendered document (`renderReport` -> report.go:120-123 `raw, err := json.MarshalIndent(r, "", "  ")`) and writes it to `~/.scout/reports/{uuidv7}.txt` with mode 0o644 (report.go:88) inside a 0o755 directory (report.go:81). The embedded `GatherResult` (report.go:43) carries `Cookies []Cookie` (gather.go:17) and `HAR []byte` (gather.go:22). So `scout gather <url> --cookies --har --report` (or the gather/health/crawl `--report` flows) writes the target's session cookies and full HAR (with Cookie/Authorization headers) into a world-readable report file.

**Evidence:**

```
report.go:88 `if err := os.WriteFile(path, []byte(content), 0o644); err != nil {`; report.go:120-123 appends `## Raw Data (JSON)` then `json.MarshalIndent(r, ...)`; gather.go:17 `Cookies     []Cookie        \`json:"cookies,omitempty"\`` and gather.go:22 `HAR         []byte          \`json:"har,omitempty"\``; report.go:43 `Gather    *GatherResult`.
```

**Exploit scenario:** An analyst runs `scout gather https://portal.example --cookies --har --report`; the generated `~/.scout/reports/<uuid>.txt` is world-readable and its embedded Raw Data JSON contains the portal's session cookies, so another local account on the machine reads the file and hijacks the session.

**Recommendation:** Write report files with mode 0o600 (report.go:88) and create the reports directory 0o700 (report.go:81). Additionally, strip or mask secret-bearing fields (Cookies, HAR auth headers) from the embedded Raw Data JSON before persisting, since reports are explicitly designed to be shared with/ingested by external AI systems.

**Verifier reasoning:** I re-read internal/engine/report.go, internal/engine/gather.go, cmd/scout/gather.go, and internal/engine/scouthome/scouthome.go. Every cited fact checks out:

1. report.go:88 writes the report with mode 0o644; report.go:81 creates the dir 0o755. Confirmed verbatim.
2. renderReport (report.go:120-128) appends the FULL Report struct serialized via json.MarshalIndent — no field filtering. Confirmed.
3. The embedded GatherResult (report.go:43) carries Cookies []Cookie (gather.go:17) and HAR []byte (gather.go:22). Gather() populates both by default (wantAll) or with --cookies/--har. HAR is captured WithHijackBodyCapture, so it contains Cookie/Authorization request headers.
4. The CLI gather --report path (cmd/scout/gather.go:60-62) passes the untrimmed `result` (with cookies/HAR) to SaveReport — the screenshot/HAR trimming at lines 96-102 only affects stdout output, NOT the saved report. So live session cookies and auth-bearing HAR genuinely land in a 0o644 file.

Trust boundary: this is a local same-host cross-user isolation boundary, not a network one. On a multi-user POSIX host (Linux/macOS) where the user's home subtree is traversable (commonly 0o755 by default), another local account CAN read ~/.local/share/scout/reports/<uuid>.txt and recover live session cookies → session hijack, exactly as the exploit scenario states.

Calibration / why not refuted: The input here is local-user-generated, but the vulnerability is not the capability itself — it's the over-permissive file mode that exposes self-generated secrets to OTHER local principals. The project's own documented security norm (CLAUDE.md, and the codebase: vault 0o600/dir 0o700, session metadata 0o600, flow capture artifacts 0o600) treats exactly this class of data as 0o600, and the report.go:52-56 comment explicitly acknowledges reports "may contain extracted page data that the user considers sensitive." The 0o644/0o755 choice is an inconsistent, real misconfiguration.

Severity/exploitability honesty: I keep medium (sensitive = live cookies/auth headers; deviates from the project's own 0o600 standard) but mark exploitability "plausible," not trivial: it requires (a) the user to run gather/--report with cookie/HAR collection, (b) a shared multi-user host, and (c) permissive ancestor-dir perms. On single-user desktops and on Windows (where Go's Unix mode bits are largely ignored and %LOCALAPPDATA% ACLs already restrict to the owner) practical impact is low, which is why this is medium rather than high. The recommended fix (0o600 file, 0o700 dir, and masking/stripping Cookies + HAR auth headers from the embedded Raw Data JSON) is correct and aligns the report path with the codebase's existing secret-handling conventions.

**Verifier notes:** Primary impactful path is the gather --report flow (cmd/scout/gather.go:61). testsite/crawl/swarm --report paths also write 0o644 but carry less sensitive payloads (issues/links/errors). Fix should also cover the dir mode at report.go:81 and ideally scrub Cookies/HAR before serialization since reports are explicitly designed to be ingested by external AI systems.

---

### [MEDIUM] gRPC CreateSession launches unbounded browser processes (no concurrent-session cap)

- **Dimension:** dos | **Location:** `grpc/server/server_session.go:205` | **CWE:** CWE-770: Allocation of Resources Without Limits or Throttling
- **Original severity:** medium -> **adjusted:** medium | **Exploitability:** plausible | **Confidence:** high

**Description:** The gRPC daemon's CreateSession RPC launches a real Chrome browser process per call (scout.New(opts...) at line 126, browser.NewPage at line 136) and stores the session into the s.sessions sync.Map (line 205) with no upper bound on the number of concurrent sessions. There is no per-peer or global session cap, and the only reaping mechanism is the daemon-wide idle timer (which shuts the whole daemon down only when fully idle) and orphan cleanup on restart - neither bounds the live session count. A paired remote device (or the same device that legitimately paired and was later compromised) can call CreateSession in a tight loop, each call spawning a new headless Chrome plus event-wiring goroutines, recorder, and sidecar file handles, until the host exhausts RAM, PIDs, or file descriptors.

**Evidence:**

```
browser, err := scout.New(opts...)  // line 126 - launches a Chrome process
...
page, err := browser.NewPage(url)   // line 136
...
s.sessions.Store(sess.id, sess)     // line 205 - no count check anywhere
s.trackPeer(ctx, sess.id)
// grep for maxSession/sessionCount/'session limit' across the repo returns no enforcement
```

**Exploit scenario:** An attacker who controls a paired (mTLS-trusted) device repeatedly invokes the CreateSession RPC against the daemon; each call forks a real Chrome process with no cap, so a few hundred calls exhaust the host's memory and process table and take down the daemon and the machine it runs on.

**Recommendation:** Enforce a configurable maximum number of concurrent sessions (and ideally a per-peer/per-deviceID quota) in CreateSession: before scout.New, count live entries in s.sessions and return codes.ResourceExhausted when the cap is reached. Add an idle/TTL reaper that closes individual leaked sessions (not just the whole daemon). Consider a default cap (e.g. 16) tunable via config.

**Verifier reasoning:** Confirmed by reading the code. grpc/server/server_session.go:126 launches a real Chrome process per CreateSession (page at :136, stored at :205) with no count check. ScoutServer (server.go) has no maxSessions field; the only reaping is the daemon-wide idle.Timer (fires only on full idleness, then tears down the whole daemon) and Reconcile() startup orphan cleanup — neither bounds live session count, and there is no per-session TTL reaper. Repo-wide grep finds no session-cap/ResourceExhausted enforcement (the WithMaxConcurrent hit is an unrelated engine scraper rate limiter, never wired to gRPC). The gRPC server is built with only message-size limits + panic-recovery interceptors — no InTapHandle, MaxConcurrentStreams, or rate limiter — so a CreateSession loop is unthrottled.

The trust boundary holds up: in default (non --insecure) mode NewTLSServer is used with host=="" binding ALL interfaces (:9551), explicitly designed for LAN remote control (confirmed by the --host flag help text). A remote caller must be a token-paired, trust-store-enrolled mTLS peer (pairing.go), so the attacker is an authenticated remote device across a network boundary — and the finding's own threat model (a paired device later compromised) is realistic. getSession enforces per-device ownership but CreateSession has no per-peer quota, so one trusted peer can spawn unbounded browsers until host RAM/PIDs/FDs are exhausted, taking down the daemon and host.

Adversarial counters fail: this is not a local-CLI-only capability (mTLS mode is network-exposed by design); mTLS/pairing gate WHO can call but do not bound resource consumption by a trusted-but-malicious peer; the idle timer never fires under sustained load and cannot reap individual leaked sessions. Impact is availability-only (no confidentiality/integrity), and exploitation requires first obtaining trust-store enrollment, so Medium severity with plausible exploitability is correctly calibrated. The recommendation (configurable concurrent-session cap + per-peer/deviceID quota returning codes.ResourceExhausted, plus an individual-session TTL reaper) is accurate and code-level actionable.

**Verifier notes:** Relevant files: D:\weaver-sync\development\personal\projects\scout\grpc\server\server_session.go (CreateSession sink at :126/:136/:205), D:\weaver-sync\development\personal\projects\scout\grpc\server\server.go (ScoutServer struct + idle timer, no cap), D:\weaver-sync\development\personal\projects\scout\grpc\server\tls.go (mTLS trust enforcement), D:\weaver-sync\development\personal\projects\scout\grpc\server\pairing.go (token-gated enrollment), D:\weaver-sync\development\personal\projects\scout\cmd\scout\server.go (mTLS binds all interfaces; only msg-size + panic interceptors, no rate limit). Headline line:205 matches the Store sink; launch is at :126.

---

### [MEDIUM] Command logger writes secret CLI arguments and full stdout/stderr to world-readable JSON logs

- **Dimension:** secrets | **Location:** `internal/logger/logger.go:177` | **CWE:** CWE-532: Insertion of Sensitive Information into Log File
- **Original severity:** high -> **adjusted:** medium | **Exploitability:** plausible | **Confidence:** high

**Description:** When the command logger feature is enabled (`scout logger --path <dir>`, persisted in `~/.cache/scout/`), the root command's PersistentPreRunE calls `log.StartExecution(cmd.Name(), args, ...)` (cmd/scout/scout.go:43) and wraps stdout/stderr. The logger then serializes the raw positional `args` and the complete captured stdout/stderr into a JSON log file via `l.slog.Info("command_start", "args", args, ...)` (logger.go:300-305) and `l.slog.Info("command_end", ... "stdout", stdout, "stderr", stderr ...)` (logger.go:354-373). The log file is opened with mode 0644 (logger.go:177, also 227, 253) inside a 0755 directory (logger.go:165), so it is world-readable. The `ignoredCommands` allowlist (internal/flags/flags.go:24) only excludes the `logger` command itself, not secret-handling subcommands. Critically, `scout vault set KEY=VALUE` (cmd/scout/vault.go:37-63) takes secret values as positional CLI arguments which are passed to `parseSecretArgs(args)` — these exact `args` are logged verbatim. Likewise any command output that echoes a token/cookie (e.g. challenge solver `--api-key` on challenge.go:77 lands in args; agent/scrape output) is captured into the same world-readable log.

**Evidence:**

```
logger.go:177 `file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)`; logger.go:300-305 logs `"args", args`; logger.go:355/363 append `"stdout", stdout` and `"stderr", stderr`; vault.go:56 `parsed, err := parseSecretArgs(args)` consumes the same `args` slice as KEY=VALUE secrets; flags.go:24 `ignoredCommands = map[string]bool{ "logger": true }` (vault/scrape/challenge NOT excluded).
```

**Exploit scenario:** On a shared/multi-user host the operator runs `scout logger --path /var/log/scout` then `scout vault set STRIPE_KEY=sk_live_abcd...`; the raw key is written into a 0644 file under /var/log/scout that any local user can read, defeating the entire encrypted-vault design (vault file itself is 0600).

**Recommendation:** Open log files with 0600 and create the log directory 0700. Add all secret-bearing commands (`set`/`vault`/`scrape`/`auth`/`capture-host`/`challenge`) to `ignoredCommands`, OR redact args matching `KEY=VALUE` and known secret flag patterns before logging, and do not capture stdout/stderr for those commands. Prefer reading secrets via prompt/stdin (as `vault set` already does for the passphrase) rather than positional args.

**Verifier reasoning:** I re-read every cited location and empirically tested the Cobra behavior. The core finding is REAL.

PRIMARY VECTOR CONFIRMED (vault set):
- internal/logger/logger.go:177/227/253 open log files with mode 0644 inside a 0755 directory (line 165); there is NO chmod or restrictive perm anywhere in the package (grep confirmed no 0600/0700).
- StartExecution logs the raw positional `args` (logger.go:302) and EndExecution logs `args` again plus full captured `stdout`/`stderr` (logger.go:342, 355, 363). There is zero redaction (grep for redact/secret/password/token in logger.go returned no matches).
- cmd/scout/scout.go:32-49: the root PersistentPreRunE is the only one in cmd/scout (grep confirmed), so it runs for every subcommand. It calls flags.ShouldIgnoreCommand(cmd.Name()), and ignoredCommands (flags.go:24) contains ONLY "logger".
- I ran a Cobra harness against the project's own cobra version: for `scout vault set STRIPE_KEY=sk_live_...`, cmd.Name()=="set" (NOT ignored) and args==["STRIPE_KEY=sk_live_..."]. vault.go:56 feeds that exact slice to parseSecretArgs as KEY=VALUE secrets. So the plaintext secret is written verbatim to a world-readable log when logging is enabled. This directly defeats the vault's deliberate 0600 protection (vault.bin is 0o600 in a 0o700 dir per design).

CWE-532 is the correct classification.

TRUST BOUNDARY: Both the trigger (scout logger --path) and the secret input (scout vault set) are local-user/CLI actions — there is no network/gRPC/MCP/agent attacker. The boundary crossed is between local OS users: a 0644 file under a shared log directory is readable by other local users on Linux/macOS (and in the Linux Docker images). That is a legitimate but narrower boundary than a remote attacker.

WHERE THE FINDING OVERSTATES: It claims the challenge solver `--api-key` "lands in args." That is wrong — `--api-key` is a Cobra flag (challenge.go:77 uses cmd.Flags().GetString), and I empirically confirmed flag values are stripped from the positional `args` slice (test printed args=[url] only). The challenge command also does not echo the key to stdout/stderr, so that secondary path does not hold. The general stdout/stderr-capture risk is real in principle but the specific examples cited beyond `vault set` are weaker.

SEVERITY ADJUSTMENT: Down from High to Medium. Preconditions are meaningful: (1) the logger feature is opt-in and off by default; (2) requires a multi-user host with a shared/world-readable log directory; (3) the world-readable consequence only bites on Linux/macOS — on Windows (the documented primary platform) Unix mode bits are largely cosmetic against the ACL model. Given a realistic ops/CI scenario (logging enabled for diagnostics on a shared Linux host), the exposure is plausible, not trivial (no default-on path) and not merely theoretical. The recommended fixes (0600 file / 0700 dir, and excluding/redacting secret-bearing positional args for vault/scrape/etc.) are correct and appropriate.

**Verifier notes:** Real CWE-532. Strongest concrete sink is `scout vault set KEY=VALUE` -> positional args -> 0644 log (verified empirically). The challenge `--api-key` example in the finding is inaccurate (flag values are not in positional args). Fix: open logs 0o600 / dir 0o700, and redact or skip logging of positional args + stdout/stderr for secret-bearing commands (or move secret input to stdin/prompt as vault already does for the passphrase). Default-off feature + multi-user-host + non-Windows precondition justifies Medium rather than High.

---

### [LOW] Agent HTTP server library API exposes unauthenticated browser JS-eval with no bind-address guard

- **Dimension:** injection | **Location:** `pkg/scout/agent/server.go:376` | **CWE:** CWE-94: Improper Control of Generation of Code (Code Injection)
- **Original severity:** medium -> **adjusted:** low | **Exploitability:** difficult | **Confidence:** medium

**Description:** The agent HTTP server exposes an `eval` tool that runs caller-supplied JavaScript verbatim in a live browser page. POST /call with {"name":"eval","arguments":{"script":"..."}} routes through Provider.Call -> handleEval (pkg/scout/agent/agent.go:317-325) -> pg.Eval(script), i.e. CDP Runtime.evaluate of attacker-controlled JS. Authentication is fully optional: authMiddleware short-circuits and calls next.ServeHTTP when config.APIKey is empty (server.go:376-379). The CLI command (cmd/scout/agent.go:79-86) compensates by refusing to bind a non-loopback address when no API key is set. However, that guard lives ONLY in the CLI wrapper. The exported library entry points agent.NewServer(ServerConfig{...}) and Server.ListenAndServe perform NO bind-address validation: ServerConfig.Addr is used as-is (server.go:73-75, 408-422) and ServerConfig.APIKey defaults to empty (=no auth). Any program embedding the agent package with Addr set to a routable interface and no APIKey gets an unauthenticated remote JS-eval / navigate (SSRF) endpoint, defeating the documented loopback-only safety posture.

**Evidence:**

```
authMiddleware (server.go:373-403): `if s.config.APIKey == "" { next.ServeHTTP(w, r); return }` -> auth bypassed when no key. handleEval (agent.go:317-325): `script, _ := args["script"].(string); ... result, err := pg.Eval(script)`. NewServer (server.go:72-75): `if cfg.Addr == "" { cfg.Addr = "localhost:9000" }` with no rejection of non-loopback + empty APIKey. The protective refusal exists only in cmd/scout/agent.go:84-85, not in the package.
```

**Exploit scenario:** A downstream integrator embeds Scout's agent package and starts agent.NewServer(agent.ServerConfig{Addr: "0.0.0.0:9000"}) (no APIKey), assuming the package is safe by default; any host on the network then POSTs {"name":"eval","arguments":{"script":"() => fetch('http://169.254.169.254/latest/meta-data/').then(r=>r.text())"}} to /call and runs arbitrary JavaScript in the host's browser (cookie theft, internal-network SSRF, local-file reads via file:// navigation).

**Recommendation:** Move the loopback-without-API-key refusal out of the CLI and into agent.NewServer / ListenAndServe so the library fails closed: when ServerConfig.APIKey is empty, reject (return an error) any Addr whose host is not a loopback address, using the same isLoopbackHost logic (and treat empty host / ":port" / "0.0.0.0" / "[::]" as non-loopback). Alternatively, make APIKey mandatory for any non-loopback bind regardless of caller.

**Verifier reasoning:** Code claims verified by reading the cited files. (1) agent.NewServer defaults Addr to localhost:9000 only when empty and otherwise uses the caller's Addr verbatim with no loopback validation (server.go:72-75, 408-422). (2) authMiddleware short-circuits when APIKey=="" (server.go:376-379), and APIKey defaults to empty. (3) handleEval runs pg.Eval(attacker JS) with no gating (agent.go:317-331). I confirmed the SSRF urlpolicy.Check is applied ONLY to handleNavigate (agent.go:215-219), not to eval — so in-page fetch() to 169.254.169.254 from eval'd JS is genuinely ungated. (4) The fail-closed non-loopback refusal exists only in the CLI wrapper (cmd/scout/agent.go:79-87 using isLoopbackHost from cmd/scout/server.go), and grep confirms no equivalent guard in the agent package.

So the finding is factually correct: the library entry point does not fail closed, while the CLI does. That inconsistency, plus the fact that ServerConfig is an exported type in the public pkg/ facade and the CLI author explicitly treated "loopback-only without auth" as the intended safety posture, makes this a legitimate defense-in-depth / insecure-default hardening item.

However, the impact as written overstates an as-shipped vulnerability. No attack exists against Scout's own artifacts: the shipped CLI (the only first-party server entry point) is fail-closed and refuses non-loopback binds without an API key. The exploit requires a HYPOTHETICAL downstream integrator to deliberately call agent.NewServer(ServerConfig{Addr:"0.0.0.0:9000"}) with no APIKey — an explicit insecure configuration choice by a third party. That is a developer-misuse / intended-capability boundary, not untrusted input crossing a trust boundary in the product under review. There is a deliberate misconfiguration step between the code and any network attacker.

The CWE is also mischaracterized: eval-of-supplied-JS is the tool's intended function; the real defect is a missing access-control/insecure-default at the library boundary (CWE-306/CWE-1188), and only when misconfigured. Net: real as a fail-closed hardening recommendation, but not exploitable as shipped — downgrade to low, exploitability difficult (requires a third-party insecure embedding that Scout's own CLI explicitly prevents).

**Verifier notes:** Recommendation is sound and worth doing: move the isLoopbackHost-without-APIKey refusal into agent.NewServer/ListenAndServe so the library fails closed (treat empty host / 0.0.0.0 / [::] as non-loopback). Independently, handleEval is not covered by the SSRF urlpolicy that gates handleNavigate — if the eval tool is ever exposed to any semi-trusted caller, in-page fetch() bypasses the URL policy entirely; consider documenting eval as a privileged tool or gating it. Severity stays low because no first-party path reaches the unsafe state; the shipped CLI is fail-closed.

---

### [LOW] Unbounded memory allocation from attacker-controlled WebSocket frame length in CDP client

- **Dimension:** deser | **Location:** `internal/engine/lib/cdp/websocket.go:175` | **CWE:** CWE-789: Memory Allocation with Excessive Size Value (also CWE-130: Improper Handling of Length Parameter Inconsistency)
- **Original severity:** medium -> **adjusted:** low | **Exploitability:** plausible | **Confidence:** high

**Description:** The internalized rod CDP WebSocket reader parses a frame-declared payload length directly from the wire (up to a full 8-byte / 64-bit value for the b==127 extended-length case) and then allocates a buffer of exactly that size with `data := make([]byte, size)` before any bytes of the payload are read. There is no upper bound on `size`, no sanity check that the declared length is consistent with the connection, and no check that `size` is non-negative. The length accumulator `size` is a plain `int`, so on a 64-bit host a malicious server can declare a payload of up to ~2^63 bytes, forcing an immediate huge allocation (out-of-memory / process kill). On a 32-bit host the `size = size<<8 + int(b)` accumulation with fieldLen=8 overflows the 32-bit int, which can wrap to a negative value and panic `make([]byte, negative)`, or wrap to an unexpected positive size. The bytes are only `io.ReadFull`'d afterward, so the allocation happens purely on the attacker's declared length, not on data actually delivered. This is the read path used by every CDP connection, including connections to a caller-supplied remote DevTools endpoint via `WithRemoteCDP(endpoint)` / `scout connect --cdp ws://...` (option.go:285), where the endpoint is an explicitly supported third-party/networked browser the host does not control.

**Evidence:**

```
websocket.go:153-178:
	size := 0
	fieldLen := 0
	b &= 0x7f
	switch {
	case b <= 125:
		size = int(b)
	case b == 126:
		fieldLen = 2
	case b == 127:
		fieldLen = 8
	}
	for i := 0; i < fieldLen; i++ {
		b, err := ws.r.ReadByte()
		if err != nil { return nil, err }
		size = size<<8 + int(b)
	}
	data := make([]byte, size)   // <-- no cap, no >=0 check; size fully attacker-controlled
	_, err = io.ReadFull(ws.r, data)
Reached from option.go:285 WithRemoteCDP(endpoint) and `scout connect --cdp` against an attacker-supplied endpoint; no MaxFrame/MaxMessage constant exists anywhere in the read path.
```

**Exploit scenario:** A user runs `scout connect --cdp ws://attacker-controlled-cdp-endpoint` (a supported feature for connecting to hosted/cloud Chrome). The hostile endpoint completes the WebSocket handshake, then sends a single frame whose 8-byte extended length field declares ~0x7FFFFFFFFFFFFFFF bytes; Scout immediately executes `make([]byte, hugeSize)`, exhausting memory and crashing the Scout host process (DoS) — without ever sending the corresponding payload bytes.

**Recommendation:** Introduce a maximum frame/message size constant (CDP messages are small JSON; a few MiB is generous) and reject oversized or invalid lengths before allocating: after computing `size`, return an error if `size < 0 || size > maxFrameBytes`. Prefer reading into a bounded buffer via `io.LimitReader`/`io.CopyN` rather than pre-allocating `make([]byte, size)` on the declared length, and treat any overflow in the length accumulation as a protocol error. Apply the same cap to the handshake `io.ReadAll(res.Body)` at websocket.go:239.

**Verifier reasoning:** Code is exactly as described. internal/engine/lib/cdp/websocket.go:153-176 accumulates a WebSocket frame payload length from up to 8 wire bytes (size = size<<8 + int(b), b==127 case) with no upper bound, no >=0 check, and no overflow handling, then immediately does data := make([]byte, size) BEFORE io.ReadFull reads any payload. This read path is the sole CDP message reader (client.go:136 cdp.ws.Read()), so the declared length is honored for every frame. It IS reachable across a real network trust boundary: WithRemoteCDP(endpoint) / `scout connect --cdp ws://...` (cmd/scout/connect.go:36) / SCOUT_CDP_ENDPOINT env (plugins/*/main.go) are documented features for connecting to remote and third-party managed Chrome services, where the CDP server is an untrusted network peer that fully controls frame headers. So the bug is genuine: a malicious/MITM'd CDP endpoint can send one frame declaring ~2^63 bytes and trigger make([]byte, hugeSize) -> OOM/process kill (or a negative-size panic on 32-bit). It is not already mitigated — no MaxFrame/MaxMessage cap exists anywhere in the read path, and the recommended fix (validate size against a maxFrameBytes constant before allocating) is correct.\n\nHowever, I downgrade severity from medium to LOW. (1) Impact is DoS only — a crash of the user's OWN single-tenant Scout CLI/daemon process; no RCE, no data exposure, no multi-tenant blast radius. (2) The endpoint is always operator-selected (flag/env/code); an attacker cannot redirect Scout on their own — they must control or MITM (plaintext ws://) an endpoint the user deliberately connects to. (3) Most importantly, by design a remote CDP server already has total control over the browser Scout is driving (it speaks CDP: arbitrary JS execution in pages, full DOM/network access, cookie theft, etc.); the ability to also crash the client process is a marginal additional capability over an already-fully-trusted-by-necessity peer. The 32-bit overflow/negative-size panic angle is largely theoretical given Scout targets 64-bit hosts. Exploitability is plausible (requires user to connect to an attacker-controlled/MITM'd endpoint), not trivial. Worth fixing as cheap defense-in-depth (bound the allocation + cap the handshake io.ReadAll at line 239), but realistic security impact is Low.

**Verifier notes:** Real input-validation/DoS defect, correctly located. Fix: add a maxFrameBytes constant, return a protocol error when size < 0 || size > maxFrameBytes before make([]byte, size), and similarly cap io.ReadAll(res.Body) at websocket.go:239. Severity reduced because impact is a self-inflicted single-process crash reachable only when the operator connects to a remote/MITM'd CDP endpoint that, by virtue of speaking CDP, already controls the driven browser.

---

### [LOW] gRPC Interactive bidirectional stream leaks an event-forwarding goroutine on send failure

- **Dimension:** dos | **Location:** `grpc/server/server_hijack_stream.go:299` | **CWE:** CWE-404: Improper Resource Shutdown or Release
- **Original severity:** low -> **adjusted:** low | **Exploitability:** plausible | **Confidence:** medium

**Description:** In the Interactive bidi-stream handler, the first command lazily subscribes the session (eventCh = sess.subscribe(subID), line 291) and spawns a goroutine that ranges over eventCh forwarding events to stream.Send (lines 299-305). That goroutine only exits when eventCh is closed (via sess.unsubscribe in the deferred cleanup) OR when stream.Send returns an error. If stream.Send errors, the goroutine returns but leaves the subscription registered; conversely the channel is buffered (256) and broadcast drops on full, so the goroutine can sit blocked in stream.Send against a wedged client while the outer Interactive loop is also blocked in stream.Recv. Because unsubscribe runs only when Interactive itself returns, a client that opens many Interactive streams and stalls (sends one command, then neither reads nor closes) accumulates parked goroutines and 256-slot channel buffers per stream, growing daemon memory and goroutine count.

**Evidence:**

```
eventCh = sess.subscribe(subID)        // 256-buffered channel, line 291
defer sess.unsubscribe(subID)          // only fires when Interactive returns
...
go func() {                            // line 299
    for ev := range eventCh {          // exits only on channel close or Send error
        if err := stream.Send(ev); err != nil { return }
    }
}()
```

**Exploit scenario:** A paired device opens many Interactive streams, sends a single command on each to trigger subscription, then stops reading and never closes the stream; each stream parks a forwarding goroutine plus a 256-entry buffered channel, and repeated over many connections this exhausts goroutines/memory on the daemon.

**Recommendation:** Tie the forwarding goroutine's lifetime to stream.Context().Done() (select on ctx.Done() alongside the channel read) so it exits promptly when the stream is broken, and call unsubscribe as soon as Send fails. Bound the number of concurrent Interactive/streaming subscriptions per peer.

**Verifier reasoning:** Confirmed the defect by reading grpc/server/server_hijack_stream.go and server.go. In Interactive (lines 266-323) the first command lazily calls eventCh = sess.subscribe(subID) (line 291; the channel is 256-buffered per server.go:54) and spawns a forwarding goroutine (lines 299-305) that exits ONLY on channel close or stream.Send error. The defer sess.unsubscribe(subID) (line 293), which closes eventCh, runs only when Interactive returns, i.e. when stream.Recv() returns. Unlike the sibling StreamHijack (lines 133-134) and StreamEvents (lines 258-259) handlers, this forwarding goroutine does NOT select on stream.Context().Done(). No server-side keepalive / MaxConnectionIdle / MaxConnectionAge / MaxConcurrentStreams is configured (only MaxRecvMsgSize/MaxSendMsgSize at cmd/scout/server.go:73-74). So an authenticated peer that sends one command then stalls (holds the HTTP/2 connection open, neither reading nor closing) parks the outer Recv indefinitely, unsubscribe never fires, and the goroutine + 256-slot channel persist per stalled stream. The bug is real and correctly classified CWE-404.

Severity is constrained to low for three reasons. (1) Trust boundary: the RPC is reachable across the network only through NewTLSServer with tls.RequireAnyClientCert + a VerifyPeerCertificate that rejects any device not in the trust store (tls.go:25-44); trust requires the out-of-band pairing token with constant-time compare (pairing.go:92-98) and fails closed if unset. The attacker must therefore be an already-paired, trusted device (insider), not an anonymous network attacker. The --insecure no-auth path is a deliberate, explicitly warned (server.go:62) local/loopback operator choice. (2) Impact is bounded and slow: each leaked stream costs ~one parked goroutine plus a 256-entry pointer channel, with no per-request amplification — the attacker pays one connection+stream per leaked unit and would need tens of thousands of held-open streams to exhaust the daemon; this is a memory/goroutine creep, not a crash primitive (matching the finding's own low rating). (3) The Send-error sub-case is largely self-healing: after stream.Send errors, the next stream.Recv() on the same broken stream also errors, returning Interactive and firing unsubscribe; only the deliberate-stall case genuinely leaks. The recommended fix (select on stream.Context().Done(), unsubscribe on Send failure, optionally cap concurrent subscriptions) is sound and mirrors the pattern already used in the same file.

**Verifier notes:** Real low-severity resource-leak defect. Authoritative file: D:\weaver-sync\development\personal\projects\scout\grpc\server\server_hijack_stream.go (Interactive lines 266-323, leaky goroutine lines 299-305). Compare correct handlers in same file: StreamHijack (112-137, ctx.Done() at 133) and StreamEvents (237-262, ctx.Done() at 258). subscribe/unsubscribe/256-buffer at D:\weaver-sync\development\personal\projects\scout\grpc\server\server.go:50-68. Auth gating: D:\weaver-sync\development\personal\projects\scout\grpc\server\tls.go:20-58 (mTLS + trust store) and pairing.go:85-98. No keepalive/idle stream caps configured: D:\weaver-sync\development\personal\projects\scout\cmd\scout\server.go:72-75. Attacker class = already-paired trusted peer (or local operator under --insecure). Fix: tie forwarding goroutine to stream.Context().Done() and unsubscribe on Send error.

---

### [LOW] HAR network recordings written world-readable (0o644) despite containing Cookie/Authorization headers

- **Dimension:** secrets | **Location:** `cmd/scout/gather.go:91` | **CWE:** CWE-312: Cleartext Storage of Sensitive Information
- **Original severity:** medium -> **adjusted:** low | **Exploitability:** plausible | **Confidence:** high

**Description:** HAR exports capture every request and response header, including `Cookie`, `Authorization`, and `Set-Cookie` (recorder.go parses `e.Request.Headers` at recorder.go:63 and `e.Response.Headers` at recorder.go:100 with no redaction). These HAR artifacts are then persisted with world-readable mode 0o644 in at least two code paths: `cmd/scout/gather.go:91` (`os.WriteFile(harFile, result.HAR, 0o644)` for `scout gather --save-har`) and `internal/engine/knowledge_writer.go:68` (`os.WriteFile(path, kp.HAR, 0o644)`). By contrast the session-managed HAR flush path correctly uses 0o600 (session/monitors.go via writeFileAtomic) and the flow capture artifact uses 0o600 (flow/capture_artifact.go:44) — proving the project's own standard for secret-bearing recordings is 0o600, which these paths violate.

**Evidence:**

```
gather.go:91 `if err := os.WriteFile(harFile, result.HAR, 0o644); err == nil {`; knowledge_writer.go:68 `_ = os.WriteFile(path, kp.HAR, 0o644)`; recorder.go:63 `headers := parseNetworkHeaders(e.Request.Headers)` and recorder.go:100 `respHeaders := parseNetworkHeaders(e.Response.Headers)` (no auth-header stripping).
```

**Exploit scenario:** A user runs `scout gather https://app.internal --har --save-har` against an authenticated app; the resulting `.har` file in the working directory is world-readable and contains the live session `Cookie` / `Authorization` header, letting any other local user copy it and replay the session.

**Recommendation:** Write all HAR artifacts with mode 0o600 (gather.go:91, knowledge_writer.go:68, plus the markdown/snapshot/manifest writes in knowledge_writer.go:45/77/102 that may embed captured data). Optionally add a header allowlist/redaction pass to the HAR exporter so `Cookie`/`Authorization`/`Set-Cookie` are stripped or masked unless the user explicitly opts in.

**Verifier reasoning:** All cited facts verified by reading the code. cmd/scout/gather.go:91 writes the HAR with os.WriteFile(harFile, result.HAR, 0o644), and internal/engine/knowledge_writer.go:68 does the same for KnowledgePage.HAR. The hijack recorder serializes every request/response header verbatim via MapToHARHeaders(req.Headers) (recorder.go:134) and MapToHARHeaders(resp.Headers) (recorder.go:159) with no redaction — Cookie/Authorization/Set-Cookie are included (the package's own bench fixture even uses "Authorization":"Bearer xxx"). The finding's line numbers for recorder.go (63/100, parseNetworkHeaders) are stale, but the substance — wholesale header capture with no stripping — is correct. The project's own standard for the same secret-bearing HAR data is 0o600: the daemon flush path writes os.WriteFile(outPath, data, 0o600) at grpc/server/server_session.go:283 and :346, and flow/capture_artifact.go:44 uses 0o600. So these two paths genuinely violate the established project convention.

Why downgrade from Medium to Low: (1) The trust boundary crossed is local-user-to-local-user file ACL, not a network/gRPC/MCP/auth boundary. The HAR is written to the invoking user's CWD by a local CLI command run with that user's own credentials; the "attacker" must be a separate local account on the same host. (2) The exploit requires a shared multi-user system with another local user holding read access to the working directory, plus a HAR that actually captured live auth headers — a real but co-tenancy-gated precondition, not remote. (3) On Windows, the documented primary platform, Go's os.WriteFile only maps the owner-write bit; the 0o644 group/other read bits do not create a POSIX-style world-readable ACL (ACLs inherit from the parent dir), so the "world-readable" claim does not hold there. The concrete world-readable exposure is limited to Linux/macOS multi-user hosts (subject to umask). That makes the realistic impact Low: a defense-in-depth inconsistency with the project's own 0o600 standard, trivially fixable, but with local-only, platform- and co-tenancy-bounded impact rather than a Medium-grade exposure. CWE-312 (cleartext storage of session secrets) is the correct weakness class. Recommended fix stands: use 0o600 at gather.go:91 and knowledge_writer.go:68 (and the sibling screenshot/snapshot/pdf/manifest writes that may embed captured data), optionally adding a Cookie/Authorization/Set-Cookie redaction/allowlist pass to ExportHAR.

**Verifier notes:** Relevant files: D:\weaver-sync\development\personal\projects\scout\cmd\scout\gather.go:91; D:\weaver-sync\development\personal\projects\scout\internal\engine\knowledge_writer.go:45,58,68,77,86,102; D:\weaver-sync\development\personal\projects\scout\internal\engine\hijack\recorder.go:134,159,249; baseline 0o600 paths: D:\weaver-sync\development\personal\projects\scout\grpc\server\server_session.go:283,346 and D:\weaver-sync\development\personal\projects\scout\pkg\scout\flow\capture_artifact.go:44.

---

### [LOW] Agent HTTP server serves browser-control API and Bearer API key over plaintext HTTP with no TLS option

- **Dimension:** tls | **Location:** `pkg/scout/agent/server.go:436` | **CWE:** CWE-319: Cleartext Transmission of Sensitive Information
- **Original severity:** medium -> **adjusted:** low | **Exploitability:** plausible | **Confidence:** high

**Description:** The agent REST server is started with `net.Listen("tcp", s.config.Addr)` and served via `srv.Serve(ln)` using a plain `http.Server` — there is no `ServeTLS`, no `TLSConfig`, and no flag/config to enable TLS anywhere in `ServerConfig` or the `scout agent serve` command. The server exposes `POST /call` and `POST /stream`, which route to `Provider.Call` and drive browser navigation/eval/screenshot on caller-supplied input. Authentication, when enabled, is a static Bearer token compared in `authMiddleware` (server.go:389-399). Because the transport is cleartext, both the API key and the full request/response bodies (target URLs, extracted page content, tool arguments) are exposed to any on-path observer when the server is reached over a network rather than loopback. The CLI refuses a non-loopback bind only when NO api-key is set (cmd/scout/agent.go:79-87); with an api-key set, a routable plaintext bind is permitted, sending the key in clear over the network.

**Evidence:**

```
server.go:414 `ln, err := net.Listen("tcp", s.config.Addr)`; server.go:421-427 `srv := &http.Server{Handler: ...}` (no TLSConfig); server.go:436 `srv.Serve(ln)` (no ServeTLS); server.go:395-396 `token := strings.TrimPrefix(auth, "Bearer ")` compared against `s.config.APIKey`. No TLS field exists in `ServerConfig` (server.go:24-39) nor any `--tls`/`--cert` flag in agent.go.
```

**Exploit scenario:** An operator sets `--api-key` and `--addr 0.0.0.0:9000` to let a remote AI host call the agent (the loopback guard is satisfied because an api-key is present). An attacker on the network path captures the `Authorization: Bearer <key>` header in cleartext from the first request, then reuses it to POST `/call` with `{"name":"navigate","arguments":{"url":"http://169.254.169.254/..."}}`, driving the host's browser to internal/cloud-metadata endpoints (SSRF) and exfiltrating extracted content — all because the channel and key were never encrypted.

**Recommendation:** Add TLS support to the agent server: accept a cert/key (or reuse the device identity) and call `srv.ServeTLS` with `TLSConfig{MinVersion: tls.VersionTLS13}` when configured. Make TLS mandatory for any non-loopback bind (extend the guard in cmd/scout/agent.go to require TLS, not just an api-key, when the host is not loopback), and document that the plaintext bind is loopback-only. Treat the Bearer key as sniffable until the transport is encrypted.

**Verifier reasoning:** The core CWE-319 claim is factually correct and I verified it directly. The agent server uses plain net.Listen + srv.Serve with no TLSConfig/ServeTLS (server.go:414, :436), ServerConfig has no TLS field (server.go:24-39), and there is no --tls/--cert flag in agent.go. A grep of pkg/scout/agent for ServeTLS/TLSConfig/crypto/tls returns zero matches. The CLI loopback guard only fires when apiKey == "" (agent.go:79-87), so an operator who sets --api-key can bind 0.0.0.0:9000, sending the static Bearer key (compared in authMiddleware, server.go:395-396) and all request/response bodies in cleartext over the network. An on-path observer can capture and replay the key. That is a genuine confidentiality issue crossing a network trust boundary (the agent HTTP server is an in-scope boundary), and the affected config is an intended remote-access path, so exploitability is plausible rather than theoretical.

However, the finding is overstated on two counts, which drives the downgrade from medium to low. First, the headline exploit (drive the browser to 169.254.169.254 cloud-metadata) is blocked by default: the agent enforces a default-deny SSRF url-policy (SCOUT_ALLOW_LOCAL_TARGETS off by default; TestHandleNavigateBlocksInternalURL, agent_test.go:898-902, asserts internal targets are blocked before the browser is touched). The metadata/SSRF pivot requires the operator to separately pass --allow-local-targets, so the dramatic escalation in the scenario does not hold out of the box. The realistic residual impact is API-key disclosure + replay to externally-allowed targets and exfiltration of extracted page content — real, but narrower. Second, the entire agent subsystem is explicitly deprecated with a removal date of 2026-07-23 (agent.go:20-29), the default bind is loopback (localhost:9000) where cleartext is harmless, and the vulnerable state only arises from an operator's conscious choice of a non-loopback bind. Net: a real cleartext-transport finding, but low severity given the opt-in/deprecated path and the default-deny SSRF gate defeating the worst-case pivot.

**Verifier notes:** Verified: server.go has no TLS wiring; agent.go loopback guard is bypassed when --api-key is set; SSRF default-deny policy (SCOUT_ALLOW_LOCAL_TARGETS off by default) blocks the metadata-endpoint pivot unless operator opts in with --allow-local-targets. Subsystem is deprecated (removal 2026-07-23). Relevant files: D:\weaver-sync\development\personal\projects\scout\pkg\scout\agent\server.go (lines 24-39, 380-441), D:\weaver-sync\development\personal\projects\scout\cmd\scout\agent.go (lines 64-130), D:\weaver-sync\development\personal\projects\scout\pkg\scout\agent\agent_test.go (898-904), D:\weaver-sync\development\personal\projects\scout\pkg\scout\urlpolicy\config.go. Recommendation in the finding (add TLS, make it mandatory for non-loopback binds) is sound; alternatively, since the subsystem is being removed, may be deprioritized in favor of the documented MCP migration.

---

### [LOW] API proxy server has no SSRF policy, binds all interfaces with no auth, and injects caller-controlled params into the target URL

- **Dimension:** ssrf | **Location:** `pkg/scout/proxy/proxy.go:176` | **CWE:** CWE-918: Server-Side Request Forgery (SSRF)
- **Original severity:** medium -> **adjusted:** low | **Exploitability:** plausible | **Confidence:** medium

**Description:** The proxy server turns websites into REST endpoints by navigating a browser to a target URL on each request. It applies NO urlpolicy/SSRF control, defaults to binding all interfaces with no authentication (cmd/scout/proxy.go builds addr as ":"+port, default :8080), and builds the target URL by substituting caller-supplied query parameters into the operator's target template via raw strings.ReplaceAll. If any route templates a parameter into the host/scheme position (or uses a param that an attacker can pad to alter the authority), an unauthenticated remote caller controls where the host browser fetches and what content is returned to them. Even with path-only params, the unauthenticated, all-interfaces exposure of a browser-backed fetcher is an egress pivot.

**Evidence:**

```
proxy.go:179-183 `targetURL := route.Target` then `targetURL = strings.ReplaceAll(targetURL, "{{."+param+"}}", val)` with val from `r.URL.Query().Get(param)` (no validation/encoding); proxy.go:235 `browser.NewPage(targetURL)` with no policy check anywhere in scrapeRoute/handleRoute; cmd/scout/proxy.go:37-39 `if addr == "" { addr = ":" + port }` (default port 8080 -> binds 0.0.0.0) with no auth middleware and no loopback guard (contrast agent.go:79-87 which refuses non-loopback without auth).
```

**Exploit scenario:** An operator deploys `scout proxy start` with a route `target: "https://api.example.com/{{.host}}/data"` on the default :8080. A remote attacker requests the route with host=`@169.254.169.254` (or any value that re-points the authority for the templated host), or simply reaches the unauthenticated service to drive browser fetches to internal hosts, reading the scraped JSON back over HTTP.

**Recommendation:** Apply urlpolicy.Check to the fully-resolved targetURL before navigation in scrapeRoute. URL-encode substituted parameters and validate they cannot alter the scheme/authority of the target. Default the proxy bind to loopback and require an explicit flag plus authentication to bind non-loopback (mirror the agent server's isLoopbackHost guard). Document the proxy as an untrusted-input network service.

**Verifier reasoning:** Code facts are all verified. proxy.go:179-183 substitutes caller-controlled query-param values into the target URL via raw strings.ReplaceAll with no encoding; proxy.go:235 navigates the browser to that URL with no urlpolicy check anywhere in scrapeRoute/handleRoute; a grep of the whole proxy package found zero auth/middleware/loopback-guard code; and cmd/scout/proxy.go:37-39 defaults addr to ":"+port (=":8080"), which Go binds on all interfaces. The contrast the finding draws is real and self-indicting: pkg/scout/urlpolicy exists in-tree (its own doc says it is enforced "at the MCP server and agent REST API only"), the agent both calls policy.Check (agent.go:215-216) AND refuses a non-loopback bind without an API key (cmd/scout/agent.go:79-87), while the proxy — an equivalent unauthenticated, all-interfaces, browser-backed fetcher — wires in neither. So a defense the codebase already ships is missing from a comparable trust boundary.

However, the finding overreaches on the headline exploit and thus on severity/confidence. The Target template is OPERATOR-controlled (from routes.yaml); only the param VALUES are caller-controlled. The specific claim that host=@169.254.169.254 "re-points the authority" is false for a path-position param: https://api.example.com/{{.host}}/data becomes https://api.example.com/@169.254.169.254/data, where the @ sits after the authority-terminating '/', so the host is unchanged. URL-authority injection only works if the operator templates the param into the host/scheme position (e.g. target: "https://{{.host}}/..." or "{{.url}}"). That is a plausible pattern given the advertised "{{.param}}" feature, and when used it yields full unauthenticated SSRF to internal hosts / cloud metadata (urlpolicy.isInternalIP, policy.go:131-139, would otherwise block link-local 169.254/16 and private ranges). With path-only params there is no host control — only a fixed-host egress/recon/DoS-amplification pivot.

Net: real missing-mitigation issue with a concrete but config-dependent SSRF path, not the guaranteed "any value re-points authority" primitive claimed. Impact is gated by operator route design, so medium is too high; Low with plausible exploitability is the honest rating.

**Verifier notes:** Remediation as written is correct: (1) call urlpolicy.Check(ctx, resolvedTargetURL) in scrapeRoute before browser.NewPage, reusing urlpolicy.FromEnv() like agent/provider.go does; (2) default the proxy bind to loopback and mirror agent.go's isLoopbackHost guard requiring an explicit non-loopback opt-in plus auth; (3) URL-encode substituted param values and/or validate they cannot introduce authority delimiters. The maintainers should also note the host-position-template route pattern as the actually-dangerous configuration in docs. The finding's "@169.254.169.254 re-points authority" example should be corrected before this goes into any ticket — it is technically wrong for path-position params.

---

### [LOW] gops diagnostic agent exposes unauthenticated local control endpoint on every Scout process (binary dump, heap/CPU profiling, GC/DoS control)

- **Dimension:** proc_perms | **Location:** `cmd/scout/scout.go:149` | **CWE:** CWE-419: Unprotected Primary Channel
- **Original severity:** medium -> **adjusted:** low | **Exploitability:** plausible | **Confidence:** high

**Description:** main() calls agent.Listen(agent.Options{ShutdownCleanup: true}) unconditionally for EVERY scout subcommand, including the long-lived secret-handling processes (`scout daemon`, `scout server` MCP, `scout agent serve`). With an empty Addr, the gops agent (github.com/google/gops v0.3.29) binds a TCP listener on 127.0.0.1:0 (a random loopback port) with NO authentication. The loopback interface is NOT user-isolated: any other local user/process on the host can connect to that port. The agent's handle() dispatches single-byte signals to: BinaryDump (streams the running scout executable to the caller), HeapProfile / StackTrace / CPUProfile / Trace (memory and goroutine introspection that can leak in-memory secrets such as vault-decrypted material, OAuth tokens, scraper auth cookies, and CDP session data present on the heap), and SetGCPercent / CPUProfile(30s)/Trace(5s) (denial of service via forced GC and profiling stalls). The agent additionally persists its port to <UserConfigDir>/gops/<pid> using os.WriteFile(..., os.ModePerm) = 0o777 (agent.go:158), though an attacker does not even need that file since loopback ports are trivially scannable.

**Evidence:**

```
cmd/scout/scout.go:149  `if err := agent.Listen(agent.Options{ShutdownCleanup: true}); err != nil {`  — no Addr set, so gops/agent/agent.go:33 `const defaultAddr = "127.0.0.1:0"` is used; agent.go:96 `listener, err = lc.Listen(context.Background(), "tcp", addr)`; agent.go:268-280 BinaryDump opens os.Executable() and copies it to the connection; agent.go:255-262 HeapProfile/CPUProfile; agent.go:287-292 SetGCPercent. Port file: agent.go:158 `return os.WriteFile(portfile, []byte(strconv.Itoa(port)), os.ModePerm)`.
```

**Exploit scenario:** On a shared/multi-user machine running `scout daemon` (the gRPC daemon that holds decrypted vault secrets and OAuth tokens in memory), a second unprivileged local user scans 127.0.0.1, finds the scout process's gops port, sends the BinaryDump signal to exfiltrate the binary and the HeapProfile signal to dump heap pages that may contain plaintext cookies/tokens, or repeatedly sends CPUProfile/Trace/GC signals to stall the daemon (DoS) — none of which require any credential.

**Recommendation:** Do not start the gops agent by default, especially for long-lived/secret-handling commands. Gate it behind an explicit opt-in env/flag (e.g. only when SCOUT_DEBUG_GOPS=1). If process discovery via gops is required for orphan detection, prefer the gops file-based registration without the network control endpoint, or bind to a Unix domain socket / abstract socket restricted to the current uid, or set agent.Options.Addr to an OS-assigned ephemeral port behind a 0o600 socket and document the local-trust caveat. At minimum, never run agent.Listen in the daemon, MCP server, and agent serve processes that hold secrets.

**Verifier reasoning:** VERIFIED REAL but partially overstated; impact downgraded to low.

Call site confirmed: cmd/scout/scout.go:149 calls agent.Listen(agent.Options{ShutdownCleanup:true}) unconditionally in main(), so it runs for EVERY subcommand including the long-lived secret-handling ones (scout daemon, MCP server, agent serve). There is no env/flag gate, no Addr, and no ConfigDir override.

gops v0.3.29 source verified at C:\Users\dyamm\go\pkg\mod\github.com\google\gops@v0.3.29\agent\agent.go: empty Addr -> defaultAddr "127.0.0.1:0" (line 33/86-88); lc.Listen("tcp", addr) with NO authentication (96); port file written with os.ModePerm=0o777 (158); handle() (207-295) dispatches single bytes to BinaryDump streaming os.Executable() to the connection (268-280), CPUProfile (30s blocking stall, 257-262), Trace (5s stall, 281-286), SetGCPercent forced-GC (287-292), StackTrace/MemStats/Stats introspection. gops' own doc comment (74-76) warns the endpoint "can be used by any program on the system."

Trust-boundary crossing is genuine: on standard Linux/Windows the loopback interface is NOT uid-isolated, so a second unprivileged local user/process can connect to the random 127.0.0.1 port (trivially scannable; the 0o777 port file isn't even needed). That is an untrusted local principal reaching an unauthenticated control sink across a uid boundary. Solidly real, unauthenticated consequences: (1) BinaryDump = exfiltrate the running executable; (2) DoS = repeatable 30s CPUProfile / 5s Trace stalls and forced GC; (3) limited introspection via StackTrace/MemStats.

Why downgraded from medium to low: (a) The most alarming claim is inaccurate. pprof.WriteHeapProfile (HeapProfile signal) emits ALLOCATION PROFILES (call stacks + sizes), NOT raw heap buffer contents, so vault material, OAuth tokens, and cookies would NOT be dumped by it. The vault additionally holds secrets in LockedBuffer with VirtualLock/Mlock + explicit zeroing (pkg/scout/vault/secmem.go). So the headline "dump heap pages containing plaintext secrets" does not hold; realistic info-leak is limited to goroutine stacks/metadata. (b) Exploit is conditional on a genuinely multi-user host running a long-lived scout daemon — uncommon for this developer-tool's dominant single-user desktop/dev usage, where no second principal exists and no boundary is crossed. (c) Realistic impact is therefore binary disclosure + local DoS + limited introspection, not credential theft.

Recommendation is sound and the fix is low-cost: Scout's actual gops dependency (session.IsScoutProcess via goprocess.Find, verified in process_unix.go/process_windows.go) inspects process build info and does NOT require the network control endpoint at all, so the agent could be gated behind an opt-in env (e.g. SCOUT_DEBUG_GOPS=1) or omitted from the daemon/MCP/agent-serve processes with no loss of functionality.

Exploitability = plausible: trivial single-byte signals once reachable, but gated behind the multi-user precondition which is atypical for this tool.

**Verifier notes:** Concrete fix: do not call agent.Listen by default; gate behind SCOUT_DEBUG_GOPS opt-in, and never enable it in scout daemon / MCP server / agent serve. Orphan detection via goprocess.Find() works without the TCP endpoint. Correct the report's heap-leak claim: HeapProfile emits allocation metadata, not buffer contents; the realistic vectors are BinaryDump (executable disclosure), DoS (CPUProfile/Trace stalls, forced GC), and goroutine-stack introspection. Severity is low for the typical single-user deployment and rises toward medium only on shared multi-user hosts.

---

### [INFO] govulncheck reports no known-vulnerable dependencies (clean toolchain + module graph)

- **Dimension:** deps_cve | **Location:** `go.mod:1` | **CWE:** CWE-1395: Dependency on Vulnerable Third-Party Component
- **Original severity:** info -> **adjusted:** info | **Exploitability:** not-exploitable | **Confidence:** high

**Description:** Adversarial dependency/toolchain review of the Scout module. Ran govulncheck v1.3.0 (Go vuln DB updated 2026-06-02, 12 days before the audit date) against Go 1.26.4 in both source-reachability mode ('govulncheck ./...') and module mode ('govulncheck -scan module'). Both passes returned 'No vulnerabilities found' with 0 findings across 158 considered OSV entries. The security-sensitive dependencies named in scope are all at patched versions: golang.org/x/net v0.55.0 (the most recent relevant advisory, GO-2026-4918 / CVE-2026-33814, is fixed in 0.53.0 and is therefore already satisfied; all prior x/net HTTP/2 and parsing advisories up to GO-2025-3503/CVE-2025-22870 fixed in 0.36.0 are likewise satisfied), golang.org/x/crypto v0.52.0, google.golang.org/grpc v1.81.1, golang.org/x/oauth2 v0.36.0, google.golang.org/protobuf v1.36.11, and github.com/modelcontextprotocol/go-sdk v1.4.1. There are no 'replace' directives masking a vulnerable fork. The CLAUDE.md-declared deps ollama, gin-gonic/gin, mattn/go-sqlite3 and charmbracelet/bubbletea are NOT present in the actual build graph ('go list -m' / go.sum confirm absence) - the tech-stack notes are stale, so they contribute no CVE surface. The npm package (npm/scout-browser/package.json) declares zero runtime dependencies (only a postinstall binary download), so there is no third-party npm supply-chain CVE surface. This entry documents the clean result; no remediation is required.

**Evidence:**

```
govulncheck ./...  -> 'No vulnerabilities found.'
govulncheck -scan module -> 'No vulnerabilities found.' (Scanner govulncheck@v1.3.0, DB https://vuln.go.dev, DB updated 2026-06-02; Go 1.26.4)
JSON parse of module scan: osv considered: 158 | findings: 0
Key advisory check: GO-2026-4918 (CVE-2026-33814) golang.org/x/net fixed in 0.53.0; in-use version go.mod:25 'golang.org/x/net v0.55.0' (>= 0.53.0, patched). go.mod:23 'golang.org/x/crypto v0.52.0', go.mod:30 'google.golang.org/grpc v1.81.1'. No 'replace' directives in go.mod.
```

**Exploit scenario:** No exploit: govulncheck confirms no reachable or at-version known-vulnerable dependency exists in the build graph, so there is no dependency-CVE attack path across any of Scout's trust boundaries (gRPC daemon, agent HTTP, MCP, plugin install, capture host).

**Recommendation:** No action required for known CVEs. Maintain hygiene by wiring 'govulncheck ./...' into CI (the reusable inovacc/workflows already runs vulncheck) so regressions are caught, and correct the stale CLAUDE.md tech-stack notes that list ollama/gin/sqlite/bubbletea as dependencies when they are absent from the module graph.

**Verifier reasoning:** This is an informational clean-result entry, and I independently reproduced every material claim. (1) Read go.mod end-to-end: dependency versions match exactly (x/net v0.55.0, x/crypto v0.52.0, grpc v1.81.1, oauth2 v0.36.0, protobuf v1.36.11, go-sdk v1.4.1), toolchain go1.26.4, and there are zero replace directives. (2) Ran govulncheck v1.3.0 (same scanner version and DB date 2026-06-02 as cited) in both modes myself: 'govulncheck -scan module' and 'govulncheck ./...' both returned 'No vulnerabilities found.' (3) JSON output confirms 158 OSV entries considered (exact match) with 0 finding records / no GO-XXXX advisory IDs flagged. (4) The x/net advisory math is correct: GO-2026-4918 is fixed in 0.53.0 and the in-use 0.55.0 satisfies it. (5) grep of go.sum confirms ollama/gin/sqlite/bubbletea are absent from the build graph (stale CLAUDE.md notes). (6) npm package.json declares zero runtime dependencies, only a postinstall script. The finding asserts there is NO dependency-CVE attack path across any trust boundary, and that assertion is accurate and fully evidenced. There is no untrusted input, no sink, and no exploit (the finding itself states 'No exploit'). Severity info is correct; not-exploitable is honest because the entry documents the absence of a vulnerability. Minor nit: '-format json' has no literal 'osv considered' line, but the count of streamed OSV records does equal 158, so the paraphrase is fair.

**Verifier notes:** Treating is_real as 'the documented security claim is accurate.' This is a clean-result/hygiene entry, not a vulnerability. Recommendation to wire govulncheck into CI is already partially satisfied per CLAUDE.md (inovacc/workflows runs vulncheck). The only actionable item is correcting stale CLAUDE.md tech-stack notes — a documentation fix, not security remediation. No rotation/no code change needed.

---
