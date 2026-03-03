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

		if err := extractZipEntry(f, target); err != nil {
			return err
		}
	}

	return nil
}

func extractZipEntry(f *zip.File, target string) error {
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

	if _, err := io.Copy(out, rc); err != nil {
		return fmt.Errorf("archive: write file %s: %w", f.Name, err)
	}

	return nil
}
