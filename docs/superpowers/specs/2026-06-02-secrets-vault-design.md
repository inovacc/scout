# Secrets-Isolation Vault — Design Spec

- **Date:** 2026-06-02
- **Status:** Design (brainstorm complete, all 8 decisions locked — ready for `writing-plans`)
- **Brainstorm record:** `docs/superpowers/specs/2026-06-02-secrets-vault-NOTES.md`
- **Sequencing:** follows session-hardening (shipped). Independent branch.

## 1. Problem

Scout handles **7 categories of secrets today with ZERO memory hygiene** — all plain Go `string`, never zeroed, some on plaintext disk:

| Secret | Location | At rest | In memory |
|--------|----------|---------|-----------|
| Session passphrase | `cmd/scout/helpers.go:readPassphrase` | — | `string`, env leaks to children |
| Argon2id-derived key | `pkg/scout/scraper/crypt/crypt.go:deriveKey` | — | `[]byte`, not zeroed |
| Decrypted session (cookies/tokens) | `pkg/scout/scraper/auth/` | AES-GCM ✓ | plaintext `map[string]string`, never cleared |
| OAuth tokens (GDrive/OneDrive) | `internal/engine/upload.go` | **plaintext** `~/.scout/upload.json` | held, no cleanup |
| Surfshark VPN creds | `internal/engine/vpn/surfshark.go` | in-mem only | plain strings, indefinite |
| Agent API key | `cmd/scout/agent.go` (`SCOUT_AGENT_API_KEY`) | — | `string`, non-constant-time compare |
| Fingerprints | `internal/engine/fingerprint/fingerprint_store.go` | **plaintext** `*.json` | unencrypted |

Plus `UserProfile` (`internal/engine/profile.go`) bundles **secret-bearing** fields (cookies, per-origin storage, auth headers) into plaintext `.scoutprofile` files.

**Goal:** an isolation layer between secrets and the scout execution process — manually set values into a profile → get an opaque **profile ID** → scout uses the ID to inject secrets where needed (browser via CDP, internal as zeroable `[]byte`) → key rotation writes back + wipes memory → clean handling of sensitive data throughout. **Never leak plaintext to child processes.**

## 2. Locked decisions (from brainstorm)

| # | Decision | Choice |
|---|----------|--------|
| Backend | At-rest | **Scout-native** — reuse `crypt.go` (Argon2id t=3/m=64MB/p=4 + AES-256-GCM). NO KeePass/gokeepasslib. |
| Injection | Secret→consumer | **CDP for browser** (cookies/storage/headers) + **in-mem `[]byte` for scout-internal**. NEVER env vars. |
| Memory | Hygiene | **`[]byte` + explicit zero + mlock/VirtualLock** (`x/sys`, no memguard). Daemon re-locks/zeros on teardown. |
| D1 | Naming/relationship | **Vault absorbs `UserProfile`'s secret fields** (cookies/storage/headers → vault; `UserProfile` keeps non-secret identity: UA/lang/tz/locale/proxy/extensions). New `pkg/scout/vault`. |
| D2 | Topology | **One encrypted vault file**, many secret profiles, single passphrase, at `<scouthome>/profiles/vault.bin`. |
| D3 | ID | **Opaque random** profile IDs. |
| D5 | Rotation | **Explicit `scout vault rotate` + auto on-passphrase-change**; atomic rewrite; zero old key + buffers. |

## 3. Architecture — `pkg/scout/vault`

```
pkg/scout/vault
├── secmem.go            LockedBuffer: []byte + lock/zero (the memory-hygiene primitive)
│   secmem_windows.go    VirtualLock/VirtualUnlock (x/sys/windows)
│   secmem_unix.go       Mlock/Munlock (x/sys/unix; //go:build !windows)
├── id.go                opaque random profile-ID generation
├── profile.go           SecretProfile type (secrets + absorbed cookies/storage/headers)
├── store.go             at-rest: serialize → crypt.Encrypt → atomic 0o600 write (+ load/decrypt)
├── vault.go             Vault: Open/Set/Get/Use/Rotate/Close (the orchestrator)
├── inject.go            Handle: CDP injection of browser-bound secrets; []byte handoff for internal
└── *_test.go
cmd/scout/vault.go       CLI: init/set/get/list/use/rotate/rm/import
```

### 3.1 `LockedBuffer` (memory hygiene — the core primitive)
- Wraps a `[]byte`; on alloc calls `VirtualLock`(Win)/`Mlock`(Unix) best-effort (skip + log on unsupported — never fatal).
- `Bytes()` returns the slice; `Zero()` overwrites with zeros then `VirtualUnlock`/`Munlock`; `Close()` = Zero.
- **Secrets are NEVER converted to `string`** (immutable + GC-retained). Token comparisons use `crypto/subtle.ConstantTimeCompare`.
- Finalizer as a backstop, but explicit `Close()`/`Zero()` is the contract.

### 3.2 `SecretProfile` (D1 — absorbs UserProfile secret fields)
```go
type SecretProfile struct {
    ID        string                 // opaque random (D3)
    Name      string                 // optional human label (never the secret)
    Secrets   map[string]*LockedBuffer // arbitrary named secrets (API keys, passwords, tokens)
    Cookies   []Cookie               // absorbed from UserProfile (browser-bound)
    Storage   map[string]OriginStore // per-origin local/session storage (browser-bound)
    Headers   map[string]*LockedBuffer // auth headers (browser-bound)
    CreatedAt, UpdatedAt time.Time
}
```
`UserProfile` keeps non-secret identity (UA/lang/tz/locale/proxy/extensions); its `Cookies`/`Storage`/`Headers` are **deprecated** in favor of the vault (see §7).

### 3.3 `Vault` API
```go
func Open(passphrase *LockedBuffer) (*Vault, error)      // Argon2id derive (salt from header) → decrypt → load into LockedBuffers
func (v *Vault) Set(in SecretProfileInput) (id string, err error)  // upsert; opaque ID; re-encrypt + atomic write
func (v *Vault) Get(id string) (*SecretProfile, error)   // caller must Close()
func (v *Vault) Use(id string) (*Handle, error)          // operational handle (injection)
func (v *Vault) List() []ProfileMeta                      // IDs + names + timestamps — NEVER secret values
func (v *Vault) Remove(id string) error
func (v *Vault) Rotate(newPassphrase *LockedBuffer) error // re-key: decrypt(old) → encrypt(new) → atomic rewrite → zero old key+buffers
func (v *Vault) Close() error                             // zero + munlock every in-memory secret
```

### 3.4 `Handle` (injection — the isolation primitive)
```go
func (h *Handle) ApplyToPage(page *scout.Page) error  // CDP: Network.setCookies + addScriptToEvaluateOnNewDocument (storage) + setExtraHTTPHeaders
func (h *Handle) Secret(name string) (*LockedBuffer, error) // scout-internal secret as zeroable []byte — handed to scraper auth / upload / etc.
func (h *Handle) Close() error                         // zero all buffers exposed via this handle, re-lock
```
**Hard rule:** no secret ever reaches a child process via the environment. Browser secrets go over CDP into the page; internal secrets stay in-process as `LockedBuffer`.

## 4. At-rest format
- One file `<scouthome>/profiles/vault.bin` (dir `0o700`, file `0o600`).
- Header: magic + version + Argon2id salt + AES-GCM nonce. Body: `crypt.Encrypt(serialize(profiles), key)` where `key = Argon2id(passphrase, salt)` (reuse `crypt.go:deriveKey`; **route to `argon2.IDKey`** — confirm crypt.go uses IDKey).
- Writes are atomic: temp file in same dir → `fsync` → `rename` (reuse the session `atomic.go` writer pattern). No partial vault on crash.

## 5. Passphrase sourcing
Precedence: (1) `term.ReadPassword` interactive (preferred), (2) a key-file path, (3) `SCOUT_VAULT_PASSPHRASE` env — **with a stderr warning** that env leaks to children (mirrors the audit finding on `SCOUT_PASSPHRASE`). Passphrase held as `LockedBuffer`, zeroed immediately after key derivation.

## 6. CLI (`cmd/scout/vault.go`)
- `scout vault init` — create vault (prompt passphrase twice).
- `scout vault set [--name X] [--from-profile <userprofile.scoutprofile>] KEY=VALUE...` → prints opaque profile ID. `--from-profile` imports a UserProfile's secret fields (cookies/storage/headers).
- `scout vault list` / `scout vault get <id>` — metadata only; **never prints secret values**.
- `scout vault use <id> --session <sid>` — inject the profile into a running session's browser via CDP.
- `scout vault rotate` — re-key (prompt new passphrase).
- `scout vault rm <id>`.

## 7. Scope

**In scope (this spec → one plan):** the `vault` package (LockedBuffer + SecretProfile + Open/Set/Get/Use/Rotate/Close), at-rest crypt + atomic store, CDP injection, the CLI, and the `--from-profile` importer for `UserProfile` secret fields.

**Out of scope → BACKLOG (each a follow-up with the deprecation policy):**
- Migrating the other 6 scattered stores (OAuth `upload.json`, scraper-auth sessions, Surfshark creds, agent API key→constant-time, fingerprints) into the vault — each wires its consumer incrementally.
- **Full removal** of `UserProfile.Cookies/Storage/Headers` — these are `// Deprecated:`-marked and dual-read (prefer vault) for ≥30 days before removal (per CLAUDE.md breaking-change policy), since `scout profile capture/load` consumers depend on them.

## 8. Security / threat model
- **Protects:** secrets at rest (disk), the in-memory exposure window (mlock + minimal unlock + zeroing), and child-process env leakage (CDP/`[]byte` only).
- **Does NOT protect:** a compromised scout process with the vault already unlocked; a full memory dump during the unlock window (mitigated, not eliminated, by mlock + short unlock + zeroing). These are explicitly out of scope.
- Constant-time comparison for any token/key check. No secret in logs, errors, or `List()` output. Vault file never world-readable.

## 9. Testing (real crypto + real browser, no mocks)
- `crypt` round-trip: Open→Set→close→Open decrypts; wrong passphrase fails with a typed error.
- `LockedBuffer`: bytes are zero after `Close()`; mlock best-effort (skip assertion where unsupported); no `string` conversion of secrets (vet/grep guard).
- `Set→Get→Use` round-trip; opaque-ID uniqueness; `List()` never exposes values.
- **CDP injection** (real browser + `httptest`): cookies/headers/storage applied to a page; the test server echoes them back; assert present.
- **Rotation:** after `Rotate`, old passphrase fails + new succeeds + old key/buffers zeroed.
- **Atomic store:** simulated mid-write failure leaves the prior vault intact (no partial file).

## 10. Success criteria
1. `scout vault set` returns an opaque ID; `scout vault use <id> --session` injects cookies/headers into the live browser **with no plaintext in any child env**.
2. After `Handle.Close()` / `Vault.Close()`, secret buffers are provably zeroed (test-verified).
3. `scout vault rotate` re-keys atomically; the old passphrase no longer opens the vault.
4. Vault file is `0o600` under a `0o700` dir; `list`/`get` never print secret values; token checks are constant-time.
5. `--from-profile` migrates a `UserProfile`'s secret fields into the vault; `UserProfile` secret fields are deprecation-marked.
