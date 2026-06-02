package vault

import (
	"fmt"
	"os"
	"path/filepath"
)

// atomicWrite writes data to path via a temp-file-then-rename, so a crash never
// leaves a partial file. The parent directory must already exist.
func atomicWrite(path string, data []byte, mode os.FileMode) error {
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
	if err := os.Chmod(tmp, mode); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("scout: vault: chmod temp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("scout: vault: rename: %w", err)
	}
	return nil
}
