package vault

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/inovacc/scout/internal/engine/scouthome"
)

// storedProfile is the on-disk (inside the encrypted blob) form of a profile.
type storedProfile struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name,omitempty"`
	Secrets   map[string][]byte      `json:"secrets,omitempty"`
	Cookies   []Cookie               `json:"cookies,omitempty"`
	Storage   map[string]OriginStore `json:"storage,omitempty"`
	Headers   map[string][]byte      `json:"headers,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

type vaultData struct {
	Version  int             `json:"version"`
	Profiles []storedProfile `json:"profiles,omitempty"`
}

// ErrVaultNotFound is returned by loadVault when the vault file does not exist.
var ErrVaultNotFound = errors.New("scout: vault: not initialized")

// defaultVaultPath resolves <scouthome>/profiles/vault.bin and creates the
// profiles directory with 0o700 permissions.
func defaultVaultPath() (string, error) {
	dir, err := scouthome.Sub("profiles")
	if err != nil {
		return "", fmt.Errorf("scout: vault: resolve home: %w", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("scout: vault: mkdir: %w", err)
	}
	_ = os.Chmod(dir, 0o700) // umask guard on Unix; no-op on Windows ACLs
	return filepath.Join(dir, "vault.bin"), nil
}

func saveVault(path string, data *vaultData, passphrase []byte) error {
	plain, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("scout: vault: marshal: %w", err)
	}
	defer zero(plain)

	blob, err := seal(plain, passphrase)
	if err != nil {
		return err
	}
	return atomicWrite(path, blob)
}

func loadVault(path string, passphrase []byte) (*vaultData, error) {
	blob, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrVaultNotFound
		}
		return nil, fmt.Errorf("scout: vault: read: %w", err)
	}
	plain, err := open(blob, passphrase)
	if err != nil {
		return nil, err
	}
	defer zero(plain)

	var data vaultData
	if err := json.Unmarshal(plain, &data); err != nil {
		return nil, fmt.Errorf("scout: vault: unmarshal: %w", err)
	}
	return &data, nil
}
