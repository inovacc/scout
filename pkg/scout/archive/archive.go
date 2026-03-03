// Package archive provides a unified interface for extracting browser archives
// in multiple formats (zip, tar.gz, deb, rpm) into the scout browser cache.
package archive

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Format identifies an archive format.
type Format string

const (
	FormatZip   Format = "zip"
	FormatTarGz Format = "tar.gz"
	FormatTar   Format = "tar"
	FormatDeb   Format = "deb"
	FormatRPM   Format = "rpm"
)

// Extractor extracts an archive into a destination directory.
type Extractor interface {
	// Extract unpacks data into destDir.
	Extract(data []byte, destDir string) error
	// Format returns the archive format this extractor handles.
	Format() Format
}

// Extract auto-detects the format from filename and extracts data into destDir.
func Extract(data []byte, filename, destDir string) error {
	ext := DetectFormat(filename)
	if ext == "" {
		return fmt.Errorf("archive: unsupported format: %s", filename)
	}

	e := ForExtractor(ext)
	if e == nil {
		return fmt.Errorf("archive: no extractor for format: %s", ext)
	}

	return e.Extract(data, destDir)
}

// DetectFormat returns the Format for a given filename.
func DetectFormat(filename string) Format {
	lower := strings.ToLower(filename)

	switch {
	case strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz"):
		return FormatTarGz
	case strings.HasSuffix(lower, ".tar"):
		return FormatTar
	case strings.HasSuffix(lower, ".zip"):
		return FormatZip
	case strings.HasSuffix(lower, ".deb"):
		return FormatDeb
	case strings.HasSuffix(lower, ".rpm"):
		return FormatRPM
	default:
		return ""
	}
}

// ForExtractor returns the appropriate Extractor for a Format.
func ForExtractor(f Format) Extractor {
	switch f {
	case FormatZip:
		return &ZipExtractor{}
	case FormatTarGz:
		return &TarGzExtractor{}
	case FormatTar:
		return &TarExtractor{}
	case FormatDeb:
		return &DebExtractor{}
	case FormatRPM:
		return &RPMExtractor{}
	default:
		return nil
	}
}

// pathSlipCheck verifies that target stays within destDir (zip-slip protection).
func pathSlipCheck(destDir, entryName string) (string, error) {
	target := filepath.Join(destDir, entryName) //nolint:gosec // trusted archives from known sources

	cleanDest := filepath.Clean(destDir) + string(os.PathSeparator)
	cleanTarget := filepath.Clean(target)

	if !strings.HasPrefix(cleanTarget, cleanDest) && cleanTarget != filepath.Clean(destDir) {
		return "", fmt.Errorf("archive: path traversal detected: %s", entryName)
	}

	return target, nil
}

// writeFile writes data to a file, creating parent directories as needed.
func writeFile(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("archive: create parent dir: %w", err)
	}

	return os.WriteFile(path, data, mode)
}
