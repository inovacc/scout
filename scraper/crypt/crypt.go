// Package crypt provides AES-256-GCM encryption for session data.
package crypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"

	"golang.org/x/crypto/argon2"
)

const (
	cryptoVersion = 0x01
	saltLen       = 32
	nonceLen      = 12
	keyLen        = 32

	// Argon2id parameters.
	argonTime    = 3
	argonMemory  = 64 * 1024 // 64 MB
	argonThreads = 4

	// HeaderLen is version (1) + salt (32) + nonce (12).
	HeaderLen = 1 + saltLen + nonceLen
)

// Version returns the current crypto format version byte.
func Version() byte { return cryptoVersion }

// deriveKey uses Argon2id to derive a 256-bit key from a passphrase and salt.
func deriveKey(passphrase string, salt []byte) []byte {
	return argon2.IDKey([]byte(passphrase), salt, argonTime, argonMemory, argonThreads, keyLen)
}

// EncryptData encrypts plaintext with AES-256-GCM using a passphrase.
// Output format: [version:1][salt:32][nonce:12][ciphertext+GCM tag].
func EncryptData(data []byte, passphrase string) ([]byte, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("crypt: encrypt: generate salt: %w", err)
	}

	key := deriveKey(passphrase, salt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypt: encrypt: new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypt: encrypt: new gcm: %w", err)
	}

	nonce := make([]byte, nonceLen)
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("crypt: encrypt: generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, data, nil)

	out := make([]byte, 0, HeaderLen+len(ciphertext))
	out = append(out, cryptoVersion)
	out = append(out, salt...)
	out = append(out, nonce...)
	out = append(out, ciphertext...)

	return out, nil
}

// DecryptData decrypts data produced by EncryptData.
func DecryptData(encrypted []byte, passphrase string) ([]byte, error) {
	if len(encrypted) < HeaderLen {
		return nil, errors.New("crypt: decrypt: data too short")
	}

	if encrypted[0] != cryptoVersion {
		return nil, fmt.Errorf("crypt: decrypt: unsupported version %d", encrypted[0])
	}

	salt := encrypted[1 : 1+saltLen]
	nonce := encrypted[1+saltLen : HeaderLen]
	ciphertext := encrypted[HeaderLen:]

	key := deriveKey(passphrase, salt)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypt: decrypt: new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypt: decrypt: new gcm: %w", err)
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("crypt: decrypt: %w", err)
	}

	return plaintext, nil
}
