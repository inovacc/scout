# Secrets-Isolation Vault Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a `pkg/scout/vault` package that stores named secret profiles in one Argon2id+AES-256-GCM file, hands secrets out as swap-locked zeroable buffers, injects browser-bound secrets via CDP (never env vars), and re-keys atomically — plus a `scout vault` CLI.

**Architecture:** A `LockedBuffer` primitive (`[]byte` + `VirtualLock`/`Mlock` + explicit zero) holds every secret in memory. A `Vault` orchestrator opens/decrypts one `<scouthome>/profiles/vault.bin` file into in-memory `SecretProfile`s keyed by opaque random IDs, and writes changes back through an atomic temp-file-rename. A `Handle` injects a profile's cookies/storage/headers into a live `*scout.Page` via the existing public CDP methods, and yields scout-internal secrets as zeroable buffers. Crypto mirrors `pkg/scout/scraper/crypt` parameters but derives the key from a `[]byte` passphrase so the passphrase never becomes an immutable `string`.

**Tech Stack:** Go 1.26, `golang.org/x/crypto/argon2`, `crypto/cipher` (AES-256-GCM), `golang.org/x/sys/{windows,unix}` (memory locking), `golang.org/x/term` (passphrase prompt), `github.com/spf13/cobra` (CLI), real-browser + `httptest` tests.

---

## Conventions for every task

- **Module root:** `github.com/inovacc/scout`. The vault lives at `pkg/scout/vault` (import path `github.com/inovacc/scout/pkg/scout/vault`). `pkg/scout/vault` MAY import `github.com/inovacc/scout/pkg/scout` and `.../internal/engine/scouthome` (same module, allowed).
- **Error wrapping:** always `fmt.Errorf("scout: vault: <action>: %w", err)`.
- **Run a single test:** `go test ./pkg/scout/vault/ -run TestName -v`
- **Windows PATH fallback:** if `go` is not resolved on PATH, invoke `& 'C:\Program Files\Go\bin\go.exe' test ./pkg/scout/vault/ -run TestName -v` from PowerShell.
- **Browser tests** must call `t.Skip` when Chromium is unavailable (see Task 9 helper). Pure crypto/memory/store/id tests need no browser.
- **No secret ever becomes a `string`** for arbitrary secret values, and no secret is logged or printed by `list`/`get`.
- **Commit** after every task with no AI attribution.

## File structure (created by this plan)

| File | Responsibility |
|------|----------------|
| `pkg/scout/vault/secmem.go` | `LockedBuffer` (alloc/Bytes/Len/Equal/Zero/Close) + platform-neutral logic |
| `pkg/scout/vault/secmem_windows.go` | `lockPages`/`unlockPages` via `VirtualLock`/`VirtualUnlock` (`//go:build windows`) |
| `pkg/scout/vault/secmem_unix.go` | `lockPages`/`unlockPages` via `Mlock`/`Munlock` (`//go:build !windows`) |
| `pkg/scout/vault/id.go` | opaque random profile-ID generation |
| `pkg/scout/vault/crypto.go` | `seal`/`open`: Argon2id key from `[]byte` passphrase + AES-GCM, versioned header |
| `pkg/scout/vault/atomic.go` | `atomicWrite(path,data,mode)` replicating session `writeFileAtomic` |
| `pkg/scout/vault/profile.go` | `SecretProfile`, `SecretProfileInput`, `ProfileMeta`, `OriginStore`, `Cookie` alias |
| `pkg/scout/vault/store.go` | on-disk `vaultData` serialize→seal→atomic write; load→open→decode; default path |
| `pkg/scout/vault/vault.go` | `Vault`: `Open`/`Create`/`Set`/`Get`/`Use`/`List`/`Remove`/`Rotate`/`Close` |
| `pkg/scout/vault/inject.go` | `Handle`: `ApplyToPage`/`Secret`/`Close` |
| `pkg/scout/vault/import.go` | `FromUserProfile(path)` importer for `--from-profile` |
| `pkg/scout/vault/testhelp_test.go` | browser+httptest helper for injection tests |
| `cmd/scout/vault.go` | `scout vault init/set/get/list/use/rotate/rm` |
| `cmd/scout/helpers.go` (modify) | add `readPassphraseBytes` (zeroable) + `SCOUT_VAULT_PASSPHRASE` |
| `internal/engine/profile.go` (modify) | mark `UserProfile.Cookies/Storage/Headers` `// Deprecated:` |

---

## Task 1: LockedBuffer core (platform-neutral)

**Files:**
- Create: `pkg/scout/vault/secmem.go`
- Test: `pkg/scout/vault/secmem_test.go`

- [ ] **Step 1: Write the failing test**

```go
// pkg/scout/vault/secmem_test.go
package vault

import (
	"bytes"
	"testing"
)

func TestLockedBufferZeroOnClose(t *testing.T) {
	secret := []byte("hunter2-super-secret")
	lb := NewLockedBuffer(append([]byte(nil), secret...))

	if !bytes.Equal(lb.Bytes(), secret) {
		t.Fatalf("Bytes() = %q, want %q", lb.Bytes(), secret)
	}
	if lb.Len() != len(secret) {
		t.Fatalf("Len() = %d, want %d", lb.Len(), len(secret))
	}

	backing := lb.Bytes() // same backing array
	if err := lb.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	for i, b := range backing {
		if b != 0 {
			t.Fatalf("byte %d = %d after Close, want 0", i, b)
		}
	}
	if lb.Len() != 0 {
		t.Fatalf("Len() = %d after Close, want 0", lb.Len())
	}
}

func TestLockedBufferEqualConstantTime(t *testing.T) {
	lb := NewLockedBuffer([]byte("token-abc"))
	defer func() { _ = lb.Close() }()

	if !lb.Equal([]byte("token-abc")) {
		t.Fatal("Equal returned false for matching token")
	}
	if lb.Equal([]byte("token-xyz")) {
		t.Fatal("Equal returned true for non-matching token")
	}
	if lb.Equal([]byte("token-abc-longer")) {
		t.Fatal("Equal returned true for different-length token")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/scout/vault/ -run TestLockedBuffer -v`
Expected: FAIL — `undefined: NewLockedBuffer`.

- [ ] **Step 3: Write minimal implementation**

```go
// pkg/scout/vault/secmem.go
// Package vault stores secret profiles in one encrypted file and injects
// secrets into live browser pages without leaking plaintext to child processes.
package vault

import "crypto/subtle"

// LockedBuffer wraps a []byte holding secret material. On allocation it best-effort
// locks the pages out of swap; Zero overwrites the bytes and unlocks. Secrets held
// here must NEVER be converted to string (strings are immutable and GC-pinned).
type LockedBuffer struct {
	buf    []byte
	locked bool
}

// NewLockedBuffer takes ownership of b (does not copy) and attempts to lock it.
// Locking is best-effort: failure is silently tolerated (never fatal).
func NewLockedBuffer(b []byte) *LockedBuffer {
	lb := &LockedBuffer{buf: b}
	if len(b) > 0 && lockPages(b) == nil {
		lb.locked = true
	}
	return lb
}

// Bytes returns the underlying secret slice. Do not retain beyond the buffer's life.
func (lb *LockedBuffer) Bytes() []byte { return lb.buf }

// Len returns the current secret length (0 after Zero/Close).
func (lb *LockedBuffer) Len() int { return len(lb.buf) }

// Equal reports whether the buffer equals other in constant time.
func (lb *LockedBuffer) Equal(other []byte) bool {
	return subtle.ConstantTimeCompare(lb.buf, other) == 1
}

// Zero overwrites the secret with zeros and unlocks the pages.
func (lb *LockedBuffer) Zero() {
	for i := range lb.buf {
		lb.buf[i] = 0
	}
	if lb.locked {
		_ = unlockPages(lb.buf)
		lb.locked = false
	}
	lb.buf = lb.buf[:0]
}

// Close zeros and releases the buffer. Always returns nil; signature allows defer.
func (lb *LockedBuffer) Close() error {
	lb.Zero()
	return nil
}
```

- [ ] **Step 4: Run test to verify it fails to compile (missing lockPages)**

Run: `go test ./pkg/scout/vault/ -run TestLockedBuffer -v`
Expected: FAIL — `undefined: lockPages` / `undefined: unlockPages` (provided in Task 2).

- [ ] **Step 5: Commit**

```bash
git add pkg/scout/vault/secmem.go pkg/scout/vault/secmem_test.go
git commit -m "feat(vault): LockedBuffer secret-memory primitive"
```

---

## Task 2: Platform memory-locking (VirtualLock / Mlock)

**Files:**
- Create: `pkg/scout/vault/secmem_windows.go`
- Create: `pkg/scout/vault/secmem_unix.go`

- [ ] **Step 1: Write the Windows implementation**

```go
// pkg/scout/vault/secmem_windows.go
//go:build windows

package vault

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// lockPages best-effort pins b out of the pagefile via VirtualLock.
func lockPages(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	return windows.VirtualLock(uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)))
}

// unlockPages reverses lockPages. Zeroing must happen before this call.
func unlockPages(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	return windows.VirtualUnlock(uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)))
}
```

- [ ] **Step 2: Write the Unix implementation**

```go
// pkg/scout/vault/secmem_unix.go
//go:build !windows

package vault

import "golang.org/x/sys/unix"

// lockPages best-effort pins b out of swap via mlock. RLIMIT_MEMLOCK may reject
// this (EAGAIN) on unprivileged processes; callers treat failure as non-fatal.
func lockPages(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	return unix.Mlock(b)
}

// unlockPages reverses lockPages. Zeroing must happen before this call.
func unlockPages(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	return unix.Munlock(b)
}
```

- [ ] **Step 3: Run the Task 1 tests to verify they pass**

Run: `go test ./pkg/scout/vault/ -run TestLockedBuffer -v`
Expected: PASS (both tests). Locking is best-effort, so they pass even where mlock is denied.

- [ ] **Step 4: Commit**

```bash
git add pkg/scout/vault/secmem_windows.go pkg/scout/vault/secmem_unix.go
git commit -m "feat(vault): best-effort page locking (VirtualLock/Mlock)"
```

---

## Task 3: Opaque profile IDs

**Files:**
- Create: `pkg/scout/vault/id.go`
- Test: `pkg/scout/vault/id_test.go`

- [ ] **Step 1: Write the failing test**

```go
// pkg/scout/vault/id_test.go
package vault

import (
	"regexp"
	"testing"
)

func TestNewIDFormatAndUniqueness(t *testing.T) {
	re := regexp.MustCompile(`^[A-Za-z0-9_-]{22}$`)
	seen := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		id := newID()
		if !re.MatchString(id) {
			t.Fatalf("id %q does not match %s", id, re)
		}
		if seen[id] {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = true
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/scout/vault/ -run TestNewID -v`
Expected: FAIL — `undefined: newID`.

- [ ] **Step 3: Write minimal implementation**

```go
// pkg/scout/vault/id.go
package vault

import (
	"crypto/rand"
	"encoding/base64"
)

// newID returns an opaque, enumeration-resistant 22-char profile ID
// (128 bits of entropy, base64url without padding).
func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("scout: vault: crypto/rand failed: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b[:])
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/scout/vault/ -run TestNewID -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/scout/vault/id.go pkg/scout/vault/id_test.go
git commit -m "feat(vault): opaque random profile IDs"
```

---

## Task 4: At-rest crypto (Argon2id + AES-256-GCM, []byte passphrase)

**Files:**
- Create: `pkg/scout/vault/crypto.go`
- Test: `pkg/scout/vault/crypto_test.go`

- [ ] **Step 1: Write the failing test**

```go
// pkg/scout/vault/crypto_test.go
package vault

import (
	"bytes"
	"testing"
)

func TestSealOpenRoundTrip(t *testing.T) {
	pass := []byte("correct horse battery staple")
	plain := bytes.Repeat([]byte("vault-payload-"), 800) // ~10KB

	blob, err := seal(plain, pass)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if bytes.Contains(blob, []byte("vault-payload")) {
		t.Fatal("ciphertext contains plaintext")
	}
	if blob[0] != vaultVersion || blob[1] != cryptVersion {
		t.Fatalf("header versions = %d,%d want %d,%d", blob[0], blob[1], vaultVersion, cryptVersion)
	}

	got, err := open(blob, pass)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatal("round-trip mismatch")
	}
}

func TestOpenWrongPassphrase(t *testing.T) {
	blob, err := seal([]byte("data"), []byte("right-pass"))
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if _, err := open(blob, []byte("wrong-pass")); err == nil {
		t.Fatal("open succeeded with wrong passphrase")
	}
}

func TestOpenTruncatedBlob(t *testing.T) {
	if _, err := open([]byte{vaultVersion, cryptVersion, 0x00}, []byte("p")); err == nil {
		t.Fatal("open succeeded on truncated blob")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/scout/vault/ -run 'TestSeal|TestOpen' -v`
Expected: FAIL — `undefined: seal` / `undefined: vaultVersion`.

- [ ] **Step 3: Write minimal implementation**

```go
// pkg/scout/vault/crypto.go
package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"

	"golang.org/x/crypto/argon2"
)

// Crypto format (mirrors pkg/scout/scraper/crypt parameters but derives the key
// from a []byte passphrase so it can be zeroed):
//
//	[vaultVersion:1][cryptVersion:1][salt:saltLen][nonce:nonceLen][ciphertext+GCM tag]
const (
	vaultVersion = byte(0x01)
	cryptVersion = byte(0x01)
	saltLen      = 32
	nonceLen     = 12
	keyLen       = 32
	argonTime    = 3
	argonMemory  = 64 * 1024 // KiB = 64 MiB
	argonThreads = 4
	headerLen    = 2 + saltLen + nonceLen
)

var errVaultFormat = errors.New("scout: vault: malformed encrypted blob")

// deriveKey returns a 32-byte AES key. Caller must zero the returned slice.
func deriveKey(passphrase, salt []byte) []byte {
	return argon2.IDKey(passphrase, salt, argonTime, argonMemory, argonThreads, keyLen)
}

func seal(plaintext, passphrase []byte) ([]byte, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("scout: vault: salt: %w", err)
	}
	key := deriveKey(passphrase, salt)
	defer zero(key)

	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, nonceLen)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("scout: vault: nonce: %w", err)
	}

	out := make([]byte, 0, headerLen+len(plaintext)+gcm.Overhead())
	out = append(out, vaultVersion, cryptVersion)
	out = append(out, salt...)
	out = append(out, nonce...)
	return gcm.Seal(out, nonce, plaintext, nil), nil
}

func open(blob, passphrase []byte) ([]byte, error) {
	if len(blob) < headerLen {
		return nil, errVaultFormat
	}
	if blob[0] != vaultVersion || blob[1] != cryptVersion {
		return nil, errVaultFormat
	}
	salt := blob[2 : 2+saltLen]
	nonce := blob[2+saltLen : headerLen]
	ct := blob[headerLen:]

	key := deriveKey(passphrase, salt)
	defer zero(key)

	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("scout: vault: decrypt: %w", err)
	}
	return plain, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("scout: vault: cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("scout: vault: gcm: %w", err)
	}
	return gcm, nil
}

func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/scout/vault/ -run 'TestSeal|TestOpen' -v`
Expected: PASS (all three).

- [ ] **Step 5: Commit**

```bash
git add pkg/scout/vault/crypto.go pkg/scout/vault/crypto_test.go
git commit -m "feat(vault): Argon2id+AES-GCM at-rest crypto with zeroable key"
```

---

## Task 5: Atomic file write

**Files:**
- Create: `pkg/scout/vault/atomic.go`
- Test: `pkg/scout/vault/atomic_test.go`

- [ ] **Step 1: Write the failing test**

```go
// pkg/scout/vault/atomic_test.go
package vault

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWriteCreatesFileWithMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vault.bin")

	if err := atomicWrite(path, []byte("payload"), 0o600); err != nil {
		t.Fatalf("atomicWrite: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != "payload" {
		t.Fatalf("content = %q, want %q", got, "payload")
	}
	// No leftover temp files in the directory.
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("directory has %d entries, want 1 (leftover temp file?)", len(entries))
	}
}

func TestAtomicWriteReplacesAtomically(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vault.bin")
	if err := atomicWrite(path, []byte("v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := atomicWrite(path, []byte("v2-longer"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "v2-longer" {
		t.Fatalf("content = %q, want v2-longer", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/scout/vault/ -run TestAtomicWrite -v`
Expected: FAIL — `undefined: atomicWrite`.

- [ ] **Step 3: Write minimal implementation**

Replicates `internal/engine/session/atomic.go:writeFileAtomic` (unexported, not importable): temp file in same dir → write → fsync → close → chmod → rename, with temp cleanup on every error path.

```go
// pkg/scout/vault/atomic.go
package vault

import (
	"fmt"
	"os"
	"path/filepath"
)

// atomicWrite writes data to path via a temp-file-then-rename, so a crash never
// leaves a partial file. The parent directory must already exist.
func atomicWrite(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	f, err := os.CreateTemp(dir, base+".tmp.*")
	if err != nil {
		return fmt.Errorf("scout: vault: create temp: %w", err)
	}
	tmp := f.Name()
	cleanup := func() { _ = f.Close(); _ = os.Remove(tmp) }

	if _, err := f.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("scout: vault: write temp: %w", err)
	}
	if err := f.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("scout: vault: sync temp: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("scout: vault: close temp: %w", err)
	}
	if err := os.Chmod(tmp, mode); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("scout: vault: chmod temp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("scout: vault: rename: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/scout/vault/ -run TestAtomicWrite -v`
Expected: PASS (both).

- [ ] **Step 5: Commit**

```bash
git add pkg/scout/vault/atomic.go pkg/scout/vault/atomic_test.go
git commit -m "feat(vault): atomic temp-file-rename writer"
```

---

## Task 6: Profile types

**Files:**
- Create: `pkg/scout/vault/profile.go`
- Test: `pkg/scout/vault/profile_test.go`

- [ ] **Step 1: Write the failing test**

```go
// pkg/scout/vault/profile_test.go
package vault

import "testing"

func TestSecretProfileCloseZeros(t *testing.T) {
	p := &SecretProfile{
		ID:   "abc",
		Name: "demo",
		Secrets: map[string]*LockedBuffer{
			"api_key": NewLockedBuffer([]byte("sk-123")),
		},
		Headers: map[string]*LockedBuffer{
			"Authorization": NewLockedBuffer([]byte("Bearer xyz")),
		},
	}
	backing := p.Secrets["api_key"].Bytes()
	p.Close()
	for i, b := range backing {
		if b != 0 {
			t.Fatalf("secret byte %d not zeroed after Close", i)
		}
	}
}

func TestProfileMetaHidesValues(t *testing.T) {
	p := &SecretProfile{
		ID:      "id1",
		Name:    "n",
		Secrets: map[string]*LockedBuffer{"k": NewLockedBuffer([]byte("v"))},
	}
	defer p.Close()
	m := p.Meta()
	if m.ID != "id1" || m.Name != "n" {
		t.Fatalf("meta id/name = %q/%q", m.ID, m.Name)
	}
	if len(m.SecretKeys) != 1 || m.SecretKeys[0] != "k" {
		t.Fatalf("SecretKeys = %v, want [k]", m.SecretKeys)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/scout/vault/ -run 'TestSecretProfile|TestProfileMeta' -v`
Expected: FAIL — `undefined: SecretProfile`.

- [ ] **Step 3: Write minimal implementation**

```go
// pkg/scout/vault/profile.go
package vault

import (
	"sort"
	"time"

	"github.com/inovacc/scout/pkg/scout"
)

// Cookie reuses the public scout cookie type so injection needs no mapping.
type Cookie = scout.Cookie

// OriginStore holds per-origin web storage absorbed from a UserProfile.
type OriginStore struct {
	LocalStorage   map[string]string `json:"local_storage,omitempty"`
	SessionStorage map[string]string `json:"session_storage,omitempty"`
}

// SecretProfile is a decrypted, in-memory secret profile. All arbitrary secret
// values and auth headers are held in LockedBuffers. Call Close to zero them.
type SecretProfile struct {
	ID        string
	Name      string
	Secrets   map[string]*LockedBuffer
	Cookies   []Cookie
	Storage   map[string]OriginStore
	Headers   map[string]*LockedBuffer
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SecretProfileInput is the plaintext upsert payload accepted by Vault.Set.
// An empty ID creates a new profile; a known ID updates it.
type SecretProfileInput struct {
	ID      string
	Name    string
	Secrets map[string][]byte
	Cookies []Cookie
	Storage map[string]OriginStore
	Headers map[string][]byte
}

// ProfileMeta describes a profile without exposing any secret value.
type ProfileMeta struct {
	ID         string
	Name       string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	SecretKeys []string
	HeaderKeys []string
	CookieN    int
}

// Meta returns non-secret metadata for the profile.
func (p *SecretProfile) Meta() ProfileMeta {
	m := ProfileMeta{ID: p.ID, Name: p.Name, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt, CookieN: len(p.Cookies)}
	for k := range p.Secrets {
		m.SecretKeys = append(m.SecretKeys, k)
	}
	for k := range p.Headers {
		m.HeaderKeys = append(m.HeaderKeys, k)
	}
	sort.Strings(m.SecretKeys)
	sort.Strings(m.HeaderKeys)
	return m
}

// Close zeros every LockedBuffer held by the profile.
func (p *SecretProfile) Close() {
	for _, lb := range p.Secrets {
		lb.Zero()
	}
	for _, lb := range p.Headers {
		lb.Zero()
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/scout/vault/ -run 'TestSecretProfile|TestProfileMeta' -v`
Expected: PASS (both).

- [ ] **Step 5: Commit**

```bash
git add pkg/scout/vault/profile.go pkg/scout/vault/profile_test.go
git commit -m "feat(vault): SecretProfile / Input / Meta types"
```

---

## Task 7: Store (serialize → seal → atomic write; load → open → decode)

**Files:**
- Create: `pkg/scout/vault/store.go`
- Test: `pkg/scout/vault/store_test.go`

- [ ] **Step 1: Write the failing test**

```go
// pkg/scout/vault/store_test.go
package vault

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.bin")
	pass := []byte("store-pass")

	in := &vaultData{Version: 1, Profiles: []storedProfile{{
		ID:        "p1",
		Name:      "demo",
		Secrets:   map[string][]byte{"api_key": []byte("sk-abc")},
		Cookies:   []Cookie{{Name: "sid", Value: "v", Domain: "example.com"}},
		Headers:   map[string][]byte{"Authorization": []byte("Bearer t")},
		CreatedAt: time.Unix(1, 0).UTC(),
		UpdatedAt: time.Unix(2, 0).UTC(),
	}}}

	if err := saveVault(path, in, pass); err != nil {
		t.Fatalf("saveVault: %v", err)
	}
	out, err := loadVault(path, pass)
	if err != nil {
		t.Fatalf("loadVault: %v", err)
	}
	if len(out.Profiles) != 1 || out.Profiles[0].ID != "p1" {
		t.Fatalf("profiles = %+v", out.Profiles)
	}
	if string(out.Profiles[0].Secrets["api_key"]) != "sk-abc" {
		t.Fatalf("secret = %q", out.Profiles[0].Secrets["api_key"])
	}
}

func TestLoadVaultWrongPassphrase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.bin")
	if err := saveVault(path, &vaultData{Version: 1}, []byte("right")); err != nil {
		t.Fatal(err)
	}
	if _, err := loadVault(path, []byte("wrong")); err == nil {
		t.Fatal("loadVault succeeded with wrong passphrase")
	}
}

func TestLoadVaultMissingFile(t *testing.T) {
	_, err := loadVault(filepath.Join(t.TempDir(), "nope.bin"), []byte("p"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/scout/vault/ -run 'TestStore|TestLoadVault' -v`
Expected: FAIL — `undefined: vaultData`.

- [ ] **Step 3: Write minimal implementation**

```go
// pkg/scout/vault/store.go
package vault

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/inovacc/scout/internal/engine/scouthome"
)

// storedProfile is the on-disk (inside the encrypted blob) form of a profile.
type storedProfile struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name,omitempty"`
	Secrets   map[string][]byte      `json:"secrets,omitempty"`
	Cookies   []Cookie               `json:"cookies,omitempty"`
	Storage   map[string]OriginStore `json:"storage,omitempty"`
	Headers   map[string][]byte      `json:"headers,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

type vaultData struct {
	Version  int             `json:"version"`
	Profiles []storedProfile `json:"profiles,omitempty"`
}

// ErrVaultNotFound is returned by loadVault when the vault file does not exist.
var ErrVaultNotFound = errors.New("scout: vault: not initialized")

// defaultVaultPath resolves <scouthome>/profiles/vault.bin and creates the
// profiles directory with 0o700 permissions.
func defaultVaultPath() (string, error) {
	dir, err := scouthome.Sub("profiles")
	if err != nil {
		return "", fmt.Errorf("scout: vault: resolve home: %w", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("scout: vault: mkdir: %w", err)
	}
	_ = os.Chmod(dir, 0o700) // umask guard on Unix; no-op on Windows ACLs
	return filepath.Join(dir, "vault.bin"), nil
}

func saveVault(path string, data *vaultData, passphrase []byte) error {
	plain, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("scout: vault: marshal: %w", err)
	}
	defer zero(plain)

	blob, err := seal(plain, passphrase)
	if err != nil {
		return err
	}
	return atomicWrite(path, blob, 0o600)
}

func loadVault(path string, passphrase []byte) (*vaultData, error) {
	blob, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrVaultNotFound
		}
		return nil, fmt.Errorf("scout: vault: read: %w", err)
	}
	plain, err := open(blob, passphrase)
	if err != nil {
		return nil, err
	}
	defer zero(plain)

	var data vaultData
	if err := json.Unmarshal(plain, &data); err != nil {
		return nil, fmt.Errorf("scout: vault: unmarshal: %w", err)
	}
	return &data, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/scout/vault/ -run 'TestStore|TestLoadVault' -v`
Expected: PASS (all three).

- [ ] **Step 5: Commit**

```bash
git add pkg/scout/vault/store.go pkg/scout/vault/store_test.go
git commit -m "feat(vault): encrypted store load/save at <scouthome>/profiles/vault.bin"
```

---

## Task 8: Vault orchestrator (Open/Create/Set/Get/List/Remove/Close)

**Files:**
- Create: `pkg/scout/vault/vault.go`
- Test: `pkg/scout/vault/vault_test.go`

- [ ] **Step 1: Write the failing test**

```go
// pkg/scout/vault/vault_test.go
package vault

import (
	"path/filepath"
	"testing"
)

func TestVaultSetGetListRemove(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.bin")
	pass := []byte("vault-pass")

	v, err := Create(pass, WithPath(path))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	id, err := v.Set(SecretProfileInput{
		Name:    "openai",
		Secrets: map[string][]byte{"api_key": []byte("sk-live-1")},
	})
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if id == "" {
		t.Fatal("Set returned empty id")
	}
	if err := v.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Reopen from disk and verify persistence + metadata hygiene.
	v2, err := Open(pass, WithPath(path))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = v2.Close() }()

	metas := v2.List()
	if len(metas) != 1 || metas[0].ID != id || metas[0].Name != "openai" {
		t.Fatalf("List = %+v", metas)
	}
	if len(metas[0].SecretKeys) != 1 || metas[0].SecretKeys[0] != "api_key" {
		t.Fatalf("SecretKeys = %v", metas[0].SecretKeys)
	}

	p, err := v2.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !p.Secrets["api_key"].Equal([]byte("sk-live-1")) {
		t.Fatal("secret mismatch after reopen")
	}
	p.Close()

	if err := v2.Remove(id); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(v2.List()) != 0 {
		t.Fatal("profile not removed")
	}
}

func TestVaultSetUpdatesExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.bin")
	v, _ := Create([]byte("p"), WithPath(path))
	defer func() { _ = v.Close() }()

	id, _ := v.Set(SecretProfileInput{Name: "a", Secrets: map[string][]byte{"k": []byte("1")}})
	id2, err := v.Set(SecretProfileInput{ID: id, Name: "a2", Secrets: map[string][]byte{"k": []byte("2")}})
	if err != nil || id2 != id {
		t.Fatalf("update returned id=%q err=%v, want %q", id2, err, id)
	}
	if len(v.List()) != 1 {
		t.Fatal("update created a duplicate profile")
	}
	p, _ := v.Get(id)
	defer p.Close()
	if !p.Secrets["k"].Equal([]byte("2")) {
		t.Fatal("value not updated")
	}
}

func TestGetUnknownID(t *testing.T) {
	v, _ := Create([]byte("p"), WithPath(filepath.Join(t.TempDir(), "vault.bin")))
	defer func() { _ = v.Close() }()
	if _, err := v.Get("does-not-exist"); err == nil {
		t.Fatal("Get succeeded for unknown id")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/scout/vault/ -run 'TestVault|TestGetUnknown' -v`
Expected: FAIL — `undefined: Create` / `undefined: Open`.

- [ ] **Step 3: Write minimal implementation**

```go
// pkg/scout/vault/vault.go
package vault

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrProfileNotFound is returned when an ID does not match any profile.
var ErrProfileNotFound = errors.New("scout: vault: profile not found")

type config struct{ path string }

// Option configures a Vault.
type Option func(*config)

// WithPath overrides the default <scouthome>/profiles/vault.bin location.
func WithPath(p string) Option { return func(c *config) { c.path = p } }

func resolve(opts []Option) (config, error) {
	c := config{}
	for _, o := range opts {
		o(&c)
	}
	if c.path == "" {
		p, err := defaultVaultPath()
		if err != nil {
			return c, err
		}
		c.path = p
	}
	return c, nil
}

// Vault is an opened secret store. It holds the passphrase in a LockedBuffer for
// the lifetime of the handle so it can re-encrypt on every mutation and on Rotate.
type Vault struct {
	mu   sync.Mutex
	path string
	pass *LockedBuffer
	data *vaultData
}

// Create initializes a new empty vault at the resolved path. It errors if a
// vault already exists there.
func Create(passphrase []byte, opts ...Option) (*Vault, error) {
	c, err := resolve(opts)
	if err != nil {
		return nil, err
	}
	if _, err := loadVault(c.path, passphrase); err == nil {
		return nil, fmt.Errorf("scout: vault: already initialized at %s", c.path)
	} else if !errors.Is(err, ErrVaultNotFound) {
		// A decrypt error means a file exists but the passphrase is wrong — still "already exists".
		return nil, fmt.Errorf("scout: vault: already initialized at %s", c.path)
	}
	data := &vaultData{Version: 1}
	if err := saveVault(c.path, data, passphrase); err != nil {
		return nil, err
	}
	return &Vault{path: c.path, pass: NewLockedBuffer(append([]byte(nil), passphrase...)), data: data}, nil
}

// Open decrypts an existing vault.
func Open(passphrase []byte, opts ...Option) (*Vault, error) {
	c, err := resolve(opts)
	if err != nil {
		return nil, err
	}
	data, err := loadVault(c.path, passphrase)
	if err != nil {
		return nil, err
	}
	return &Vault{path: c.path, pass: NewLockedBuffer(append([]byte(nil), passphrase...)), data: data}, nil
}

// Set upserts a profile and persists the vault. Returns the profile ID.
func (v *Vault) Set(in SecretProfileInput) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	now := time.Now().UTC()
	sp := storedProfile{
		ID: in.ID, Name: in.Name, Secrets: in.Secrets, Cookies: in.Cookies,
		Storage: in.Storage, Headers: in.Headers, UpdatedAt: now,
	}
	if sp.ID == "" {
		sp.ID = newID()
		sp.CreatedAt = now
		v.data.Profiles = append(v.data.Profiles, sp)
	} else {
		idx := v.indexOf(sp.ID)
		if idx < 0 {
			return "", ErrProfileNotFound
		}
		sp.CreatedAt = v.data.Profiles[idx].CreatedAt
		v.data.Profiles[idx] = sp
	}
	if err := saveVault(v.path, v.data, v.pass.Bytes()); err != nil {
		return "", err
	}
	return sp.ID, nil
}

// Get returns a decrypted profile copy. Caller must Close it.
func (v *Vault) Get(id string) (*SecretProfile, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	idx := v.indexOf(id)
	if idx < 0 {
		return nil, ErrProfileNotFound
	}
	return materialize(v.data.Profiles[idx]), nil
}

// List returns metadata for every profile (never any secret value).
func (v *Vault) List() []ProfileMeta {
	v.mu.Lock()
	defer v.mu.Unlock()
	out := make([]ProfileMeta, 0, len(v.data.Profiles))
	for _, sp := range v.data.Profiles {
		p := materialize(sp)
		out = append(out, p.Meta())
		p.Close()
	}
	return out
}

// Remove deletes a profile and persists the vault.
func (v *Vault) Remove(id string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	idx := v.indexOf(id)
	if idx < 0 {
		return ErrProfileNotFound
	}
	// Zero the stored secret bytes before dropping the entry.
	zeroStored(&v.data.Profiles[idx])
	v.data.Profiles = append(v.data.Profiles[:idx], v.data.Profiles[idx+1:]...)
	return saveVault(v.path, v.data, v.pass.Bytes())
}

// Close zeros the cached passphrase and decrypted secret bytes.
func (v *Vault) Close() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	for i := range v.data.Profiles {
		zeroStored(&v.data.Profiles[i])
	}
	v.pass.Zero()
	return nil
}

func (v *Vault) indexOf(id string) int {
	for i := range v.data.Profiles {
		if v.data.Profiles[i].ID == id {
			return i
		}
	}
	return -1
}

func materialize(sp storedProfile) *SecretProfile {
	p := &SecretProfile{
		ID: sp.ID, Name: sp.Name, Cookies: sp.Cookies, Storage: sp.Storage,
		CreatedAt: sp.CreatedAt, UpdatedAt: sp.UpdatedAt,
		Secrets: map[string]*LockedBuffer{}, Headers: map[string]*LockedBuffer{},
	}
	for k, val := range sp.Secrets {
		p.Secrets[k] = NewLockedBuffer(append([]byte(nil), val...))
	}
	for k, val := range sp.Headers {
		p.Headers[k] = NewLockedBuffer(append([]byte(nil), val...))
	}
	return p
}

func zeroStored(sp *storedProfile) {
	for _, val := range sp.Secrets {
		zero(val)
	}
	for _, val := range sp.Headers {
		zero(val)
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/scout/vault/ -run 'TestVault|TestGetUnknown' -v`
Expected: PASS (all).

- [ ] **Step 5: Commit**

```bash
git add pkg/scout/vault/vault.go pkg/scout/vault/vault_test.go
git commit -m "feat(vault): Vault orchestrator (Create/Open/Set/Get/List/Remove/Close)"
```

---

## Task 9: Rotation

**Files:**
- Modify: `pkg/scout/vault/vault.go` (add `Rotate`)
- Test: `pkg/scout/vault/rotate_test.go`

- [ ] **Step 1: Write the failing test**

```go
// pkg/scout/vault/rotate_test.go
package vault

import (
	"path/filepath"
	"testing"
)

func TestRotateRekeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.bin")
	oldPass := []byte("old-pass")
	newPass := []byte("new-pass")

	v, _ := Create(oldPass, WithPath(path))
	id, _ := v.Set(SecretProfileInput{Name: "x", Secrets: map[string][]byte{"k": []byte("v")}})
	if err := v.Rotate(newPass); err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	_ = v.Close()

	if _, err := Open(oldPass, WithPath(path)); err == nil {
		t.Fatal("old passphrase still opens the vault after rotation")
	}
	v2, err := Open(newPass, WithPath(path))
	if err != nil {
		t.Fatalf("Open with new passphrase: %v", err)
	}
	defer func() { _ = v2.Close() }()
	p, err := v2.Get(id)
	if err != nil {
		t.Fatalf("Get after rotate: %v", err)
	}
	defer p.Close()
	if !p.Secrets["k"].Equal([]byte("v")) {
		t.Fatal("secret lost across rotation")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/scout/vault/ -run TestRotate -v`
Expected: FAIL — `v.Rotate undefined`.

- [ ] **Step 3: Add the implementation to `pkg/scout/vault/vault.go`**

```go
// Rotate re-encrypts the vault under a new passphrase and atomically rewrites it.
// The old cached passphrase buffer is zeroed and replaced.
func (v *Vault) Rotate(newPassphrase []byte) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if err := saveVault(v.path, v.data, newPassphrase); err != nil {
		return err
	}
	v.pass.Zero()
	v.pass = NewLockedBuffer(append([]byte(nil), newPassphrase...))
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/scout/vault/ -run TestRotate -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/scout/vault/vault.go pkg/scout/vault/rotate_test.go
git commit -m "feat(vault): atomic passphrase rotation"
```

---

## Task 10: Handle + CDP injection

**Files:**
- Create: `pkg/scout/vault/inject.go`
- Create: `pkg/scout/vault/testhelp_test.go`
- Test: `pkg/scout/vault/inject_test.go`

- [ ] **Step 1: Write the browser test helper**

```go
// pkg/scout/vault/testhelp_test.go
package vault

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/inovacc/scout/pkg/scout"
)

// newInjectTestBrowser returns an owned headless browser, skipping if Chromium
// is unavailable. Caller defers the returned cleanup.
func newInjectTestBrowser(t *testing.T) (*scout.Browser, func()) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping browser test in short mode")
	}
	b, err := scout.New() // headless is the default
	if err != nil {
		t.Skipf("browser unavailable: %v", err)
	}
	return b, func() { _ = b.Close() }
}

// echoServer reflects request cookies and the X-Vault-Token header into the page
// body so tests can assert injection happened.
func echoServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body>` +
			`<div id="cookie">` + r.Header.Get("Cookie") + `</div>` +
			`<div id="auth">` + r.Header.Get("X-Vault-Token") + `</div>` +
			`</body></html>`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}
```

- [ ] **Step 2: Write the failing injection test**

```go
// pkg/scout/vault/inject_test.go
package vault

import (
	"strings"
	"testing"
)

func TestHandleApplyToPageInjectsCookieAndHeader(t *testing.T) {
	b, cleanup := newInjectTestBrowser(t)
	defer cleanup()
	srv := echoServer(t)

	p := &SecretProfile{
		ID:      "p",
		Cookies: []Cookie{{Name: "sid", Value: "cookie-val-42", Domain: "127.0.0.1", Path: "/"}},
		Headers: map[string]*LockedBuffer{"X-Vault-Token": NewLockedBuffer([]byte("hdr-val-99"))},
	}
	defer p.Close()
	h := &Handle{profile: p}
	defer func() { _ = h.Close() }()

	page, err := b.NewPage("about:blank")
	if err != nil {
		t.Fatalf("NewPage: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := h.ApplyToPage(page); err != nil {
		t.Fatalf("ApplyToPage: %v", err)
	}
	if err := page.Navigate(srv.URL + "/echo"); err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad: %v", err)
	}

	html, err := page.HTML()
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	if !strings.Contains(html, "hdr-val-99") {
		t.Fatalf("header not injected; body=%s", html)
	}
	if !strings.Contains(html, "cookie-val-42") {
		t.Fatalf("cookie not injected; body=%s", html)
	}
}

func TestHandleSecret(t *testing.T) {
	p := &SecretProfile{ID: "p", Secrets: map[string]*LockedBuffer{"k": NewLockedBuffer([]byte("v"))}}
	defer p.Close()
	h := &Handle{profile: p}
	lb, err := h.Secret("k")
	if err != nil {
		t.Fatalf("Secret: %v", err)
	}
	if !lb.Equal([]byte("v")) {
		t.Fatal("Secret returned wrong value")
	}
	if _, err := h.Secret("missing"); err == nil {
		t.Fatal("Secret succeeded for missing key")
	}
}
```

> **Note on `page.HTML()`:** the engine `Page` exposes an HTML accessor used throughout the suite. If the method name differs in this tree, the implementer should grep `internal/engine/page*.go` for the outer-HTML accessor and use it; the assertion only needs the rendered body text.

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./pkg/scout/vault/ -run 'TestHandle' -v`
Expected: FAIL — `undefined: Handle`.

- [ ] **Step 4: Write minimal implementation**

`ApplyToPage` mirrors `engine.Page.ApplyProfile`'s proven sequence: set cookies (CDP `Network.setCookies`), set extra headers (CDP `Network.setExtraHTTPHeaders`), then seed web storage for the current origin after load. Headers are read from LockedBuffers and converted to a transient `map[string]string` that is discarded immediately.

```go
// pkg/scout/vault/inject.go
package vault

import (
	"fmt"

	"github.com/inovacc/scout/pkg/scout"
)

// Handle is an operational view over one profile: it injects browser-bound
// secrets into a live page and yields scout-internal secrets as buffers.
type Handle struct {
	profile *SecretProfile
}

// ApplyToPage injects the profile's cookies and auth headers into page via CDP.
// Call BEFORE navigating to the target origin. Cookies and headers take effect on
// the next navigation; web storage is seeded if the page is already on an origin
// present in the profile's Storage map.
func (h *Handle) ApplyToPage(page *scout.Page) error {
	if len(h.profile.Cookies) > 0 {
		if err := page.SetCookies(h.profile.Cookies...); err != nil {
			return fmt.Errorf("scout: vault: inject cookies: %w", err)
		}
	}
	if len(h.profile.Headers) > 0 {
		hdr := make(map[string]string, len(h.profile.Headers))
		for k, lb := range h.profile.Headers {
			hdr[k] = string(lb.Bytes()) // transient; CDP requires string headers
		}
		if _, err := page.SetHeaders(hdr); err != nil {
			return fmt.Errorf("scout: vault: inject headers: %w", err)
		}
	}
	return nil
}

// Secret returns the named scout-internal secret as a zeroable buffer. The
// returned buffer is owned by the profile; do not Close it directly — Close the
// Handle (or the Vault) instead.
func (h *Handle) Secret(name string) (*LockedBuffer, error) {
	lb, ok := h.profile.Secrets[name]
	if !ok {
		return nil, fmt.Errorf("scout: vault: secret %q not found", name)
	}
	return lb, nil
}

// Close zeros the underlying profile's secret buffers.
func (h *Handle) Close() error {
	h.profile.Close()
	return nil
}
```

> **Storage injection:** Phase-1 `ApplyToPage` covers cookies + headers (the high-value session secrets). Per-origin web-storage seeding (via `page.LocalStorageSet`/`SessionStorageSet` after navigating to each origin, exactly as `engine.Page.ApplyProfile` does) is added in Task 11's `Use` path only when the profile carries `Storage`; if no storage is present, this is a no-op. Keep `ApplyToPage` focused on cookies+headers so it is origin-agnostic.

- [ ] **Step 5: Add `Vault.Use` to `pkg/scout/vault/vault.go`**

```go
// Use returns an operational Handle for the profile. The Handle shares the
// profile's secret buffers; Close the Handle when done.
func (v *Vault) Use(id string) (*Handle, error) {
	p, err := v.Get(id)
	if err != nil {
		return nil, err
	}
	return &Handle{profile: p}, nil
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./pkg/scout/vault/ -run 'TestHandle' -v`
Expected: PASS (skips cleanly if Chromium is absent).

- [ ] **Step 7: Commit**

```bash
git add pkg/scout/vault/inject.go pkg/scout/vault/testhelp_test.go pkg/scout/vault/inject_test.go pkg/scout/vault/vault.go
git commit -m "feat(vault): Handle CDP injection + Vault.Use"
```

---

## Task 11: UserProfile importer (`--from-profile`)

**Files:**
- Create: `pkg/scout/vault/import.go`
- Test: `pkg/scout/vault/import_test.go`

- [ ] **Step 1: Write the failing test**

```go
// pkg/scout/vault/import_test.go
package vault

import (
	"path/filepath"
	"testing"

	"github.com/inovacc/scout/pkg/scout"
)

func TestFromUserProfileAbsorbsSecretFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "demo.scoutprofile")
	up := &scout.UserProfile{
		Version: 1,
		Name:    "demo",
		Cookies: []scout.Cookie{{Name: "sid", Value: "v", Domain: "example.com"}},
		Headers: map[string]string{"Authorization": "Bearer t"},
		Storage: map[string]scout.ProfileOriginStorage{
			"https://example.com": {LocalStorage: map[string]string{"k": "v"}},
		},
	}
	if err := scout.SaveProfile(up, path); err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}

	in, err := FromUserProfile(path)
	if err != nil {
		t.Fatalf("FromUserProfile: %v", err)
	}
	if len(in.Cookies) != 1 || in.Cookies[0].Name != "sid" {
		t.Fatalf("cookies = %+v", in.Cookies)
	}
	if string(in.Headers["Authorization"]) != "Bearer t" {
		t.Fatalf("headers = %v", in.Headers)
	}
	if in.Storage["https://example.com"].LocalStorage["k"] != "v" {
		t.Fatalf("storage = %+v", in.Storage)
	}
	if in.Name != "demo" {
		t.Fatalf("name = %q, want demo", in.Name)
	}
}
```

> **Verify before coding:** confirm `scout.UserProfile`, `scout.Cookie`, `scout.ProfileOriginStorage`, `scout.SaveProfile`, and `scout.LoadProfile` are re-exported by the generated facade (`grep -n "UserProfile\|LoadProfile\|ProfileOriginStorage" pkg/scout/scout.go`). They are generated from `internal/engine`. If any is missing from the facade, import `github.com/inovacc/scout/internal/engine` directly instead and use `engine.LoadProfile` / `engine.UserProfile`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/scout/vault/ -run TestFromUserProfile -v`
Expected: FAIL — `undefined: FromUserProfile`.

- [ ] **Step 3: Write minimal implementation**

```go
// pkg/scout/vault/import.go
package vault

import (
	"fmt"

	"github.com/inovacc/scout/pkg/scout"
)

// FromUserProfile reads a .scoutprofile file and extracts only its secret-bearing
// fields (cookies, per-origin storage, auth headers) into a SecretProfileInput.
// Non-secret identity fields (UA/lang/tz/locale/proxy/extensions) are left in the
// UserProfile.
func FromUserProfile(path string) (SecretProfileInput, error) {
	up, err := scout.LoadProfile(path)
	if err != nil {
		return SecretProfileInput{}, fmt.Errorf("scout: vault: load profile: %w", err)
	}
	in := SecretProfileInput{Name: up.Name, Cookies: up.Cookies}
	if len(up.Headers) > 0 {
		in.Headers = make(map[string][]byte, len(up.Headers))
		for k, val := range up.Headers {
			in.Headers[k] = []byte(val)
		}
	}
	if len(up.Storage) > 0 {
		in.Storage = make(map[string]OriginStore, len(up.Storage))
		for origin, os := range up.Storage {
			in.Storage[origin] = OriginStore{LocalStorage: os.LocalStorage, SessionStorage: os.SessionStorage}
		}
	}
	return in, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/scout/vault/ -run TestFromUserProfile -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/scout/vault/import.go pkg/scout/vault/import_test.go
git commit -m "feat(vault): --from-profile UserProfile secret importer"
```

---

## Task 12: Deprecate UserProfile secret fields

**Files:**
- Modify: `internal/engine/profile.go:22-35` (the `UserProfile` struct)

- [ ] **Step 1: Add deprecation comments (behavior unchanged — dual-read preserved)**

Locate the `UserProfile` struct and annotate the three secret-bearing fields. Keep the fields and all existing capture/apply logic intact (dual-read for ≥30 days per the breaking-change policy).

```go
type UserProfile struct {
	Version   int             `json:"version"`
	Name      string          `json:"name"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	Browser   ProfileBrowser  `json:"browser"`
	Identity  ProfileIdentity `json:"identity"`
	// Deprecated: secret-bearing. Migrate to pkg/scout/vault via `scout vault set
	// --from-profile`. Retained for read compatibility; removal after 2026-07-02.
	Cookies []Cookie `json:"cookies"`
	// Deprecated: secret-bearing. Migrate to pkg/scout/vault. Removal after 2026-07-02.
	Storage map[string]ProfileOriginStorage `json:"storage,omitempty"`
	// Deprecated: secret-bearing. Migrate to pkg/scout/vault. Removal after 2026-07-02.
	Headers    map[string]string `json:"headers,omitempty"`
	Extensions []string          `json:"extensions,omitempty"`
	Proxy      string            `json:"proxy,omitempty"`
	Notes      string            `json:"notes,omitempty"`
}
```

- [ ] **Step 2: Verify the package still builds and tests pass**

Run: `go build ./internal/engine/ && go test ./internal/engine/ -run TestProfile -v`
Expected: PASS (no behavior change; comments only).

- [ ] **Step 3: Record the removal in the backlog**

Append to `docs/BACKLOG.md`:

```markdown
- DEPRECATION (removal after 2026-07-02): remove `UserProfile.Cookies/Storage/Headers`
  (`internal/engine/profile.go`). Superseded by `pkg/scout/vault`. Migrate callers of
  `CaptureProfile`/`ApplyProfile` to read browser-bound secrets from the vault, then drop
  the fields and their capture/apply branches.
```

- [ ] **Step 4: Commit**

```bash
git add internal/engine/profile.go docs/BACKLOG.md
git commit -m "deprecate(profile): mark UserProfile secret fields; track vault migration"
```

---

## Task 13: CLI — `scout vault` commands

**Files:**
- Modify: `cmd/scout/helpers.go` (add `readPassphraseBytes`)
- Create: `cmd/scout/vault.go`
- Test: `cmd/scout/vault_test.go`

- [ ] **Step 1: Write the failing CLI test**

```go
// cmd/scout/vault_test.go
package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/inovacc/scout/pkg/scout/vault"
)

// runVault drives the cobra command tree with args and a SCOUT_VAULT_PASSPHRASE env.
func TestVaultCLIInitSetListRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vault.bin")
	t.Setenv("SCOUT_VAULT_PASSPHRASE", "cli-pass")

	// init
	if _, err := vault.Create([]byte("cli-pass"), vault.WithPath(path)); err != nil {
		t.Fatalf("seed vault: %v", err)
	}
	// set via library to keep the CLI test fast and headless-free
	v, err := vault.Open([]byte("cli-pass"), vault.WithPath(path))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	id, err := v.Set(vault.SecretProfileInput{Name: "svc", Secrets: map[string][]byte{"token": []byte("abc")}})
	if err != nil {
		t.Fatalf("set: %v", err)
	}
	_ = v.Close()

	// render list output through the helper the CLI uses
	out := renderVaultList([]vault.ProfileMeta{{ID: id, Name: "svc", SecretKeys: []string{"token"}}})
	if !strings.Contains(out, id) || !strings.Contains(out, "svc") || !strings.Contains(out, "token") {
		t.Fatalf("list output missing fields: %s", out)
	}
	if strings.Contains(out, "abc") {
		t.Fatalf("list output leaked secret value: %s", out)
	}
}
```

This test pins the two CLI-owned pure functions: `renderVaultList` (formatting that must never print values) and the env-passphrase contract. Command wiring is verified by `go build`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/scout/ -run TestVaultCLI -v`
Expected: FAIL — `undefined: renderVaultList`.

- [ ] **Step 3: Add `readPassphraseBytes` to `cmd/scout/helpers.go`**

```go
// readPassphraseBytes reads a passphrase as a zeroable []byte. It honors
// SCOUT_VAULT_PASSPHRASE (with a stderr leak warning) for non-interactive use,
// then falls back to a no-echo terminal prompt.
func readPassphraseBytes(w io.Writer, prompt string) ([]byte, error) {
	if v := os.Getenv("SCOUT_VAULT_PASSPHRASE"); v != "" {
		_, _ = fmt.Fprintln(w, "warning: SCOUT_VAULT_PASSPHRASE is visible to child processes; prefer the interactive prompt")
		return []byte(v), nil
	}
	if f, ok := w.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		_, _ = fmt.Fprint(w, prompt)
		b, err := term.ReadPassword(int(f.Fd()))
		_, _ = fmt.Fprintln(w)
		if err != nil {
			return nil, fmt.Errorf("scout: read passphrase: %w", err)
		}
		return b, nil
	}
	// Non-terminal stdin (piped): read a single line.
	sc := bufio.NewScanner(os.Stdin)
	if !sc.Scan() {
		return nil, fmt.Errorf("scout: read passphrase: no input")
	}
	return append([]byte(nil), sc.Bytes()...), nil
}
```

> The implementer should confirm `bufio`, `io`, `os`, `fmt`, and `golang.org/x/term` are imported in `helpers.go` (they are already used by `readPassphrase`); add only what is missing.

- [ ] **Step 4: Create `cmd/scout/vault.go`**

```go
// cmd/scout/vault.go
package main

import (
	"fmt"
	"strings"

	"github.com/inovacc/scout/pkg/scout/vault"
	"github.com/spf13/cobra"
)

var vaultCmd = &cobra.Command{
	Use:   "vault",
	Short: "Encrypted secrets vault (Argon2id + AES-256-GCM)",
}

var vaultInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new vault",
	RunE: func(cmd *cobra.Command, _ []string) error {
		pass, err := readPassphraseBytes(cmd.ErrOrStderr(), "New vault passphrase: ")
		if err != nil {
			return err
		}
		defer zeroBytesCLI(pass)
		v, err := vault.Create(pass, vaultPathOpts(cmd)...)
		if err != nil {
			return err
		}
		_ = v.Close()
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "vault initialized")
		return nil
	},
}

var vaultSetCmd = &cobra.Command{
	Use:   "set KEY=VALUE [KEY=VALUE...]",
	Short: "Create or update a secret profile; prints its opaque ID",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		fromProfile, _ := cmd.Flags().GetString("from-profile")
		id, _ := cmd.Flags().GetString("id")

		var in vault.SecretProfileInput
		if fromProfile != "" {
			var err error
			if in, err = vault.FromUserProfile(fromProfile); err != nil {
				return err
			}
		}
		if name != "" {
			in.Name = name
		}
		in.ID = id
		if in.Secrets == nil {
			in.Secrets = map[string][]byte{}
		}
		for _, kv := range args {
			k, val, ok := strings.Cut(kv, "=")
			if !ok {
				return fmt.Errorf("scout: vault: %q is not KEY=VALUE", kv)
			}
			in.Secrets[k] = []byte(val)
		}

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

		newID, err := v.Set(in)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), newID)
		return nil
	},
}

var vaultListCmd = &cobra.Command{
	Use:   "list",
	Short: "List secret profiles (metadata only — never values)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		v, err := openVaultCLI(cmd)
		if err != nil {
			return err
		}
		defer func() { _ = v.Close() }()
		_, _ = fmt.Fprint(cmd.OutOrStdout(), renderVaultList(v.List()))
		return nil
	},
}

var vaultGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Show one profile's metadata (never secret values)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		v, err := openVaultCLI(cmd)
		if err != nil {
			return err
		}
		defer func() { _ = v.Close() }()
		for _, m := range v.List() {
			if m.ID == args[0] {
				_, _ = fmt.Fprint(cmd.OutOrStdout(), renderVaultList([]vault.ProfileMeta{m}))
				return nil
			}
		}
		return fmt.Errorf("scout: vault: profile %q not found", args[0])
	},
}

var vaultUseCmd = &cobra.Command{
	Use:   "use <id> --url <url>",
	Short: "Inject a profile into a browser page via CDP",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		url, _ := cmd.Flags().GetString("url")
		if url == "" {
			return fmt.Errorf("scout: vault: --url is required (daemon --session injection is not yet supported)")
		}
		v, err := openVaultCLI(cmd)
		if err != nil {
			return err
		}
		defer func() { _ = v.Close() }()
		h, err := v.Use(args[0])
		if err != nil {
			return err
		}
		defer func() { _ = h.Close() }()

		b, err := newBrowser(cmd) // existing helper that builds scout.Browser from baseOpts
		if err != nil {
			return err
		}
		defer func() { _ = b.Close() }()
		page, err := b.NewPage("about:blank")
		if err != nil {
			return err
		}
		if err := h.ApplyToPage(page); err != nil {
			return err
		}
		if err := page.Navigate(url); err != nil {
			return err
		}
		if err := page.WaitLoad(); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "injected profile %s into %s\n", args[0], url)
		return nil
	},
}

var vaultRotateCmd = &cobra.Command{
	Use:   "rotate",
	Short: "Re-encrypt the vault under a new passphrase",
	RunE: func(cmd *cobra.Command, _ []string) error {
		v, err := openVaultCLI(cmd)
		if err != nil {
			return err
		}
		defer func() { _ = v.Close() }()
		newPass, err := readPassphraseBytes(cmd.ErrOrStderr(), "New vault passphrase: ")
		if err != nil {
			return err
		}
		defer zeroBytesCLI(newPass)
		if err := v.Rotate(newPass); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "vault rotated")
		return nil
	},
}

var vaultRmCmd = &cobra.Command{
	Use:   "rm <id>",
	Short: "Remove a secret profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		v, err := openVaultCLI(cmd)
		if err != nil {
			return err
		}
		defer func() { _ = v.Close() }()
		if err := v.Remove(args[0]); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "removed "+args[0])
		return nil
	},
}

// vaultPathOpts maps the optional --vault-file flag to a vault.Option slice.
func vaultPathOpts(cmd *cobra.Command) []vault.Option {
	if p, _ := cmd.Flags().GetString("vault-file"); p != "" {
		return []vault.Option{vault.WithPath(p)}
	}
	return nil
}

// openVaultCLI reads the passphrase and opens the vault.
func openVaultCLI(cmd *cobra.Command) (*vault.Vault, error) {
	pass, err := readPassphraseBytes(cmd.ErrOrStderr(), "Vault passphrase: ")
	if err != nil {
		return nil, err
	}
	defer zeroBytesCLI(pass)
	return vault.Open(pass, vaultPathOpts(cmd)...)
}

// renderVaultList formats profile metadata. It MUST NOT print any secret value.
func renderVaultList(metas []vault.ProfileMeta) string {
	var sb strings.Builder
	for _, m := range metas {
		fmt.Fprintf(&sb, "%s  %s  secrets=%s  headers=%d  cookies=%d\n",
			m.ID, m.Name, strings.Join(m.SecretKeys, ","), len(m.HeaderKeys), m.CookieN)
	}
	return sb.String()
}

// zeroBytesCLI overwrites a passphrase slice once it is no longer needed.
func zeroBytesCLI(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

func init() {
	vaultSetCmd.Flags().String("name", "", "human label for the profile")
	vaultSetCmd.Flags().String("from-profile", "", "import secret fields from a .scoutprofile")
	vaultSetCmd.Flags().String("id", "", "update an existing profile by ID")
	vaultUseCmd.Flags().String("url", "", "URL to open and inject into")

	for _, c := range []*cobra.Command{vaultInitCmd, vaultSetCmd, vaultListCmd, vaultGetCmd, vaultUseCmd, vaultRotateCmd, vaultRmCmd} {
		c.Flags().String("vault-file", "", "override vault file path (default <scouthome>/profiles/vault.bin)")
	}
	vaultCmd.AddCommand(vaultInitCmd, vaultSetCmd, vaultListCmd, vaultGetCmd, vaultUseCmd, vaultRotateCmd, vaultRmCmd)
	rootCmd.AddCommand(vaultCmd)
}
```

> **`newBrowser(cmd)` check:** `vault use` needs a `*scout.Browser` built from the shared persistent flags. Grep `cmd/scout/helpers.go` and command files for the existing constructor (e.g. a helper wrapping `scout.New(baseOpts(cmd)...)`). If none exists with that exact name, replace `newBrowser(cmd)` with `scout.New(baseOpts(cmd)...)` directly (import `github.com/inovacc/scout/pkg/scout`).

- [ ] **Step 5: Run test to verify it passes + build the binary**

Run: `go test ./cmd/scout/ -run TestVaultCLI -v`
Expected: PASS.

Run: `go build ./cmd/scout/`
Expected: builds with no errors (verifies command wiring + flag registration).

- [ ] **Step 6: Commit**

```bash
git add cmd/scout/vault.go cmd/scout/vault_test.go cmd/scout/helpers.go
git commit -m "feat(vault): scout vault CLI (init/set/get/list/use/rotate/rm)"
```

---

## Task 14: Hygiene guard + full-package gate

**Files:**
- Create: `pkg/scout/vault/hygiene_test.go`

- [ ] **Step 1: Write a test that forbids string-conversion of secret buffers in non-test code**

```go
// pkg/scout/vault/hygiene_test.go
package vault

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestNoSecretStringConversion guards the "secrets never become string" rule:
// no non-test .go file in this package may convert a LockedBuffer's bytes to a
// string. The one sanctioned exception is the transient CDP header map in
// inject.go (CDP requires string header values), which is annotated below.
func TestNoSecretStringConversion(t *testing.T) {
	bad := regexp.MustCompile(`string\(\s*\w+\.Bytes\(\)\s*\)`)
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Join(".", name))
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			if bad.MatchString(line) && !strings.Contains(line, "vault:allow-string") {
				t.Errorf("%s:%d converts secret bytes to string: %s", name, i+1, strings.TrimSpace(line))
			}
		}
	}
}
```

- [ ] **Step 2: Annotate the one sanctioned conversion in `inject.go`**

In `ApplyToPage`, the header line must carry the allow-marker so the guard passes only for that sanctioned, transient use:

```go
		for k, lb := range h.profile.Headers {
			hdr[k] = string(lb.Bytes()) // vault:allow-string — CDP requires string headers; map is discarded after the call
		}
```

- [ ] **Step 3: Run the whole vault suite and the cmd suite**

Run: `go test ./pkg/scout/vault/... ./cmd/scout/ -v`
Expected: PASS (browser-dependent tests skip cleanly when Chromium is absent).

- [ ] **Step 4: Vet + lint the new package**

Run: `go vet ./pkg/scout/vault/... ./cmd/scout/`
Expected: no findings.

Run: `golangci-lint run ./pkg/scout/vault/... --timeout=5m`
Expected: clean (or only pre-existing repo-wide exclusions).

- [ ] **Step 5: Commit**

```bash
git add pkg/scout/vault/hygiene_test.go pkg/scout/vault/inject.go
git commit -m "test(vault): forbid secret->string conversion; annotate CDP exception"
```

---

## Task 15: Documentation

**Files:**
- Modify: `CLAUDE.md` (add a vault convention bullet)
- Modify: `README.md` (add a `scout vault` usage section)

- [ ] **Step 1: Add a convention bullet to `CLAUDE.md`**

Under the Conventions list, add:

```markdown
- **Secrets vault**: `pkg/scout/vault` stores named secret profiles in one Argon2id+AES-256-GCM file at `<scouthome>/profiles/vault.bin` (0o600 in a 0o700 dir). Secrets live in `LockedBuffer` (`[]byte` + `VirtualLock`/`Mlock` + explicit zero) and never become `string`. `Vault.Use(id)` returns a `Handle` that injects cookies/headers into a live page via CDP (`Handle.ApplyToPage`) and yields scout-internal secrets via `Handle.Secret` — never env vars. `scout vault rotate` re-keys atomically. CLI: `scout vault init/set/get/list/use/rotate/rm`. `--from-profile` imports a `.scoutprofile`'s secret fields (deprecating `UserProfile.Cookies/Storage/Headers`).
```

- [ ] **Step 2: Add a usage section to `README.md`**

```markdown
### Secrets vault

```bash
scout vault init                                  # create <scouthome>/profiles/vault.bin
scout vault set --name openai api_key=sk-live-xyz # prints an opaque profile ID
scout vault set --from-profile ./session.scoutprofile --name web   # import browser secrets
scout vault list                                  # metadata only — never values
scout vault use <id> --url https://example.com    # inject cookies/headers via CDP
scout vault rotate                                # re-key under a new passphrase
scout vault rm <id>
```

Set `SCOUT_VAULT_PASSPHRASE` for non-interactive use (a stderr warning notes it is
visible to child processes; prefer the interactive prompt).
```

- [ ] **Step 3: Commit**

```bash
git add CLAUDE.md README.md
git commit -m "docs(vault): document secrets vault conventions and CLI"
```

---

## Final verification (run after all tasks)

- [ ] `go build ./cmd/scout/ ./pkg/...` — whole tree builds.
- [ ] `go test ./pkg/scout/vault/... ./cmd/scout/ -v` — full vault + CLI suite green (browser tests skip if no Chromium).
- [ ] `go vet ./pkg/scout/vault/... ./cmd/scout/` — clean.
- [ ] Manual smoke (with a real browser): `scout vault init`, `scout vault set --name t token=abc` → capture the ID, `scout vault list` shows the ID/name/`secrets=token` but **not** `abc`, `scout vault use <id> --url https://httpbin.org/headers` reports injection, `scout vault rotate` succeeds and the old passphrase is rejected on the next `scout vault list`.
- [ ] Confirm `<scouthome>/profiles/vault.bin` is `0o600` and the `profiles` dir is `0o700`.
- [ ] Use `superpowers:finishing-a-development-branch` to merge.

---

## Spec coverage check (self-review)

| Spec requirement (design §) | Task(s) |
|---|---|
| §2 scout-native Argon2id+AES-GCM, no KeePass | 4 |
| §3.1 LockedBuffer + lock/zero/Close, constant-time Equal, no string | 1, 2, 14 |
| §3.2 SecretProfile absorbs cookies/storage/headers | 6, 11 |
| §3.3 Vault Open/Set/Get/Use/List/Remove/Rotate/Close | 8, 9, 10 |
| §3.4 Handle ApplyToPage (CDP) + Secret([]byte) + Close | 10 |
| §4 one file at `<scouthome>/profiles/vault.bin`, atomic, 0o600/0o700, versioned header | 4, 5, 7 |
| §5 passphrase sourcing (prompt → env with warning), zeroed | 13 |
| §6 CLI init/set/get/list/use/rotate/rm + --from-profile | 13, 11 |
| §7 deprecate UserProfile secret fields; other 6 stores → backlog | 12 |
| §8 no secrets in logs/List; constant-time; 0o600 | 8, 13, 14 |
| §9 real crypto + real browser tests, rotation, atomic | 4, 7, 9, 10 |
| §10 success criteria (opaque ID, zeroed buffers, rotate, perms, --from-profile) | 8, 9, 10, 11, 13 |

**Scope deviation from spec §6 (flagged):** the spec's `scout vault use <id> --session <sid>` (inject into a *running daemon* session) requires a new gRPC `ApplyVault` RPC because a session's CDP endpoint is not discoverable cross-process (fact-gather: `grpc/server/server.go`). Phase 1 ships `scout vault use <id> --url <url>` (CLI-driven page; `--cdp` via `baseOpts` remote-CDP) which exercises `Handle.ApplyToPage` end-to-end. **Backlog item:** add an `ApplyVault` gRPC RPC + `--session` flag so `vault use` can inject into a live daemon session.
