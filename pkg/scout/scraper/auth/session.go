package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/inovacc/scout/pkg/scout/scraper/crypt"
)

// SaveEncrypted marshals the session to JSON, encrypts it, and writes it to the
// given path atomically (temp-file-then-rename) so a crash never leaves a
// partial/corrupt blob. The parent directory is created 0o700 and the file is
// written 0o600.
func SaveEncrypted(session *Session, path, passphrase string) error {
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("auth: save session: marshal: %w", err)
	}

	encrypted, err := crypt.EncryptData(data, passphrase)
	if err != nil {
		return fmt.Errorf("auth: save session: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("auth: save session: mkdir: %w", err)
	}

	f, err := os.CreateTemp(dir, filepath.Base(path)+".tmp.*") // CreateTemp makes the file 0o600
	if err != nil {
		return fmt.Errorf("auth: save session: create temp: %w", err)
	}
	tmp := f.Name()

	if _, err := f.Write(encrypted); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("auth: save session: write: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("auth: save session: close: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("auth: save session: rename: %w", err)
	}

	return nil
}

// LoadEncrypted reads an encrypted session file, decrypts it, and unmarshals into Session.
func LoadEncrypted(path, passphrase string) (*Session, error) {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("auth: load session: read: %w", err)
	}

	plaintext, err := crypt.DecryptData(data, passphrase)
	if err != nil {
		return nil, fmt.Errorf("auth: load session: %w", err)
	}

	var session Session
	if err := json.Unmarshal(plaintext, &session); err != nil {
		return nil, fmt.Errorf("auth: load session: unmarshal: %w", err)
	}

	return &session, nil
}
