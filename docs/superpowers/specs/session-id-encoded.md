# SPEC — Encoded session ID

## Goal

Move static session attributes (browser kind, run mode, etc.) from
`scout.pid` into the session ID itself so `ls <scouthome>/sessions/` and
`scout session audit` can classify without reading any file. Random prefix
keeps collision-resistance; attribute suffix is fixed-width and parseable
by position.

Replaces UUID v7. Hard cutover (existing sessions purged by binary-pid
hard cutover already shipped in be59a84).

## ID shape

Two candidate layouts (pick one — see open questions):

### Option A — UUID-shaped (recommended)
```
XXXXXXXX-XXXX-XXXX-XXXX-AAAAAAAAAAAA
└── 20 random alpha ──┘ └ 12 attr ─┘
```
36 chars including 4 dashes; matches UUID v4/v7 token shape so existing
parsers and regexes keep working.

### Option B — Flat 36-char
```
XXXXXXXXXXXXXXXXXXXXXXXXAAAAAAAAAAAA
└── 24 random alpha ──┘└ 12 attr ┘
```
Matches user's literal description (24 X + 12 A). No dashes.

## Random portion

- Alphabet: `[A-Za-z]` (52 symbols).
- Entropy:
  - Option A — 20 chars → 52²⁰ ≈ 1.99 × 10³⁴ ≈ 113 bits.
  - Option B — 24 chars → 52²⁴ ≈ 1.45 × 10⁴¹ ≈ 137 bits.

Both safely larger than the 1-million-session collision budget by many
orders of magnitude. UUID v4 is 122 bits for reference.

## Attribute suffix (12 chars, fixed positions)

| Pos | Meaning  | Values |
|----:|----------|--------|
| 0   | Version  | `1` (next free: `2`, …) |
| 1   | Browser  | `C`=chrome, `B`=brave, `E`=edge, `X`=electron, `M`=chromium |
| 2   | Mode     | `H`=headless, `V`=visible/headed |
| 3   | Lifetime | `P`=persistent (reusable), `E`=ephemeral |
| 4   | Stealth  | `S`=stealth, `N`=normal |
| 5   | Bridge   | `B`=bridge on, `N`=bridge off |
| 6   | VPN      | `V`=vpn on, `N`=no vpn |
| 7   | Sandbox  | `S`=sandbox, `N`=no-sandbox |
| 8   | FPRot    | `S`=perSession, `P`=perPage, `D`=perDomain, `I`=interval, `N`=none |
| 9   | Reserved | `-` placeholder for v2 |
| 10  | Reserved | `-` placeholder for v2 |
| 11  | Reserved | `-` placeholder for v2 |

Position 0 (version) is the parser's first check. Unknown version → treat
as foreign / reject. Position 1 (browser) resolves the E-collision: edge
is `E`, electron is `X` (not `E`).

## What stays in scout.pid

Mutable / per-process state remains in the 432-byte binary:

- ScoutPID, BrowserPID, BrowserParentPID
- BrowserStartToken
- CreatedAt, LastUsed, ExpiresAt
- Exec, BuildVersion, DomainHash, Domain

Removed from `SessionInfo` (now derivable from ID):

- `Browser`, `Reusable`, `Headless`

(Flag-state attributes encoded in the ID are read-only after creation —
matches the lifecycle: you don't toggle stealth on an existing browser.)

## API

```go
type SessionID string  // typed alias for compile-time safety

type SessionAttrs struct {
    Version  byte
    Browser  Browser   // ChromeKind / BraveKind / ...
    Headless bool
    Reusable bool
    Stealth  bool
    Bridge   bool
    VPN      bool
    Sandbox  bool
    FPRot    FPRotStrategy
}

func NewSessionID(a SessionAttrs) SessionID
func (id SessionID) Attrs() (SessionAttrs, error)
func (id SessionID) Random() string  // the X-part
func (id SessionID) IsValid() bool
```

## Audit integration

`scout session audit` can now classify with zero `scout.pid` reads:

```
SESSION                                          BROWSER MODE  LIFE  STATUS
XYzZab12-cdef-3456-7890-1CHPSBNNS---  chrome  headless persistent HEALTHY
```

…and only fall back to scout.pid for PID liveness + ExpiresAt checks.

## Migration

None. Hard cutover (consistent with binary-pid commit). All existing
UUID-v7 dirs are removed on next startup by `CleanStaleSessions`
because their scout.pid format already does not match (they were the
6 leftover legacy dirs — gone now).

## Files affected

- `internal/engine/session/id.go` — NEW; encoder/decoder, char tables
- `internal/engine/session/id_test.go` — NEW; roundtrip, invalid, attrs
- `internal/engine/session/session_track.go` — drop Browser/Reusable/Headless
  from SessionInfo; binary layout shrinks by ~34 bytes
- `internal/engine/session/info_binary.go` — drop field offsets for removed
  fields; bump version to 2 OR keep v1 with reserved slots
- `internal/engine/browser.go` — session ID generator: replace `uuid.NewV7()`
  with `NewSessionID(SessionAttrs{...})`; populate from Option fields
- `cmd/scout/session_audit.go` — derive Browser/Mode/Lifetime from ID instead
  of SessionInfo
- `cmd/scout/session.go` and other CLI consumers — adopt SessionID type
- gRPC: `session_id string` field semantics unchanged (still opaque to client)
- Tests: any test that hard-codes a UUID-format ID needs regen

## Open questions

1. **Shape A or B** (UUID-shaped with dashes vs flat)? Recommendation: A
   for parser-compatibility.
2. **Browser code for electron** — `X` confirmed? (E reserved for edge.)
3. **Attribute set complete?** I listed 9 attrs + 3 reserved. Want any of:
   - `T` = trace (OTEL on/off)
   - `M` = mobile / touch emulation
   - `R` = remote CDP
4. **Backwards compat for `--session <uuid>`** CLI flags? Reject old-format
   UUIDs unconditionally, or keep a permissive parser for one release?

## Tests

- ID roundtrip: encode attrs → string → decode → equal
- Invalid version rejected
- Invalid browser code rejected
- Random portion entropy spot-check (no duplicates in 100k generated)
- Audit table renders classification from ID alone (no scout.pid read)
