package auth

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/inovacc/scout/scraper"
)

// SaveEncrypted marshals the session to JSON, encrypts it, and writes to the given path.
func SaveEncrypted(session *Session, path, passphrase string) error {
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("auth: save session: marshal: %w", err)
	}

	encrypted, err := scraper.EncryptData(data, passphrase)
	if err != nil {
		return fmt.Errorf("auth: save session: %w", err)
	}

	if err := os.WriteFile(path, encrypted, 0o600); err != nil {
		return fmt.Errorf("auth: save session: write: %w", err)
	}

	return nil
}

// LoadEncrypted reads an encrypted session file, decrypts it, and unmarshals into Session.
func LoadEncrypted(path, passphrase string) (*Session, error) {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("auth: load session: read: %w", err)
	}

	plaintext, err := scraper.DecryptData(data, passphrase)
	if err != nil {
		return nil, fmt.Errorf("auth: load session: %w", err)
	}

	var session Session
	if err := json.Unmarshal(plaintext, &session); err != nil {
		return nil, fmt.Errorf("auth: load session: unmarshal: %w", err)
	}

	return &session, nil
}
