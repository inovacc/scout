# Flow-Capture → Deterministic Go Codegen — Idea Capture (QUEUED)

- **Date:** 2026-06-02
- **Status:** Captured, NOT yet brainstormed. Queue position: after `feat/session-hardening`, alongside the secrets-vault (user sequencing: "finish session-hardening first").
- **Source ask:** "create an agent and subagents to capture behavior and replicate into code — a browser flow becomes a Go 'job app'; no web scraping, only capture HAR/API call/payload and convert to deterministic code."

## The idea in one line
Record a real browser flow's network traffic (HAR + hijack: requests, responses, payloads, headers, auth), then **generate a standalone deterministic Go program** that reproduces the underlying API calls directly — no browser, no DOM scraping.

## How it maps onto what scout ALREADY has (huge head start — verify at brainstorm)
- **Hijack capture** (`internal/engine/hijack/`): real-time HTTP+WS capture via CDP, `CapturedRequest`/`CapturedResponse`/`WebSocketFrame`, body capture, HAR export. → this IS the capture layer.
- **HAR recorder** + `ExportHAR()` / WS HAR extension. → the input artifact.
- **Runbooks** (`pkg/scout/runbook/`): extract → automate → analyze, Plan/Apply. → closest existing "record→replay" abstraction; codegen could be a new runbook sink/target.
- **Guide recorder** (`pkg/scout/guide/`): records step-by-step flows. → behavior capture precedent.
- **Strategy files** (`pkg/scout/strategy/`): declarative YAML/JSON workflows + sinks. → a "go-codegen" sink is a natural fit.
- **Proxy** (`pkg/scout/proxy/`): YAML routes + browser extraction + caching. → related replay surface.
- **CLI precedent:** `scout hijack watch`, `scout gather --har`, `scout runbook`.

So the feature is plausibly: `scout` captures (existing) → an **analyzer** correlates the call graph (which responses feed which later request params: auth tokens, CSRF, IDs, cursors) → a **codegen** emits Go using `net/http` (or scout's HTTP client) that replays the calls deterministically, with dynamic values parameterized/extracted from prior responses.

## The "agent + subagents" angle
Likely an AI-assisted pipeline (Claude Code agent + subagents, mirroring this repo's `.claude/agents/` pattern) that:
1. ingests the HAR/hijack capture,
2. a subagent classifies calls (auth / data / noise / static asset),
3. a subagent infers value-correlation (response field → later request param),
4. a subagent synthesizes idiomatic, deterministic Go (typed structs from JSON payloads, sequenced calls, externalized secrets/params),
5. a verifier subagent runs the generated job against golden HAR and diffs.

## Load-bearing open questions (for the real brainstorm)
1. **Output target:** standalone Go module (`go build`-able job app) vs a scout plugin/runbook vs a `scout job run` artifact?
2. **Scope:** HTTP REST only first? Include WebSocket replay? GraphQL? multipart/streaming?
3. **Dynamic correlation:** how aggressively to auto-detect token/CSRF/ID chaining vs require human annotation? (the hard part — determinism vs real-world session coupling.)
4. **Secrets:** generated code must externalize auth (ties DIRECTLY to the secrets-vault feature — the job app would pull creds from a vault profile, not hardcode them).
5. **Codegen mechanism:** template-based deterministic emitter vs LLM-synthesized vs hybrid (LLM for structure, deterministic for the call sequence).
6. **Verification:** golden-HAR replay diff as the acceptance gate (parity testing).
7. **Capture source:** reuse `scout hijack`/`gather --har` as the front door; new `scout flow capture` + `scout flow codegen <har>`?

## Dependency note
This feature's "externalize secrets" requirement (Q4) depends on the **secrets-vault** feature — sequence vault before (or with) the generated-job auth story.
