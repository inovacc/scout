package vault

import (
	"bytes"
	"errors"
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

func TestOpenTamperedCiphertextFails(t *testing.T) {
	blob, err := seal([]byte("integrity-matters"), []byte("pass"))
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	// Flip the last byte (inside the GCM tag / ciphertext) — auth must fail.
	blob[len(blob)-1] ^= 0xFF
	if _, err := open(blob, []byte("pass")); err == nil {
		t.Fatal("open accepted a tampered ciphertext")
	}
}

func TestOpenWrongVersionByteFails(t *testing.T) {
	blob, err := seal([]byte("data"), []byte("pass"))
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	blob[0] = vaultVersion + 1 // corrupt the vault version byte
	if _, err := open(blob, []byte("pass")); err == nil {
		t.Fatal("open accepted an unknown vault version")
	} else if !errors.Is(err, errVaultFormat) {
		t.Fatalf("want errVaultFormat for bad version, got %v", err)
	}
}

func TestSealIsNondeterministic(t *testing.T) {
	pass := []byte("pass")
	plain := []byte("same-plaintext")
	a, err := seal(plain, pass)
	if err != nil {
		t.Fatalf("seal a: %v", err)
	}
	b, err := seal(plain, pass)
	if err != nil {
		t.Fatalf("seal b: %v", err)
	}
	if bytes.Equal(a, b) {
		t.Fatal("two seals of identical input produced identical blobs (salt/nonce not random)")
	}
	// Both must still decrypt back to the same plaintext.
	for _, blob := range [][]byte{a, b} {
		got, err := open(blob, pass)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		if !bytes.Equal(got, plain) {
			t.Fatal("decrypt mismatch")
		}
	}
}
