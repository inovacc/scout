package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"

	"golang.org/x/crypto/argon2"
)

// Crypto format (mirrors pkg/scout/scraper/crypt parameters but derives the key
// from a []byte passphrase so it can be zeroed):
//
//	[vaultVersion:1][cryptVersion:1][salt:saltLen][nonce:nonceLen][ciphertext+GCM tag]
const (
	vaultVersion = byte(0x01)
	cryptVersion = byte(0x01)
	saltLen      = 32
	nonceLen     = 12
	keyLen       = 32
	argonTime    = 3
	argonMemory  = 64 * 1024 // KiB = 64 MiB
	argonThreads = 4
	headerLen    = 2 + saltLen + nonceLen
)

var errVaultFormat = errors.New("scout: vault: malformed encrypted blob")

// deriveKey returns a 32-byte AES key. Caller must zero the returned slice.
func deriveKey(passphrase, salt []byte) []byte {
	return argon2.IDKey(passphrase, salt, argonTime, argonMemory, argonThreads, keyLen)
}

func seal(plaintext, passphrase []byte) ([]byte, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("scout: vault: salt: %w", err)
	}
	key := deriveKey(passphrase, salt)
	defer zero(key)

	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, nonceLen)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("scout: vault: nonce: %w", err)
	}

	out := make([]byte, 0, headerLen+len(plaintext)+gcm.Overhead())
	out = append(out, vaultVersion, cryptVersion)
	out = append(out, salt...)
	out = append(out, nonce...)
	return gcm.Seal(out, nonce, plaintext, nil), nil
}

func open(blob, passphrase []byte) ([]byte, error) {
	if len(blob) < headerLen {
		return nil, errVaultFormat
	}
	if blob[0] != vaultVersion || blob[1] != cryptVersion {
		return nil, errVaultFormat
	}
	salt := blob[2 : 2+saltLen]
	nonce := blob[2+saltLen : headerLen]
	ct := blob[headerLen:]

	key := deriveKey(passphrase, salt)
	defer zero(key)

	gcm, err := newGCM(key)
	if err != nil {
		return nil, err
	}
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("scout: vault: decrypt: %w", err)
	}
	return plain, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("scout: vault: cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("scout: vault: gcm: %w", err)
	}
	return gcm, nil
}

func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
