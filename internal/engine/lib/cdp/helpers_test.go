package cdp

import (
	"bytes"
	"net"
	"net/url"
	"testing"
	"time"
)

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse url %q: %v", raw, err)
	}
	return u
}

// bytesConn is a minimal, non-blocking net.Conn used to capture frames written
// by WebSocket.send. Writes accumulate in an in-memory buffer; no real socket,
// no goroutine coupling, so framing assertions can never deadlock.
type bytesConn struct {
	written bytes.Buffer
}

var _ net.Conn = (*bytesConn)(nil)

func (c *bytesConn) Write(p []byte) (int, error) { return c.written.Write(p) }
func (c *bytesConn) Read(p []byte) (int, error)  { return c.written.Read(p) }
func (c *bytesConn) Close() error                { return nil }
func (c *bytesConn) LocalAddr() net.Addr         { return fakeAddr{} }
func (c *bytesConn) RemoteAddr() net.Addr        { return fakeAddr{} }
func (c *bytesConn) SetDeadline(time.Time) error      { return nil }
func (c *bytesConn) SetReadDeadline(time.Time) error  { return nil }
func (c *bytesConn) SetWriteDeadline(time.Time) error { return nil }

type fakeAddr struct{}

func (fakeAddr) Network() string { return "fake" }
func (fakeAddr) String() string  { return "fake" }
