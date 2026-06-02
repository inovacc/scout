# Secrets-Isolation Vault — Brainstorm Decisions (PAUSED)

- **Date:** 2026-06-02
- **Status:** Brainstorm paused — resume AFTER `feat/session-hardening` lands (user sequencing choice).
- **Discovery brief:** workflow `w4igzjlpn` synthesis (see task output); key facts inlined below.

## Decisions locked (from brainstorm Q&A)

| # | Decision | Choice | Consequence |
|---|----------|--------|-------------|
| Backend | At-rest store | **Scout-native vault** (reuse `pkg/scout/scraper/crypt/crypt.go` Argon2id + AES-256-GCM) | **gokeepasslib hardening/internalization is DROPPED.** No KDBX, no KeePass interop. `crypt.go` already uses Argon2id(t=3,m=64MB,p=4)+AES-256-GCM — stronger than gokeepasslib's KDBX3/Argon2d/iter=2 defaults. |
| Injection | Secret → consumer | **CDP for browser + in-memory `[]byte` for scout** | Browser-bound secrets (cookies/storage/headers) injected via CDP into the page (mirrors `UserProfile` apply). scout-internal secrets handed as `[]byte`, zeroed after use. **NEVER env vars** (leak to children). |
| Memory | Hygiene level | **`[]byte` + explicit zero + mlock/VirtualLock** | Secrets as `[]byte` not `string`; zero after use + on rotation; lock out of swap via `x/sys` `VirtualLock`(Win)/`Mlock`(Unix) — already a dep, no stale 3rd-party (no memguard). Daemon re-locks/zeros on teardown. |
| Sequencing | vs session-hardening | **Finish session-hardening first** | Vault gets its own spec + branch after the reaper work ships. |

## Decisions RESOLVED (2026-06-02) — brainstorm COMPLETE; next step = writing-plans (spec → plan)

- **D1 — Naming/relationship:** the **vault ABSORBS UserProfile's secret-bearing fields** (cookies/tokens move into the vault; `UserProfile` keeps only non-secret identity: UA/lang/tz/proxy). Bigger refactor, maximal unification. New `pkg/scout/vault`.
- **D2 — Topology:** **one encrypted vault file** (Argon2id + AES-256-GCM, reuse `crypt.go`) holding many secret profiles, single passphrase, at `<scouthome>/profiles/`.
- **D3 — ID:** **opaque random** profile IDs (`Set(values) -> profileID`), enumeration-resistant.
- **D5 — Rotation:** **explicit `scout vault rotate` + auto on-passphrase-change**; atomic rewrite; zero old key + decrypted buffers; no background timer.

(Combined with the earlier 4: scout-native backend, CDP+`[]byte` injection / no env vars, `[]byte`+zero+mlock/VirtualLock hygiene, sequenced after session-hardening — now done.) **All 8 decisions locked → ready for a SPEC, then writing-plans.**

### (original open-decision detail, now resolved above)

- **D1 — Naming / collision.** `UserProfile` + `.scoutprofile` + `scout profile capture|load|show` ALREADY exist (browser identity: cookies/storage/headers/identity/proxy, `internal/engine/profile.go`, `cmd/scout/profile.go`, ~860 `profile` refs/40 files). The secrets feature needs a distinct noun — proposed **`vault`** (pkg `pkg/scout/vault` or `internal/engine/vault`); stored unit = a **secret profile** with an opaque ID; store at `<scouthome>/profiles/` (user-specified path). Confirm noun + whether the vault absorbs `UserProfile`'s secret-bearing fields (cookies/tokens) or stays a sibling.
- **D2 — Topology.** One native vault file for the install vs one file per secret-profile. (KeePass single-DB model no longer applies since native chosen.) Proposed: one encrypted vault file holding many profiles, single passphrase; migration path for the 7 scattered stores.
- **D3 — Profile-ID format.** Opaque random (enumeration-resistant) vs reuse `pkg/id` encoded scheme vs human name. Proposed: opaque random ID; `Set(values) -> profileID`.
- **D5 — Rotation semantics.** Lazy CLI (`scout vault rotate`) vs daemon-scheduled vs on-passphrase-change; atomic rewrite; zero old key + decrypted buffers; the daemon currently holds a fully-decrypted session in heap for its whole life — rotation must re-lock/zero it.

## Inventory the vault must eventually wrap (7 secret types, ZERO memory hygiene today)

passphrase (`cmd/scout/helpers.go:35`), Argon2id key (`crypt.go:33`, not zeroed), encrypted session→plaintext maps (`auth/session.go`, `auth/provider.go`), OAuth tokens (plaintext `~/.scout/upload.json`, `internal/engine/upload.go:254`), Surfshark creds (`internal/engine/vpn/surfshark.go:52`, in-mem strings), agent API key (`cmd/scout/agent.go:72`, non-constant-time), device ECDSA key (`pkg/scout/identity/identity.go:130`), fingerprints (`fingerprint_store.go:55`, unencrypted). Use `crypto/subtle.ConstantTimeCompare` for token checks.

## API sketch (to refine in the real spec)

`vault.Set(values map[string][]byte) (profileID string, err error)` → `vault.Use(profileID) (handle, err)` where handle yields secrets as zeroable `[]byte`, injects browser-bound ones via CDP, and `handle.Close()` zeros + re-locks. `vault.Rotate(profileID)` re-keys + wipes old buffers. All buffers `mlock`/`VirtualLock`-ed.
