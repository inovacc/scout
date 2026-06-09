# Profile → Vault Secret Migration (Phase 1) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move browser secret capture/apply (cookies + web storage) onto `pkg/scout/vault` and decouple `internal/engine` and the CLI from `UserProfile`'s deprecated `Cookies/Storage/Headers` fields, so those fields can be removed on their scheduled date (Phase 2, after 2026-07-02).

**Architecture:** `pkg/scout/vault` imports `pkg/scout`, so `internal/engine` cannot import vault (import cycle). The vault layer gains capture (`CaptureFromPage`) and storage-apply (`Handle.ApplyStorageToPage`); `internal/engine/profile.go` only *loses* secret behavior; the CLI orchestrates. The three deprecated fields stay on `UserProfile` in Phase 1, read only by the existing `FromUserProfile` legacy-import bridge and legacy `profile show`.

**Tech Stack:** Go 1.26, real browser + `httptest` tests (`-short`-gated per repo convention), cobra CLI, `pkg/scout/vault` (Argon2id + AES-256-GCM, `LockedBuffer`/mlock).

**Spec:** `docs/superpowers/specs/2026-06-09-profile-vault-migration-design.md`

---

## Working conventions (read before Task 1)

- **One feature branch.** At the start, run `git switch -c feat/profile-vault-phase1`. Every task commits to it.
- **Commits stay LOCAL.** Do NOT `git push`. The final ff-merge to `main` + push is deferred until the user explicitly asks (no GHA runs).
- **No AI attribution** in commit messages.
- **Verify locally** only: `go build`, `go vet`, `go test -short` (Chromium-heavy tests are `-short`-gated and skip).
- Browser tests in `pkg/scout/vault` use `newInjectTestBrowser(t)` (in `testhelp_test.go`); in `internal/engine` use `newOwnedTestBrowser(t)` (in `testutil_test.go`). Both `-short`-skip.

## File map

- Create `pkg/scout/vault/capture.go` — `CaptureFromPage` + `originFrom` helper.
- Create `pkg/scout/vault/capture_test.go` — capture round-trip test.
- Modify `pkg/scout/vault/inject.go` — add `Handle.ApplyStorageToPage`.
- Modify `pkg/scout/vault/inject_test.go` — add storage-apply test.
- Create `pkg/scout/vault/roundtrip_test.go` — capture→set→use equivalence test.
- Modify `internal/engine/profile.go` — `CaptureProfile` drops secret capture; `ApplyProfile` → deprecated no-op; deprecate `Save/LoadProfileEncrypted`.
- Modify `internal/engine/profile_test.go` — add capture-omits-secrets + apply-no-op tests.
- Modify `cmd/scout/vault.go` — new `vault capture`; wire `ApplyStorageToPage` into `vault use`.
- Modify `cmd/scout/vault_test.go` — capture command registration test.
- Modify `cmd/scout/profile.go` — make `scout profile` non-secret (help, summaries, `show` guard).
- Create `cmd/scout/profile_test.go` — `profile show` legacy-guard test.
- Modify `grpc/server/server_hijack_stream.go` — doc-comment only (no proto change).

---

### Task 0: Create the feature branch

- [ ] **Step 1: Branch**

```bash
git switch -c feat/profile-vault-phase1
```

No commit yet.

---

### Task 1: vault `CaptureFromPage` (+ `originFrom` helper)

**Files:**
- Create: `pkg/scout/vault/capture.go`
- Test: `pkg/scout/vault/capture_test.go`

- [ ] **Step 1: Write the failing test**

Create `pkg/scout/vault/capture_test.go`:

```go
package vault

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCaptureFromPageGrabsCookiesAndStorage(t *testing.T) {
	b, cleanup := newInjectTestBrowser(t)
	defer cleanup()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "sid", Value: "cap-cookie-7", Path: "/"})
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><script>
localStorage.setItem('lk','lv');
sessionStorage.setItem('sk','sv');
</script>ok</body></html>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	page, err := b.NewPage(srv.URL + "/")
	if err != nil {
		t.Fatalf("NewPage: %v", err)
	}
	defer func() { _ = page.Close() }()
	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad: %v", err)
	}

	in, err := CaptureFromPage(page, "cap")
	if err != nil {
		t.Fatalf("CaptureFromPage: %v", err)
	}
	if in.Name != "cap" {
		t.Errorf("Name = %q, want cap", in.Name)
	}

	var found bool
	for _, c := range in.Cookies {
		if c.Name == "sid" && c.Value == "cap-cookie-7" {
			found = true
		}
	}
	if !found {
		t.Errorf("captured cookies %v missing sid=cap-cookie-7", in.Cookies)
	}

	origin := originFrom(srv.URL)
	st, ok := in.Storage[origin]
	if !ok {
		t.Fatalf("no storage captured for origin %q (have %v)", origin, in.Storage)
	}
	if st.LocalStorage["lk"] != "lv" {
		t.Errorf("localStorage lk = %q, want lv", st.LocalStorage["lk"])
	}
	if st.SessionStorage["sk"] != "sv" {
		t.Errorf("sessionStorage sk = %q, want sv", st.SessionStorage["sk"])
	}
}
```

- [ ] **Step 2: Run test to verify it fails (compile error)**

Run: `go test ./pkg/scout/vault/ -run TestCaptureFromPageGrabsCookiesAndStorage`
Expected: FAIL — `undefined: CaptureFromPage` / `undefined: originFrom`.

- [ ] **Step 3: Write the implementation**

Create `pkg/scout/vault/capture.go`:

```go
package vault

import (
	"fmt"
	"net/url"

	"github.com/inovacc/scout/pkg/scout"
)

// CaptureFromPage snapshots a live page's secret-bearing browser state — cookies
// plus the current origin's localStorage/sessionStorage — into a SecretProfileInput
// named name. Auth headers are not captured (script cannot read outbound request
// headers). Local sessions only; the page must already be navigated to (and
// authenticated at) the target origin.
func CaptureFromPage(page *scout.Page, name string) (SecretProfileInput, error) {
	if page == nil {
		return SecretProfileInput{}, fmt.Errorf("scout: vault: capture: nil page")
	}

	in := SecretProfileInput{Name: name}

	cookies, err := page.GetCookies()
	if err != nil {
		return SecretProfileInput{}, fmt.Errorf("scout: vault: capture cookies: %w", err)
	}
	in.Cookies = cookies

	pageURL, _ := page.URL()
	if origin := originFrom(pageURL); origin != "" {
		store := OriginStore{}
		if ls, err := page.LocalStorageGetAll(); err == nil && len(ls) > 0 {
			store.LocalStorage = ls
		}
		if ss, err := page.SessionStorageGetAll(); err == nil && len(ss) > 0 {
			store.SessionStorage = ss
		}
		if len(store.LocalStorage) > 0 || len(store.SessionStorage) > 0 {
			in.Storage = map[string]OriginStore{origin: store}
		}
	}

	return in, nil
}

// originFrom extracts the scheme+host origin from a URL string ("" if unparseable).
func originFrom(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/scout/vault/ -run TestCaptureFromPageGrabsCookiesAndStorage`
Expected: PASS (or SKIP if Chromium unavailable — acceptable locally; must PASS where Chromium exists).

- [ ] **Step 5: Commit (local only)**

```bash
git add pkg/scout/vault/capture.go pkg/scout/vault/capture_test.go
git commit -m "feat(vault): add CaptureFromPage to snapshot page cookies + web storage"
```

---

### Task 2: vault `Handle.ApplyStorageToPage`

**Files:**
- Modify: `pkg/scout/vault/inject.go`
- Test: `pkg/scout/vault/inject_test.go`

- [ ] **Step 1: Write the failing test**

Append to `pkg/scout/vault/inject_test.go` (the file already imports `net/http`, `net/http/httptest`, `testing`):

```go
func TestHandleApplyStorageToPageSeedsWebStorage(t *testing.T) {
	b, cleanup := newInjectTestBrowser(t)
	defer cleanup()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body>ok</body></html>"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	origin := originFrom(srv.URL)
	p := &SecretProfile{
		ID: "p",
		Storage: map[string]OriginStore{
			origin: {
				LocalStorage:   map[string]string{"lk": "seeded-lv"},
				SessionStorage: map[string]string{"sk": "seeded-sv"},
			},
		},
	}
	defer p.Close()
	h := &Handle{profile: p}
	defer func() { _ = h.Close() }()

	page, err := b.NewPage(srv.URL + "/")
	if err != nil {
		t.Fatalf("NewPage: %v", err)
	}
	defer func() { _ = page.Close() }()
	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad: %v", err)
	}

	if err := h.ApplyStorageToPage(page); err != nil {
		t.Fatalf("ApplyStorageToPage: %v", err)
	}

	ls, err := page.LocalStorageGetAll()
	if err != nil {
		t.Fatalf("LocalStorageGetAll: %v", err)
	}
	if ls["lk"] != "seeded-lv" {
		t.Errorf("localStorage lk = %q, want seeded-lv", ls["lk"])
	}
	ss, err := page.SessionStorageGetAll()
	if err != nil {
		t.Fatalf("SessionStorageGetAll: %v", err)
	}
	if ss["sk"] != "seeded-sv" {
		t.Errorf("sessionStorage sk = %q, want seeded-sv", ss["sk"])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/scout/vault/ -run TestHandleApplyStorageToPageSeedsWebStorage`
Expected: FAIL — `h.ApplyStorageToPage undefined`.

- [ ] **Step 3: Write the implementation**

Append to `pkg/scout/vault/inject.go` (already imports `fmt` and `scout`):

```go
// ApplyStorageToPage seeds the current origin's localStorage and sessionStorage
// from the profile. Call AFTER navigating to the target origin (cookies and
// headers are applied pre-navigation via ApplyToPage). Storage for origins other
// than the page's current origin is ignored. No-op when the profile has no storage.
func (h *Handle) ApplyStorageToPage(page *scout.Page) error {
	if len(h.profile.Storage) == 0 {
		return nil
	}
	pageURL, _ := page.URL()
	origin := originFrom(pageURL)
	if origin == "" {
		return nil
	}
	store, ok := h.profile.Storage[origin]
	if !ok {
		return nil
	}
	for k, v := range store.LocalStorage {
		if err := page.LocalStorageSet(k, v); err != nil {
			return fmt.Errorf("scout: vault: inject localStorage %q: %w", k, err)
		}
	}
	for k, v := range store.SessionStorage {
		if err := page.SessionStorageSet(k, v); err != nil {
			return fmt.Errorf("scout: vault: inject sessionStorage %q: %w", k, err)
		}
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/scout/vault/ -run 'TestHandleApply'`
Expected: PASS (both ApplyToPage and ApplyStorageToPage tests).

- [ ] **Step 5: Commit (local only)**

```bash
git add pkg/scout/vault/inject.go pkg/scout/vault/inject_test.go
git commit -m "feat(vault): add Handle.ApplyStorageToPage for post-nav web storage"
```

---

### Task 2b: Equivalence round-trip (capture → vault → use)

Proves the full local path the CLI will drive: an authenticated session's cookie, captured via `CaptureFromPage`, persisted through the encrypted vault, and re-applied via `Use`/`ApplyToPage`, is sent back to the server. (Storage store-level round-trip is already covered by `store_test.go`.)

**Files:**
- Create: `pkg/scout/vault/roundtrip_test.go`

- [ ] **Step 1: Write the failing test**

Create `pkg/scout/vault/roundtrip_test.go`:

```go
package vault

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestCaptureSetUseRoundTripReproducesCookie(t *testing.T) {
	b, cleanup := newInjectTestBrowser(t)
	defer cleanup()

	var mu sync.Mutex
	var sawCookie string
	mux := http.NewServeMux()
	mux.HandleFunc("/set", func(w http.ResponseWriter, _ *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "auth", Value: "rt-token-9", Path: "/"})
		_, _ = w.Write([]byte("<html><body>set</body></html>"))
	})
	mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		sawCookie = r.Header.Get("Cookie")
		mu.Unlock()
		_, _ = w.Write([]byte("<html><body>echo</body></html>"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// 1. Authenticated session sets a cookie; capture it.
	p1, err := b.NewPage(srv.URL + "/set")
	if err != nil {
		t.Fatalf("NewPage: %v", err)
	}
	if err := p1.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad: %v", err)
	}
	in, err := CaptureFromPage(p1, "rt")
	if err != nil {
		t.Fatalf("CaptureFromPage: %v", err)
	}
	_ = p1.Close()

	// 2. Persist into a temp vault, then Use it.
	vpath := filepath.Join(t.TempDir(), "vault.bin")
	if _, err := Create([]byte("pw"), WithPath(vpath)); err != nil {
		t.Fatalf("Create: %v", err)
	}
	v, err := Open([]byte("pw"), WithPath(vpath))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = v.Close() }()
	id, err := v.Set(in)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	h, err := v.Use(id)
	if err != nil {
		t.Fatalf("Use: %v", err)
	}
	defer func() { _ = h.Close() }()

	// 3. Apply to a fresh page; the server must see the captured cookie.
	p2, err := b.NewPage("about:blank")
	if err != nil {
		t.Fatalf("NewPage2: %v", err)
	}
	defer func() { _ = p2.Close() }()
	if err := h.ApplyToPage(p2); err != nil {
		t.Fatalf("ApplyToPage: %v", err)
	}
	if err := p2.Navigate(srv.URL + "/echo"); err != nil {
		t.Fatalf("Navigate: %v", err)
	}
	if err := p2.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if !strings.Contains(sawCookie, "rt-token-9") {
		t.Fatalf("round-trip failed: server saw Cookie=%q, want auth=rt-token-9", sawCookie)
	}
}
```

- [ ] **Step 2: Run test to verify it passes**

Run: `go test ./pkg/scout/vault/ -run TestCaptureSetUseRoundTripReproducesCookie`
Expected: PASS (Tasks 1–2 supply `CaptureFromPage`; `Create`/`Open`/`Set`/`Use`/`ApplyToPage` already exist). SKIP is acceptable when Chromium is unavailable.

- [ ] **Step 3: Commit (local only)**

```bash
git add pkg/scout/vault/roundtrip_test.go
git commit -m "test(vault): capture->set->use round-trip reproduces session cookie"
```

---

### Task 3: engine `CaptureProfile` drops secret capture

**Files:**
- Modify: `internal/engine/profile.go:133-157` (remove the Cookies + Storage capture blocks)
- Test: `internal/engine/profile_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/engine/profile_test.go`:

```go
func TestCaptureProfile_OmitsSecrets(t *testing.T) {
	b := newOwnedTestBrowser(t)
	defer func() { _ = b.Close() }()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "sid", Value: "should-not-capture", Path: "/"})
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><script>localStorage.setItem('k','v')</script>ok</body></html>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	page, err := b.NewPage(srv.URL + "/")
	if err != nil {
		t.Fatalf("NewPage: %v", err)
	}
	defer func() { _ = page.Close() }()
	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad: %v", err)
	}

	prof, err := CaptureProfile(page)
	if err != nil {
		t.Fatalf("CaptureProfile: %v", err)
	}
	if len(prof.Cookies) != 0 {
		t.Errorf("CaptureProfile captured %d cookies; secrets must go to vault, not the profile", len(prof.Cookies))
	}
	if len(prof.Storage) != 0 {
		t.Errorf("CaptureProfile captured %d storage origins; secrets must go to vault", len(prof.Storage))
	}
	// Non-secret identity is still captured.
	if prof.Identity.UserAgent == "" {
		t.Error("expected non-secret identity (UserAgent) to still be captured")
	}
}
```

Ensure `internal/engine/profile_test.go` imports `net/http` and `net/http/httptest` (add to the import block if missing).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/engine/ -run TestCaptureProfile_OmitsSecrets`
Expected: FAIL — `CaptureProfile captured 1 cookies` (current code still captures them).

- [ ] **Step 3: Remove the secret-capture blocks**

In `internal/engine/profile.go`, delete the **Cookies** block and the **Storage** block from `CaptureProfile` (lines 133–157). Delete exactly:

```go
	// Cookies.
	if cookies, err := page.GetCookies(); err == nil {
		p.Cookies = cookies
	}

	// Storage for current origin.
	pageURL, _ := page.URL()
	if pageURL != "" {
		origin := originFromURL(pageURL)
		if origin != "" {
			os := ProfileOriginStorage{}

			if ls, err := page.LocalStorageGetAll(); err == nil && len(ls) > 0 {
				os.LocalStorage = ls
			}

			if ss, err := page.SessionStorageGetAll(); err == nil && len(ss) > 0 {
				os.SessionStorage = ss
			}

			if len(os.LocalStorage) > 0 || len(os.SessionStorage) > 0 {
				p.Storage = map[string]ProfileOriginStorage{origin: os}
			}
		}
	}
```

Leave the "Headers from browser options" block (it only sets `p.Proxy`) and the `return p, nil` intact. Do NOT remove `originFromURL` — it is still used by `ApplyProfile` (until Task 4) and is covered by `TestOriginFromURL`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -short ./internal/engine/ -run 'TestCaptureProfile|TestOriginFromURL|TestProfileFullWorkflow'`
Expected: PASS (new test passes; existing Save/Load + origin tests still pass).

- [ ] **Step 5: Commit (local only)**

```bash
git add internal/engine/profile.go internal/engine/profile_test.go
git commit -m "refactor(engine): CaptureProfile no longer captures cookies/storage (vault owns secrets)"
```

---

### Task 4: engine `ApplyProfile` → deprecated no-op

**Files:**
- Modify: `internal/engine/profile.go:311-357` (`Page.ApplyProfile`)
- Test: `internal/engine/profile_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/engine/profile_test.go`:

```go
func TestApplyProfile_NoOpDoesNotApplySecrets(t *testing.T) {
	b := newOwnedTestBrowser(t)
	defer func() { _ = b.Close() }()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><body>ok</body></html>"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	page, err := b.NewPage(srv.URL + "/")
	if err != nil {
		t.Fatalf("NewPage: %v", err)
	}
	defer func() { _ = page.Close() }()
	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad: %v", err)
	}

	prof := &UserProfile{
		Version: 1, Name: "legacy",
		Cookies: []Cookie{{Name: "legacy", Value: "x", Domain: "127.0.0.1", Path: "/"}},
	}
	if err := page.ApplyProfile(prof); err != nil {
		t.Fatalf("ApplyProfile (no-op) returned error: %v", err)
	}

	got, err := page.GetCookies()
	if err != nil {
		t.Fatalf("GetCookies: %v", err)
	}
	for _, c := range got {
		if c.Name == "legacy" {
			t.Errorf("ApplyProfile applied a cookie; it must be a no-op (secrets go through vault)")
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/engine/ -run TestApplyProfile_NoOpDoesNotApplySecrets`
Expected: FAIL — the `legacy` cookie is applied by the current `ApplyProfile`.

- [ ] **Step 3: Gut the body to a deprecated no-op**

Replace the entire `ApplyProfile` method body in `internal/engine/profile.go` (keep the nil guards, drop the cookie/header/storage application). New full method:

```go
// ApplyProfile previously restored cookies, headers, and web storage from a
// UserProfile. Those secret-bearing operations have moved to pkg/scout/vault
// (Handle.ApplyToPage for cookies+headers, Handle.ApplyStorageToPage for web
// storage). This method is now a no-op kept for source compatibility.
//
// Deprecated: applies nothing; use pkg/scout/vault. Removal after 2026-07-02.
func (p *Page) ApplyProfile(prof *UserProfile) error {
	if p == nil || p.page == nil {
		return fmt.Errorf("scout: profile: apply: nil page")
	}

	if prof == nil {
		return fmt.Errorf("scout: profile: apply: nil profile")
	}

	return nil
}
```

After this, `originFromURL` is used only by `TestOriginFromURL` and `MergeProfiles`/`DiffProfiles`/`Validate` do not call it — that is fine; the function stays (its test keeps it live). If `go vet`/build reports `originFromURL` as unused, that means `TestOriginFromURL` was removed — it must NOT be; keep it.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -short ./internal/engine/ -run 'TestApplyProfile|TestOriginFromURL'`
Expected: PASS (new no-op test + existing `TestApplyProfile_NilPage`/`NilProfile`/`TestOriginFromURL`).

- [ ] **Step 5: Commit (local only)**

```bash
git add internal/engine/profile.go internal/engine/profile_test.go
git commit -m "refactor(engine): ApplyProfile is now a deprecated no-op (secrets via vault)"
```

---

### Task 5: deprecation bookkeeping (encrypted profile funcs + gRPC note)

**Files:**
- Modify: `internal/engine/profile.go` (doc comments on `SaveProfileEncrypted` / `LoadProfileEncrypted`)
- Modify: `grpc/server/server_hijack_stream.go` (doc comment on `LoadProfile` handler)

No new test — doc-only. Existing encrypted tests keep passing (the functions still work; only their doc changes).

- [ ] **Step 1: Annotate the encrypted profile functions**

In `internal/engine/profile.go`, prepend to the doc comment of `SaveProfileEncrypted`:

```go
// Deprecated: the profile no longer carries secrets, so encryption protects
// nothing of value. Store secrets in pkg/scout/vault instead. Removal after 2026-07-02.
```

and prepend the same `// Deprecated: ...` line to `LoadProfileEncrypted`'s doc comment (keep the existing description lines below it).

- [ ] **Step 2: Document the gRPC LoadProfile no-op**

In `grpc/server/server_hijack_stream.go`, above the `func (s *ScoutServer) LoadProfile(...)` handler, add:

```go
// NOTE: post secret-migration, UserProfile carries no secrets and Page.ApplyProfile
// is a no-op, so this RPC no longer applies cookies/headers/storage to the session.
// It is retained as a compatibility stub; flag for deprecation/removal in Phase 2
// (after 2026-07-02). Local secret apply is done via `scout vault use`.
```

Do not change the proto or the handler logic.

- [ ] **Step 3: Verify build + existing tests**

Run: `go build ./internal/engine/ ./grpc/... && go test -short ./internal/engine/ -run 'Encrypted'`
Expected: build OK; `TestSaveLoadProfileEncrypted` + `TestProfileFullWorkflowEncrypted` + `TestLoadProfileEncrypted_WrongPassphrase` PASS.

- [ ] **Step 4: Commit (local only)**

```bash
git add internal/engine/profile.go grpc/server/server_hijack_stream.go
git commit -m "docs(profile): deprecate encrypted-profile funcs; note gRPC LoadProfile no-op"
```

---

### Task 6: CLI — add `vault capture` + wire storage into `vault use`

**Files:**
- Modify: `cmd/scout/vault.go` (new `vaultCaptureCmd`; one line in `vaultUseCmd`)
- Test: `cmd/scout/vault_test.go`

- [ ] **Step 1: Write the failing test**

Append to `cmd/scout/vault_test.go`:

```go
func TestVaultCaptureCmdRegistered(t *testing.T) {
	var found *cobra.Command
	for _, c := range vaultCmd.Commands() {
		if c.Name() == "capture" {
			found = c
		}
	}
	if found == nil {
		t.Fatal("vault capture command not registered under vaultCmd")
	}
	// Requires exactly <name> <url>.
	if err := found.Args(found, []string{"only-one"}); err == nil {
		t.Error("vault capture should reject a single argument")
	}
	if err := found.Args(found, []string{"name", "https://x"}); err != nil {
		t.Errorf("vault capture should accept <name> <url>: %v", err)
	}
}
```

Add `"github.com/spf13/cobra"` to the `cmd/scout/vault_test.go` import block.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/scout/ -run TestVaultCaptureCmdRegistered`
Expected: FAIL — capture command not registered.

- [ ] **Step 3a: Add the capture command**

In `cmd/scout/vault.go`, add the command var (after `vaultUseCmd`):

```go
var vaultCaptureCmd = &cobra.Command{
	Use:   "capture <name> <url>",
	Short: "Capture a live local session's cookies + web storage into a vault profile",
	Long: `Launches a local browser, navigates to <url>, and stores the session's
cookies and the current origin's localStorage/sessionStorage into a vault profile
named <name>. Prints the profile's opaque ID.

For an authenticated capture, pair with --user-data-dir (an existing Chrome
profile) or a headed interactive login so the session is logged in before capture.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name, url := args[0], args[1]

		v, err := openVaultCLI(cmd)
		if err != nil {
			return err
		}
		defer func() { _ = v.Close() }()

		b, err := scout.New(baseOpts(cmd)...)
		if err != nil {
			return err
		}
		defer func() { _ = b.Close() }()

		page, err := b.NewPage(url)
		if err != nil {
			return err
		}
		if err := page.WaitLoad(); err != nil {
			return err
		}

		in, err := vault.CaptureFromPage(page, name)
		if err != nil {
			return err
		}
		id, err := v.Set(in)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), id)
		return nil
	},
}
```

In `func init()` of `cmd/scout/vault.go`, add `vaultCaptureCmd` to both the `--vault-file` loop and `vaultCmd.AddCommand(...)`:

```go
	for _, c := range []*cobra.Command{vaultInitCmd, vaultSetCmd, vaultListCmd, vaultGetCmd, vaultUseCmd, vaultCaptureCmd, vaultRotateCmd, vaultRmCmd} {
		c.Flags().String("vault-file", "", "override vault file path (default <scouthome>/profiles/vault.bin)")
	}
	vaultCmd.AddCommand(vaultInitCmd, vaultSetCmd, vaultListCmd, vaultGetCmd, vaultUseCmd, vaultCaptureCmd, vaultRotateCmd, vaultRmCmd)
```

- [ ] **Step 3b: Wire storage apply into `vault use`**

In `vaultUseCmd.RunE`, immediately after the existing `page.WaitLoad()` success (just before the final `Fprintf("injected profile ...")`), add:

```go
		if err := h.ApplyStorageToPage(page); err != nil {
			return err
		}
```

(`ApplyToPage` already ran pre-navigation; this seeds web storage now that the page is at the origin.)

- [ ] **Step 4: Run test + build**

Run: `go test ./cmd/scout/ -run TestVaultCaptureCmdRegistered && go build ./cmd/scout/`
Expected: PASS + build OK.

- [ ] **Step 5: Commit (local only)**

```bash
git add cmd/scout/vault.go cmd/scout/vault_test.go
git commit -m "feat(cli): add 'scout vault capture'; apply web storage in 'vault use'"
```

---

### Task 7: CLI — make `scout profile` non-secret

**Files:**
- Modify: `cmd/scout/profile.go` (help text, secret summary lines, `show` guard)
- Test: `cmd/scout/profile_test.go` (new)

- [ ] **Step 1: Write the failing test**

Create `cmd/scout/profile_test.go`:

```go
package main

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/inovacc/scout/pkg/scout"
)

// A legacy .scoutprofile that still carries secret fields must show a deprecation
// note pointing at the vault; a non-secret profile must not show secret sections.
func TestProfileShow_LegacySecretsCarryDeprecationNote(t *testing.T) {
	dir := t.TempDir()

	legacy := filepath.Join(dir, "legacy.scoutprofile")
	if err := scout.SaveProfile(&scout.UserProfile{
		Version: 1, Name: "legacy",
		Cookies: []scout.Cookie{{Name: "sid", Value: "x", Domain: ".e.com", Path: "/"}},
	}, legacy); err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}

	var out bytes.Buffer
	cmd := profileShowCmd
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.RunE(cmd, []string{legacy}); err != nil {
		t.Fatalf("profile show: %v", err)
	}
	got := out.String()
	if !bytes.Contains(out.Bytes(), []byte("vault")) {
		t.Errorf("legacy secret profile show should point users to the vault; got:\n%s", got)
	}
}

func TestProfileShow_NonSecretOmitsSecretSections(t *testing.T) {
	dir := t.TempDir()
	clean := filepath.Join(dir, "clean.scoutprofile")
	if err := scout.SaveProfile(&scout.UserProfile{
		Version: 1, Name: "clean",
		Identity: scout.ProfileIdentity{UserAgent: "UA/1"},
	}, clean); err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}

	var out bytes.Buffer
	cmd := profileShowCmd
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.RunE(cmd, []string{clean}); err != nil {
		t.Fatalf("profile show: %v", err)
	}
	if bytes.Contains(out.Bytes(), []byte("Cookies:")) {
		t.Errorf("non-secret profile must not print a Cookies section; got:\n%s", out.String())
	}
}
```

Note: `profileShowCmd` is the existing `var` in `cmd/scout/profile.go` (the `show` command). If its variable name differs, use the actual name. Confirm with: `grep -n 'profileShowCmd\|Use:   "show' cmd/scout/profile.go`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/scout/ -run TestProfileShow_`
Expected: FAIL — current `show` always prints a `Cookies:` section and no `vault` note.

- [ ] **Step 3: Edit `cmd/scout/profile.go`**

3a. **`show` secret sections → guard + note.** The three secret regions in `profileShowCmd.RunE` are **NOT contiguous** — the non-secret `Extensions` block sits between Storage and Headers. Make three precise edits; leave `Extensions` untouched.

First, **delete** the current unconditional Cookies block:

```go
		_, _ = fmt.Fprintf(w, "Cookies:        %d\n", len(prof.Cookies))
		for _, c := range prof.Cookies {
			_, _ = fmt.Fprintf(w, "  %-30s  domain=%-20s  secure=%v  httpOnly=%v\n",
				truncate(c.Name, 30), c.Domain, c.Secure, c.HTTPOnly)
		}
```

and the current Storage block:

```go
		for origin, s := range prof.Storage {
			_, _ = fmt.Fprintf(w, "Storage [%s]:\n", origin)
			_, _ = fmt.Fprintf(w, "  localStorage:   %d keys\n", len(s.LocalStorage))

			for k := range s.LocalStorage {
				_, _ = fmt.Fprintf(w, "    %s\n", truncate(k, 60))
			}

			_, _ = fmt.Fprintf(w, "  sessionStorage: %d keys\n", len(s.SessionStorage))
			for k := range s.SessionStorage {
				_, _ = fmt.Fprintf(w, "    %s\n", truncate(k, 60))
			}
		}
```

and the current Headers block (further down, after `Extensions`):

```go
		if len(prof.Headers) > 0 {
			_, _ = fmt.Fprintf(w, "Headers:        %d\n", len(prof.Headers))
			for k, v := range prof.Headers {
				_, _ = fmt.Fprintf(w, "  %s: %s\n", k, truncate(v, 60))
			}
		}
```

Then **insert** this single guarded block where the Cookies block used to be (cookie values masked; `Extensions` stays in its original place):

```go
		if len(prof.Cookies) > 0 || len(prof.Storage) > 0 || len(prof.Headers) > 0 {
			_, _ = fmt.Fprintln(w, "[deprecated] This profile carries secret fields. Migrate them with")
			_, _ = fmt.Fprintln(w, "  scout vault set --from-profile <file>   (removal after 2026-07-02)")

			if len(prof.Cookies) > 0 {
				_, _ = fmt.Fprintf(w, "Cookies:        %d\n", len(prof.Cookies))
				for _, c := range prof.Cookies {
					_, _ = fmt.Fprintf(w, "  %-30s  domain=%-20s  value=***\n", truncate(c.Name, 30), c.Domain)
				}
			}
			for origin, s := range prof.Storage {
				_, _ = fmt.Fprintf(w, "Storage [%s]: localStorage=%d sessionStorage=%d\n",
					origin, len(s.LocalStorage), len(s.SessionStorage))
			}
			if len(prof.Headers) > 0 {
				_, _ = fmt.Fprintf(w, "Headers:        %d (values hidden)\n", len(prof.Headers))
			}
		}
```

3b. **`capture` / `load` summaries.** Delete the now-misleading secret summary lines so they do not print `0`:
- In `profileCaptureCmd.RunE`, delete the `Fprintf(w, "  Cookies:        %d\n", len(prof.Cookies))` line and the `storageKeys` loop + `Fprintf(w, "  Storage keys:   %d\n", storageKeys)` lines (~lines 87–94).
- In `profileLoadCmd.RunE`, delete the `Fprintf(w, "  Cookies:        %d\n", len(prof.Cookies))` line (~line 151).
- In `profileSessionLoadCmd.RunE`, delete the `Fprintf(w, "  Cookies: %d\n", ...)` and `Fprintf(w, "  Storage: %d origin(s)\n", ...)` lines (~lines 462–463).

3c. **Help text.** Update the `Long:`/`Short:` strings that mention secrets:
- `profileCaptureCmd` `Long` (~line 26): change "cookies, localStorage, sessionStorage, user agent, language, timezone, window size." to "user agent, language, timezone, locale, window size, extensions, and proxy. Secrets (cookies, web storage) are NOT captured — use `scout vault capture`."
- `profileLoadCmd` `Long` (~line 104-105): change the line listing "cookies, localStorage, sessionStorage, ... and headers" to "Restores user agent, window size, and proxy. Secrets are applied separately via `scout vault use`."

- [ ] **Step 4: Run test + build**

Run: `go test ./cmd/scout/ -run TestProfileShow_ && go build ./cmd/scout/`
Expected: PASS + build OK.

- [ ] **Step 5: Commit (local only)**

```bash
git add cmd/scout/profile.go cmd/scout/profile_test.go
git commit -m "refactor(cli): make 'scout profile' non-secret; point secrets to the vault"
```

---

### Task 8: Full local verification

**Files:** none (verification + bookkeeping only)

- [ ] **Step 1: Build the whole tree**

Run: `go build ./cmd/scout/ ./pkg/... ./grpc/... ./internal/...`
Expected: no output (success). Root has no main, so do not `go build ./...`.

- [ ] **Step 2: Vet the touched packages**

Run: `go vet ./cmd/scout/ ./pkg/scout/vault/ ./internal/engine/ ./grpc/...`
Expected: clean (no new findings; the pre-existing `go.mod x/mod` + `swarm.go SA4011` notes are out of scope).

- [ ] **Step 3: Run the gated test suite for touched packages**

Run: `go test -short -count=1 ./cmd/scout/ ./pkg/scout/vault/ ./internal/engine/`
Expected: PASS / no failures (browser tests skip under `-short`).

- [ ] **Step 4: Run the new behavior tests WITH a browser (no -short), if Chromium is available**

Run: `go test -count=1 -run 'CaptureFromPage|ApplyStorageToPage|CaptureProfile_OmitsSecrets|ApplyProfile_NoOp' ./pkg/scout/vault/ ./internal/engine/`
Expected: PASS (or SKIP if Chromium unavailable). These prove the real capture/apply round-trips.

- [ ] **Step 5: Confirm the deprecated fields still compile (Phase 1 invariant)**

Run: `grep -n 'Cookies \[\]Cookie\|Storage map\[string\]ProfileOriginStorage\|Headers    map\[string\]string' internal/engine/profile.go`
Expected: the three fields are still present (Phase 2 removes them after 2026-07-02).

- [ ] **Step 6: Stop — await push approval**

Do NOT push or ff-merge to `main`. Report completion and let the user decide when to integrate `feat/profile-vault-phase1`. When approved, the integration is:

```bash
git switch main && git merge --ff-only feat/profile-vault-phase1 && git push origin main
```

---

## Out of scope (Phase 2 / future)

- Removing `UserProfile.Cookies/Storage/Headers` + `ProfileOriginStorage` (dated cleanup after 2026-07-02).
- Removing `FromUserProfile` + `vault set --from-profile`, `Save/LoadProfileEncrypted`, and the secret branches of `Merge`/`Diff`/`Validate`/`profile show`.
- Deprecating/removing the gRPC `LoadProfile` RPC.
- Remote-daemon-session secret transfer; vault-secret gRPC RPCs.
