package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/inovacc/scout/pkg/scout"
)

// --- Session tests ---

func TestSession_ProviderName(t *testing.T) {
	s := &Session{Provider: "slack"}
	if got := s.ProviderName(); got != "slack" {
		t.Errorf("ProviderName() = %q, want %q", got, "slack")
	}
}

func TestSession_ProviderName_Nil(t *testing.T) {
	var s *Session
	if got := s.ProviderName(); got != "" {
		t.Errorf("nil Session.ProviderName() = %q, want empty", got)
	}
}

// --- AuthError tests ---

func TestAuthError_Error(t *testing.T) {
	e := &AuthError{Reason: "token expired"}
	want := "auth: token expired"
	if got := e.Error(); got != want {
		t.Errorf("AuthError.Error() = %q, want %q", got, want)
	}
}

// --- Registry tests ---

type fakeProvider struct {
	name string
}

func (f *fakeProvider) Name() string     { return f.name }
func (f *fakeProvider) LoginURL() string { return "https://example.com/login" }
func (f *fakeProvider) DetectAuth(_ context.Context, _ *scout.Page) (bool, error) {
	return false, nil
}
func (f *fakeProvider) CaptureSession(_ context.Context, _ *scout.Page) (*Session, error) {
	return &Session{Provider: f.name}, nil
}
func (f *fakeProvider) ValidateSession(_ context.Context, _ *Session) error {
	return nil
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := &Registry{providers: make(map[string]Provider)}

	p := &fakeProvider{name: "test"}
	r.Register(p)

	got, err := r.Get("test")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Name() != "test" {
		t.Errorf("got provider name %q, want %q", got.Name(), "test")
	}
}

func TestRegistry_GetUnknown(t *testing.T) {
	r := &Registry{providers: make(map[string]Provider)}

	_, err := r.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
	if !strings.Contains(err.Error(), "unknown provider") {
		t.Errorf("error = %q, want to contain 'unknown provider'", err.Error())
	}
}

func TestRegistry_List(t *testing.T) {
	r := &Registry{providers: make(map[string]Provider)}

	r.Register(&fakeProvider{name: "alpha"})
	r.Register(&fakeProvider{name: "beta"})

	names := r.List()
	sort.Strings(names)

	if len(names) != 2 || names[0] != "alpha" || names[1] != "beta" {
		t.Errorf("List() = %v, want [alpha beta]", names)
	}
}

func TestRegistry_RegisterOverwrite(t *testing.T) {
	r := &Registry{providers: make(map[string]Provider)}

	r.Register(&fakeProvider{name: "dup"})
	r.Register(&fakeProvider{name: "dup"}) // overwrite

	names := r.List()
	if len(names) != 1 {
		t.Errorf("expected 1 provider after overwrite, got %d", len(names))
	}
}

// --- Package-level functions (use DefaultRegistry indirectly) ---

func TestPackageLevelRegisterGetList(t *testing.T) {
	// Save and restore DefaultRegistry
	orig := DefaultRegistry
	DefaultRegistry = &Registry{providers: make(map[string]Provider)}
	defer func() { DefaultRegistry = orig }()

	Register(&fakeProvider{name: "pkg-test"})

	p, err := Get("pkg-test")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if p.Name() != "pkg-test" {
		t.Errorf("Name() = %q", p.Name())
	}

	names := List()
	if len(names) != 1 || names[0] != "pkg-test" {
		t.Errorf("List() = %v", names)
	}
}

// --- PKCEChallenge tests ---

func TestPKCEChallenge(t *testing.T) {
	verifier, challenge, err := PKCEChallenge()
	if err != nil {
		t.Fatalf("PKCEChallenge() error = %v", err)
	}

	if verifier == "" {
		t.Error("verifier is empty")
	}
	if challenge == "" {
		t.Error("challenge is empty")
	}

	// Verify the challenge is SHA256(verifier) base64url-encoded
	h := sha256.Sum256([]byte(verifier))
	expected := base64.RawURLEncoding.EncodeToString(h[:])
	if challenge != expected {
		t.Errorf("challenge = %q, want SHA256(%q) = %q", challenge, verifier, expected)
	}
}

func TestPKCEChallenge_Unique(t *testing.T) {
	v1, _, _ := PKCEChallenge()
	v2, _, _ := PKCEChallenge()
	if v1 == v2 {
		t.Error("two calls produced the same verifier")
	}
}

// --- OAuthServer tests ---

func TestOAuthServer_CallbackURL(t *testing.T) {
	srv, err := NewOAuthServer()
	if err != nil {
		t.Fatalf("NewOAuthServer() error = %v", err)
	}
	defer srv.Close()

	url := srv.CallbackURL()
	if !strings.HasPrefix(url, "http://127.0.0.1:") {
		t.Errorf("CallbackURL() = %q, want http://127.0.0.1:*", url)
	}
	if !strings.HasSuffix(url, "/callback") {
		t.Errorf("CallbackURL() = %q, want */callback", url)
	}
}

func TestOAuthServer_HandleCallback(t *testing.T) {
	srv, err := NewOAuthServer()
	if err != nil {
		t.Fatalf("NewOAuthServer() error = %v", err)
	}
	defer srv.Close()

	// Simulate an OAuth callback
	callbackURL := srv.CallbackURL() + "?code=abc123&state=xyz"
	resp, err := http.Get(callbackURL) //nolint:gosec,noctx
	if err != nil {
		t.Fatalf("callback request error = %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := srv.Wait(ctx)
	if err != nil {
		t.Fatalf("Wait() error = %v", err)
	}

	if result.Code != "abc123" {
		t.Errorf("Code = %q, want %q", result.Code, "abc123")
	}
	if result.State != "xyz" {
		t.Errorf("State = %q, want %q", result.State, "xyz")
	}
	if result.Error != "" {
		t.Errorf("Error = %q, want empty", result.Error)
	}
}

func TestOAuthServer_HandleCallbackError(t *testing.T) {
	srv, err := NewOAuthServer()
	if err != nil {
		t.Fatalf("NewOAuthServer() error = %v", err)
	}
	defer srv.Close()

	callbackURL := srv.CallbackURL() + "?error=access_denied"
	resp, err := http.Get(callbackURL) //nolint:gosec,noctx
	if err != nil {
		t.Fatalf("callback request error = %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	result, err := srv.Wait(ctx)
	if err != nil {
		t.Fatalf("Wait() error = %v", err)
	}

	if result.Error != "access_denied" {
		t.Errorf("Error = %q, want %q", result.Error, "access_denied")
	}
}

func TestOAuthServer_WaitTimeout(t *testing.T) {
	srv, err := NewOAuthServer()
	if err != nil {
		t.Fatalf("NewOAuthServer() error = %v", err)
	}
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err = srv.Wait(ctx)
	if err == nil {
		t.Fatal("expected context deadline error")
	}
}

func TestOAuthServer_CloseIdempotent(t *testing.T) {
	srv, err := NewOAuthServer()
	if err != nil {
		t.Fatalf("NewOAuthServer() error = %v", err)
	}

	srv.Close()
	srv.Close() // should not panic
}

// --- SaveEncrypted / LoadEncrypted tests ---

func TestSaveLoadEncrypted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.enc")

	session := &Session{
		Provider:  "test",
		Version:   "1",
		Timestamp: time.Now().UTC().Truncate(time.Second),
		URL:       "https://example.com",
		Tokens:    map[string]string{"token": "abc"},
	}

	err := SaveEncrypted(session, path, "secret")
	if err != nil {
		t.Fatalf("SaveEncrypted() error = %v", err)
	}

	// File should exist and be non-empty
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("file not found: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("encrypted file is empty")
	}

	loaded, err := LoadEncrypted(path, "secret")
	if err != nil {
		t.Fatalf("LoadEncrypted() error = %v", err)
	}

	if loaded.Provider != session.Provider {
		t.Errorf("Provider = %q, want %q", loaded.Provider, session.Provider)
	}
	if loaded.URL != session.URL {
		t.Errorf("URL = %q, want %q", loaded.URL, session.URL)
	}
	if loaded.Tokens["token"] != "abc" {
		t.Errorf("Tokens[token] = %q, want %q", loaded.Tokens["token"], "abc")
	}
}

func TestLoadEncrypted_WrongPassphrase(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.enc")

	session := &Session{Provider: "test", Version: "1"}
	if err := SaveEncrypted(session, path, "correct"); err != nil {
		t.Fatalf("SaveEncrypted() error = %v", err)
	}

	_, err := LoadEncrypted(path, "wrong")
	if err == nil {
		t.Fatal("expected error for wrong passphrase")
	}
}

func TestLoadEncrypted_FileNotFound(t *testing.T) {
	_, err := LoadEncrypted("/nonexistent/path/session.enc", "pass")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

// --- DefaultBrowserAuthOptions tests ---

func TestDefaultBrowserAuthOptions(t *testing.T) {
	opts := DefaultBrowserAuthOptions()

	if opts.Headless {
		t.Error("Headless should be false by default")
	}
	if !opts.Stealth {
		t.Error("Stealth should be true by default")
	}
	if opts.Timeout != 5*time.Minute {
		t.Errorf("Timeout = %v, want 5m", opts.Timeout)
	}
	if opts.PollInterval != 2*time.Second {
		t.Errorf("PollInterval = %v, want 2s", opts.PollInterval)
	}
}

// --- reportProgress tests ---

func TestReportProgress_NilCallback(t *testing.T) {
	// Should not panic with nil callback
	reportProgress(nil, "test", 0, 0, "msg")
}

func TestReportProgress_WithCallback(t *testing.T) {
	var got Progress
	fn := func(p Progress) { got = p }

	reportProgress(fn, "auth", 1, 2, "working")

	if got.Phase != "auth" {
		t.Errorf("Phase = %q, want %q", got.Phase, "auth")
	}
	if got.Current != 1 || got.Total != 2 {
		t.Errorf("Current/Total = %d/%d, want 1/2", got.Current, got.Total)
	}
	if got.Message != "working" {
		t.Errorf("Message = %q, want %q", got.Message, "working")
	}
}

// --- OAuthServer handleCallback via httptest ---

func TestOAuthServer_HandleCallbackViaHTTPTest(t *testing.T) {
	srv, err := NewOAuthServer()
	if err != nil {
		t.Fatalf("NewOAuthServer() error = %v", err)
	}
	defer srv.Close()

	// Use httptest to directly test the handler
	req := httptest.NewRequest(http.MethodGet, "/callback?code=test-code&state=test-state&error=", nil)
	w := httptest.NewRecorder()

	srv.handleCallback(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "Authentication complete") {
		t.Errorf("body = %q, want to contain 'Authentication complete'", body)
	}
}
