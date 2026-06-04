package archive

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ZipExtractor extracts .zip archives.
type ZipExtractor struct{}

func (z *ZipExtractor) Format() Format { return FormatZip }

func (z *ZipExtractor) Extract(data []byte, destDir string) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("archive: open zip: %w", err)
	}

	if len(zr.File) > maxEntries {
		return fmt.Errorf("archive: zip has too many entries (%d > %d)", len(zr.File), maxEntries)
	}

	var total int64

	for _, f := range zr.File {
		target, err := pathSlipCheck(destDir, f.Name)
		if err != nil {
			return err
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("archive: create dir %s: %w", f.Name, err)
			}

			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("archive: create parent dir: %w", err)
		}

		if err := extractZipEntry(f, target, &total); err != nil {
			return err
		}
	}

	return nil
}

func extractZipEntry(f *zip.File, target string, total *int64) error {
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("archive: open zip entry %s: %w", f.Name, err)
	}

	defer func() { _ = rc.Close() }()

	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return fmt.Errorf("archive: create file %s: %w", f.Name, err)
	}

	defer func() { _ = out.Close() }()

	// Cap this entry at the smaller of the per-entry limit and the remaining
	// total budget; the +1 sentinel detects overflow.
	remaining := maxTotalBytes - *total
	if remaining > maxEntryBytes {
		remaining = maxEntryBytes
	}

	n, err := io.Copy(out, io.LimitReader(rc, remaining+1))
	if err != nil {
		return fmt.Errorf("archive: write file %s: %w", f.Name, err)
	}

	if n > remaining {
		return fmt.Errorf("archive: zip entry %s exceeds size cap", f.Name)
	}

	*total += n

	return nil
}
