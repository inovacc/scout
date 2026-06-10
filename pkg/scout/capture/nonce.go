package capture

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
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
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("scout: capture: mkdir for nonce: %w", err)
	}
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
