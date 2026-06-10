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
