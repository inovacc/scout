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
