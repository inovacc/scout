package vault

import (
	"fmt"
	"os"
	"path/filepath"
)

// atomicWrite writes data to path via a temp-file-then-rename, so a crash never
// leaves a partial file. The parent directory must already exist. The file is
// always created with mode 0o600.
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	f, err := os.CreateTemp(dir, base+".tmp.*")
	if err != nil {
		return fmt.Errorf("scout: vault: create temp: %w", err)
	}
	tmp := f.Name()
	cleanup := func() { _ = f.Close(); _ = os.Remove(tmp) }

	if _, err := f.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("scout: vault: write temp: %w", err)
	}
	if err := f.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("scout: vault: sync temp: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("scout: vault: close temp: %w", err)
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("scout: vault: chmod temp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("scout: vault: rename: %w", err)
	}
	// Best-effort fsync of the parent directory so the rename of the rotated/
	// updated vault survives a crash. Directory fsync is a no-op/unsupported on
	// some platforms (e.g. Windows); ignore its error.
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}
