package crypt

import (
	"bytes"
	"testing"
)

func TestVersion(t *testing.T) {
	if got := Version(); got != 0x01 {
		t.Errorf("Version() = %d, want 1", got)
	}
}

func TestHeaderLen(t *testing.T) {
	// version(1) + salt(32) + nonce(12) = 45
	if HeaderLen != 45 {
		t.Errorf("HeaderLen = %d, want 45", HeaderLen)
	}
}

func TestEncryptDecryptRoundtrip(t *testing.T) {
	tests := []struct {
		name       string
		data       []byte
		passphrase string
	}{
		{"simple text", []byte("hello world"), "mysecret"},
		{"empty data", []byte{}, "pass"},
		{"binary data", []byte{0x00, 0xff, 0x80, 0x01}, "binary-pass"},
		{"long passphrase", []byte("test"), "a-very-long-passphrase-that-exceeds-normal-length-requirements"},
		{"unicode passphrase", []byte("data"), "\u00e9\u00e0\u00fc\u00f1"},
		{"large payload", bytes.Repeat([]byte("A"), 10000), "bigdata"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encrypted, err := EncryptData(tt.data, tt.passphrase)
			if err != nil {
				t.Fatalf("EncryptData() error = %v", err)
			}

			if len(encrypted) < HeaderLen {
				t.Fatalf("encrypted data too short: %d < %d", len(encrypted), HeaderLen)
			}

			// Check version byte
			if encrypted[0] != cryptoVersion {
				t.Errorf("version byte = %d, want %d", encrypted[0], cryptoVersion)
			}

			decrypted, err := DecryptData(encrypted, tt.passphrase)
			if err != nil {
				t.Fatalf("DecryptData() error = %v", err)
			}

			if !bytes.Equal(decrypted, tt.data) {
				t.Errorf("roundtrip mismatch: got %q, want %q", decrypted, tt.data)
			}
		})
	}
}

func TestDecryptData_TooShort(t *testing.T) {
	_, err := DecryptData(make([]byte, HeaderLen-1), "pass")
	if err == nil {
		t.Fatal("expected error for short data")
	}
	if got := err.Error(); got != "crypt: decrypt: data too short" {
		t.Errorf("error = %q, want 'crypt: decrypt: data too short'", got)
	}
}

func TestDecryptData_WrongVersion(t *testing.T) {
	data := make([]byte, HeaderLen+16) // enough bytes
	data[0] = 0xFF                     // invalid version
	_, err := DecryptData(data, "pass")
	if err == nil {
		t.Fatal("expected error for wrong version")
	}
	if got := err.Error(); got != "crypt: decrypt: unsupported version 255" {
		t.Errorf("error = %q", got)
	}
}

func TestDecryptData_WrongPassphrase(t *testing.T) {
	encrypted, err := EncryptData([]byte("secret data"), "correct-pass")
	if err != nil {
		t.Fatalf("EncryptData() error = %v", err)
	}

	_, err = DecryptData(encrypted, "wrong-pass")
	if err == nil {
		t.Fatal("expected error for wrong passphrase")
	}
}

func TestEncryptData_DifferentCiphertexts(t *testing.T) {
	data := []byte("same data")
	pass := "same-pass"

	enc1, err := EncryptData(data, pass)
	if err != nil {
		t.Fatalf("first EncryptData() error = %v", err)
	}

	enc2, err := EncryptData(data, pass)
	if err != nil {
		t.Fatalf("second EncryptData() error = %v", err)
	}

	// Random salt+nonce means ciphertexts must differ
	if bytes.Equal(enc1, enc2) {
		t.Error("two encryptions of the same data produced identical output")
	}

	// Both must decrypt correctly
	dec1, _ := DecryptData(enc1, pass)
	dec2, _ := DecryptData(enc2, pass)
	if !bytes.Equal(dec1, data) || !bytes.Equal(dec2, data) {
		t.Error("one of the decryptions failed")
	}
}

func TestDecryptData_TamperedCiphertext(t *testing.T) {
	encrypted, err := EncryptData([]byte("integrity check"), "pass")
	if err != nil {
		t.Fatalf("EncryptData() error = %v", err)
	}

	// Flip a byte in the ciphertext portion
	encrypted[HeaderLen+1] ^= 0xFF

	_, err = DecryptData(encrypted, "pass")
	if err == nil {
		t.Fatal("expected error for tampered ciphertext")
	}
}
