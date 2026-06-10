package capture

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/segmentio/ksuid"

	"github.com/inovacc/scout/internal/engine/scouthome"
)

// SpoolDir resolves <scouthome>/captures/spool, creating it 0700.
func SpoolDir() (string, error) {
	base, err := scouthome.Sub("captures")
	if err != nil {
		return "", fmt.Errorf("scout: capture: resolve home: %w", err)
	}
	dir := filepath.Join(base, "spool")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("scout: capture: mkdir spool: %w", err)
	}
	return dir, nil
}

// WriteSpool seals plaintext to pub and writes it as <ksuid>.cap (0600) in spoolDir.
func WriteSpool(spoolDir string, pub *[32]byte, plaintext []byte) (string, error) {
	if err := os.MkdirAll(spoolDir, 0o700); err != nil {
		return "", fmt.Errorf("scout: capture: mkdir spool: %w", err)
	}
	sealed, err := Seal(pub, plaintext)
	if err != nil {
		return "", err
	}
	id := ksuid.New().String()
	path := filepath.Join(spoolDir, id+".cap")
	if err := os.WriteFile(path, sealed, 0o600); err != nil {
		return "", fmt.Errorf("scout: capture: write spool: %w", err)
	}
	return id, nil
}

// ListSpool returns the full paths of all .cap files in spoolDir, sorted (ksuid = time-ordered).
func ListSpool(spoolDir string) ([]string, error) {
	entries, err := os.ReadDir(spoolDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("scout: capture: read spool: %w", err)
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".cap" {
			out = append(out, filepath.Join(spoolDir, e.Name()))
		}
	}
	return out, nil
}

// OpenSpoolFile reads and decrypts a spool file. ok is false on any failure.
func OpenSpoolFile(path string, pub, priv *[32]byte) ([]byte, bool) {
	sealed, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil, false
	}
	return Open(pub, priv, sealed)
}

// SecureDelete best-effort overwrites a file with zeros then removes it.
func SecureDelete(path string) error {
	if fi, err := os.Stat(path); err == nil {
		if f, err := os.OpenFile(path, os.O_WRONLY, 0o600); err == nil {
			_, _ = f.Write(make([]byte, fi.Size()))
			_ = f.Sync()
			_ = f.Close()
		}
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("scout: capture: remove spool file: %w", err)
	}
	return nil
}

// Quarantine renames a bad spool file to <name>.bad so the drain can continue.
func Quarantine(path string) error {
	if err := os.Rename(path, path+".bad"); err != nil {
		return fmt.Errorf("scout: capture: quarantine: %w", err)
	}
	return nil
}
