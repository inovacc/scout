# Profile → Vault Secret Migration — Design

**Date:** 2026-06-09
**Status:** Approved (brainstorming complete; ready for implementation plan)
**Topic:** Migrate the deprecated `UserProfile.Cookies/Storage/Headers` secret fields onto `pkg/scout/vault`, so the fields can be dropped on their scheduled date.

## Context

`internal/engine/profile.go` defines `UserProfile`, a portable browser identity that mixes
**non-secret** configuration (browser type/window, identity UA/lang/tz/locale, extensions, proxy,
notes) with three **secret-bearing** fields that were deprecated 2026-06-02:

- `Cookies []Cookie`
- `Storage map[string]ProfileOriginStorage` (per-origin localStorage + sessionStorage)
- `Headers map[string]string`

These are marked **removal after 2026-07-02** (`docs/BACKLOG.md`, P2 DEPRECATION). The sanctioned
replacement is `pkg/scout/vault` — an Argon2id + AES-256-GCM, `LockedBuffer`/mlock secret store.
This spec migrates every caller off the secret fields so the dated removal can proceed cleanly.

### Real migration surface (verified 2026-06-09)

- `internal/engine/profile.go` — defines the fields; `CaptureProfile` writes Cookies + Storage
  (it never populates Headers); `Page.ApplyProfile` reads all three; `Merge`/`Diff`/`Validate`
  also touch them; `Save/Load` + `Save/LoadEncrypted` round-trip them.
- `grpc/server/server_hijack_stream.go` — `CaptureProfile` RPC marshals the full profile to
  `ProfileJson`; `LoadProfile` RPC unmarshals + calls `ApplyProfile`.
- `cmd/scout/profile.go` — the `scout profile` CLI group (capture/load/show/merge/diff +
  session-capture/session-load), displays and round-trips the secret fields.
- `cmd/scout/vault.go` — `vault set --from-profile` (the legacy import bridge, already built).
- `cmd/scout/session.go` — `--profile` on session create.

**Not in scope (false positives):** `pkg/scout/scraper/modes/{slack,twitter}` matched `UserProfile`
only via their own `slackUserProfile` / `parseUserProfileResponse` symbols. They use
`page.GetCookies()`/`SetCookies()` directly and never touch `scout.UserProfile`'s secret fields.

### Approved scope decisions

1. **Clean cutover** — the vault owns all secrets. `.scoutprofile` and the gRPC profile RPCs become
   secret-free (non-secret browser/identity config only).
2. **Vault owns web storage too** — localStorage/sessionStorage move into vault (it already persists
   a `Storage` field; only capture + apply are missing).
3. **Local-only secrets** — vault capture/apply works for local/in-process browser sessions. The
   gRPC profile RPCs carry only non-secret config; secrets drop out automatically. Remote-daemon
   secret transfer and new vault gRPC RPCs are out of scope.

### Architectural constraint

`pkg/scout/vault` imports `pkg/scout` (its `Handle.ApplyToPage(page *scout.Page)`), so
`internal/engine` **cannot** import vault (import cycle). All vault-based capture/apply logic lives
at the `pkg/scout/vault` layer; the CLI orchestrates. `internal/engine/profile.go` only loses
behavior — it gains nothing vault-related.

### Grounded vault facts

- `vault.Cookie = scout.Cookie` (type alias) → no cookie mapping needed.
- `SecretProfile`, `SecretProfileInput`, and on-disk `storedProfile` **already** carry
  `Storage map[string]OriginStore`. The store persists storage today.
- `Handle.ApplyToPage` applies cookies + headers **pre-navigation**; it deliberately does NOT seed
  web storage (origin-specific, must run post-navigation).
- `FromUserProfile(path)` already extracts cookies + headers + storage from a `.scoutprofile`.

## Design

### §1 — Vault extension (`pkg/scout/vault`)

Two new functions; storage persistence already exists.

```go
// CaptureFromPage snapshots a live page's secret-bearing state (cookies + the
// current origin's localStorage/sessionStorage) into a SecretProfileInput named `name`.
// Headers are not captured (parity with the old CaptureProfile). Local sessions only.
func CaptureFromPage(page *scout.Page, name string) (SecretProfileInput, error)

// ApplyStorageToPage seeds the current origin's localStorage/sessionStorage from the
// profile. Call AFTER navigating to the target origin (cookies/headers go via
// ApplyToPage pre-navigation). Mirrors the old ApplyProfile storage branch.
func (h *Handle) ApplyStorageToPage(page *scout.Page) error
```

`ApplyToPage` is unchanged. Together, `ApplyToPage` (pre-nav cookies+headers) + `ApplyStorageToPage`
(post-nav storage) make the vault a true superset of the old `ApplyProfile`.

### §2 — `internal/engine/profile.go` (callers off the fields; fields retained until the date)

- `CaptureProfile`: remove the Cookies capture block and the Storage capture block. The result is a
  non-secret `UserProfile` (browser/identity/extensions/proxy only).
- `Page.ApplyProfile`: remove the cookies, headers, and storage injection branches → the method
  becomes a no-op returning `nil`. Annotate
  `// Deprecated: applies nothing post-cutover; use vault Handle.ApplyToPage + ApplyStorageToPage. Removal after 2026-07-02.`
- The three fields, `ProfileOriginStorage`, and `Merge`/`Diff`/`Validate`/`Save*`/`Load*` stay
  **unchanged** in Phase 1. They remain populated only by `FromUserProfile` reading legacy files,
  and read only by `profile show` (legacy display) and the import bridge. They are removed in Phase 2.

### §3 — CLI + gRPC

- **New** `scout vault capture <name> <url>` — launch/use a local browser, navigate to `url`,
  then `CaptureFromPage` → `Vault.Set`. Local-only. Mirrors `scout vault use`. Like the old
  `scout profile capture`, it captures whatever secrets the live session holds; authenticated
  capture is done by pairing with `--user-data-dir` (the existing Chrome-profile flag) or an
  interactive headed login. Reuses `baseOpts(cmd)`.
- Wire `Handle.ApplyStorageToPage` into the `scout vault use` apply path (post-navigation) so
  `vault use` restores storage in addition to cookies+headers.
- `scout profile capture` / `load` / `session-capture` / `session-load`: non-secret config only.
  Update help text (drop "cookies, localStorage, sessionStorage, headers"); remove the secret
  summary lines. Point users to `scout vault capture` / `scout vault use`.
- `scout profile show`: display Cookies/Storage/Headers **only if a loaded legacy file still
  carries them**, with a one-line deprecation note; otherwise omit those sections.
- `scout profile merge` / `diff`: retained; they operate on non-secret config (secret counts are 0
  for newly captured profiles).
- Deprecate `SaveProfileEncrypted` / `LoadProfileEncrypted` and the profile `--encrypt` / `--decrypt`
  flags (nothing secret remains to protect). Mark `// Deprecated: … Removal after 2026-07-02.`
- gRPC `CaptureProfile` / `LoadProfile`: **no proto or handler change**. `CaptureProfile`'s
  `ProfileJson` becomes secret-free automatically. `LoadProfile`'s only effect was applying secrets
  to a running session (launch config cannot change post-launch), so post-cutover it is a no-op;
  document this and flag the RPC for deprecation in **Phase 2** (out of this plan's scope) — no proto
  change now.

### §4 — Tests (real browser + `httptest`, `-short`-gated per repo convention)

- vault: `CaptureFromPage` round-trips cookies + localStorage + sessionStorage; `ApplyStorageToPage`
  seeds storage (verify by reading it back via JS). Existing cookie/header apply tests stay green.
- engine: assert `CaptureProfile` returns empty `Cookies`/`Storage`; assert `ApplyProfile` returns
  `nil` and mutates nothing.
- hygiene (mirrors `pkg/scout/flow/hygiene_test.go`): a freshly captured `.scoutprofile` JSON
  contains no `cookies` / `storage` / `headers` content.
- equivalence/characterization: a session whose secrets are captured via `vault capture` and applied
  via `vault use` reproduces the authenticated state the old `profile capture` → `profile load`
  produced.

### §5 — Two-phase sequencing (honors the dated-deprecation policy)

- **Phase 1 — now (this plan):** §1–§4. The three secret fields remain on `UserProfile`, populated
  only by the legacy `FromUserProfile` import bridge; no engine/CLI path writes them. `//nolint:SA1019`
  stays where the bridge / legacy `show` reads them.
- **Phase 2 — dated cleanup commit, after 2026-07-02 (separate, not in this plan):**
  remove `Cookies`/`Storage`/`Headers` + `ProfileOriginStorage` from `UserProfile`; remove
  `FromUserProfile` + `vault set --from-profile`; remove `SaveProfileEncrypted`/`LoadProfileEncrypted`;
  strip the secret branches from `Merge`/`Diff`/`Validate`/`profile show`. Update the BACKLOG P2 item
  to DONE.

## Acceptance criteria

- `scout vault capture <name>` captures a local session's cookies + web storage into a vault profile.
- `scout vault use <name>` restores cookies, headers, **and** web storage to a page.
- `scout profile capture` produces a `.scoutprofile` with zero secret content (hygiene test passes).
- `CaptureProfile` populates no secret fields; `ApplyProfile` is a no-op; both have tests.
- gRPC `CaptureProfile`/`LoadProfile` carry only non-secret config (no proto change).
- `go build ./cmd/scout/ ./pkg/... ./grpc/...` and `go vet` are clean; `go test -short ./...` green
  (the new blocking CI gate must stay green).
- The three `UserProfile` secret fields still compile (Phase 2 removes them after the date).

## Out of scope

- Removing the secret fields themselves (Phase 2, dated).
- Remote-daemon-session secret capture/apply; vault-secret gRPC RPCs (different security posture).
- Touching the slack/twitter scraper modes (they never used these fields).
