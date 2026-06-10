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
