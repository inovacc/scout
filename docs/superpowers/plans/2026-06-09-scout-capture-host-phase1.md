# Scout Capture — Phase 1 (Secure Backend) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Go-only, browser-free secure backend for Scout Capture: an X25519 keypair held in the vault, an encrypted append-only spool, a native-messaging host that validates + encrypts captures into the spool, and an interactive `import-captures` that drains the spool into per-site vault profiles.

**Architecture:** New leaf package `pkg/scout/capture` (depends on `pkg/scout/vault`) owns the keypair, spool crypto (`nacl/box` anonymous sealed boxes), the versioned wire protocol + framing, the pairing nonce, and the host loop. `cmd/scout` wires three CLI commands (`vault capture-key init`, `capture-host`, `vault import-captures`). The browser-facing host holds only the **public** key + pairing nonce — it can encrypt to the spool but never decrypt the spool or the vault.

**Tech Stack:** Go 1.26, `golang.org/x/crypto/nacl/box` (anonymous sealed box: Curve25519 + XSalsa20-Poly1305), `pkg/scout/vault` (Argon2id+AES-GCM, `LockedBuffer`/mlock), `segmentio/ksuid`, `internal/engine/scouthome`, cobra. Tests are unit-level (no browser); the host is tested via in-memory stdio stubs.

**Spec:** `docs/superpowers/specs/2026-06-09-scout-capture-design.md`

---

## Working conventions (read before Task 0)

- **One feature branch:** `git switch -c feat/scout-capture-host` (Task 0). Every task commits to it.
- **Commits stay LOCAL.** Do NOT `git push` (no GHA runs). Final ff-merge deferred to the user.
- **No AI attribution** in commit messages.
- **Verify locally:** `go build ./...` won't work (root has no main) — use `go build ./cmd/scout/ ./pkg/...`, `go vet`, `go test` per touched package. These are non-browser tests, so plain `go test` (no `-short` needed) runs them.

## Grounded API facts (verified 2026-06-09)

- `vault.Create(passphrase []byte, opts ...vault.Option) (*vault.Vault, error)`; `vault.Open(...)`; `vault.WithPath(p string) vault.Option`.
- `(*vault.Vault).Set(in vault.SecretProfileInput) (id string, err error)`; `.Get(id string) (*vault.SecretProfile, error)`; `.List() []vault.ProfileMeta`; `.Close() error`.
- `vault.SecretProfileInput{ ID, Name string; Secrets map[string][]byte; Cookies []vault.Cookie; Storage map[string]vault.OriginStore; Headers map[string][]byte }`.
- `vault.SecretProfile{ ID, Name string; Secrets map[string]*vault.LockedBuffer; Cookies []vault.Cookie; Storage map[string]vault.OriginStore; ... }`; `.Close()`.
- `vault.NewLockedBuffer(b []byte) *vault.LockedBuffer`; `(*LockedBuffer).Bytes() []byte`; `.Equal([]byte) bool`; `.Zero()`.
- `vault.ProfileMeta{ ID, Name string; ... }`.
- `scouthome.Sub(subdir string) (string, error)` — returns `<scouthome>/<subdir>`, creating it.
- `ksuid.New().String()` → sortable unique id.
- cmd helpers (package `main`): `readPassphraseBytes(w io.Writer, prompt string) ([]byte, error)`, `zeroBytesCLI(b []byte)`, `vaultPathOpts(cmd) []vault.Option`.
- `box.GenerateKey(rand io.Reader) (pub, priv *[32]byte, err error)`; `box.SealAnonymous(out, msg []byte, recipientPub *[32]byte, rand io.Reader) ([]byte, error)`; `box.OpenAnonymous(out, sealed []byte, pub, priv *[32]byte) ([]byte, bool)`.

## File map

- Create `pkg/scout/capture/keys.go` — keypair init/rotate (priv in vault, pub to file), pub/priv loaders.
- Create `pkg/scout/capture/keys_test.go`.
- Create `pkg/scout/capture/spool.go` — spool dir, seal+write `.cap`, list, open, secure-delete, quarantine.
- Create `pkg/scout/capture/spool_test.go`.
- Create `pkg/scout/capture/protocol.go` — wire message types, framing (`[uint32 LE][JSON]`, ≤1 MiB), validation.
- Create `pkg/scout/capture/protocol_test.go`.
- Create `pkg/scout/capture/nonce.go` — pairing nonce gen/store/verify.
- Create `pkg/scout/capture/nonce_test.go`.
- Create `pkg/scout/capture/host.go` — the host loop (`RunHost`).
- Create `pkg/scout/capture/host_test.go`.
- Create `pkg/scout/capture/import.go` — drain spool → per-site vault upsert (testable, non-interactive core).
- Create `pkg/scout/capture/import_test.go`.
- Create `cmd/scout/capture.go` — `scout capture-host` (run) + `scout vault capture-key init` + `scout vault import-captures`.
- Create `cmd/scout/capture_manifest_windows.go` / `cmd/scout/capture_manifest_unix.go` — native-messaging manifest install/uninstall (`scout capture-host install/uninstall`).

Reserved vault profile for the keypair: **Name `__scout_capture__`**, `Secrets["x25519_priv"]` = the 32-byte private key.

---

### Task 0: Feature branch

- [ ] **Step 1:** `git switch -c feat/scout-capture-host`

---

### Task 1: Keypair (priv in vault, pub to file)

**Files:** Create `pkg/scout/capture/keys.go`, `pkg/scout/capture/keys_test.go`.

- [ ] **Step 1: Write the failing test** (`keys_test.go`):

```go
package capture

import (
	"path/filepath"
	"testing"

	"github.com/inovacc/scout/pkg/scout/vault"
)

func newTempVault(t *testing.T) (*vault.Vault, string) {
	t.Helper()
	dir := t.TempDir()
	vpath := filepath.Join(dir, "vault.bin")
	if _, err := vault.Create([]byte("pw"), vault.WithPath(vpath)); err != nil {
		t.Fatalf("Create: %v", err)
	}
	v, err := vault.Open([]byte("pw"), vault.WithPath(vpath))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = v.Close() })
	return v, dir
}

func TestInitKeypairRoundTrip(t *testing.T) {
	v, dir := newTempVault(t)
	pubPath := filepath.Join(dir, "capture.pub")

	pub, err := InitKeypair(v, pubPath, false)
	if err != nil {
		t.Fatalf("InitKeypair: %v", err)
	}

	gotPub, err := LoadPub(pubPath)
	if err != nil {
		t.Fatalf("LoadPub: %v", err)
	}
	if *gotPub != *pub {
		t.Fatal("LoadPub != InitKeypair pub")
	}

	priv, err := LoadPriv(v)
	if err != nil {
		t.Fatalf("LoadPriv: %v", err)
	}

	// Seal to pub, open with priv → round-trips.
	sealed, err := Seal(pub, []byte("secret-payload"))
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	out, ok := Open(pub, priv, sealed)
	if !ok || string(out) != "secret-payload" {
		t.Fatalf("Open failed: ok=%v out=%q", ok, out)
	}
}

func TestInitKeypairIdempotent(t *testing.T) {
	v, dir := newTempVault(t)
	pubPath := filepath.Join(dir, "capture.pub")
	p1, err := InitKeypair(v, pubPath, false)
	if err != nil {
		t.Fatalf("InitKeypair 1: %v", err)
	}
	p2, err := InitKeypair(v, pubPath, false)
	if err != nil {
		t.Fatalf("InitKeypair 2: %v", err)
	}
	if *p1 != *p2 {
		t.Fatal("non-rotate re-init changed the key")
	}
	p3, err := InitKeypair(v, pubPath, true) // rotate
	if err != nil {
		t.Fatalf("InitKeypair rotate: %v", err)
	}
	if *p3 == *p1 {
		t.Fatal("rotate did not change the key")
	}
}
```

- [ ] **Step 2: Run** `go test ./pkg/scout/capture/ -run TestInitKeypair` → FAIL (`undefined: InitKeypair`, etc.).

- [ ] **Step 3: Implement** `pkg/scout/capture/keys.go`:

```go
// Package capture is the Go backend for the Scout Capture browser extension:
// an X25519 keypair held in the vault, an encrypted append-only spool, a
// native-messaging host, and the spool drain. The browser-facing host holds
// only the public key + pairing nonce and can never decrypt the spool/vault.
package capture

import (
	"crypto/rand"
	"fmt"
	"os"

	"golang.org/x/crypto/nacl/box"

	"github.com/inovacc/scout/pkg/scout/vault"
)

// keyProfileName is the reserved vault profile holding the capture private key.
const keyProfileName = "__scout_capture__"
const privSecretKey = "x25519_priv"

// InitKeypair ensures an X25519 capture keypair exists: the private key is stored
// inside the vault (a Secret in the reserved profile), the public key is written
// to pubPath (0644). Idempotent unless rotate is true. Returns the public key.
func InitKeypair(v *vault.Vault, pubPath string, rotate bool) (*[32]byte, error) {
	if !rotate {
		if priv, err := LoadPriv(v); err == nil {
			pub := pubFromPriv(priv)
			if err := writePub(pubPath, pub); err != nil {
				return nil, err
			}
			return pub, nil
		}
	}

	pub, priv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("scout: capture: generate keypair: %w", err)
	}

	id, err := findProfileID(v, keyProfileName)
	if err != nil {
		return nil, err
	}
	if _, err := v.Set(vault.SecretProfileInput{
		ID:      id, // empty = create, known = update
		Name:    keyProfileName,
		Secrets: map[string][]byte{privSecretKey: priv[:]},
	}); err != nil {
		return nil, fmt.Errorf("scout: capture: store private key: %w", err)
	}

	if err := writePub(pubPath, pub); err != nil {
		return nil, err
	}
	return pub, nil
}

// LoadPub reads a 32-byte public key from pubPath.
func LoadPub(pubPath string) (*[32]byte, error) {
	b, err := os.ReadFile(pubPath) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("scout: capture: read public key: %w", err)
	}
	if len(b) != 32 {
		return nil, fmt.Errorf("scout: capture: public key is %d bytes, want 32", len(b))
	}
	var pub [32]byte
	copy(pub[:], b)
	return &pub, nil
}

// LoadPriv reads the private key from the vault's reserved capture profile.
func LoadPriv(v *vault.Vault) (*[32]byte, error) {
	id, err := findProfileID(v, keyProfileName)
	if err != nil {
		return nil, err
	}
	if id == "" {
		return nil, fmt.Errorf("scout: capture: no capture key (run `scout vault capture-key init`)")
	}
	sp, err := v.Get(id)
	if err != nil {
		return nil, fmt.Errorf("scout: capture: load capture profile: %w", err)
	}
	defer sp.Close()
	lb, ok := sp.Secrets[privSecretKey]
	if !ok || len(lb.Bytes()) != 32 {
		return nil, fmt.Errorf("scout: capture: capture private key missing or malformed")
	}
	var priv [32]byte
	copy(priv[:], lb.Bytes())
	return &priv, nil
}

// Seal encrypts plaintext to recipient pub using an anonymous sealed box.
func Seal(pub *[32]byte, plaintext []byte) ([]byte, error) {
	out, err := box.SealAnonymous(nil, plaintext, pub, rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("scout: capture: seal: %w", err)
	}
	return out, nil
}

// Open decrypts a sealed box. ok is false on any tamper/auth failure.
func Open(pub, priv *[32]byte, sealed []byte) ([]byte, bool) {
	return box.OpenAnonymous(nil, sealed, pub, priv)
}

func pubFromPriv(priv *[32]byte) *[32]byte {
	// X25519 base-point scalar mult; box keys are Curve25519.
	var pub [32]byte
	curve25519ScalarBaseMult(&pub, priv)
	return &pub
}

func writePub(pubPath string, pub *[32]byte) error {
	if err := os.WriteFile(pubPath, pub[:], 0o644); err != nil {
		return fmt.Errorf("scout: capture: write public key: %w", err)
	}
	return nil
}

// findProfileID returns the ID of the profile named name, or "" if absent.
func findProfileID(v *vault.Vault, name string) (string, error) {
	for _, m := range v.List() {
		if m.Name == name {
			return m.ID, nil
		}
	}
	return "", nil
}
```

Add `pkg/scout/capture/curve.go`:

```go
package capture

import "golang.org/x/crypto/curve25519"

// curve25519ScalarBaseMult derives the Curve25519 public key for priv.
func curve25519ScalarBaseMult(dst, priv *[32]byte) {
	pub, _ := curve25519.X25519(priv[:], curve25519.Basepoint)
	copy(dst[:], pub)
}
```

- [ ] **Step 4: Run** `go test ./pkg/scout/capture/ -run TestInitKeypair` → PASS. Also `go vet ./pkg/scout/capture/`.

- [ ] **Step 5: Commit (local):**

```bash
git add pkg/scout/capture/keys.go pkg/scout/capture/curve.go pkg/scout/capture/keys_test.go
git commit -m "feat(capture): X25519 keypair (priv in vault, pub on disk) + sealed-box seal/open"
```

---

### Task 2: Spool (encrypted append-only inbox)

**Files:** Create `pkg/scout/capture/spool.go`, `pkg/scout/capture/spool_test.go`.

- [ ] **Step 1: Write the failing test** (`spool_test.go`):

```go
package capture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSpoolWriteListOpenDelete(t *testing.T) {
	v, dir := newTempVault(t)
	pubPath := filepath.Join(dir, "capture.pub")
	pub, err := InitKeypair(v, pubPath, false)
	if err != nil {
		t.Fatalf("InitKeypair: %v", err)
	}
	spoolDir := filepath.Join(dir, "spool")

	id, err := WriteSpool(spoolDir, pub, []byte(`{"hello":"world"}`))
	if err != nil {
		t.Fatalf("WriteSpool: %v", err)
	}

	files, err := ListSpool(spoolDir)
	if err != nil || len(files) != 1 {
		t.Fatalf("ListSpool: files=%v err=%v", files, err)
	}

	priv, _ := LoadPriv(v)
	plain, ok := OpenSpoolFile(files[0], pub, priv)
	if !ok || string(plain) != `{"hello":"world"}` {
		t.Fatalf("OpenSpoolFile: ok=%v plain=%q", ok, plain)
	}

	if err := SecureDelete(files[0]); err != nil {
		t.Fatalf("SecureDelete: %v", err)
	}
	if _, err := os.Stat(files[0]); !os.IsNotExist(err) {
		t.Fatalf("spool file %s (id %s) not deleted", files[0], id)
	}
}

func TestOpenSpoolFileTamperFails(t *testing.T) {
	v, dir := newTempVault(t)
	pubPath := filepath.Join(dir, "capture.pub")
	pub, _ := InitKeypair(v, pubPath, false)
	spoolDir := filepath.Join(dir, "spool")
	if _, err := WriteSpool(spoolDir, pub, []byte("data")); err != nil {
		t.Fatalf("WriteSpool: %v", err)
	}
	files, _ := ListSpool(spoolDir)

	raw, _ := os.ReadFile(files[0])
	raw[len(raw)-1] ^= 0xFF // flip a ciphertext byte
	_ = os.WriteFile(files[0], raw, 0o600)

	priv, _ := LoadPriv(v)
	if _, ok := OpenSpoolFile(files[0], pub, priv); ok {
		t.Fatal("tampered spool file opened; AEAD must reject it")
	}
}
```

- [ ] **Step 2: Run** `go test ./pkg/scout/capture/ -run 'TestSpool|TestOpenSpool'` → FAIL (`undefined: WriteSpool`, ...).

- [ ] **Step 3: Implement** `pkg/scout/capture/spool.go`:

```go
package capture

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/segmentio/ksuid"

	"github.com/inovacc/scout/internal/engine/scouthome"
)

// SpoolDir resolves <scouthome>/captures/spool, creating it 0700.
func SpoolDir() (string, error) {
	base, err := scouthome.Sub("captures")
	if err != nil {
		return "", fmt.Errorf("scout: capture: resolve home: %w", err)
	}
	dir := filepath.Join(base, "spool")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("scout: capture: mkdir spool: %w", err)
	}
	return dir, nil
}

// WriteSpool seals plaintext to pub and writes it as <ksuid>.cap (0600) in spoolDir.
func WriteSpool(spoolDir string, pub *[32]byte, plaintext []byte) (string, error) {
	if err := os.MkdirAll(spoolDir, 0o700); err != nil {
		return "", fmt.Errorf("scout: capture: mkdir spool: %w", err)
	}
	sealed, err := Seal(pub, plaintext)
	if err != nil {
		return "", err
	}
	id := ksuid.New().String()
	path := filepath.Join(spoolDir, id+".cap")
	if err := os.WriteFile(path, sealed, 0o600); err != nil {
		return "", fmt.Errorf("scout: capture: write spool: %w", err)
	}
	return id, nil
}

// ListSpool returns the full paths of all .cap files in spoolDir, sorted (ksuid = time-ordered).
func ListSpool(spoolDir string) ([]string, error) {
	entries, err := os.ReadDir(spoolDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("scout: capture: read spool: %w", err)
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".cap" {
			out = append(out, filepath.Join(spoolDir, e.Name()))
		}
	}
	return out, nil
}

// OpenSpoolFile reads and decrypts a spool file. ok is false on any failure.
func OpenSpoolFile(path string, pub, priv *[32]byte) ([]byte, bool) {
	sealed, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil, false
	}
	return Open(pub, priv, sealed)
}

// SecureDelete best-effort overwrites a file with zeros then removes it.
func SecureDelete(path string) error {
	if fi, err := os.Stat(path); err == nil {
		if f, err := os.OpenFile(path, os.O_WRONLY, 0o600); err == nil {
			_, _ = f.Write(make([]byte, fi.Size()))
			_ = f.Sync()
			_ = f.Close()
		}
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("scout: capture: remove spool file: %w", err)
	}
	return nil
}

// Quarantine renames a bad spool file to <name>.bad so the drain can continue.
func Quarantine(path string) error {
	if err := os.Rename(path, path+".bad"); err != nil {
		return fmt.Errorf("scout: capture: quarantine: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run** `go test ./pkg/scout/capture/ -run 'TestSpool|TestOpenSpool'` → PASS; `go vet`.

- [ ] **Step 5: Commit (local):**

```bash
git add pkg/scout/capture/spool.go pkg/scout/capture/spool_test.go
git commit -m "feat(capture): encrypted append-only spool (seal/write/list/open/secure-delete/quarantine)"
```

---

### Task 3: Wire protocol (types, framing, validation)

**Files:** Create `pkg/scout/capture/protocol.go`, `pkg/scout/capture/protocol_test.go`.

- [ ] **Step 1: Write the failing test** (`protocol_test.go`):

```go
package capture

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"
)

func TestFrameRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteFrame(&buf, Msg{V: 1, Type: "ack", ID: "x"}); err != nil {
		t.Fatalf("WriteFrame: %v", err)
	}
	got, err := ReadFrame(&buf)
	if err != nil {
		t.Fatalf("ReadFrame: %v", err)
	}
	if got.Type != "ack" || got.ID != "x" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestReadFrameRejectsOversize(t *testing.T) {
	var buf bytes.Buffer
	var hdr [4]byte
	binary.LittleEndian.PutUint32(hdr[:], maxFrame+1)
	buf.Write(hdr[:])
	if _, err := ReadFrame(&buf); err == nil {
		t.Fatal("oversize frame accepted")
	}
}

func TestReadFrameRejectsGarbageJSON(t *testing.T) {
	var buf bytes.Buffer
	body := []byte("not json")
	var hdr [4]byte
	binary.LittleEndian.PutUint32(hdr[:], uint32(len(body)))
	buf.Write(hdr[:])
	buf.Write(body)
	if _, err := ReadFrame(&buf); err == nil {
		t.Fatal("garbage JSON accepted")
	}
}

func TestValidateRejectsBadVersionAndType(t *testing.T) {
	if err := Validate(Msg{V: 2, Type: "hello"}); err == nil {
		t.Error("bad version accepted")
	}
	if err := Validate(Msg{V: 1, Type: "bogus"}); err == nil {
		t.Error("unknown type accepted")
	}
	if err := Validate(Msg{V: 1, Type: "capture_login", Site: "s", Username: "u", Password: "p"}); err != nil {
		t.Errorf("valid capture_login rejected: %v", err)
	}
}

func TestErrorMsgNeverEchoesSecret(t *testing.T) {
	// A login with a bad (empty) site must error WITHOUT including the password.
	err := Validate(Msg{V: 1, Type: "capture_login", Site: "", Username: "u", Password: "hunter2"})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if strings.Contains(err.Error(), "hunter2") {
		t.Fatalf("validation error leaked the password: %v", err)
	}
}
```

- [ ] **Step 2: Run** `go test ./pkg/scout/capture/ -run 'TestFrame|TestReadFrame|TestValidate|TestErrorMsg'` → FAIL.

- [ ] **Step 3: Implement** `pkg/scout/capture/protocol.go`:

```go
package capture

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

// maxFrame is the native-messaging per-message cap (browser limit is 1 MiB).
const maxFrame = 1 << 20

// Msg is the union of every wire message (extension <-> host). Secret-bearing
// fields (Cookies/Storage/Password) appear only on inbound capture_* messages and
// are NEVER echoed back.
type Msg struct {
	V        int                          `json:"v"`
	Type     string                       `json:"type"`
	ExtID    string                       `json:"ext_id,omitempty"`
	Nonce    string                       `json:"nonce,omitempty"`
	Site     string                       `json:"site,omitempty"`
	Cookies  []WireCookie                 `json:"cookies,omitempty"`
	Storage  map[string]WireOriginStorage `json:"storage,omitempty"`
	Username string                       `json:"username,omitempty"`
	Password string                       `json:"password,omitempty"`
	At       string                       `json:"at,omitempty"`
	// host -> ext only:
	ID          string `json:"id,omitempty"`
	HostVersion string `json:"host_version,omitempty"`
	Code        string `json:"code,omitempty"`
	Message     string `json:"message,omitempty"`
}

// WireCookie mirrors the fields the extension can supply for a cookie.
type WireCookie struct {
	Name, Value, Domain, Path string
	Secure, HTTPOnly          bool
}

// WireOriginStorage holds per-origin web storage for one origin.
type WireOriginStorage struct {
	Local   map[string]string `json:"local,omitempty"`
	Session map[string]string `json:"session,omitempty"`
}

// ReadFrame reads one length-prefixed JSON frame ([uint32 LE len][JSON]).
func ReadFrame(r io.Reader) (Msg, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return Msg{}, err // io.EOF on clean close
	}
	n := binary.LittleEndian.Uint32(hdr[:])
	if n == 0 || n > maxFrame {
		return Msg{}, fmt.Errorf("scout: capture: frame length %d out of range", n)
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(r, body); err != nil {
		return Msg{}, fmt.Errorf("scout: capture: short frame body: %w", err)
	}
	var m Msg
	dec := json.NewDecoder(jsonReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return Msg{}, fmt.Errorf("scout: capture: decode frame: %w", err)
	}
	return m, nil
}

// WriteFrame writes one length-prefixed JSON frame.
func WriteFrame(w io.Writer, m Msg) error {
	body, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("scout: capture: encode frame: %w", err)
	}
	if len(body) > maxFrame {
		return fmt.Errorf("scout: capture: outbound frame too large (%d)", len(body))
	}
	var hdr [4]byte
	binary.LittleEndian.PutUint32(hdr[:], uint32(len(body)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err = w.Write(body)
	return err
}

// Validate enforces version + type allowlist + required fields. It NEVER includes
// secret values (password/cookies/storage) in the returned error.
func Validate(m Msg) error {
	if m.V != 1 {
		return fmt.Errorf("scout: capture: unsupported version %d", m.V)
	}
	switch m.Type {
	case "hello":
		if m.ExtID == "" || m.Nonce == "" {
			return fmt.Errorf("scout: capture: hello missing ext_id/nonce")
		}
	case "capture_session":
		if m.Site == "" {
			return fmt.Errorf("scout: capture: capture_session missing site")
		}
	case "capture_login":
		if m.Site == "" || m.Username == "" || m.Password == "" {
			return fmt.Errorf("scout: capture: capture_login missing site/username/password")
		}
	default:
		return fmt.Errorf("scout: capture: unknown message type %q", m.Type)
	}
	return nil
}

func jsonReader(b []byte) io.Reader { return bytesReader(b) }
```

Add a tiny `pkg/scout/capture/io.go` (avoids importing `bytes` in protocol.go twice / keeps helpers explicit):

```go
package capture

import "bytes"

func bytesReader(b []byte) *bytes.Reader { return bytes.NewReader(b) }
```

- [ ] **Step 4: Run** the protocol tests → PASS; `go vet`.

- [ ] **Step 5: Commit (local):**

```bash
git add pkg/scout/capture/protocol.go pkg/scout/capture/io.go pkg/scout/capture/protocol_test.go
git commit -m "feat(capture): versioned native-messaging wire protocol + framing + strict validation"
```

---

### Task 4: Pairing nonce

**Files:** Create `pkg/scout/capture/nonce.go`, `pkg/scout/capture/nonce_test.go`.

- [ ] **Step 1: Write the failing test** (`nonce_test.go`):

```go
package capture

import (
	"path/filepath"
	"testing"
)

func TestNonceEnsureAndVerify(t *testing.T) {
	p := filepath.Join(t.TempDir(), "pairing.nonce")
	n1, err := EnsureNonce(p)
	if err != nil {
		t.Fatalf("EnsureNonce: %v", err)
	}
	if len(n1) < 32 {
		t.Fatalf("nonce too short: %q", n1)
	}
	n2, err := EnsureNonce(p) // idempotent
	if err != nil || n2 != n1 {
		t.Fatalf("EnsureNonce not idempotent: %q vs %q", n1, n2)
	}
	if !VerifyNonce(p, n1) {
		t.Fatal("VerifyNonce rejected the correct nonce")
	}
	if VerifyNonce(p, "wrong") {
		t.Fatal("VerifyNonce accepted a wrong nonce")
	}
}
```

- [ ] **Step 2: Run** `go test ./pkg/scout/capture/ -run TestNonce` → FAIL.

- [ ] **Step 3: Implement** `pkg/scout/capture/nonce.go`:

```go
package capture

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

// EnsureNonce returns the pairing nonce at path, generating + storing (0600) a
// 32-byte random hex nonce on first call. Idempotent.
func EnsureNonce(path string) (string, error) {
	if b, err := os.ReadFile(path); err == nil { //nolint:gosec
		return strings.TrimSpace(string(b)), nil
	}
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("scout: capture: gen nonce: %w", err)
	}
	n := hex.EncodeToString(raw[:])
	if err := os.WriteFile(path, []byte(n), 0o600); err != nil {
		return "", fmt.Errorf("scout: capture: write nonce: %w", err)
	}
	return n, nil
}

// VerifyNonce constant-time compares got against the stored nonce.
func VerifyNonce(path, got string) bool {
	b, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return false
	}
	want := strings.TrimSpace(string(b))
	return subtle.ConstantTimeCompare([]byte(want), []byte(got)) == 1
}
```

- [ ] **Step 4: Run** `go test ./pkg/scout/capture/ -run TestNonce` → PASS; `go vet`.

- [ ] **Step 5: Commit (local):**

```bash
git add pkg/scout/capture/nonce.go pkg/scout/capture/nonce_test.go
git commit -m "feat(capture): first-run pairing nonce (gen/store 0600, constant-time verify)"
```

---

### Task 5: Host loop

**Files:** Create `pkg/scout/capture/host.go`, `pkg/scout/capture/host_test.go`.

- [ ] **Step 1: Write the failing test** (`host_test.go`):

```go
package capture

import (
	"bytes"
	"path/filepath"
	"testing"
)

// drive runs the host against a sequence of inbound frames, returning the spool
// dir and the host's outbound frames.
func drive(t *testing.T, cfg HostConfig, msgs ...Msg) ([]string, []Msg) {
	t.Helper()
	var in, out bytes.Buffer
	for _, m := range msgs {
		if err := WriteFrame(&in, m); err != nil {
			t.Fatalf("WriteFrame: %v", err)
		}
	}
	if err := RunHost(&in, &out, cfg); err != nil {
		t.Fatalf("RunHost: %v", err)
	}
	var replies []Msg
	for {
		m, err := ReadFrame(&out)
		if err != nil {
			break
		}
		replies = append(replies, m)
	}
	files, _ := ListSpool(cfg.SpoolDir)
	return files, replies
}

func baseCfg(t *testing.T) (HostConfig, string) {
	t.Helper()
	v, dir := newTempVault(t)
	pub, err := InitKeypair(v, filepath.Join(dir, "capture.pub"), false)
	if err != nil {
		t.Fatalf("InitKeypair: %v", err)
	}
	noncePath := filepath.Join(dir, "pairing.nonce")
	nonce, err := EnsureNonce(noncePath)
	if err != nil {
		t.Fatalf("EnsureNonce: %v", err)
	}
	return HostConfig{
		Pub:          pub,
		SpoolDir:     filepath.Join(dir, "spool"),
		AllowedExtID: "abc123",
		NoncePath:    noncePath,
	}, nonce
}

func TestHostHappyPath(t *testing.T) {
	cfg, nonce := baseCfg(t)
	files, replies := drive(t, cfg,
		Msg{V: 1, Type: "hello", ExtID: "abc123", Nonce: nonce},
		Msg{V: 1, Type: "capture_login", Site: "example.com", Username: "alice", Password: "hunter2"},
	)
	if len(files) != 1 {
		t.Fatalf("want 1 spool file, got %d", len(files))
	}
	if len(replies) != 2 || replies[0].Type != "hello_ack" || replies[1].Type != "ack" {
		t.Fatalf("unexpected replies: %+v", replies)
	}
}

func TestHostRejectsWrongOrigin(t *testing.T) {
	cfg, nonce := baseCfg(t)
	files, replies := drive(t, cfg,
		Msg{V: 1, Type: "hello", ExtID: "WRONG", Nonce: nonce},
		Msg{V: 1, Type: "capture_login", Site: "x", Username: "u", Password: "topsecret"},
	)
	if len(files) != 0 {
		t.Fatal("spooled despite wrong origin")
	}
	if len(replies) == 0 || replies[0].Type != "error" {
		t.Fatalf("expected error reply, got %+v", replies)
	}
	for _, r := range replies {
		if bytes.Contains([]byte(r.Message), []byte("topsecret")) {
			t.Fatal("error reply leaked the password")
		}
	}
}

func TestHostRejectsMissingNonce(t *testing.T) {
	cfg, _ := baseCfg(t)
	files, replies := drive(t, cfg,
		Msg{V: 1, Type: "hello", ExtID: "abc123", Nonce: "bad"},
		Msg{V: 1, Type: "capture_session", Site: "x"},
	)
	if len(files) != 0 || replies[0].Type != "error" {
		t.Fatalf("missing/bad nonce not rejected: files=%d replies=%+v", len(files), replies)
	}
}

func TestHostRejectsCaptureBeforeHello(t *testing.T) {
	cfg, _ := baseCfg(t)
	files, replies := drive(t, cfg,
		Msg{V: 1, Type: "capture_session", Site: "x"},
	)
	if len(files) != 0 || len(replies) == 0 || replies[0].Type != "error" {
		t.Fatalf("capture before hello not rejected: files=%d replies=%+v", len(files), replies)
	}
}
```

- [ ] **Step 2: Run** `go test ./pkg/scout/capture/ -run TestHost` → FAIL (`undefined: HostConfig`, `RunHost`).

- [ ] **Step 3: Implement** `pkg/scout/capture/host.go`:

```go
package capture

import (
	"encoding/json"
	"errors"
	"io"
)

// hostVersion is reported in hello_ack.
const hostVersion = "1.0.0"

// HostConfig configures one RunHost session.
type HostConfig struct {
	Pub          *[32]byte
	SpoolDir     string
	AllowedExtID string
	NoncePath    string
}

func (c HostConfig) nonceOK(got string) bool {
	return VerifyNonce(c.NoncePath, got)
}

// RunHost reads length-prefixed frames from r until EOF, validating and spooling
// captures, writing ack/error frames to w. It returns nil on clean EOF; a
// transport/encode error otherwise. It NEVER writes secret values back to w.
func RunHost(r io.Reader, w io.Writer, cfg HostConfig) error {
	paired := false
	for {
		m, err := ReadFrame(r)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			// Malformed/oversized frame: report and stop (stream is desynced).
			_ = WriteFrame(w, Msg{V: 1, Type: "error", Code: "bad_frame", Message: "malformed frame"})
			return nil
		}
		if verr := Validate(m); verr != nil {
			if werr := WriteFrame(w, Msg{V: 1, Type: "error", Code: "invalid", Message: verr.Error()}); werr != nil {
				return werr
			}
			continue
		}
		switch m.Type {
		case "hello":
			if m.ExtID != cfg.AllowedExtID || !cfg.nonceOK(m.Nonce) {
				if werr := WriteFrame(w, Msg{V: 1, Type: "error", Code: "unauthorized", Message: "origin/nonce rejected"}); werr != nil {
					return werr
				}
				continue
			}
			paired = true
			if werr := WriteFrame(w, Msg{V: 1, Type: "hello_ack", HostVersion: hostVersion}); werr != nil {
				return werr
			}
		case "capture_session", "capture_login":
			if !paired {
				if werr := WriteFrame(w, Msg{V: 1, Type: "error", Code: "not_paired", Message: "send hello first"}); werr != nil {
					return werr
				}
				continue
			}
			payload, _ := json.Marshal(m) // re-marshal the validated message as the spool record
			id, serr := WriteSpool(cfg.SpoolDir, cfg.Pub, payload)
			if serr != nil {
				if werr := WriteFrame(w, Msg{V: 1, Type: "error", Code: "spool", Message: "could not store capture"}); werr != nil {
					return werr
				}
				continue
			}
			if werr := WriteFrame(w, Msg{V: 1, Type: "ack", ID: id}); werr != nil {
				return werr
			}
		}
	}
}
```

- [ ] **Step 4: Run** `go test ./pkg/scout/capture/ -run TestHost` → PASS; `go vet`.

- [ ] **Step 5: Commit (local):**

```bash
git add pkg/scout/capture/host.go pkg/scout/capture/host_test.go
git commit -m "feat(capture): native-messaging host loop (hello/nonce gate, spool captures, ack/error only)"
```

---

### Task 6: Import (drain spool → per-site vault profile)

**Files:** Create `pkg/scout/capture/import.go`, `pkg/scout/capture/import_test.go`.

- [ ] **Step 1: Write the failing test** (`import_test.go`):

```go
package capture

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/inovacc/scout/pkg/scout/vault"
)

func TestImportMergesPerSiteAndDeletes(t *testing.T) {
	v, dir := newTempVault(t)
	pub, _ := InitKeypair(v, filepath.Join(dir, "capture.pub"), false)
	spoolDir := filepath.Join(dir, "spool")

	login, _ := json.Marshal(Msg{V: 1, Type: "capture_login", Site: "example.com", Username: "alice", Password: "hunter2"})
	sess, _ := json.Marshal(Msg{V: 1, Type: "capture_session", Site: "example.com",
		Cookies: []WireCookie{{Name: "sid", Value: "v", Domain: "example.com", Path: "/"}}})
	if _, err := WriteSpool(spoolDir, pub, login); err != nil {
		t.Fatalf("WriteSpool login: %v", err)
	}
	if _, err := WriteSpool(spoolDir, pub, sess); err != nil {
		t.Fatalf("WriteSpool session: %v", err)
	}

	priv, _ := LoadPriv(v)
	report, err := ImportSpool(v, spoolDir, pub, priv)
	if err != nil {
		t.Fatalf("ImportSpool: %v", err)
	}
	if report.Imported != 2 {
		t.Fatalf("Imported = %d, want 2", report.Imported)
	}

	// One per-site profile named example.com with cookie + login secret.
	var id string
	for _, m := range v.List() {
		if m.Name == "example.com" {
			id = m.ID
		}
	}
	if id == "" {
		t.Fatal("no example.com profile created")
	}
	sp, _ := v.Get(id)
	defer sp.Close()
	if len(sp.Cookies) != 1 {
		t.Errorf("cookies = %d, want 1", len(sp.Cookies))
	}
	lb, ok := sp.Secrets["login:alice"]
	if !ok || !lb.Equal([]byte("hunter2")) {
		t.Errorf("login secret missing/wrong")
	}

	// Spool emptied.
	files, _ := ListSpool(spoolDir)
	if len(files) != 0 {
		t.Errorf("spool not drained: %v", files)
	}
}

func TestImportQuarantinesUndecryptable(t *testing.T) {
	v, dir := newTempVault(t)
	pub, _ := InitKeypair(v, filepath.Join(dir, "capture.pub"), false)
	spoolDir := filepath.Join(dir, "spool")
	_ = os.MkdirAll(spoolDir, 0o700)
	_ = os.WriteFile(filepath.Join(spoolDir, "junk.cap"), []byte("not a sealed box"), 0o600)

	priv, _ := LoadPriv(v)
	report, err := ImportSpool(v, spoolDir, pub, priv)
	if err != nil {
		t.Fatalf("ImportSpool: %v", err)
	}
	if report.Quarantined != 1 || report.Imported != 0 {
		t.Fatalf("report = %+v, want Quarantined=1 Imported=0", report)
	}
	if _, err := os.Stat(filepath.Join(spoolDir, "junk.cap.bad")); err != nil {
		t.Errorf("undecryptable file not quarantined: %v", err)
	}
}
```

- [ ] **Step 2: Run** `go test ./pkg/scout/capture/ -run TestImport` → FAIL.

- [ ] **Step 3: Implement** `pkg/scout/capture/import.go`:

```go
package capture

import (
	"encoding/json"
	"fmt"

	"github.com/inovacc/scout/pkg/scout/vault"
)

// ImportReport summarizes a drain.
type ImportReport struct {
	Imported    int
	Quarantined int
	Sites       []string
}

// ImportSpool decrypts every spool file, upserts it into the per-site vault
// profile, secure-deletes the file on success, and quarantines undecryptable ones.
func ImportSpool(v *vault.Vault, spoolDir string, pub, priv *[32]byte) (ImportReport, error) {
	files, err := ListSpool(spoolDir)
	if err != nil {
		return ImportReport{}, err
	}
	var rep ImportReport
	seen := map[string]bool{}
	for _, f := range files {
		plain, ok := OpenSpoolFile(f, pub, priv)
		if !ok {
			if qerr := Quarantine(f); qerr != nil {
				return rep, qerr
			}
			rep.Quarantined++
			continue
		}
		var m Msg
		if jerr := json.Unmarshal(plain, &m); jerr != nil || Validate(m) != nil {
			if qerr := Quarantine(f); qerr != nil {
				return rep, qerr
			}
			rep.Quarantined++
			continue
		}
		if uerr := upsertSite(v, m); uerr != nil {
			return rep, uerr
		}
		if derr := SecureDelete(f); derr != nil {
			return rep, derr
		}
		rep.Imported++
		if !seen[m.Site] {
			seen[m.Site] = true
			rep.Sites = append(rep.Sites, m.Site)
		}
	}
	return rep, nil
}

// upsertSite merges one capture message into the vault profile named m.Site.
func upsertSite(v *vault.Vault, m Msg) error {
	id := ""
	for _, meta := range v.List() {
		if meta.Name == m.Site {
			id = meta.ID
		}
	}

	in := vault.SecretProfileInput{ID: id, Name: m.Site, Secrets: map[string][]byte{}}

	// Carry forward existing secrets so a later login doesn't drop earlier ones.
	if id != "" {
		if sp, err := v.Get(id); err == nil {
			for k, lb := range sp.Secrets {
				in.Secrets[k] = append([]byte(nil), lb.Bytes()...)
			}
			in.Cookies = sp.Cookies
			in.Storage = sp.Storage
			sp.Close()
		}
	}

	switch m.Type {
	case "capture_session":
		in.Cookies = toVaultCookies(m.Cookies)
		in.Storage = toVaultStorage(m.Storage)
	case "capture_login":
		in.Secrets["login:"+m.Username] = []byte(m.Password)
	}

	if _, err := v.Set(in); err != nil {
		return fmt.Errorf("scout: capture: upsert %q: %w", m.Site, err)
	}
	return nil
}

func toVaultCookies(cs []WireCookie) []vault.Cookie {
	out := make([]vault.Cookie, 0, len(cs))
	for _, c := range cs {
		out = append(out, vault.Cookie{Name: c.Name, Value: c.Value, Domain: c.Domain, Path: c.Path, Secure: c.Secure, HTTPOnly: c.HTTPOnly})
	}
	return out
}

func toVaultStorage(s map[string]WireOriginStorage) map[string]vault.OriginStore {
	if len(s) == 0 {
		return nil
	}
	out := make(map[string]vault.OriginStore, len(s))
	for origin, st := range s {
		out[origin] = vault.OriginStore{LocalStorage: st.Local, SessionStorage: st.Session}
	}
	return out
}
```

(`vault.Cookie` = `scout.Cookie` = `engine.Cookie` with fields `Name, Value, URL, Domain, Path, Expires, Secure, HTTPOnly, SameSite` — the `toVaultCookies` above matches verbatim.)

- [ ] **Step 4: Run** `go test ./pkg/scout/capture/ -run TestImport` → PASS; full `go test ./pkg/scout/capture/`; `go vet`.

- [ ] **Step 5: Commit (local):**

```bash
git add pkg/scout/capture/import.go pkg/scout/capture/import_test.go
git commit -m "feat(capture): drain spool into per-site vault profiles (merge, secure-delete, quarantine)"
```

---

### Task 7: CLI wiring (`capture-key init`, `capture-host`, `import-captures`)

**Files:** Create `cmd/scout/capture.go`.

This task wires the package into cobra. The interactive passphrase paths reuse `readPassphraseBytes`. The command-registration is testable; the interactive flows are thin.

- [ ] **Step 1: Write the failing test** (`cmd/scout/capture_test.go`):

```go
package main

import "testing"

func TestCaptureCommandsRegistered(t *testing.T) {
	names := map[string]bool{}
	for _, c := range rootCmd.Commands() {
		names[c.Name()] = true
		for _, sub := range c.Commands() {
			names[c.Name()+" "+sub.Name()] = true
		}
	}
	if !names["capture-host"] {
		t.Error("capture-host not registered")
	}
	if !names["vault capture-key"] {
		t.Error("vault capture-key not registered")
	}
	if !names["vault import-captures"] {
		t.Error("vault import-captures not registered")
	}
}
```

- [ ] **Step 2: Run** `go test ./cmd/scout/ -run TestCaptureCommandsRegistered` → FAIL.

- [ ] **Step 3: Implement** `cmd/scout/capture.go`:

```go
package main

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/inovacc/scout/internal/engine/scouthome"
	"github.com/inovacc/scout/pkg/scout/capture"
	"github.com/inovacc/scout/pkg/scout/vault"
)

func capturePubPath() (string, error) {
	base, err := scouthome.Sub("captures")
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "capture.pub"), nil
}

func captureNoncePath() (string, error) {
	base, err := scouthome.Sub("captures")
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "pairing.nonce"), nil
}

var vaultCaptureKeyCmd = &cobra.Command{
	Use:   "capture-key",
	Short: "Manage the Scout Capture keypair (sub: init)",
}

var vaultCaptureKeyInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create the X25519 capture keypair (private key stored in the vault)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		rotate, _ := cmd.Flags().GetBool("rotate")
		pass, err := readPassphraseBytes(cmd.ErrOrStderr(), "Vault passphrase: ")
		if err != nil {
			return err
		}
		defer zeroBytesCLI(pass)
		v, err := vault.Open(pass, vaultPathOpts(cmd)...)
		if err != nil {
			return err
		}
		defer func() { _ = v.Close() }()
		pubPath, err := capturePubPath()
		if err != nil {
			return err
		}
		if _, err := capture.InitKeypair(v, pubPath, rotate); err != nil {
			return err
		}
		nonceP, err := captureNoncePath()
		if err != nil {
			return err
		}
		nonce, err := capture.EnsureNonce(nonceP)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "capture key ready (public: %s)\npairing nonce: %s\n", pubPath, nonce)
		return nil
	},
}

var captureHostCmd = &cobra.Command{
	Use:    "capture-host",
	Short:  "Native-messaging host for the Scout Capture extension (launched by the browser)",
	Hidden: true, // not a day-to-day command; the browser launches it
	RunE: func(cmd *cobra.Command, _ []string) error {
		extID, _ := cmd.Flags().GetString("ext-id")
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
		nonceP, err := captureNoncePath()
		if err != nil {
			return err
		}
		return capture.RunHost(cmd.InOrStdin(), cmd.OutOrStdout(), capture.HostConfig{
			Pub:          pub,
			SpoolDir:     spoolDir,
			AllowedExtID: extID,
			NoncePath:    nonceP,
		})
	},
}

var vaultImportCapturesCmd = &cobra.Command{
	Use:   "import-captures",
	Short: "Drain the capture spool into per-site vault profiles (review + confirm)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		pass, err := readPassphraseBytes(cmd.ErrOrStderr(), "Vault passphrase: ")
		if err != nil {
			return err
		}
		defer zeroBytesCLI(pass)
		v, err := vault.Open(pass, vaultPathOpts(cmd)...)
		if err != nil {
			return err
		}
		defer func() { _ = v.Close() }()
		pubPath, err := capturePubPath()
		if err != nil {
			return err
		}
		pub, err := capture.LoadPub(pubPath)
		if err != nil {
			return err
		}
		priv, err := capture.LoadPriv(v)
		if err != nil {
			return err
		}
		spoolDir, err := capture.SpoolDir()
		if err != nil {
			return err
		}
		rep, err := capture.ImportSpool(v, spoolDir, pub, priv)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "imported %d capture(s) across %d site(s); %d quarantined\n",
			rep.Imported, len(rep.Sites), rep.Quarantined)
		return nil
	},
}

func init() {
	vaultCaptureKeyInitCmd.Flags().Bool("rotate", false, "replace any existing capture keypair")
	vaultCaptureKeyInitCmd.Flags().String("vault-file", "", "override vault file path")
	vaultImportCapturesCmd.Flags().String("vault-file", "", "override vault file path")
	captureHostCmd.Flags().String("ext-id", "", "the extension ID permitted to connect")

	vaultCaptureKeyCmd.AddCommand(vaultCaptureKeyInitCmd)
	vaultCmd.AddCommand(vaultCaptureKeyCmd, vaultImportCapturesCmd)
	rootCmd.AddCommand(captureHostCmd)
}
```

- [ ] **Step 4: Run** `go test ./cmd/scout/ -run TestCaptureCommandsRegistered` → PASS; `go build ./cmd/scout/`; `go vet ./cmd/scout/`.

- [ ] **Step 5: Commit (local):**

```bash
git add cmd/scout/capture.go cmd/scout/capture_test.go
git commit -m "feat(cli): wire scout capture-host + vault capture-key init + vault import-captures"
```

---

### Task 8: Native-messaging manifest install/uninstall

**Files:** Create `cmd/scout/capture_manifest_unix.go` (`//go:build !windows`), `cmd/scout/capture_manifest_windows.go`.

The manifest tells the browser how to launch `scout capture-host` and which extension may connect.

- [ ] **Step 1: Add the `install`/`uninstall` subcommands** to `cmd/scout/capture.go` (append to its `init()` and add the command vars):

```go
var captureHostInstallCmd = &cobra.Command{
	Use:   "install <extension-id>",
	Short: "Register the native-messaging host manifest for the given extension ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := installNativeManifest(args[0])
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "installed native-messaging manifest: %s\n", path)
		return nil
	},
}

var captureHostUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove the native-messaging host manifest",
	RunE: func(cmd *cobra.Command, _ []string) error {
		if err := uninstallNativeManifest(); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "removed native-messaging manifest")
		return nil
	},
}
```

And in `init()` add: `captureHostCmd.AddCommand(captureHostInstallCmd, captureHostUninstallCmd)`.

- [ ] **Step 2: Implement** `cmd/scout/capture_manifest_unix.go`:

```go
//go:build !windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const nativeHostName = "com.inovacc.scout.capture"

// nativeManifestDir returns the Chrome/Chromium NativeMessagingHosts dir for this user.
func nativeManifestDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Google", "Chrome", "NativeMessagingHosts"), nil
	default: // linux
		return filepath.Join(home, ".config", "google-chrome", "NativeMessagingHosts"), nil
	}
}

func installNativeManifest(extID string) (string, error) {
	dir, err := nativeManifestDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("scout: capture: mkdir manifest dir: %w", err)
	}
	exe, err := exec.LookPath(os.Args[0])
	if err != nil {
		exe, _ = filepath.Abs(os.Args[0])
	}
	manifest := map[string]any{
		"name":           nativeHostName,
		"description":    "Scout Capture native messaging host",
		"path":           exe,
		"type":           "stdio",
		"allowed_origins": []string{"chrome-extension://" + extID + "/"},
	}
	b, _ := json.MarshalIndent(manifest, "", "  ")
	path := filepath.Join(dir, nativeHostName+".json")
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return "", fmt.Errorf("scout: capture: write manifest: %w", err)
	}
	return path, nil
}

func uninstallNativeManifest() error {
	dir, err := nativeManifestDir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, nativeHostName+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("scout: capture: remove manifest: %w", err)
	}
	return nil
}
```

- [ ] **Step 3: Implement** `cmd/scout/capture_manifest_windows.go`:

```go
//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

const nativeHostName = "com.inovacc.scout.capture"

// On Windows the manifest is a JSON file pointed to by a registry key.
func installNativeManifest(extID string) (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "Scout", "NativeMessagingHosts")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("scout: capture: mkdir manifest dir: %w", err)
	}
	exe, err := os.Executable()
	if err != nil {
		exe = os.Args[0]
	}
	manifest := map[string]any{
		"name":            nativeHostName,
		"description":     "Scout Capture native messaging host",
		"path":            exe,
		"type":            "stdio",
		"allowed_origins": []string{"chrome-extension://" + extID + "/"},
	}
	b, _ := json.MarshalIndent(manifest, "", "  ")
	path := filepath.Join(dir, nativeHostName+".json")
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return "", fmt.Errorf("scout: capture: write manifest: %w", err)
	}
	k, _, err := registry.CreateKey(registry.CURRENT_USER,
		`Software\Google\Chrome\NativeMessagingHosts\`+nativeHostName, registry.SET_VALUE)
	if err != nil {
		return "", fmt.Errorf("scout: capture: create registry key: %w", err)
	}
	defer func() { _ = k.Close() }()
	if err := k.SetStringValue("", path); err != nil {
		return "", fmt.Errorf("scout: capture: set registry value: %w", err)
	}
	return path, nil
}

func uninstallNativeManifest() error {
	_ = registry.DeleteKey(registry.CURRENT_USER,
		`Software\Google\Chrome\NativeMessagingHosts\`+nativeHostName)
	base, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	path := filepath.Join(base, "Scout", "NativeMessagingHosts", nativeHostName+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("scout: capture: remove manifest: %w", err)
	}
	return nil
}
```

> NOTE: `golang.org/x/sys` is already a direct dependency (`go.mod` v0.45.0), so `golang.org/x/sys/windows/registry` imports cleanly — no `go get`/`go mod tidy` needed. The registry key under `HKCU\Software\Google\Chrome\NativeMessagingHosts\` is required for Chrome to locate the host on Windows.

- [ ] **Step 4: Verify** cross-compile both: `go build ./cmd/scout/` (host OS) and `GOOS=windows go build ./cmd/scout/` + `GOOS=linux go build ./cmd/scout/`. `go vet ./cmd/scout/`.

- [ ] **Step 5: Commit (local):**

```bash
git add cmd/scout/capture.go cmd/scout/capture_manifest_unix.go cmd/scout/capture_manifest_windows.go
git commit -m "feat(cli): install/uninstall the per-OS native-messaging host manifest"
```

---

### Task 9: Full verification

- [ ] **Step 1:** `go build ./cmd/scout/ ./pkg/...` → success.
- [ ] **Step 2:** `GOOS=windows go build ./cmd/scout/` and `GOOS=linux go build ./cmd/scout/` → both succeed (manifest build tags).
- [ ] **Step 3:** `go vet ./cmd/scout/ ./pkg/scout/capture/` → clean.
- [ ] **Step 4:** `go test ./pkg/scout/capture/ ./cmd/scout/` → all pass (no browser needed).
- [ ] **Step 5:** Confirm the host never returns secrets: `grep -n 'Type: "ack"\|Type: "error"' pkg/scout/capture/host.go` — every outbound message sets only `ID`/`Code`/`Message`/`HostVersion`, never `Password`/`Cookies`/`Storage`.
- [ ] **Step 6: Stop — await push approval.** Do NOT push or ff-merge. When approved: `git switch main && git merge --ff-only feat/scout-capture-host && git push origin main`.

---

## Out of scope (later phases)

- The MV3 extension (`extensions/scout-capture/`) — Phase 2 (session capture) + Phase 3 (consented login capture).
- Extension signing / reproducible build / packaging — Phase 4.
- Firefox parity — Phase 5.
