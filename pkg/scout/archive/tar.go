package archive

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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

	var (
		entries int
		total   int64
	)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return fmt.Errorf("archive: read tar entry: %w", err)
		}

		entries++
		if entries > maxEntries {
			return fmt.Errorf("archive: tar has too many entries (>%d)", maxEntries)
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

			if err := writeFromReader(target, tr, hdr.FileInfo().Mode(), &total); err != nil {
				return fmt.Errorf("archive: write file %s: %w", hdr.Name, err)
			}

		case tar.TypeSymlink:
			// Guard the symlink TARGET too: pathSlipCheck only validated the link
			// path. A link whose Linkname escapes destDir enables zip-slip via a
			// later entry writing through it (or the link being followed at install).
			if err := symlinkTargetWithinDest(destDir, target, hdr.Linkname); err != nil {
				return err
			}

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

// symlinkTargetWithinDest rejects a symlink whose resolved target would point
// outside destDir. Linkname is interpreted relative to the link's directory
// (an absolute target is rejected outright), mirroring pathSlipCheck's
// prefix-containment semantics.
func symlinkTargetWithinDest(destDir, linkPath, linkname string) error {
	if filepath.IsAbs(linkname) {
		return fmt.Errorf("archive: refusing absolute symlink target: %s", linkname)
	}

	resolved := filepath.Clean(filepath.Join(filepath.Dir(linkPath), linkname))
	cleanDest := filepath.Clean(destDir) + string(os.PathSeparator)

	if !strings.HasPrefix(resolved, cleanDest) && resolved != filepath.Clean(destDir) {
		return fmt.Errorf("archive: symlink target escapes destination: %s -> %s", linkPath, linkname)
	}

	return nil
}

func writeFromReader(path string, r io.Reader, mode os.FileMode, total *int64) error {
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}

	defer func() { _ = out.Close() }()

	// Cap this entry at the smaller of the per-entry limit and the remaining
	// total budget; the +1 sentinel detects overflow (tar is streamed, so there
	// is no entry count or size known up front).
	remaining := maxTotalBytes - *total
	if remaining > maxEntryBytes {
		remaining = maxEntryBytes
	}

	n, err := io.Copy(out, io.LimitReader(r, remaining+1))
	if err != nil {
		return err
	}

	if n > remaining {
		return fmt.Errorf("archive: tar entry exceeds size cap")
	}

	*total += n

	return nil
}
