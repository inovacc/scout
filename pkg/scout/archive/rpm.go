package archive

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// RPMExtractor extracts .rpm archives.
// An RPM contains a lead, signature header, main header, then a cpio archive
// (typically gzip-compressed). We decompress the payload and parse the cpio entries.
type RPMExtractor struct{}

func (r *RPMExtractor) Format() Format { return FormatRPM }

func (r *RPMExtractor) Extract(data []byte, destDir string) error {
	payload, err := extractRPMPayload(data)
	if err != nil {
		return err
	}

	return extractCPIO(payload, destDir)
}

// extractRPMPayload skips the RPM lead + headers and returns the compressed payload.
func extractRPMPayload(data []byte) ([]byte, error) {
	r := bytes.NewReader(data)

	// RPM lead: 96 bytes, magic 0xEDABEEDB.
	lead := make([]byte, 96)
	if _, err := io.ReadFull(r, lead); err != nil {
		return nil, fmt.Errorf("archive: read rpm lead: %w", err)
	}

	if binary.BigEndian.Uint32(lead[0:4]) != 0xEDABEEDB {
		return nil, fmt.Errorf("archive: not an RPM file")
	}

	// Skip signature header.
	if err := skipRPMHeader(r); err != nil {
		return nil, fmt.Errorf("archive: skip rpm signature: %w", err)
	}

	// Align to 8-byte boundary after signature.
	pos, _ := r.Seek(0, io.SeekCurrent)
	if pad := pos % 8; pad != 0 {
		if _, err := r.Seek(8-pad, io.SeekCurrent); err != nil {
			return nil, fmt.Errorf("archive: align rpm signature: %w", err)
		}
	}

	// Skip main header.
	if err := skipRPMHeader(r); err != nil {
		return nil, fmt.Errorf("archive: skip rpm header: %w", err)
	}

	// Rest is the compressed payload.
	payload, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("archive: read rpm payload: %w", err)
	}

	return payload, nil
}

// skipRPMHeader reads an RPM header structure (magic + index + store).
func skipRPMHeader(r io.ReadSeeker) error {
	// Header magic: 3 bytes (8E AD E8) + 1 byte version + 4 reserved + 4 nindex + 4 hsize = 16 bytes.
	hdr := make([]byte, 16)
	if _, err := io.ReadFull(r, hdr); err != nil {
		return err
	}

	if hdr[0] != 0x8E || hdr[1] != 0xAD || hdr[2] != 0xE8 {
		return fmt.Errorf("bad rpm header magic")
	}

	nindex := binary.BigEndian.Uint32(hdr[8:12])
	hsize := binary.BigEndian.Uint32(hdr[12:16])

	// Skip index entries (16 bytes each) + data store.
	skip := int64(nindex)*16 + int64(hsize)
	if _, err := r.Seek(skip, io.SeekCurrent); err != nil {
		return err
	}

	return nil
}

// extractCPIO parses a gzip-compressed cpio (newc format) archive.
func extractCPIO(data []byte, destDir string) error {
	gr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("archive: open rpm gzip payload: %w", err)
	}

	defer func() { _ = gr.Close() }()

	cpioData, err := io.ReadAll(gr)
	if err != nil {
		return fmt.Errorf("archive: decompress rpm payload: %w", err)
	}

	return parseCPIONewc(cpioData, destDir)
}

// parseCPIONewc parses cpio "newc" (SVR4) format.
func parseCPIONewc(data []byte, destDir string) error {
	offset := 0

	for offset+110 <= len(data) {
		// newc header: "070701" magic + fields in hex ASCII.
		magic := string(data[offset : offset+6])
		if magic != "070701" && magic != "070702" {
			return fmt.Errorf("archive: bad cpio magic at offset %d: %s", offset, magic)
		}

		filesize := parseHex(data[offset+54 : offset+62])
		namesize := parseHex(data[offset+94 : offset+102])
		mode := parseHex(data[offset+14 : offset+22])

		// Name starts at offset+110, padded to 4 bytes.
		nameStart := offset + 110
		nameEnd := nameStart + int(namesize) - 1 // exclude null terminator
		name := string(data[nameStart : nameEnd])

		if name == "TRAILER!!!" {
			break
		}

		// Data starts after name, padded to 4 bytes.
		dataStart := nameStart + int(namesize)
		if dataStart%4 != 0 {
			dataStart += 4 - (dataStart % 4)
		}

		dataEnd := dataStart + int(filesize)

		// Strip leading "./" or "/" from name.
		cleanName := name
		for len(cleanName) > 0 && (cleanName[0] == '.' || cleanName[0] == '/') {
			cleanName = cleanName[1:]
		}

		if cleanName != "" {
			target, err := pathSlipCheck(destDir, cleanName)
			if err != nil {
				return err
			}

			isDir := (mode & 0o170000) == 0o040000

			if isDir {
				if err := os.MkdirAll(target, 0o755); err != nil {
					return fmt.Errorf("archive: create dir %s: %w", cleanName, err)
				}
			} else if filesize > 0 {
				fmode := os.FileMode(mode & 0o777)
				if err := writeFile(target, data[dataStart:dataEnd], fmode); err != nil {
					return fmt.Errorf("archive: write %s: %w", cleanName, err)
				}
			}
		}

		// Next entry, padded to 4 bytes.
		offset = dataEnd
		if offset%4 != 0 {
			offset += 4 - (offset % 4)
		}
	}

	return nil
}

func parseHex(b []byte) int64 {
	var n int64

	for _, c := range b {
		n <<= 4

		switch {
		case c >= '0' && c <= '9':
			n |= int64(c - '0')
		case c >= 'a' && c <= 'f':
			n |= int64(c-'a') + 10
		case c >= 'A' && c <= 'F':
			n |= int64(c-'A') + 10
		}
	}

	return n
}

// newByteReader creates a bytes.Reader (helper used across extractors).
func newByteReader(data []byte) *bytes.Reader {
	return bytes.NewReader(data)
}
