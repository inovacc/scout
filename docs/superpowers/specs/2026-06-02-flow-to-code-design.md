# Scout Flow — Capture → Analyze → Replay (Flow-to-Code) Design Spec

- **Date:** 2026-06-02
- **Status:** Design (brainstorm complete, all decisions locked — ready for `writing-plans`)
- **Idea-capture record:** `docs/superpowers/specs/2026-06-02-flow-to-code-NOTES.md`
- **Supersedes:** the NOTES file's 7 open questions (resolved below).
- **Dependency:** secrets-vault (`pkg/scout/vault`) — **shipped 2026-06-02**; provides `auth.profile`.

## 1. Problem

A real browser flow (login → navigate → act) is driven through the DOM, but underneath it is a sequence of HTTP/GraphQL API calls with dynamic, per-session values (auth tokens, CSRF tokens, IDs, cursors). Re-running that flow today means re-driving a browser (slow, brittle, DOM-coupled). We want to **record the flow once and replay the underlying API calls deterministically with no browser** — turning a captured flow into a reviewed, runnable Scout job.

## 2. Locked decisions (from brainstorm)

| # | Decision | Choice |
|---|----------|--------|
| Output target | What the artifact is | **Scout-hosted job artifact** — runs via `scout flow run`, reuses Scout's HTTP client, vault, logging, tracing. NOT a standalone Go module. |
| Artifact form | How the flow is expressed | **Declarative job spec (`flow.yaml`)** interpreted by a new Scout job runtime — NOT generated Go. Consistent with `strategy`/`runbook`/`proxy`. No `go build` needed. |
| Correlation | How dynamic-value chains are detected | **AI analyzer (agent + subagents)** — staged LLM passes classify calls and infer correlations semantically, emit the spec; **human reviews/approves**. Non-determinism is confined to compile time. |
| Scope (v1) | Protocols | **HTTP REST/JSON + GraphQL.** Defer WebSocket, multipart/file-upload, SSE/streaming. |
| Architecture | Where the analyzer lives | **Scout-native pipeline (Approach A)** — `scout flow capture/analyze/run/verify`; analyze uses Scout's own `internal/engine/llm` provider in staged passes. Self-contained; works in CI/headless. |
| Secrets | Auth handling | Generated spec references a **vault profile id** (`auth.profile`); the runtime resolves secrets via `pkg/scout/vault` at run time and zeroes them after. Specs **never embed raw secrets**. |
| Verification | Acceptance gate | **Golden-HAR replay diff** (`scout flow verify`) — re-run the flow and diff actual vs captured. |

**Reconciliation of "AI agent" + "deterministic":** the AI does the *one-time* semantic understanding (classify + correlate) and emits a spec; the human reviews it; then it replays deterministically forever. The reviewed `flow.yaml` is the contract between the non-deterministic analyzer and the deterministic runtime.

## 3. Architecture

```
 scout flow capture        scout flow analyze          (human review)      scout flow run         scout flow verify
 browser + hijack/HAR   →  staged LLM passes        →  edit flow.yaml   →  deterministic      →   golden-HAR
 (existing engine)         classify→correlate→emit      + report.md         runtime (no browser)    parity diff
```

Stage boundaries are clean and independently testable: **capture** produces a normalized artifact; **analyze** is the only non-deterministic stage and its output is human-gated; **run** is pure replay; **verify** is a diff.

## 4. Components — new `pkg/scout/flow/` + `cmd/scout/flow.go`

| File | Responsibility |
|------|----------------|
| `pkg/scout/flow/spec.go` | `FlowSpec` type + YAML/JSON (un)marshal + `Validate()`. The schema is the central abstraction (§5). |
| `pkg/scout/flow/capture.go` | Orchestrates a guided capture over existing `internal/engine/hijack` + HAR → normalized `capture.har` (+ captured bodies, de-duplicated). Thin wrapper; reuses `Browser.NewSessionHijacker` + `ExportHAR`. |
| `pkg/scout/flow/analyze.go` | The AI analyzer. Loads a capture, runs staged passes via `internal/engine/llm` (classify calls → infer correlations → synthesize spec), emits `flow.yaml` + `analysis-report.md`. The "subagents" are the staged passes. |
| `pkg/scout/flow/runtime.go` | Deterministic executor: parse `FlowSpec`, run steps in order with `net/http`, apply extract/inject, resolve auth from vault, emit results + `internal/tracing` spans. Handles REST + GraphQL step types. |
| `pkg/scout/flow/extract.go` | Extraction DSL: pull named vars from a response by JSONPath (`$.a.b`), header name, or regex. |
| `pkg/scout/flow/inject.go` | Injection DSL: substitute `${var}` into a request's url/headers/query/json/graphql-variables before sending. |
| `pkg/scout/flow/verify.go` | Re-run the flow and diff actual responses vs the golden capture (status + JSON shape drift); report, don't crash. |
| `cmd/scout/flow.go` | CLI: `scout flow capture|analyze|run|verify` (Cobra, registered to `rootCmd`). |

**Reuses (no new heavy deps):** `internal/engine/hijack` (capture), `internal/engine/llm` (analyze), `pkg/scout/vault` (auth), `internal/tracing` (spans), `x/net/html`/`gjson`-style JSON nav already in the tree where possible.

## 5. The `FlowSpec` schema (key abstraction)

```yaml
version: "1"
name: checkout-flow
auth:
  profile: "<vault-profile-id>"          # secrets resolved at run time; never embedded
vars:                                     # optional seeded inputs (overridable via --var)
  baseURL: "https://api.example.com"
  cartId: "c-123"
steps:
  - id: login
    request:
      method: POST
      url: "${baseURL}/login"
      headers: { Content-Type: application/json }
      json: { username: "${secret.USERNAME}", password: "${secret.PASSWORD}" }   # from vault profile
    extract:
      - { var: token, from: response.json,   path: "$.access_token" }
      - { var: csrf,  from: response.header, path: "X-CSRF-Token" }
    expect: { status: 200 }
  - id: cart
    request:
      method: POST
      url: "${baseURL}/graphql"
      headers: { Authorization: "Bearer ${token}", X-CSRF-Token: "${csrf}" }
      graphql:
        operationName: Cart
        query: "query Cart($id: ID!) { cart(id: $id) { id total } }"
        variables: { id: "${cartId}" }
    extract:
      - { var: total, from: response.json, path: "$.data.cart.total" }
    expect: { status: 200 }
```

- **`extract`**: `{ var, from: response.json|response.header|response.body, path }` — JSONPath, header name, or regex; binds a named var.
- **`inject`** is implicit via `${...}` templating anywhere in a later request (url/headers/json/graphql.variables).
- **`${...}` resolution (single syntax, explicit namespaces):** `${secret.NAME}` → a secret from the `auth.profile` vault profile (resolved at send time, zeroed after); `${var.NAME}` or bare `${NAME}` → a seeded `vars` entry **or** an `extract`-bound variable, where an extracted var shadows a seeded one of the same name. Unknown `${...}` is a `Validate()` error. This keeps secret references greppable and auditable.
- **`auth.profile`**: a `pkg/scout/vault` profile id. At run time the runtime opens the vault, injects the profile's cookies/headers, resolves `${secret.*}` references, and zeroes the buffers after the run.
- **`graphql` step type**: when present, the analyzer/runtime treat `operationName`/`query`/`variables` specially (variables are first-class injection points; the query string is preserved verbatim).
- **`expect`**: optional per-step assertion (status; later: response-shape) used by `run` (fail) and `verify` (diff).

## 6. Data flow / CLI

1. `scout flow capture <url> [--session <id>] [-o capture.har]` — drive a browser (existing engine), record HTTP + HAR, write a normalized `capture.har` (+ bodies).
2. `scout flow analyze <capture.har> [--llm <provider>] [-o flow.yaml]` — staged LLM passes → `flow.yaml` + `analysis-report.md`. **Human edits/approves** the spec + report.
3. `scout flow run <flow.yaml> [--profile <vaultID>] [--var k=v]` — deterministic replay, no browser → results (JSON/stdout) + tracing spans.
4. `scout flow verify <flow.yaml> --golden <capture.har>` — re-run + diff actual vs golden; the parity acceptance gate.

## 7. Error handling

- **Capture:** reuse hijack error handling; partial captures are still analyzable.
- **Analyze:** LLM/provider failure → emit a **heuristic skeleton spec** (raw call sequence, no inferred chains) with *everything* flagged for review in `analysis-report.md` — never silently produce a wrong-but-plausible spec. Low-confidence correlations are marked with their confidence.
- **Run:** per-step failure wrapped `scout: flow: step <id>: %w`; `expect` mismatch fails the step with the actual vs expected; secrets resolved from vault are zeroed after the run (`Handle.Close`).
- **Verify:** drift is reported as a structured diff (status + shape), never a crash.

## 8. Testing (real servers, no mocks where avoidable)

- **runtime** against `httptest.Server` replaying a known REST + GraphQL flow: assert extract binds vars and inject substitutes them across steps (the deterministic core).
- **analyze** against a checked-in golden `capture.har` fixture: assert the **deterministic post-processing** of the LLM output (spec assembly, GraphQL detection, de-noising) — the LLM call itself stubbed/faked at the provider boundary so the test is deterministic.
- **verify** parity self-test: a flow that matches its golden passes; a drifted one reports the diff.
- **capture** real-browser test (skip if Chromium unavailable, per the suite's `newTestBrowser` pattern).
- **secrets hygiene** test: a `FlowSpec` round-trip never serializes a raw secret value — only `${secret.*}` refs + `auth.profile` (mirrors the vault package's `string`-conversion guard).

## 9. Scope

**In scope (this spec → one plan):** `pkg/scout/flow` (`FlowSpec` + capture wrapper + staged-LLM analyzer + deterministic runtime with extract/inject + verify), the 4-verb CLI, REST/JSON + GraphQL, vault-sourced auth, golden-HAR verification.

**Out of scope → BACKLOG:**
- WebSocket replay (Scout already *captures* WS; replay is stateful/timing-sensitive — separate spec).
- multipart/file-upload + SSE/streaming request/response bodies.
- a fully-automatic "no human review" analyze mode.
- emitting a standalone Go module (the rejected Output-target option) — could be a later `flow export --go` if demand appears.

## 10. Security / threat model

- Specs are **secret-free**: only `${secret.*}` references + an opaque `auth.profile` id. A hygiene test enforces this.
- At run time secrets live in vault `LockedBuffer`s and are zeroed after the run; auth never reaches a child process via env (consistent with the vault's CDP/`[]byte` model).
- `analysis-report.md` must not echo captured secret values — the analyzer redacts header/cookie/token *values* (keeps names + classification).

## 11. Success criteria

1. `scout flow capture` + `scout flow analyze` turn a real REST+GraphQL login-and-act capture into a `flow.yaml` whose inferred token/CSRF/ID chains are visible in `analysis-report.md` for review.
2. `scout flow run <flow.yaml> --profile <id>` reproduces the flow's API calls **with no browser**, pulling auth from the vault, with **no secret value in the spec or logs**.
3. `scout flow verify` re-runs the flow and reports status/shape parity vs the golden capture.
4. The runtime's extract/inject chaining is proven deterministic against `httptest` (REST + GraphQL) without any LLM in the run path.
5. Analyze degrades safely: on LLM failure it emits a review-flagged skeleton, never a silently-wrong spec.
