package cdp

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"net"
	"net/http"
	"testing"
)

func TestBadHandshakeError(t *testing.T) {
	e := &BadHandshakeError{Status: "403 Forbidden", Body: "denied"}
	want := "websocket bad handshake: 403 Forbidden. denied"
	if got := e.Error(); got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}

	empty := &BadHandshakeError{}
	if got := empty.Error(); got != "websocket bad handshake: . " {
		t.Fatalf("Error() empty = %q", got)
	}
}

func computeAccept(key string) string {
	h := sha1.New()
	h.Write([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func TestVerifyWebSocketAccept(t *testing.T) {
	const key = "dGhlIHNhbXBsZSBub25jZQ=="
	valid := computeAccept(key)

	tests := []struct {
		name   string
		accept string
		key    string
		want   bool
	}{
		{name: "correct accept value", accept: valid, key: key, want: true},
		{name: "wrong accept value", accept: "bogus", key: key, want: false},
		{name: "empty accept header", accept: "", key: key, want: false},
		{name: "accept for different key", accept: computeAccept("other"), key: key, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hdr := http.Header{}
			if tt.accept != "" {
				hdr.Set("Sec-WebSocket-Accept", tt.accept)
			}
			if got := verifyWebSocketAccept(hdr, tt.key); got != tt.want {
				t.Fatalf("verifyWebSocketAccept = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInitDialer(t *testing.T) {
	t.Run("ws scheme uses net.Dialer", func(t *testing.T) {
		ws := &WebSocket{}
		u := mustParseURL(t, "ws://127.0.0.1:1234/devtools")
		ws.initDialer(u)
		if _, ok := ws.Dialer.(*net.Dialer); !ok {
			t.Fatalf("expected *net.Dialer for ws scheme, got %T", ws.Dialer)
		}
		if u.Host != "127.0.0.1:1234" {
			t.Fatalf("host should be unchanged for ws, got %q", u.Host)
		}
	})

	t.Run("wss scheme uses tlsDialer and adds default port", func(t *testing.T) {
		ws := &WebSocket{}
		u := mustParseURL(t, "wss://example.com/devtools")
		ws.initDialer(u)
		if _, ok := ws.Dialer.(*tlsDialer); !ok {
			t.Fatalf("expected *tlsDialer for wss scheme, got %T", ws.Dialer)
		}
		if u.Host != "example.com:443" {
			t.Fatalf("expected :443 appended for wss without port, got %q", u.Host)
		}
	})

	t.Run("wss scheme keeps explicit port", func(t *testing.T) {
		ws := &WebSocket{}
		u := mustParseURL(t, "wss://example.com:8443/x")
		ws.initDialer(u)
		if u.Host != "example.com:8443" {
			t.Fatalf("explicit port must be preserved, got %q", u.Host)
		}
	})

	t.Run("existing dialer is not replaced", func(t *testing.T) {
		ws := &WebSocket{}
		sentinel := &net.Dialer{}
		ws.Dialer = sentinel
		u := mustParseURL(t, "wss://example.com/x")
		ws.initDialer(u)
		if ws.Dialer != sentinel {
			t.Fatalf("existing dialer must not be replaced")
		}
		if u.Host != "example.com" {
			t.Fatalf("host must not be mutated when dialer preset, got %q", u.Host)
		}
	})
}

// TestSendFraming verifies the exact bytes written for small, 16-bit, and 64-bit
// length-encoded frames, including the FIN+text opcode, mask flag, the 4-byte
// masking key {0,1,2,3}, and that the payload is XOR-masked with that key.
//
// The frame layout produced by send is:
//
//	byte0:        FIN(1)+opcode text = 0x81
//	byte1:        MASK bit (0x80) | length-or-marker
//	[extLen]:     0, 2 or 8 bytes of extended length (big-endian)
//	[mask key]:   4 bytes {0,1,2,3}
//	[payload]:    XOR-masked bytes
//
// A bytesConn captures the write in memory so there is no blocking I/O.
func TestSendFraming(t *testing.T) {
	mask := []byte{0, 1, 2, 3}

	maskPayload := func(p []byte) []byte {
		out := make([]byte, len(p))
		for i := range p {
			out[i] = p[i] ^ mask[i%4]
		}
		return out
	}

	tests := []struct {
		name       string
		size       int
		wantLenIdx []byte // expected extended length bytes (after the first 2 header bytes)
		wantLen0   byte   // expected low 7 bits of header[1]
	}{
		{name: "small frame <=125", size: 5, wantLen0: 5},
		{name: "boundary 125", size: 125, wantLen0: 125},
		{name: "16-bit length 126", size: 126, wantLen0: 126, wantLenIdx: []byte{0x00, 0x7e}},
		{name: "16-bit max 65535", size: 65535, wantLen0: 126, wantLenIdx: []byte{0xff, 0xff}},
		{name: "64-bit length 65536", size: 65536, wantLen0: 127, wantLenIdx: []byte{0, 0, 0, 0, 0, 1, 0, 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := &bytesConn{}
			ws := &WebSocket{conn: conn}

			payload := bytes.Repeat([]byte("A"), tt.size)
			// send mutates the payload in place (masking), so capture expected first.
			expectedMasked := maskPayload(bytes.Repeat([]byte("A"), tt.size))

			if err := ws.send(payload); err != nil {
				t.Fatalf("send error: %v", err)
			}

			got := conn.written.Bytes()
			wantTotal := 2 + len(tt.wantLenIdx) + 4 + tt.size
			if len(got) != wantTotal {
				t.Fatalf("frame length = %d, want %d", len(got), wantTotal)
			}

			if got[0] != 0b1000_0001 {
				t.Fatalf("byte0 = %08b, want FIN+text 10000001", got[0])
			}
			if got[1]&0x80 == 0 {
				t.Fatalf("mask bit not set in byte1 = %08b", got[1])
			}
			if got[1]&0x7f != tt.wantLen0 {
				t.Fatalf("len field = %d, want %d", got[1]&0x7f, tt.wantLen0)
			}

			off := 2
			if len(tt.wantLenIdx) > 0 {
				gotLen := got[off : off+len(tt.wantLenIdx)]
				if !bytes.Equal(gotLen, tt.wantLenIdx) {
					t.Fatalf("extended len bytes = % x, want % x", gotLen, tt.wantLenIdx)
				}
				off += len(tt.wantLenIdx)
			}

			gotMaskKey := got[off : off+4]
			if !bytes.Equal(gotMaskKey, mask) {
				t.Fatalf("mask key = % x, want % x", gotMaskKey, mask)
			}
			off += 4

			gotPayload := got[off:]
			if !bytes.Equal(gotPayload, expectedMasked) {
				t.Fatalf("payload not masked as expected (size=%d)", tt.size)
			}
		})
	}
}

// TestReadFraming verifies read() decodes the payload length from 7-bit, 16-bit,
// and 64-bit forms and returns exactly the declared number of bytes.
func TestReadFraming(t *testing.T) {
	tests := []struct {
		name  string
		frame []byte
		want  []byte
	}{
		{
			name:  "7-bit length",
			frame: append([]byte{0x81, 0x03}, []byte("abc")...),
			want:  []byte("abc"),
		},
		{
			name:  "mask bit is ignored on read",
			frame: append([]byte{0x81, 0x83}, []byte("xyz")...), // 0x80 mask bit set, low7=3
			want:  []byte("xyz"),
		},
		{
			name:  "16-bit length",
			frame: append([]byte{0x81, 126, 0x00, 0x04}, []byte("data")...),
			want:  []byte("data"),
		},
		{
			name:  "64-bit length",
			frame: append([]byte{0x81, 127, 0, 0, 0, 0, 0, 0, 0, 0x02}, []byte("hi")...),
			want:  []byte("hi"),
		},
		{
			name:  "zero-length payload",
			frame: []byte{0x81, 0x00},
			want:  []byte{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws := &WebSocket{r: bufio.NewReader(bytes.NewReader(tt.frame))}
			got, err := ws.read()
			if err != nil {
				t.Fatalf("read error: %v", err)
			}
			if !bytes.Equal(got, tt.want) {
				t.Fatalf("read() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReadTruncatedReturnsError(t *testing.T) {
	// Declares 10 bytes but only supplies 3 -> io.ReadFull should error.
	frame := append([]byte{0x81, 0x0a}, []byte("abc")...)
	ws := &WebSocket{r: bufio.NewReader(bytes.NewReader(frame))}
	if _, err := ws.read(); err == nil {
		t.Fatal("expected error for truncated payload, got nil")
	}
}

func TestReadEmptyReaderReturnsError(t *testing.T) {
	ws := &WebSocket{r: bufio.NewReader(bytes.NewReader(nil))}
	if _, err := ws.read(); err == nil {
		t.Fatal("expected error reading from empty stream, got nil")
	}
}
