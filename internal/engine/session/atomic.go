package session

import (
	"fmt"
	"os"
	"path/filepath"
)

// writeFileAtomic writes data to path atomically: writes to a temp file in the
// same directory, fsyncs, then renames over the target. A crash between
// truncate and write (the failure mode of os.WriteFile) leaves the original
// file intact instead of producing a zero-byte file.
//
// The rename is atomic on the same filesystem (POSIX) and supported on Windows
// via Go's os.Rename using MoveFileEx with MOVEFILE_REPLACE_EXISTING.
//
// Hardening H3 — see docs/quality/SESSION_HARDENING.md.
func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	f, err := os.CreateTemp(dir, base+".tmp.*")
	if err != nil {
		return fmt.Errorf("scout: create temp: %w", err)
	}

	tmpPath := f.Name()

	cleanup := func() { _ = os.Remove(tmpPath) }

	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		cleanup()

		return fmt.Errorf("scout: write temp: %w", err)
	}

	if err := f.Sync(); err != nil {
		_ = f.Close()
		cleanup()

		return fmt.Errorf("scout: fsync temp: %w", err)
	}

	if err := f.Close(); err != nil {
		cleanup()

		return fmt.Errorf("scout: close temp: %w", err)
	}

	if err := os.Chmod(tmpPath, mode); err != nil {
		cleanup()

		return fmt.Errorf("scout: chmod temp: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()

		return fmt.Errorf("scout: rename temp: %w", err)
	}

	return nil
}
