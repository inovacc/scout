package archive

import "testing"

// cpioHeader builds a 110-byte cpio "newc" header (6-byte magic + 13 eight-hex
// fields) with the given namesize/filesize hex fields, all others zero.
func cpioHeader(namesizeHex, filesizeHex string) []byte {
	h := make([]byte, 110)
	copy(h[0:6], "070701")
	for i := 6; i < 110; i += 8 {
		copy(h[i:i+8], "00000000")
	}
	copy(h[54:62], filesizeHex)  // c_filesize
	copy(h[94:102], namesizeHex) // c_namesize
	return h
}

// TestParseCPIONewcRejectsMalformed proves crafted cpio size fields produce a
// clean error instead of a slice-out-of-range panic. Before the bounds checks a
// namesize of 0 sliced data[110:109] (low > high) and an oversized
// namesize/filesize sliced past the buffer — both panic the whole process,
// turning a malformed .rpm into a remote crash.
func TestParseCPIONewcRejectsMalformed(t *testing.T) {
	dir := t.TempDir()

	cases := []struct {
		name               string
		namesize, filesize string
	}{
		{"zero namesize", "00000000", "00000000"},
		{"oversized namesize", "ffffffff", "00000000"},
		{"oversized filesize", "0000000b", "ffffffff"},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			hdr := cpioHeader(c.namesize, c.filesize)

			err := func() (e error) {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("parseCPIONewc panicked on %s: %v", c.name, r)
					}
				}()
				return parseCPIONewc(hdr, dir)
			}()

			if err == nil {
				t.Fatalf("parseCPIONewc(%s) = nil, want a clean error", c.name)
			}
		})
	}
}
