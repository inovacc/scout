# Scout Capture — Phase 2 (MV3 Session-Capture Extension) Implementation Plan

> **STATUS: DONE + VERIFIED 2026-06-10.** Implemented and merged (`26acf3b`); a
> post-merge release-blocker (missing `"storage"` permission → non-functional SW)
> was fixed in `9dc80ba`. End-to-end behavior is verified by the automated driver
> `hacks/captureE2E` (`075ccb5`) — 20/20 steps, exit 0 against `main` `4a4cf76`
> (`go run ./hacks/captureE2E`). The unchecked boxes below are not a TODO list;
> the work shipped (git history is the source of truth) and is proven by the E2E.
> Sole residual: the rendered popup `activeTab` click gesture is human-only (not
> CDP-automatable) — see the acceptance checklist's automation legend.

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship an MV3 browser extension that, on an explicit user click, captures the active tab's cookies + web storage and hands them to the local `scout capture-host` (Phase 1) over native messaging, so `scout vault import-captures` + `scout vault use` can replay the operator's own session headlessly — with **zero password handling** (that is Phase 3).

**Architecture:** Three already-isolated units (extension → native host → vault) from the Phase 0 design. Phase 1 built the host/spool/import in Go. Phase 2 adds (A) the small Go glue that lets Chrome actually *launch* the host (native-messaging passes the caller origin as `argv[1]`, which Cobra cannot parse) plus deterministic extension-ID tooling, and (B) the MV3 extension itself: a popup that pairs once with a nonce and captures the active tab's session. The extension speaks the **exact** Phase-1 wire schema (`pkg/scout/capture` `Msg`); the host re-validates everything as hostile.

**Tech Stack:** Go 1.26 (`pkg/scout/capture`, `cmd/scout`), `crypto/rand`+`crypto/rsa`+`crypto/x509`+`crypto/sha256` (extension-ID derivation), MV3 (manifest v3 service worker, `chrome.runtime.connectNative`, `chrome.cookies`, `chrome.scripting`), `node --check` for JS syntax gating. No new third-party deps.

---

## Critical contract facts (read before any task)

These are the load-bearing constraints discovered by reading the Phase-1 code (`pkg/scout/capture/protocol.go`, `host.go`, `keys.go`, `cmd/scout/capture.go`, `cmd/scout/capture_manifest_windows.go`). Getting any of these wrong silently breaks the wire.

1. **The host decodes with `DisallowUnknownFields()`** (`protocol.go:62`). Every JSON object the extension posts may contain **only** fields declared on `Msg`: `v, type, ext_id, nonce, site, cookies, storage, username, password, at` (and the host→ext fields `id, host_version, code, message`, which the extension never sends). Any extra key → decode error → `ReadFrame` errors → `RunHost` writes one `error{code:"bad_frame"}` and **closes the connection**.
2. **`WireCookie` has NO json tags** (`protocol.go:35`). Its JSON keys are the Go field names: **`Name`, `Value`, `Domain`, `Path`, `Secure`, `HTTPOnly`** (capitalised). `encoding/json` matches case-insensitively on decode, so lowercase keys would still bind — but `chrome.cookies.getAll()` returns objects with **extra** fields (`hostOnly`, `session`, `sameSite`, `expirationDate`, `storeId`) that are NOT on `WireCookie`, so they trip `DisallowUnknownFields`. **The extension MUST down-map every cookie to exactly those six keys.**
3. **`storage` is `map[origin]WireOriginStorage`** where `WireOriginStorage` keys ARE tagged lowercase `local`/`session` (`protocol.go:41-44`). Shape: `{"https://site.example": {"local": {k:v}, "session": {k:v}}}`.
4. **Native messaging frames are handled by Chrome.** The extension calls `port.postMessage(obj)`; Chrome serialises JSON + prepends the `uint32` length. Do **not** manually frame. Chrome enforces a **1 MiB** per-message cap (matches `maxFrame = 1<<20`, `protocol.go:11`); an oversized session bundle simply fails — v1 has no chunking (note it in the UI).
5. **`hello` must precede any capture** (`host.go:62-68`): host replies `error{not_paired}` otherwise. `hello` = `{v:1, type:"hello", ext_id, nonce}`; host checks `ext_id == cfg.AllowedExtID` AND `VerifyNonce(nonce)` (`host.go:52`).
6. **Chrome launches the native host by executable path with no args** (the manifest `path` = `os.Executable()`, `capture_manifest_windows.go:26-35`). It passes the caller origin as `argv[1]` (`chrome-extension://<id>/`) and, on Windows, `--parent-window=<handle>`. The current binary would hand that to Cobra → error. **`main()` must detect this and route to the host before Cobra runs.**
7. **stdout is the wire** in host mode. The launch-routing path must run *before* any gops/logger/tracing/bootstrap that could print to stdout, and must use `os.Stdin`/`os.Stdout` directly.
8. **`AllowedExtID` for the host** is the ID passed to `scout capture-host install <id>`. The extension sends `ext_id: chrome.runtime.id`, which equals that same ID. Phase 2 persists it at install so the launch-routing path can load it.
9. **Vault import already maps the wire to the vault** (`import.go:82-84`, `toVaultCookies`, `toVaultStorage`). Phase 2 changes **nothing** in `import.go`/`spool.go`/`keys.go`; the extension just feeds the existing `capture_session` path.

---

## File structure

**Go — new (`pkg/scout/capture/`):**
- `extid.go` — `ExtensionID(derSPKI) string`, `ManifestKey(derSPKI) string`, `OriginToExtID(origin) (string,bool)`, `IsNativeMessagingLaunch(args) (string,bool)`. Pure functions, fully unit-tested.
- `extid_test.go` — vectors + properties for the above.

**Go — new (`cmd/scout/`):**
- `capture_launch.go` — `runCaptureHostLaunch(origin string)` (builds `HostConfig` from persisted state, runs `RunHost`, exits) + the `main()` hook helper.
- `capture_launch_test.go` — end-to-end test of the routing over an in-memory stdio pair (no browser).

**Go — modified (`cmd/scout/`):**
- `scout.go` (`main()`) — add the native-messaging launch check as the first statement.
- `capture.go` — `keygen` subcommand; persist/remove the ext-id on install/uninstall; `ExtIDPath`/`SaveExtID`/`LoadExtID` helpers.

**Extension — new (`extensions/scout-capture/`):**
- `manifest.json` — MV3, least-privilege, optional pinned `key`.
- `background.js` — service worker: native port, pairing handshake, capture orchestration, metadata audit.
- `snapshot.js` — injected function returning `{local,session}` web storage for the top frame.
- `popup.html`, `popup.js` — pairing-nonce field, capture button, status, captured-items (metadata) list.
- `README.md` — setup, pairing, and the manual E2E test procedure.

**Docs — modified:**
- `docs/ROADMAP.md` (Phase 2 → done entry), `docs/BACKLOG.md` (Phase 3 follow-ups), `CLAUDE.md` (one capture-extension convention line), `.gitignore` (extension private key + ext-id artifacts).

---

## Group A — Go glue: make the host browser-launchable + ext-ID tooling (TDD, no browser)

### Task A1: Parse a chrome-extension origin → extension ID

**Files:**
- Create: `pkg/scout/capture/extid.go`
- Test: `pkg/scout/capture/extid_test.go`

- [ ] **Step 1: Write the failing test**

```go
package capture

import "testing"

func TestOriginToExtID(t *testing.T) {
	const id = "abcdefghijklmnopabcdefghijklmnop" // 32 chars, all in a-p
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"chrome-extension://" + id + "/", id, true},
		{"chrome-extension://" + id, id, true},
		{"chrome-extension://short/", "", false},
		{"https://example.com/", "", false},
		{"chrome-extension://ABCDEFGHIJKLMNOPABCDEFGHIJKLMNOP/", "", false}, // uppercase not in a-p
		{"", "", false},
	}
	for _, c := range cases {
		got, ok := OriginToExtID(c.in)
		if got != c.want || ok != c.ok {
			t.Fatalf("OriginToExtID(%q) = (%q,%v), want (%q,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestIsNativeMessagingLaunch(t *testing.T) {
	origin, ok := IsNativeMessagingLaunch([]string{"scout", "chrome-extension://abcdefghijklmnopabcdefghijklmnop/", "--parent-window=123"})
	if !ok || origin != "chrome-extension://abcdefghijklmnopabcdefghijklmnop/" {
		t.Fatalf("got (%q,%v)", origin, ok)
	}
	if _, ok := IsNativeMessagingLaunch([]string{"scout", "vault", "list"}); ok {
		t.Fatal("should not detect a normal subcommand invocation")
	}
	if _, ok := IsNativeMessagingLaunch([]string{"scout"}); ok {
		t.Fatal("bare invocation is not a launch")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./pkg/scout/capture/ -run 'TestOriginToExtID|TestIsNativeMessagingLaunch' -v`
Expected: FAIL — `undefined: OriginToExtID` / `undefined: IsNativeMessagingLaunch`.

- [ ] **Step 3: Write the minimal implementation**

```go
// Package-local additions for extension-identity + native-messaging launch.
package capture

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
)

const extIDLen = 32 // Chrome maps the first 16 bytes of SHA-256 to 32 chars in a-p.

// OriginToExtID extracts and validates the extension ID from a
// chrome-extension:// origin (the value Chrome passes as argv[1] when it
// launches a native-messaging host). ok is false for anything else.
func OriginToExtID(origin string) (string, bool) {
	const p = "chrome-extension://"
	if !strings.HasPrefix(origin, p) {
		return "", false
	}
	id := strings.TrimSuffix(strings.TrimPrefix(origin, p), "/")
	if len(id) != extIDLen {
		return "", false
	}
	for _, c := range id {
		if c < 'a' || c > 'p' {
			return "", false
		}
	}
	return id, true
}

// IsNativeMessagingLaunch reports whether os.Args indicates Chrome launched us
// as a native-messaging host (any arg is a chrome-extension:// origin). It
// returns the origin so the caller can cross-check the ID.
func IsNativeMessagingLaunch(args []string) (string, bool) {
	for _, a := range args {
		if strings.HasPrefix(a, "chrome-extension://") {
			return a, true
		}
	}
	return "", false
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./pkg/scout/capture/ -run 'TestOriginToExtID|TestIsNativeMessagingLaunch' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/scout/capture/extid.go pkg/scout/capture/extid_test.go
git commit -m "feat(capture): parse chrome-extension origin + detect native-messaging launch"
```

---

### Task A2: Derive Chrome's deterministic extension ID + manifest key

**Files:**
- Modify: `pkg/scout/capture/extid.go`
- Test: `pkg/scout/capture/extid_test.go`

- [ ] **Step 1: Write the failing test**

`ExtensionID` follows Chromium's `crx_file::id_util::GenerateId`: `sha256(DER-SPKI)`, take the first 16 bytes, and map each nibble `0..15` to `'a'..'p'` (32 chars). The test pins determinism, charset, length, key-sensitivity, and the base64 round-trip — no fabricated external vector.

```go
package capture

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"testing"
)

func TestExtensionIDAndManifestKey(t *testing.T) {
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(&k.PublicKey)
	if err != nil {
		t.Fatal(err)
	}

	id := ExtensionID(der)
	if len(id) != 32 {
		t.Fatalf("id len = %d, want 32", len(id))
	}
	for _, c := range id {
		if c < 'a' || c > 'p' {
			t.Fatalf("id char %q out of a-p", c)
		}
	}
	if ExtensionID(der) != id {
		t.Fatal("ExtensionID not deterministic")
	}

	// ManifestKey is base64(DER) and must round-trip back to the DER bytes.
	mk := ManifestKey(der)
	back, err := base64.StdEncoding.DecodeString(mk)
	if err != nil {
		t.Fatalf("manifest key not valid base64: %v", err)
	}
	if string(back) != string(der) {
		t.Fatal("manifest key does not round-trip to DER")
	}

	// A different key yields a different ID.
	k2, _ := rsa.GenerateKey(rand.Reader, 2048)
	der2, _ := x509.MarshalPKIXPublicKey(&k2.PublicKey)
	if ExtensionID(der2) == id {
		t.Fatal("distinct keys produced the same id")
	}

	// The ID derived here must satisfy OriginToExtID.
	if _, ok := OriginToExtID("chrome-extension://" + id + "/"); !ok {
		t.Fatal("derived id rejected by OriginToExtID")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./pkg/scout/capture/ -run TestExtensionIDAndManifestKey -v`
Expected: FAIL — `undefined: ExtensionID` / `undefined: ManifestKey`.

- [ ] **Step 3: Write the minimal implementation**

Append to `pkg/scout/capture/extid.go`:

```go
// ExtensionID derives the stable Chrome extension ID from a DER-encoded SPKI
// public key, matching Chromium's crx_file::id_util::GenerateId: the first 16
// bytes of SHA-256(der), each nibble mapped 0..15 -> 'a'..'p'.
func ExtensionID(derSPKI []byte) string {
	sum := sha256.Sum256(derSPKI)
	var b strings.Builder
	b.Grow(extIDLen)
	for _, c := range sum[:extIDLen/2] {
		b.WriteByte('a' + (c >> 4))
		b.WriteByte('a' + (c & 0x0f))
	}
	return b.String()
}

// ManifestKey returns the base64 value to place in manifest.json "key" so the
// loaded extension gets the stable ExtensionID(derSPKI).
func ManifestKey(derSPKI []byte) string {
	return base64.StdEncoding.EncodeToString(derSPKI)
}
```

(Confirm `encoding/base64` is imported — added in this task's import block.)

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./pkg/scout/capture/ -run TestExtensionIDAndManifestKey -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/scout/capture/extid.go pkg/scout/capture/extid_test.go
git commit -m "feat(capture): derive Chrome extension ID + manifest key from a public key"
```

---

### Task A3: `scout capture-host keygen` — generate the pinned extension keypair

**Files:**
- Modify: `cmd/scout/capture.go`
- Test: `cmd/scout/capture_test.go`

- [ ] **Step 1: Write the failing test**

The keygen helper is split out as a pure function `generateExtensionKey(dir string) (keyValue, extID string, err error)` so it is testable without spawning a process: it generates RSA-2048, writes the private key PEM `0600` under `dir`, and returns the manifest `key` value + the derived ID.

```go
func TestGenerateExtensionKey(t *testing.T) {
	dir := t.TempDir()
	keyValue, extID, err := generateExtensionKey(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(extID) != 32 {
		t.Fatalf("extID len = %d", len(extID))
	}
	// The PEM must exist and be 0600.
	pem := filepath.Join(dir, "extension_key.pem")
	fi, err := os.Stat(pem)
	if err != nil {
		t.Fatalf("private key not written: %v", err)
	}
	if runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Fatalf("private key mode = %v, want 0600", fi.Mode().Perm())
	}
	// keyValue must be valid base64 of a DER SPKI that re-derives extID.
	der, err := base64.StdEncoding.DecodeString(keyValue)
	if err != nil {
		t.Fatalf("key value not base64: %v", err)
	}
	if capture.ExtensionID(der) != extID {
		t.Fatal("returned extID does not match key value")
	}
}
```

Add imports to the test file as needed: `"encoding/base64"`, `"os"`, `"path/filepath"`, `"runtime"`, and `"github.com/inovacc/scout/pkg/scout/capture"`.

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./cmd/scout/ -run TestGenerateExtensionKey -v`
Expected: FAIL — `undefined: generateExtensionKey`.

- [ ] **Step 3: Write the minimal implementation**

Add to `cmd/scout/capture.go` (imports: `crypto/rand`, `crypto/rsa`, `crypto/x509`, `encoding/base64`, `encoding/pem`, `os`, `path/filepath`):

```go
// generateExtensionKey creates an RSA-2048 keypair for the extension, writes the
// PKCS#8 private key PEM (0600) into dir, and returns the manifest.json "key"
// value (base64 DER SPKI) plus the derived stable extension ID.
func generateExtensionKey(dir string) (keyValue, extID string, err error) {
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", fmt.Errorf("scout: capture: generate extension key: %w", err)
	}
	der, err := x509.MarshalPKIXPublicKey(&k.PublicKey)
	if err != nil {
		return "", "", fmt.Errorf("scout: capture: marshal public key: %w", err)
	}
	priv, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		return "", "", fmt.Errorf("scout: capture: marshal private key: %w", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", "", fmt.Errorf("scout: capture: mkdir key dir: %w", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: priv})
	if err := os.WriteFile(filepath.Join(dir, "extension_key.pem"), pemBytes, 0o600); err != nil {
		return "", "", fmt.Errorf("scout: capture: write extension key: %w", err)
	}
	return capture.ManifestKey(der), capture.ExtensionID(der), nil
}

var captureHostKeygenCmd = &cobra.Command{
	Use:   "keygen",
	Short: "Generate a pinned extension keypair; print the manifest key + stable extension ID",
	RunE: func(cmd *cobra.Command, _ []string) error {
		base, err := scouthome.Sub("captures")
		if err != nil {
			return err
		}
		keyValue, extID, err := generateExtensionKey(base)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(),
			"extension id: %s\n\nAdd this to extensions/scout-capture/manifest.json:\n  \"key\": \"%s\"\n\nThen run: scout capture-host install %s\n",
			extID, keyValue, extID)
		return nil
	},
}
```

Register it in `init()` alongside the existing host subcommands:

```go
captureHostCmd.AddCommand(captureHostInstallCmd, captureHostUninstallCmd, captureHostKeygenCmd)
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./cmd/scout/ -run TestGenerateExtensionKey -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/scout/capture.go cmd/scout/capture_test.go
git commit -m "feat(cli): scout capture-host keygen (pinned extension key + stable id)"
```

---

### Task A4: Persist the allowed extension ID at install (so the launch path can load it)

**Files:**
- Modify: `cmd/scout/capture.go`
- Test: `cmd/scout/capture_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestExtIDPersistence(t *testing.T) {
	t.Setenv("SCOUT_HOME", t.TempDir())
	const id = "abcdefghijklmnopabcdefghijklmnop"
	if err := saveExtID(id); err != nil {
		t.Fatal(err)
	}
	got, err := loadExtID()
	if err != nil {
		t.Fatal(err)
	}
	if got != id {
		t.Fatalf("loadExtID = %q, want %q", got, id)
	}
	// The file must be 0600.
	p, _ := extIDPath()
	fi, _ := os.Stat(p)
	if runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Fatalf("ext_id mode = %v, want 0600", fi.Mode().Perm())
	}
	if err := removeExtID(); err != nil {
		t.Fatal(err)
	}
	if _, err := loadExtID(); err == nil {
		t.Fatal("loadExtID should fail after removeExtID")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./cmd/scout/ -run TestExtIDPersistence -v`
Expected: FAIL — `undefined: saveExtID` etc.

- [ ] **Step 3: Write the minimal implementation**

Add to `cmd/scout/capture.go`:

```go
func extIDPath() (string, error) {
	base, err := scouthome.Sub("captures")
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "ext_id"), nil
}

func saveExtID(id string) error {
	p, err := extIDPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return fmt.Errorf("scout: capture: mkdir for ext_id: %w", err)
	}
	if err := os.WriteFile(p, []byte(id), 0o600); err != nil {
		return fmt.Errorf("scout: capture: write ext_id: %w", err)
	}
	return nil
}

func loadExtID() (string, error) {
	p, err := extIDPath()
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(p) //nolint:gosec
	if err != nil {
		return "", fmt.Errorf("scout: capture: read ext_id (run `scout capture-host install <id>`): %w", err)
	}
	return strings.TrimSpace(string(b)), nil
}

func removeExtID() error {
	p, err := extIDPath()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("scout: capture: remove ext_id: %w", err)
	}
	return nil
}
```

Add `"strings"` to the import block. Then wire persistence into the existing install/uninstall commands' `RunE` (in `capture.go`):

In `captureHostInstallCmd.RunE`, after the successful `installNativeManifest(args[0])`:

```go
		if err := saveExtID(args[0]); err != nil {
			return err
		}
```

In `captureHostUninstallCmd.RunE`, after `uninstallNativeManifest()`:

```go
		if err := removeExtID(); err != nil {
			return err
		}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./cmd/scout/ -run TestExtIDPersistence -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/scout/capture.go cmd/scout/capture_test.go
git commit -m "feat(cli): persist allowed extension id on install; remove on uninstall"
```

---

### Task A5: Native-messaging launch routing in `main()`

**Files:**
- Create: `cmd/scout/capture_launch.go`
- Create: `cmd/scout/capture_launch_test.go`
- Modify: `cmd/scout/scout.go` (top of `main()`)

- [ ] **Step 1: Write the failing test**

The routing core is `runCaptureHostStreams(r io.Reader, w io.Writer, origin string) error` — everything `main()` needs except the process plumbing — so it is testable over an in-memory stdio pair. It loads the persisted ext-id, the public key, the spool dir, and the nonce path, then calls `capture.RunHost`. The test drives a real `hello` + `capture_session` and asserts a sealed spool file lands.

```go
package main

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/inovacc/scout/pkg/scout/capture"
	"github.com/inovacc/scout/pkg/scout/vault"
)

func TestRunCaptureHostStreams_EndToEnd(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SCOUT_HOME", home)

	// Set up a vault + capture key + nonce + persisted ext-id, exactly as the
	// one-time operator setup would.
	pass := []byte("correct horse battery staple")
	v, err := vault.Create(pass, vaultFileFor(home))
	if err != nil {
		t.Fatal(err)
	}
	pubPath := filepath.Join(home, "captures", "capture.pub")
	if _, err := capture.InitKeypair(v, pubPath, false); err != nil {
		t.Fatal(err)
	}
	_ = v.Close()

	nonce, err := capture.EnsureNonce(filepath.Join(home, "captures", "pairing.nonce"))
	if err != nil {
		t.Fatal(err)
	}
	const id = "abcdefghijklmnopabcdefghijklmnop"
	if err := saveExtID(id); err != nil {
		t.Fatal(err)
	}

	// Build a hello + capture_session frame stream from the "extension".
	var in bytes.Buffer
	mustFrame(t, &in, capture.Msg{V: 1, Type: "hello", ExtID: id, Nonce: nonce})
	mustFrame(t, &in, capture.Msg{V: 1, Type: "capture_session", Site: "example.com",
		Cookies: []capture.WireCookie{{Name: "sid", Value: "x", Domain: "example.com", Path: "/"}},
		Storage: map[string]capture.WireOriginStorage{"https://example.com": {Local: map[string]string{"k": "v"}}},
		At:      "2026-06-10T00:00:00Z"})

	var out bytes.Buffer
	if err := runCaptureHostStreams(&in, &out, "chrome-extension://"+id+"/"); err != nil {
		t.Fatalf("runCaptureHostStreams: %v", err)
	}

	// One sealed capture must have landed in the spool.
	spool, _ := capture.SpoolDir()
	files, _ := capture.ListSpool(spool)
	if len(files) != 1 {
		t.Fatalf("spool files = %d, want 1", len(files))
	}
	// And the host must have acked (never echoed a secret).
	if !bytes.Contains(out.Bytes(), []byte("hello_ack")) || !bytes.Contains(out.Bytes(), []byte("ack")) {
		t.Fatalf("missing acks in host output: %q", out.String())
	}
	if bytes.Contains(out.Bytes(), []byte("\"sid\"")) || bytes.Contains(out.Bytes(), []byte("\"x\"")) {
		t.Fatal("host echoed secret material")
	}
}

func mustFrame(t *testing.T, w *bytes.Buffer, m capture.Msg) {
	t.Helper()
	if err := capture.WriteFrame(w, m); err != nil {
		t.Fatal(err)
	}
}
```

This test references a `vaultFileFor(home)` + `vaultFileFor` helper and `vaultFileFor`’s sibling `vaultFileFor`. If `cmd/scout` already exposes a vault-path helper for tests, reuse it; otherwise add this tiny helper to `capture_launch.go`:

```go
// vaultFileFor returns the default vault path under a given scout home (test helper-friendly).
func vaultFileFor(home string) vault.Option {
	return vault.WithPath(filepath.Join(home, "profiles", "vault.bin"))
}
```

> NOTE for the implementer: confirm the real `vault` option/constructor names against `pkg/scout/vault` (`vault.Create`, `vault.WithPath`/`vault.Path`, and the default path used by `vaultPathOpts` in `cmd/scout/vault.go`). Match whatever `vaultPathOpts(cmd)` already does so the launch path and the CLI agree on the file. Adjust `vaultFileFor` and the `vault.Create` call to the actual signatures; the test asserts behaviour, not those exact names.

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./cmd/scout/ -run TestRunCaptureHostStreams_EndToEnd -v`
Expected: FAIL — `undefined: runCaptureHostStreams`.

- [ ] **Step 3: Write the minimal implementation**

Create `cmd/scout/capture_launch.go`:

```go
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/inovacc/scout/pkg/scout/capture"
)

// runCaptureHostStreams runs the native-messaging host over the given streams.
// It loads the operator-provisioned state (allowed ext-id, public key, spool,
// pairing nonce) and delegates to capture.RunHost. origin is the chrome-extension
// origin Chrome passed as argv; it is cross-checked against the persisted ext-id.
func runCaptureHostStreams(r io.Reader, w io.Writer, origin string) error {
	allowed, err := loadExtID()
	if err != nil {
		return err
	}
	if id, ok := capture.OriginToExtID(origin); ok && id != allowed {
		return fmt.Errorf("scout: capture: launching origin %q does not match installed ext id", origin)
	}
	pubPath, err := capturePubPath()
	if err != nil {
		return err
	}
	pub, err := capture.LoadPub(pubPath)
	if err != nil {
		return err
	}
	spoolDir, err := capture.SpoolDir()
	if err != nil {
		return err
	}
	noncePath, err := captureNoncePath()
	if err != nil {
		return err
	}
	return capture.RunHost(r, w, capture.HostConfig{
		Pub:          pub,
		SpoolDir:     spoolDir,
		AllowedExtID: allowed,
		NoncePath:    noncePath,
	})
}

// maybeRunCaptureHost handles the case where Chrome launched this binary as a
// native-messaging host (argv carries a chrome-extension:// origin). It runs the
// host on os.Stdin/os.Stdout and exits the process. It MUST be called as the very
// first thing in main(), before any bootstrap writes to stdout. Returns without
// acting for a normal CLI invocation.
func maybeRunCaptureHost() {
	origin, ok := capture.IsNativeMessagingLaunch(os.Args[1:])
	if !ok {
		return
	}
	if err := runCaptureHostStreams(os.Stdin, os.Stdout, origin); err != nil {
		// Never print to stdout (the wire); a non-zero exit + stderr is enough.
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./cmd/scout/ -run TestRunCaptureHostStreams_EndToEnd -v`
Expected: PASS. (If the vault helper names differ, fix per the NOTE until green.)

- [ ] **Step 5: Wire `main()` and verify the whole package builds**

In `cmd/scout/scout.go`, make `maybeRunCaptureHost()` the **first** statement of `main()`:

```go
func main() {
	maybeRunCaptureHost() // native-messaging host mode: returns immediately for normal CLI use
	// ... existing bootstrap (gops, CleanStaleSessions, tracing, logger, Execute) unchanged ...
}
```

Run: `go build ./cmd/scout/ && go vet ./cmd/scout/ ./pkg/scout/capture/`
Expected: exit 0, no output.

- [ ] **Step 6: Commit**

```bash
git add cmd/scout/capture_launch.go cmd/scout/capture_launch_test.go cmd/scout/scout.go
git commit -m "feat(cli): route Chrome native-messaging launch to capture-host before Cobra"
```

---

## Group B — MV3 extension: session capture (implement + syntax gate + manual test plan)

> JS has no unit-test harness in this repo (tests are real-browser + httptest). Each JS task's automated gate is `node --check <file>` (syntax) and a strict review against the **Critical contract facts**. Behavioural verification is the manual E2E plan in Task C1.

### Task B1: MV3 manifest (least privilege)

**Files:**
- Create: `extensions/scout-capture/manifest.json`

- [ ] **Step 1: Write the manifest**

```json
{
  "manifest_version": 3,
  "name": "Scout Capture",
  "version": "0.1.0",
  "description": "Capture THIS browser's own session (cookies + web storage) for the active tab into your local Scout vault, on an explicit click. Local-only; no network.",
  "permissions": ["nativeMessaging", "activeTab", "scripting", "cookies"],
  "action": { "default_popup": "popup.html", "default_title": "Scout Capture" },
  "background": { "service_worker": "background.js" }
}
```

Notes enforced by review (Critical fact #6, design §6): **no** `host_permissions`, **no** `<all_urls>`, **no** `tabs`/`webRequest`/`debugger`, **no** `content_scripts`. The optional pinned `"key": "<from scout capture-host keygen>"` is added by the operator per the README; omitting it just yields a random dev ID (still works — install with whatever ID Chrome shows).

- [ ] **Step 2: Validate it is well-formed JSON**

Run: `node -e "JSON.parse(require('fs').readFileSync('extensions/scout-capture/manifest.json','utf8')); console.log('ok')"`
Expected: `ok`.

- [ ] **Step 3: Commit**

```bash
git add extensions/scout-capture/manifest.json
git commit -m "feat(ext): Scout Capture MV3 manifest (least-privilege session capture)"
```

---

### Task B2: Web-storage snapshot function

**Files:**
- Create: `extensions/scout-capture/snapshot.js`

- [ ] **Step 1: Write the snapshot function**

Injected via `chrome.scripting.executeScript({func})` into the **top frame only** (the call site passes no `allFrames`, so it defaults to the top frame). Returns the exact `WireOriginStorage` shape (`local`/`session`).

```js
// snapshot.js — runs IN the page (top frame). Returns this origin's web storage
// as { origin, store: { local: {...}, session: {...} } }. No secrets logged.
function scoutCaptureSnapshot() {
  function dump(s) {
    const out = {};
    try {
      for (let i = 0; i < s.length; i++) {
        const k = s.key(i);
        out[k] = s.getItem(k);
      }
    } catch (e) {
      // storage may be blocked (e.g. about: pages); return what we have.
    }
    return out;
  }
  return {
    origin: location.origin,
    store: { local: dump(window.localStorage), session: dump(window.sessionStorage) },
  };
}
```

- [ ] **Step 2: Syntax-check**

Run: `node --check extensions/scout-capture/snapshot.js`
Expected: exit 0 (no output).

- [ ] **Step 3: Commit**

```bash
git add extensions/scout-capture/snapshot.js
git commit -m "feat(ext): web-storage snapshot function (top frame, WireOriginStorage shape)"
```

---

### Task B3: Background service worker (native port + capture orchestration)

**Files:**
- Create: `extensions/scout-capture/background.js`

- [ ] **Step 1: Write the service worker**

This is the heart of the wire correctness. It (a) opens the native port on demand, (b) sends `hello` with the stored pairing nonce, (c) on a `capture` request from the popup, reads cookies via `chrome.cookies.getAll({url})` and **down-maps each to exactly the six `WireCookie` keys**, injects `snapshot.js` for storage, (d) posts a single `capture_session` with only the allowed fields, (e) relays `ack`/`error` to the popup and appends a **metadata-only** audit entry.

```js
// background.js — Scout Capture service worker.
// Wire contract: messages must contain ONLY fields on capture.Msg, and cookies
// ONLY {Name,Value,Domain,Path,Secure,HTTPOnly} (host uses DisallowUnknownFields).
const NATIVE_HOST = "com.inovacc.scout.capture";

function connectHost() {
  const port = chrome.runtime.connectNative(NATIVE_HOST);
  return port;
}

// One request/response round-trip helper over a fresh port.
function captureSession(tab) {
  return new Promise((resolve) => {
    chrome.storage.local.get(["pairingNonce"], (prefs) => {
      const nonce = (prefs && prefs.pairingNonce) || "";
      if (!nonce) {
        resolve({ ok: false, error: "Enter the pairing nonce first (Scout: run `scout vault capture-key init`)." });
        return;
      }
      let port;
      try {
        port = connectHost();
      } catch (e) {
        resolve({ ok: false, error: "Native host not reachable. Run `scout capture-host install <id>`." });
        return;
      }
      let helloAcked = false;
      const fail = (msg) => { try { port.disconnect(); } catch (e) {} resolve({ ok: false, error: msg }); };

      port.onDisconnect.addListener(() => {
        const le = chrome.runtime.lastError;
        if (!helloAcked) fail(le ? le.message : "host disconnected before pairing");
      });

      port.onMessage.addListener((msg) => {
        if (!msg || msg.v !== 1) { fail("bad host reply"); return; }
        if (msg.type === "error") { fail(msg.message || msg.code || "host error"); return; }
        if (msg.type === "hello_ack") {
          helloAcked = true;
          buildAndSend(tab, port, resolve, fail);
          return;
        }
        if (msg.type === "ack") {
          recordAudit(tab, msg.id);
          try { port.disconnect(); } catch (e) {}
          resolve({ ok: true, id: msg.id });
        }
      });

      // Step 1 of the handshake: hello.
      port.postMessage({ v: 1, type: "hello", ext_id: chrome.runtime.id, nonce: nonce });
    });
  });
}

function buildAndSend(tab, port, resolve, fail) {
  const url = tab.url || "";
  let site = "";
  try { site = new URL(url).hostname; } catch (e) { fail("active tab has no capturable URL"); return; }

  // Cookies for this tab's URL, down-mapped to the six WireCookie keys ONLY.
  chrome.cookies.getAll({ url }, (cookies) => {
    const wireCookies = (cookies || []).map((c) => ({
      Name: c.name,
      Value: c.value,
      Domain: c.domain,
      Path: c.path,
      Secure: !!c.secure,
      HTTPOnly: !!c.httpOnly,
    }));

    // Web storage via an injected snapshot (top frame of the active tab only).
    chrome.scripting.executeScript({ target: { tabId: tab.id }, files: ["snapshot.js"] }, () => {
      chrome.scripting.executeScript({ target: { tabId: tab.id }, func: scoutCaptureSnapshotCaller }, (res) => {
        const snap = (res && res[0] && res[0].result) || { origin: url, store: { local: {}, session: {} } };
        const storage = {};
        storage[snap.origin] = { local: snap.store.local || {}, session: snap.store.session || {} };

        const payload = {
          v: 1,
          type: "capture_session",
          site: site,
          cookies: wireCookies,
          storage: storage,
          at: new Date().toISOString(),
        };
        // Size guard mirrors the host's 1 MiB frame cap (no chunking in v1).
        if (JSON.stringify(payload).length > 1000000) {
          fail("session too large to capture in one message (>1 MiB)");
          return;
        }
        port.postMessage(payload);
      });
    });
  });
}

// scoutCaptureSnapshotCaller is injected to invoke the function defined in
// snapshot.js (already injected via files:[...]) and return its result.
function scoutCaptureSnapshotCaller() {
  return scoutCaptureSnapshot();
}

function recordAudit(tab, id) {
  chrome.storage.local.get(["audit"], (data) => {
    const audit = (data && data.audit) || [];
    let site = "";
    try { site = new URL(tab.url).hostname; } catch (e) {}
    audit.unshift({ site: site, id: id, at: new Date().toISOString() }); // metadata only, never values
    chrome.storage.local.set({ audit: audit.slice(0, 100) });
  });
}

// Popup → background command channel.
chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
  if (message && message.cmd === "capture") {
    chrome.tabs.query({ active: true, currentWindow: true }, (tabs) => {
      if (!tabs || !tabs[0]) { sendResponse({ ok: false, error: "no active tab" }); return; }
      captureSession(tabs[0]).then(sendResponse);
    });
    return true; // async response
  }
  return false;
});
```

> Implementer note: `chrome.scripting.executeScript` with `func:` requires the referenced symbol to be in scope at injection time. The two-step inject (first `files:["snapshot.js"]`, then `func: scoutCaptureSnapshotCaller`) is used so the page-side function and the caller stay in the page world. If review prefers a single injection, fold `scoutCaptureSnapshot`'s body directly into a `func:` and drop `snapshot.js` — keep whichever the manual test in C1 proves works, but do not add extra permissions.

- [ ] **Step 2: Syntax-check**

Run: `node --check extensions/scout-capture/background.js`
Expected: exit 0.

- [ ] **Step 3: Review against the wire contract**

Confirm by reading the file: the only message types posted are `hello` and `capture_session`; the only keys on the posted objects are `{v,type,ext_id,nonce}` and `{v,type,site,cookies,storage,at}`; cookies carry only the six capitalised keys; no `console.log` prints a cookie value, storage value, or the nonce.

- [ ] **Step 4: Commit**

```bash
git add extensions/scout-capture/background.js
git commit -m "feat(ext): service worker — pair via nonce, capture session to native host"
```

---

### Task B4: Popup UI (pair + capture + audit list)

**Files:**
- Create: `extensions/scout-capture/popup.html`
- Create: `extensions/scout-capture/popup.js`

- [ ] **Step 1: Write `popup.html`** (no inline JS — MV3 CSP forbids it)

```html
<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8" />
  <style>
    body { font: 13px system-ui, sans-serif; width: 320px; margin: 0; padding: 12px; }
    h1 { font-size: 14px; margin: 0 0 8px; }
    input { width: 100%; box-sizing: border-box; margin: 4px 0 8px; padding: 6px; }
    button { width: 100%; padding: 8px; cursor: pointer; }
    #status { margin-top: 8px; min-height: 16px; }
    .ok { color: #137333; } .err { color: #c5221f; }
    ul { margin: 8px 0 0; padding-left: 16px; max-height: 120px; overflow:auto; }
    li { color: #555; }
    .muted { color: #777; font-size: 11px; }
  </style>
</head>
<body>
  <h1>Scout Capture</h1>
  <label for="nonce">Pairing nonce</label>
  <input id="nonce" type="password" placeholder="from `scout vault capture-key init`" autocomplete="off" />
  <button id="save">Save nonce</button>
  <button id="capture">Capture session for this tab</button>
  <div id="status"></div>
  <div class="muted">Captured (metadata only):</div>
  <ul id="audit"></ul>
  <script src="popup.js"></script>
</body>
</html>
```

- [ ] **Step 2: Write `popup.js`**

```js
// popup.js — Scout Capture popup controller.
function setStatus(text, ok) {
  const el = document.getElementById("status");
  el.textContent = text;
  el.className = ok ? "ok" : "err";
}

function renderAudit() {
  chrome.storage.local.get(["audit"], (data) => {
    const ul = document.getElementById("audit");
    ul.textContent = "";
    const audit = (data && data.audit) || [];
    for (const e of audit) {
      const li = document.createElement("li");
      li.textContent = `${e.site || "?"} — ${e.at}`; // metadata only, no values
      ul.appendChild(li);
    }
  });
}

document.getElementById("save").addEventListener("click", () => {
  const nonce = document.getElementById("nonce").value.trim();
  if (!nonce) { setStatus("Enter the nonce first.", false); return; }
  chrome.storage.local.set({ pairingNonce: nonce }, () => {
    document.getElementById("nonce").value = "";
    setStatus("Nonce saved.", true);
  });
});

document.getElementById("capture").addEventListener("click", () => {
  setStatus("Capturing…", true);
  chrome.runtime.sendMessage({ cmd: "capture" }, (resp) => {
    if (chrome.runtime.lastError) { setStatus(chrome.runtime.lastError.message, false); return; }
    if (resp && resp.ok) { setStatus("Captured ✓ (spool id " + resp.id + ")", true); renderAudit(); }
    else { setStatus((resp && resp.error) || "capture failed", false); }
  });
});

renderAudit();
```

- [ ] **Step 3: Syntax-check + JSON-free review**

Run: `node --check extensions/scout-capture/popup.js`
Expected: exit 0. Review: the nonce field is `type="password"`, cleared after save; the audit list renders site + timestamp only.

- [ ] **Step 4: Commit**

```bash
git add extensions/scout-capture/popup.html extensions/scout-capture/popup.js
git commit -m "feat(ext): popup UI — pair nonce, capture button, metadata audit list"
```

---

### Task B5: Extension README (setup + pairing)

**Files:**
- Create: `extensions/scout-capture/README.md`

- [ ] **Step 1: Write the README**

````markdown
# Scout Capture (MV3) — session capture

Captures **this** browser's own session (cookies + web storage) for the **active tab**,
on an explicit click, into your local Scout vault via the `scout capture-host`
native-messaging host. Local only — no network, no passwords (passwords are Phase 3).

## One-time setup

1. **Create the vault capture key + pairing nonce** (prints the nonce):
   ```
   scout vault capture-key init
   ```
2. **(Optional) Pin a stable extension ID.** Run:
   ```
   scout capture-host keygen
   ```
   Copy the printed `"key": "..."` line into `manifest.json`. Skip this to use a
   random dev ID instead.
3. **Load the extension:** Chrome → `chrome://extensions` → enable Developer mode →
   *Load unpacked* → select `extensions/scout-capture/`. Note the **extension ID** shown.
4. **Register the native host for that ID:**
   ```
   scout capture-host install <extension-id>
   ```
5. **Pair:** open the extension popup, paste the pairing nonce from step 1, click
   *Save nonce*.

## Use

- Open the popup on any tab you are logged into → *Capture session for this tab*.
- Drain captures into the vault (prompts the passphrase, shows a review):
  ```
  scout vault import-captures
  ```
- Replay headlessly to prove it worked:
  ```
  scout vault use <site>
  ```

## Security

Least privilege: `nativeMessaging`, `activeTab`, `scripting`, `cookies` — no
`<all_urls>`, no `tabs`/`webRequest`/`debugger`, no host permissions, no remote
code. The popup nonce and capture are the only ways anything leaves a page, and
only for the active tab on your click. Secrets never touch `chrome.storage`
(only the nonce + metadata audit live there); they go straight to the encrypted
spool and then the vault.
````

- [ ] **Step 2: Commit**

```bash
git add extensions/scout-capture/README.md
git commit -m "docs(ext): Scout Capture setup, pairing, and usage README"
```

---

## Group C — Integration proof + docs

### Task C1: Manual end-to-end test plan (the Phase 2 acceptance gate)

**Files:**
- Create: `docs/superpowers/specs/2026-06-10-scout-capture-phase2-e2e-checklist.md`

- [ ] **Step 1: Write the checklist** (the human-run proof that the browser→host→vault→replay loop works)

```markdown
# Scout Capture Phase 2 — manual E2E acceptance checklist

Prereqs: Chrome installed; `go build ./cmd/scout/` produced a `scout` on PATH;
a throwaway test site you can log into (or a local page that sets a cookie +
localStorage).

1. [ ] `scout vault capture-key init` → records the pairing nonce. (If no vault
       exists yet, create one first per `scout vault init`.)
2. [ ] Load `extensions/scout-capture/` unpacked; copy the extension ID.
3. [ ] `scout capture-host install <id>` → "installed native-messaging manifest".
4. [ ] Popup → paste nonce → Save nonce → "Nonce saved."
5. [ ] Log into the test site in the active tab.
6. [ ] Popup → "Capture session for this tab" → expect "Captured ✓ (spool id …)".
       The audit list shows `<site> — <timestamp>` (NO values).
7. [ ] Confirm a sealed spool file exists: it is unreadable (sealed box), 0600.
8. [ ] `scout vault import-captures` → enter passphrase → review summary shows the
       site + item kinds (no values) → confirm → "imported 1 capture…".
9. [ ] `scout vault use <site>` in a headless Scout run → the site loads
       **already authenticated** (cookies + storage replayed). ✅ = pass.
10. [ ] Negative checks: with the WRONG nonce saved, capture yields an `error`
        ("origin/nonce rejected") and writes NO spool file. Uninstall
        (`scout capture-host uninstall`) then capture → host unreachable, no spool.
```

- [ ] **Step 2: Commit**

```bash
git add docs/superpowers/specs/2026-06-10-scout-capture-phase2-e2e-checklist.md
git commit -m "docs(spec): Scout Capture Phase 2 manual E2E acceptance checklist"
```

---

### Task C2: Update project docs + gitignore the extension secret artifacts

**Files:**
- Modify: `docs/ROADMAP.md` (mark Phase 2 done)
- Modify: `docs/BACKLOG.md` (Phase 3 follow-ups)
- Modify: `CLAUDE.md` (one convention line)
- Modify: `.gitignore`

- [ ] **Step 1: ROADMAP — add a Phase 2 done entry**

Under the Scout Capture area (or the relevant phase list), add:

```markdown
### Scout Capture Phase 2 — MV3 session-capture extension [DONE]
- `extensions/scout-capture/` MV3 extension: popup-driven, consent-first capture of the
  active tab's cookies + web storage to `scout capture-host` over native messaging.
- Go glue: native-messaging launch routing in `main()` (Chrome passes the origin as argv);
  `scout capture-host keygen` (pinned ID); allowed ext-id persisted at install.
- Zero password handling (Phase 3). Acceptance: manual E2E checklist
  `docs/superpowers/specs/2026-06-10-scout-capture-phase2-e2e-checklist.md`.
```

- [ ] **Step 2: BACKLOG — record Phase 3 follow-ups**

```markdown
| P2 | Scout Capture Phase 3 — consented credential (password) capture | After Phase 2. content_consent.js on form submit, "Save this login to Scout?" per-event prompt, same-origin top frame only, no iframe/keylog. Dedicated security review gate. |
| P3 | Scout Capture Phase 4 — extension signing + reproducible build + captured-items revoke view + full security sign-off | Packaging/hardening; embed `extensions/scout-capture` via embed.FS if shipping in-binary. |
```

- [ ] **Step 3: CLAUDE.md — one convention line** (in the conventions list)

```markdown
- **Capture extension (Phase 2)**: `extensions/scout-capture/` (MV3) captures the active tab's session (cookies + web storage) on a click to `scout capture-host` over native messaging. Chrome launches the host by exe path with the caller origin as `argv[1]`; `main()` detects this via `capture.IsNativeMessagingLaunch` and routes to `runCaptureHostStreams` before Cobra. Allowed ext-id is persisted at `scout capture-host install <id>`; pin a stable ID with `scout capture-host keygen`. Wire = `pkg/scout/capture` `Msg` (cookies down-mapped to the six `WireCookie` keys; host uses `DisallowUnknownFields`). No passwords until Phase 3.
```

- [ ] **Step 4: .gitignore — never commit the extension private key or per-machine ids**

Append:

```gitignore
# Scout Capture local artifacts (per-machine; never commit)
extensions/scout-capture/key.pem
**/captures/extension_key.pem
```

- [ ] **Step 5: Verify the whole build + full short suite still green**

Run: `go build ./cmd/scout/ ./pkg/... && go vet ./cmd/scout/ ./pkg/scout/capture/ && go test -short ./...`
Expected: build/vet exit 0; tests all `ok`/`no test files`, 0 `FAIL`.

- [ ] **Step 6: Commit**

```bash
git add docs/ROADMAP.md docs/BACKLOG.md CLAUDE.md .gitignore
git commit -m "docs: record Scout Capture Phase 2 done + Phase 3/4 backlog; ignore ext key"
```

---

## Self-review notes (completed by plan author)

**Spec coverage (design §1–10 → tasks):** transport/native-messaging wire (§3) → A1/A5 + B3; unlock/spool reuse (§2) unchanged, exercised by A5 test + C1; vault per-site mapping (§4) reuses `import.go` unchanged, proven by C1 step 8–9; consent UX session path (§5, session half only) → B3/B4 (login half explicitly deferred to Phase 3); MV3 hardening (§6) → B1 + C2 gitignore + the ext-id pin tooling A2/A3; passphrase isolation (§7) preserved — extension never sees it (C1 step 8 enters it only in the CLI); STRIDE/non-goals (§8) → B1 permission minimisation + B3 review step + C1 negative checks. Build order (§10) honoured: this is Phase 2 = session only, no passwords.

**Placeholder scan:** no TBD/TODO; every code step shows complete code; the one open name (`vault.Create`/`vault.WithPath`) is flagged with an explicit implementer NOTE in A5 to reconcile with `cmd/scout/vault.go`'s real `vaultPathOpts`, not left vague.

**Type consistency:** `Msg`/`WireCookie`/`WireOriginStorage` field names + JSON keys match `pkg/scout/capture/protocol.go` exactly (verified: cookies `Name/Value/Domain/Path/Secure/HTTPOnly`, storage `local/session`, message keys `v/type/ext_id/nonce/site/cookies/storage/at`). `HostConfig{Pub,SpoolDir,AllowedExtID,NoncePath}`, `RunHost`, `InitKeypair`, `EnsureNonce`, `SpoolDir`, `ListSpool`, `LoadPub`, `WriteFrame` all match the Phase-1 signatures read from the codebase. Helper names are consistent across tasks (`saveExtID`/`loadExtID`/`removeExtID`/`extIDPath`; `runCaptureHostStreams`/`maybeRunCaptureHost`; `generateExtensionKey`; `ExtensionID`/`ManifestKey`/`OriginToExtID`/`IsNativeMessagingLaunch`).

**Known risk to resolve during execution:** the `chrome.scripting.executeScript` two-step injection (B3) and the exact `vault` constructor names (A5) are the two spots most likely to need a small real-API adjustment; both are called out inline so the implementer fixes them against the live API rather than guessing.
````
