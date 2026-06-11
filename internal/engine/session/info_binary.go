package session

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

// Binary scout.pid format v1 (432 bytes, little-endian).
//
// Replaces the legacy JSON-encoded SessionInfo. Fixed-width layout enables
// zero-allocation reads via direct byte slicing and supports cross-process
// LockFileEx/flock advisory locking on the same file (atomic-rename writes
// invalidate file-handle locks, so we write in place under exclusive lock).
//
// Layout — see docs/superpowers/specs/scout-pid-binary.md for the table.
const (
	binMagicStr = "SCT1"
	binVersion  = uint16(1)
	binSize     = 432

	binMagicOff = 0
	binMagicLen = 4

	binVerOff   = 4
	binFlagsOff = 6

	binScoutPIDOff   = 8
	binBrowserPIDOff = 12
	binParentPIDOff  = 16
	binReservedOff   = 20

	binCreatedAtOff = 24
	binLastUsedOff  = 32
	binExpiresAtOff = 40

	binBrowserOff = 48
	binBrowserCap = 32

	binDomainHashOff = 80
	binDomainHashCap = 32

	binDomainOff = 112
	binDomainCap = 64

	binExecOff = 176
	binExecCap = 128

	binBuildVerOff = 304
	binBuildVerCap = 64

	binTokenOff = 368
	binTokenCap = 64

	flagReusable uint16 = 1 << 0
	flagHeadless uint16 = 1 << 1
)

// ErrLegacyFormat is returned by unmarshalBinary when the file does not begin
// with the v1 magic. Callers (startup purge) use this to identify and remove
// legacy JSON-format scout.pid files.
var ErrLegacyFormat = errors.New("scout: legacy scout.pid format")

// ErrCorruptInfo is returned when the magic matches but the body is malformed
// (wrong version, truncated, etc.).
var ErrCorruptInfo = errors.New("scout: corrupt scout.pid")

// marshalBinary encodes a SessionInfo into the v1 fixed-width layout.
// Strings exceeding their field cap are silently truncated at the byte level
// (callers should keep paths and tokens within the documented limits).
func marshalBinary(info *SessionInfo) []byte {
	buf := make([]byte, binSize)
	copy(buf[binMagicOff:binMagicOff+binMagicLen], binMagicStr)
	binary.LittleEndian.PutUint16(buf[binVerOff:], binVersion)

	var flags uint16
	if info.Reusable {
		flags |= flagReusable
	}
	if info.Headless {
		flags |= flagHeadless
	}
	binary.LittleEndian.PutUint16(buf[binFlagsOff:], flags)

	binary.LittleEndian.PutUint32(buf[binScoutPIDOff:], uint32(info.ScoutPID))
	binary.LittleEndian.PutUint32(buf[binBrowserPIDOff:], uint32(info.BrowserPID))
	binary.LittleEndian.PutUint32(buf[binParentPIDOff:], uint32(info.BrowserParentPID))

	binary.LittleEndian.PutUint64(buf[binCreatedAtOff:], uint64(toNano(info.CreatedAt)))
	binary.LittleEndian.PutUint64(buf[binLastUsedOff:], uint64(toNano(info.LastUsed)))
	binary.LittleEndian.PutUint64(buf[binExpiresAtOff:], uint64(toNano(info.ExpiresAt)))

	writeStr(buf, binBrowserOff, binBrowserCap, info.Browser)
	writeStr(buf, binDomainHashOff, binDomainHashCap, info.DomainHash)
	writeStr(buf, binDomainOff, binDomainCap, info.Domain)
	writeStr(buf, binExecOff, binExecCap, info.Exec)
	writeStr(buf, binBuildVerOff, binBuildVerCap, info.BuildVersion)
	writeStr(buf, binTokenOff, binTokenCap, info.BrowserStartToken)

	return buf
}

// unmarshalBinary decodes a v1 scout.pid into SessionInfo. Returns
// ErrLegacyFormat for non-magic input so the startup purge can detect JSON
// remnants and remove them.
func unmarshalBinary(data []byte) (*SessionInfo, error) {
	if len(data) < binMagicLen || string(data[binMagicOff:binMagicOff+binMagicLen]) != binMagicStr {
		return nil, ErrLegacyFormat
	}
	if len(data) < binSize {
		return nil, fmt.Errorf("%w: short read %d/%d", ErrCorruptInfo, len(data), binSize)
	}

	ver := binary.LittleEndian.Uint16(data[binVerOff:])
	if ver != binVersion {
		return nil, fmt.Errorf("%w: version %d unsupported", ErrCorruptInfo, ver)
	}

	flags := binary.LittleEndian.Uint16(data[binFlagsOff:])

	info := &SessionInfo{
		ScoutPID:          int(int32(binary.LittleEndian.Uint32(data[binScoutPIDOff:]))),
		BrowserPID:        int(int32(binary.LittleEndian.Uint32(data[binBrowserPIDOff:]))),
		BrowserParentPID:  int(int32(binary.LittleEndian.Uint32(data[binParentPIDOff:]))),
		Reusable:          flags&flagReusable != 0,
		Headless:          flags&flagHeadless != 0,
		CreatedAt:         fromNano(int64(binary.LittleEndian.Uint64(data[binCreatedAtOff:]))),
		LastUsed:          fromNano(int64(binary.LittleEndian.Uint64(data[binLastUsedOff:]))),
		ExpiresAt:         fromNano(int64(binary.LittleEndian.Uint64(data[binExpiresAtOff:]))),
		Browser:           readStr(data, binBrowserOff, binBrowserCap),
		DomainHash:        readStr(data, binDomainHashOff, binDomainHashCap),
		Domain:            readStr(data, binDomainOff, binDomainCap),
		Exec:              readStr(data, binExecOff, binExecCap),
		BuildVersion:      readStr(data, binBuildVerOff, binBuildVerCap),
		BrowserStartToken: readStr(data, binTokenOff, binTokenCap),
	}

	return info, nil
}

func writeStr(buf []byte, off, capacity int, s string) {
	// Zero the field first so a shorter overwrite doesn't leave stale tail bytes.
	for i := range capacity {
		buf[off+i] = 0
	}
	b := []byte(s)
	if len(b) > capacity {
		b = b[:capacity]
	}
	copy(buf[off:off+capacity], b)
}

func readStr(buf []byte, off, capacity int) string {
	field := buf[off : off+capacity]
	end := bytes.IndexByte(field, 0)
	if end < 0 {
		end = capacity
	}
	return string(field[:end])
}

// toNano returns the time as Unix nanoseconds, or 0 for the zero value so the
// round-trip preserves "not set" semantics.
func toNano(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixNano()
}

// fromNano converts Unix nanoseconds back to a time.Time, treating 0 as the
// zero value (matches toNano's "not set" encoding).
func fromNano(n int64) time.Time {
	if n == 0 {
		return time.Time{}
	}
	return time.Unix(0, n)
}
