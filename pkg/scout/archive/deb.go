package archive

import (
	"bytes"
	"fmt"
	"io"
)

// DebExtractor extracts .deb archives.
// A .deb is an ar(1) archive containing control.tar.* and data.tar.*.
// We extract only the data.tar.* member.
type DebExtractor struct{}

func (d *DebExtractor) Format() Format { return FormatDeb }

func (d *DebExtractor) Extract(data []byte, destDir string) error {
	dataTar, err := extractDebDataTar(data)
	if err != nil {
		return err
	}

	tgz := &TarGzExtractor{}

	return tgz.Extract(dataTar, destDir)
}

// extractDebDataTar parses the ar archive and returns the data.tar.gz payload.
func extractDebDataTar(data []byte) ([]byte, error) {
	r := bytes.NewReader(data)

	// ar magic: "!<arch>\n"
	magic := make([]byte, 8)
	if _, err := io.ReadFull(r, magic); err != nil {
		return nil, fmt.Errorf("archive: read ar magic: %w", err)
	}

	if string(magic) != "!<arch>\n" {
		return nil, fmt.Errorf("archive: not an ar archive")
	}

	for {
		// ar header: 60 bytes
		hdr := make([]byte, 60)
		if _, err := io.ReadFull(r, hdr); err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}

			return nil, fmt.Errorf("archive: read ar header: %w", err)
		}

		name := trimRight(string(hdr[0:16]))
		sizeStr := trimRight(string(hdr[48:58]))

		var size int64

		if _, err := fmt.Sscanf(sizeStr, "%d", &size); err != nil {
			return nil, fmt.Errorf("archive: parse ar entry size %q: %w", sizeStr, err)
		}

		if isDataTar(name) {
			payload := make([]byte, size)
			if _, err := io.ReadFull(r, payload); err != nil {
				return nil, fmt.Errorf("archive: read data.tar: %w", err)
			}

			return payload, nil
		}

		// Skip this entry (pad to even boundary).
		skip := size
		if skip%2 != 0 {
			skip++
		}

		if _, err := r.Seek(skip, io.SeekCurrent); err != nil {
			return nil, fmt.Errorf("archive: skip ar entry: %w", err)
		}
	}

	return nil, fmt.Errorf("archive: data.tar.* not found in deb")
}

func isDataTar(name string) bool {
	return name == "data.tar.gz" || name == "data.tar.xz" ||
		name == "data.tar.zst" || name == "data.tar.bz2" ||
		name == "data.tar"
}

func trimRight(s string) string {
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '/') {
		s = s[:len(s)-1]
	}

	return s
}
