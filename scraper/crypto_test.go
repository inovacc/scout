package scraper

import (
	"bytes"
	"testing"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	plaintext := []byte("sensitive session data with xoxc-token-12345")
	passphrase := "correct-horse-battery-staple"

	encrypted, err := EncryptData(plaintext, passphrase)
	if err != nil {
		t.Fatalf("EncryptData: %v", err)
	}

	if len(encrypted) <= headerLen {
		t.Fatalf("encrypted data too short: %d bytes", len(encrypted))
	}

	if encrypted[0] != cryptoVersion {
		t.Fatalf("version byte = %d, want %d", encrypted[0], cryptoVersion)
	}

	decrypted, err := DecryptData(encrypted, passphrase)
	if err != nil {
		t.Fatalf("DecryptData: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("decrypted = %q, want %q", decrypted, plaintext)
	}
}

func TestDecryptWrongPassphrase(t *testing.T) {
	plaintext := []byte("secret data")

	encrypted, err := EncryptData(plaintext, "right-passphrase")
	if err != nil {
		t.Fatalf("EncryptData: %v", err)
	}

	_, err = DecryptData(encrypted, "wrong-passphrase")
	if err == nil {
		t.Fatal("DecryptData with wrong passphrase should fail")
	}
}

func TestDecryptCorruptedData(t *testing.T) {
	plaintext := []byte("secret data")

	encrypted, err := EncryptData(plaintext, "passphrase")
	if err != nil {
		t.Fatalf("EncryptData: %v", err)
	}

	// Flip a byte in the ciphertext area
	corrupted := make([]byte, len(encrypted))
	copy(corrupted, encrypted)
	corrupted[len(corrupted)-1] ^= 0xFF

	_, err = DecryptData(corrupted, "passphrase")
	if err == nil {
		t.Fatal("DecryptData with corrupted data should fail")
	}
}

func TestDecryptTooShort(t *testing.T) {
	// Less than header length
	_, err := DecryptData([]byte{0x01, 0x02, 0x03}, "passphrase")
	if err == nil {
		t.Fatal("DecryptData with truncated data should fail")
	}
}

func TestDecryptUnsupportedVersion(t *testing.T) {
	data := make([]byte, headerLen+16)
	data[0] = 0xFF // unsupported version

	_, err := DecryptData(data, "passphrase")
	if err == nil {
		t.Fatal("DecryptData with unsupported version should fail")
	}
}

func TestEncryptProducesDifferentOutput(t *testing.T) {
	plaintext := []byte("same data")
	passphrase := "same-passphrase"

	enc1, err := EncryptData(plaintext, passphrase)
	if err != nil {
		t.Fatalf("EncryptData (1): %v", err)
	}

	enc2, err := EncryptData(plaintext, passphrase)
	if err != nil {
		t.Fatalf("EncryptData (2): %v", err)
	}

	if bytes.Equal(enc1, enc2) {
		t.Fatal("two encryptions of same data should produce different output (random salt/nonce)")
	}
}
