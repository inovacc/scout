package capture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSpoolWriteListOpenDelete(t *testing.T) {
	v, dir := newTempVault(t)
	pubPath := filepath.Join(dir, "capture.pub")
	pub, err := InitKeypair(v, pubPath, false)
	if err != nil {
		t.Fatalf("InitKeypair: %v", err)
	}
	spoolDir := filepath.Join(dir, "spool")

	id, err := WriteSpool(spoolDir, pub, []byte(`{"hello":"world"}`))
	if err != nil {
		t.Fatalf("WriteSpool: %v", err)
	}

	files, err := ListSpool(spoolDir)
	if err != nil || len(files) != 1 {
		t.Fatalf("ListSpool: files=%v err=%v", files, err)
	}

	priv, _ := LoadPriv(v)
	plain, ok := OpenSpoolFile(files[0], pub, priv)
	if !ok || string(plain) != `{"hello":"world"}` {
		t.Fatalf("OpenSpoolFile: ok=%v plain=%q", ok, plain)
	}

	if err := SecureDelete(files[0]); err != nil {
		t.Fatalf("SecureDelete: %v", err)
	}
	if _, err := os.Stat(files[0]); !os.IsNotExist(err) {
		t.Fatalf("spool file %s (id %s) not deleted", files[0], id)
	}
}

func TestOpenSpoolFileTamperFails(t *testing.T) {
	v, dir := newTempVault(t)
	pubPath := filepath.Join(dir, "capture.pub")
	pub, _ := InitKeypair(v, pubPath, false)
	spoolDir := filepath.Join(dir, "spool")
	if _, err := WriteSpool(spoolDir, pub, []byte("data")); err != nil {
		t.Fatalf("WriteSpool: %v", err)
	}
	files, _ := ListSpool(spoolDir)

	raw, _ := os.ReadFile(files[0])
	raw[len(raw)-1] ^= 0xFF // flip a ciphertext byte
	_ = os.WriteFile(files[0], raw, 0o600)

	priv, _ := LoadPriv(v)
	if _, ok := OpenSpoolFile(files[0], pub, priv); ok {
		t.Fatal("tampered spool file opened; AEAD must reject it")
	}
}
