package identity

import (
	"crypto/x509"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateIdentity(t *testing.T) {
	id, err := GenerateIdentity()
	if err != nil {
		t.Fatalf("GenerateIdentity: %v", err)
	}

	if id.DeviceID == "" {
		t.Fatal("device ID is empty")
	}

	if !ValidateDeviceID(id.DeviceID) {
		t.Fatalf("generated device ID is invalid: %s", id.DeviceID)
	}

	// Check format: 8 groups of 7 chars separated by dashes
	parts := strings.Split(id.DeviceID, "-")
	if len(parts) != 8 {
		t.Fatalf("expected 8 dash-separated groups, got %d: %s", len(parts), id.DeviceID)
	}
	for i, p := range parts {
		if len(p) != 7 {
			t.Fatalf("group %d has length %d, want 7: %q", i, len(p), p)
		}
	}
}

func TestDeviceIDDeterministic(t *testing.T) {
	id, err := GenerateIdentity()
	if err != nil {
		t.Fatal(err)
	}

	cert, err := x509.ParseCertificate(id.Certificate.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}

	// Computing device ID again from same cert should give same result
	id2 := DeviceIDFromCert(cert)
	if id.DeviceID != id2 {
		t.Fatalf("device ID not deterministic: %s != %s", id.DeviceID, id2)
	}
}

func TestValidateDeviceID(t *testing.T) {
	id, err := GenerateIdentity()
	if err != nil {
		t.Fatal(err)
	}

	if !ValidateDeviceID(id.DeviceID) {
		t.Fatal("valid device ID rejected")
	}

	// Corrupt a character
	bad := []byte(id.DeviceID)
	if bad[0] == 'A' {
		bad[0] = 'B'
	} else {
		bad[0] = 'A'
	}
	if ValidateDeviceID(string(bad)) {
		t.Fatal("corrupted device ID accepted")
	}

	if ValidateDeviceID("") {
		t.Fatal("empty device ID accepted")
	}

	if ValidateDeviceID("too-short") {
		t.Fatal("short device ID accepted")
	}
}

func TestShortID(t *testing.T) {
	id, err := GenerateIdentity()
	if err != nil {
		t.Fatal(err)
	}

	short := ShortID(id.DeviceID)
	if len(short) != 7 {
		t.Fatalf("short ID length %d, want 7", len(short))
	}
}

func TestSaveLoadIdentity(t *testing.T) {
	dir := t.TempDir()

	id, err := GenerateIdentity()
	if err != nil {
		t.Fatal(err)
	}

	if err := SaveIdentity(id, dir); err != nil {
		t.Fatalf("SaveIdentity: %v", err)
	}

	// Check files exist
	if _, err := os.Stat(filepath.Join(dir, "cert.pem")); err != nil {
		t.Fatal("cert.pem not created")
	}
	if _, err := os.Stat(filepath.Join(dir, "key.pem")); err != nil {
		t.Fatal("key.pem not created")
	}

	loaded, err := LoadIdentity(dir)
	if err != nil {
		t.Fatalf("LoadIdentity: %v", err)
	}

	if loaded.DeviceID != id.DeviceID {
		t.Fatalf("device ID mismatch: %s != %s", loaded.DeviceID, id.DeviceID)
	}
}

func TestLoadOrGenerate(t *testing.T) {
	dir := t.TempDir()

	// First call should generate
	id1, err := LoadOrGenerate(dir)
	if err != nil {
		t.Fatalf("LoadOrGenerate (generate): %v", err)
	}

	// Second call should load same identity
	id2, err := LoadOrGenerate(dir)
	if err != nil {
		t.Fatalf("LoadOrGenerate (load): %v", err)
	}

	if id1.DeviceID != id2.DeviceID {
		t.Fatalf("device ID changed after reload: %s != %s", id1.DeviceID, id2.DeviceID)
	}
}

func TestLuhnRoundTrip(t *testing.T) {
	// 52-char base32 string
	input := "MFZWI3DBONSGYZLOOQQGC3LFOQQHG2DFEBZGK4TTNFWHIZLTOQ"
	if len(input) != 52 {
		// pad to 52
		input = input + strings.Repeat("A", 52-len(input))
	}

	luhnified, err := luhnify(input)
	if err != nil {
		t.Fatalf("luhnify: %v", err)
	}
	if len(luhnified) != 56 {
		t.Fatalf("luhnified length %d, want 56", len(luhnified))
	}

	back, err := unluhnify(luhnified)
	if err != nil {
		t.Fatalf("unluhnify: %v", err)
	}
	if back != input {
		t.Fatalf("round-trip failed: %q != %q", back, input)
	}
}

func TestTrustStore(t *testing.T) {
	dir := t.TempDir()
	ts, err := NewTrustStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	id, err := GenerateIdentity()
	if err != nil {
		t.Fatal(err)
	}

	if ts.IsTrusted(id.DeviceID) {
		t.Fatal("untrusted device reported as trusted")
	}

	if err := ts.Trust(id.DeviceID, id.Certificate.Certificate[0]); err != nil {
		t.Fatalf("Trust: %v", err)
	}

	if !ts.IsTrusted(id.DeviceID) {
		t.Fatal("trusted device reported as untrusted")
	}

	devices, err := ts.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(devices) != 1 {
		t.Fatalf("expected 1 trusted device, got %d", len(devices))
	}
	if devices[0].DeviceID != id.DeviceID {
		t.Fatalf("wrong device ID: %s", devices[0].DeviceID)
	}

	pool, err := ts.CertPool()
	if err != nil {
		t.Fatalf("CertPool: %v", err)
	}
	if pool == nil {
		t.Fatal("cert pool is nil")
	}

	if err := ts.Remove(id.DeviceID); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	if ts.IsTrusted(id.DeviceID) {
		t.Fatal("removed device still trusted")
	}
}
