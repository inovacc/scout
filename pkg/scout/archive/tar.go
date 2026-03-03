package archive

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// TarExtractor extracts .tar archives.
type TarExtractor struct{}

func (t *TarExtractor) Format() Format { return FormatTar }

func (t *TarExtractor) Extract(data []byte, destDir string) error {
	return extractTar(newByteReader(data), destDir)
}

// TarGzExtractor extracts .tar.gz / .tgz archives.
type TarGzExtractor struct{}

func (tg *TarGzExtractor) Format() Format { return FormatTarGz }

func (tg *TarGzExtractor) Extract(data []byte, destDir string) error {
	gr, err := gzip.NewReader(newByteReader(data))
	if err != nil {
		return fmt.Errorf("archive: open gzip: %w", err)
	}

	defer func() { _ = gr.Close() }()

	return extractTar(gr, destDir)
}

func extractTar(r io.Reader, destDir string) error {
	tr := tar.NewReader(r)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return fmt.Errorf("archive: read tar entry: %w", err)
		}

		target, err := pathSlipCheck(destDir, hdr.Name)
		if err != nil {
			return err
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("archive: create dir %s: %w", hdr.Name, err)
			}

		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("archive: create parent dir: %w", err)
			}

			if err := writeFromReader(target, tr, hdr.FileInfo().Mode()); err != nil {
				return fmt.Errorf("archive: write file %s: %w", hdr.Name, err)
			}

		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("archive: create parent dir: %w", err)
			}

			_ = os.Remove(target)

			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return fmt.Errorf("archive: symlink %s: %w", hdr.Name, err)
			}
		}
	}

	return nil
}

func writeFromReader(path string, r io.Reader, mode os.FileMode) error {
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}

	defer func() { _ = out.Close() }()

	_, err = io.Copy(out, r)

	return err
}
