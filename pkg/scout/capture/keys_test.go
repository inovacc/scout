package capture

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/inovacc/scout/pkg/scout/vault"
)

// TestInitKeypairCreatesMissingParentDir guards the clean-install path: the
// captures/ dir does not exist yet, so InitKeypair (writePub) and EnsureNonce
// must create the parent before writing. Regression for the Phase-1 review's
// release-blocking finding.
func TestInitKeypairCreatesMissingParentDir(t *testing.T) {
	v, dir := newTempVault(t)

	pubPath := filepath.Join(dir, "captures", "capture.pub") // parent absent
	if _, err := InitKeypair(v, pubPath, false); err != nil {
		t.Fatalf("InitKeypair into missing dir: %v", err)
	}
	if _, err := os.Stat(pubPath); err != nil {
		t.Fatalf("public key not written: %v", err)
	}

	noncePath := filepath.Join(dir, "captures2", "pairing.nonce") // parent absent
	if _, err := EnsureNonce(noncePath); err != nil {
		t.Fatalf("EnsureNonce into missing dir: %v", err)
	}
	if _, err := os.Stat(noncePath); err != nil {
		t.Fatalf("nonce not written: %v", err)
	}
}

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
