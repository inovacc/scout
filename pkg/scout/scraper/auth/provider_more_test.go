package auth

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
)

// TestAuthError_ErrorsAs verifies AuthError participates correctly in the
// errors.As chain when wrapped with %w, and that the message is preserved.
func TestAuthError_ErrorsAs(t *testing.T) {
	base := &AuthError{Reason: "session expired"}
	wrapped := fmt.Errorf("scraper: login: %w", base)

	var ae *AuthError
	if !errors.As(wrapped, &ae) {
		t.Fatalf("errors.As did not find *AuthError in %v", wrapped)
	}
	if ae.Reason != "session expired" {
		t.Errorf("Reason = %q, want %q", ae.Reason, "session expired")
	}
	if got := wrapped.Error(); !strings.Contains(got, "auth: session expired") {
		t.Errorf("wrapped.Error() = %q, want to contain 'auth: session expired'", got)
	}
}

// TestAuthError_Table covers several reasons including the empty case.
func TestAuthError_Table(t *testing.T) {
	tests := []struct {
		name   string
		reason string
		want   string
	}{
		{name: "simple", reason: "token missing", want: "auth: token missing"},
		{name: "empty", reason: "", want: "auth: "},
		{name: "with colon", reason: "detect: failed", want: "auth: detect: failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &AuthError{Reason: tt.reason}
			if got := e.Error(); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestRegistry_GetUnknownErrorFormat asserts the not-found error names the
// requested provider (so callers/tests can pattern-match it).
func TestRegistry_GetUnknownErrorFormat(t *testing.T) {
	r := &Registry{providers: make(map[string]Provider)}

	_, err := r.Get("ghost")
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
	if !strings.Contains(err.Error(), `unknown provider "ghost"`) {
		t.Errorf("error = %q, want to quote the provider name", err.Error())
	}
}

// TestRegistry_ListEmpty verifies an empty registry returns a non-nil, empty
// slice (callers may range over it without a nil check).
func TestRegistry_ListEmpty(t *testing.T) {
	r := &Registry{providers: make(map[string]Provider)}

	names := r.List()
	if names == nil {
		t.Fatal("List() returned nil, want non-nil empty slice")
	}
	if len(names) != 0 {
		t.Errorf("List() = %v, want empty", names)
	}
}

// TestRegistry_ConcurrentAccess exercises the RWMutex guarding the map under
// simultaneous registers and reads; the race detector validates safety.
func TestRegistry_ConcurrentAccess(t *testing.T) {
	r := &Registry{providers: make(map[string]Provider)}

	var wg sync.WaitGroup
	for i := range 16 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			r.Register(&fakeProvider{name: fmt.Sprintf("p%d", n)})
			_ = r.List()
			_, _ = r.Get(fmt.Sprintf("p%d", n))
		}(i)
	}
	wg.Wait()

	if got := len(r.List()); got != 16 {
		t.Errorf("registered providers = %d, want 16", got)
	}
}

// TestNewState_UniqueAndDecodable verifies newState produces distinct,
// URL-safe base64 tokens of the expected entropy width (32 raw bytes).
func TestNewState_UniqueAndDecodable(t *testing.T) {
	seen := make(map[string]struct{}, 64)
	for range 64 {
		s, err := newState()
		if err != nil {
			t.Fatalf("newState() error = %v", err)
		}
		if s == "" {
			t.Fatal("newState() returned empty string")
		}
		if _, dup := seen[s]; dup {
			t.Fatalf("newState() produced a duplicate token %q", s)
		}
		seen[s] = struct{}{}

		// Must be valid raw-url base64 decoding to exactly 32 bytes of entropy.
		raw, err := base64.RawURLEncoding.DecodeString(s)
		if err != nil {
			t.Fatalf("newState() token %q is not raw-url base64: %v", s, err)
		}
		if len(raw) != 32 {
			t.Errorf("newState() decoded to %d bytes, want 32", len(raw))
		}
	}
}
