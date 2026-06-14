# Field Report — `browser-automation` runaway under model-API rate limiting

- **Date:** 2026-06-14
- **Reporter:** External integration (Claude Code, driving the `tally` project)
- **Scout build:** `dev` (loaded as a Claude Code plugin via the marketplace)
- **Agent exercised:** `scout:browser-automation` (see `agents/browser-automation.md`)
- **Severity:** High (usability) — any upstream model-API throttling turns a trivial task into a multi-hour, high-cost no-op.

## What I was trying to do

A deliberately simple browser task: log into a local web app
(`http://127.0.0.1:8787`, a Go + React-Flow SPA), open one page (`/wire`),
take a screenshot, and count the rendered nodes/edges to confirm a topology.
Login + navigate + screenshot + DOM count — one sequential browser session's
worth of work.

## What actually happened (observed)

- The single `browser-automation` invocation **fanned out into 150+ internal
  agent runs** (the swarm engine).
- **Every run failed with the identical upstream error:**
  `API Error: Server is temporarily limiting requests (not your usage limit) · Rate limited`
  (an Anthropic API HTTP 429 returned to each agent's model call).
- Runs **did not abort early.** Individual agent durations ran into the
  millions of milliseconds (up to ~5.27M ms ≈ **88 minutes**), and the swarm
  **kept dispatching new runs** while every call was being throttled.
- **Net result:** 0 screenshots, 0 analysis, 150+ failed agent runs,
  an estimated 300k+ tokens burned, hours of cumulative wall-clock, and a flood
  of ~150 identical failure notifications to the caller. The parent agent
  itself also terminated with the same rate-limit error and no result.

## Root-cause hypotheses (grounded in the code)

1. **Two different "rate limits" are conflated / one is missing.** The swarm's
   rate-limit machinery (`internal/engine/swarm/queue.go` `DomainQueue`,
   `internal/engine/swarm/types.go` `DefaultRateLimit: time.Second`) governs
   **politeness toward target *website* domains** — how often to hit a scraped
   site. There is **no handling for the upstream *model API* (Anthropic)
   throttling the agents themselves.**
2. **No 429 backoff/retry in the model path.** A grep of `pkg/scout/aihost/**`
   for `429|rate.?limit|backoff|RetryAfter|MaxRetr` found nothing outside the
   domain-politeness queue. A throttled model call appears to fail the agent
   immediately rather than back off and retry.
3. **No circuit breaker linking model-API failures to the coordinator.** While
   calls are being 429'd, `internal/engine/swarm/coordinator.go` keeps
   dispatching workers — a thundering herd that *compounds* the throttling.
   (`coordinator_cap_test.go` implies a concurrency cap exists; if so it did
   not prevent the storm here — worth verifying it applies to the
   `browser-automation` path.)
4. **Disproportionate fan-out.** A login + screenshot task should be one
   sequential browser session, not a large swarm.
5. **No aggregated failure surfacing.** The caller received ~150 identical
   notifications instead of one actionable "aborted: model API rate-limited".

## Recommendations (actionable)

- **A. Separate the two concepts explicitly:** (i) target-site politeness
  (existing `DomainQueue`) vs (ii) upstream model-API throttling (missing).
  Add first-class handling for (ii).
- **B. Backoff on 429 in `aihost/claude`:** exponential backoff + jitter,
  honor `Retry-After`, cap attempts. This alone would have let the task ride
  out a transient throttle.
- **C. Swarm circuit breaker:** after K consecutive model-API 429s across
  workers, pause dispatch (global cooldown); if it persists past a deadline,
  **abort the whole run and return a single clear error** rather than spawning
  more agents.
- **D. Bound simple tasks:** cap concurrency / total agent count for
  interactive single-target tasks; login+navigate+screenshot must not fan out.
- **E. Fail fast & propagate:** the first *persistent* 429 should short-circuit
  in seconds, not run for ~88 minutes per worker.
- **F. One aggregated result to the caller:** e.g. `"M/N agents failed
  (model API rate-limited); aborted after backoff; suggested retry-after Xs"`.
- **G. Non-LLM fast path for deterministic work.** "Screenshot a URL" and
  "count DOM nodes" need no model orchestration. Scout already has rod-based
  capture (`pkg/scout/capture`); exposing a direct screenshot/inspect path for
  the browser-automation agent's simple cases would have completed this task in
  seconds and been **immune to the model-API outage entirely.**
- **H. Localhost reachability is unverified.** Because every run died at the
  model step, I could not confirm Scout's browser can reach `127.0.0.1` on the
  host. If browsers run in a container/sandbox where `127.0.0.1 != host`, local
  targets will silently fail. Add a documented local-target smoke test.

## Repro

Invoke `scout:browser-automation` against any target while the Anthropic API is
returning 429s (or under a deliberately low model-API rate limit). Observe the
fan-out with no convergence and no early abort.

## Ground-truth fixture (the expected output, for a regression test)

The task *should* have produced this. Offered so Scout has a known-good target
and a baseline for the "fast path" in recommendation G. For `/wire` on entity
`inovacloud-br`:

- **6 nodes:** 1 company `INOVACLOUD CONSULTORIA LTDA`; 4 clients
  `JAVALI HOLDING PATRIMONIAL SA`, `ORION SOFTWARE GMBH` (R$ 31.200,00),
  `NORTHWIND CAPITAL LLC` (R$ 42.500,00), `ACME COMERCIO DIGITAL LTDA`
  (R$ 9.800,00); 1 service `convenia` nested inside the Javali client.
- **5 edges:** 4 `service_provider` (each client → company) + 1 `grant`
  (convenia service → company).
- **No "No topology yet" empty card; zero console errors.**

For reference, I verified exactly this with a ~30-line Playwright script
(API login → `goto /wire` → count `.react-flow__node` / `.react-flow__edge-path`
→ screenshot) in **under 2 seconds** — i.e. the deterministic path that
recommendation G would expose survives the model-API outage that took down the
agent swarm.

## Suggested triage

Promote items B/C (backoff + circuit breaker) into `docs/BUGS.md` /
`docs/ISSUES.md` as the high-priority fix; items D–G into `docs/BACKLOG.md`.
