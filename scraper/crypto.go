package scraper

import "github.com/inovacc/scout/scraper/crypt"

// EncryptData encrypts plaintext with AES-256-GCM using a passphrase.
// Delegates to scraper/crypt.
func EncryptData(data []byte, passphrase string) ([]byte, error) {
	return crypt.EncryptData(data, passphrase)
}

// DecryptData decrypts data produced by EncryptData.
// Delegates to scraper/crypt.
func DecryptData(encrypted []byte, passphrase string) ([]byte, error) {
	return crypt.DecryptData(encrypted, passphrase)
}
