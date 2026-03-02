package scout

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func init() {
	registerTestRoutes(func(mux *http.ServeMux) {
		mux.HandleFunc("/hijack-page", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Hijack Test</title></head>
<body><h1>Hijack</h1>
<script>fetch('/hijack-api').then(r => r.json());</script>
</body></html>`)
		})

		mux.HandleFunc("/hijack-api", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"hijacked":true}`)
		})

		mux.HandleFunc("/hijack-ws-page", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			// Build ws URL from the request host.
			wsURL := "ws://" + r.Host + "/hijack-ws"
			_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><title>WS Test</title></head>
<body>
<script>
var ws = new WebSocket(%q);
ws.onopen = function() { ws.send("hello from browser"); };
ws.onmessage = function(e) { document.title = "ws:" + e.data; ws.close(); };
</script>
</body></html>`, wsURL)
		})

		mux.HandleFunc("/hijack-ws", func(w http.ResponseWriter, r *http.Request) {
			// Minimal WebSocket upgrade (RFC 6455).
			if !strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade") {
				http.Error(w, "not a websocket", http.StatusBadRequest)
				return
			}

			key := r.Header.Get("Sec-WebSocket-Key")
			if key == "" {
				http.Error(w, "missing key", http.StatusBadRequest)
				return
			}

			accept := wsAcceptKey(key)

			hj, ok := w.(http.Hijacker)
			if !ok {
				http.Error(w, "hijack unsupported", http.StatusInternalServerError)
				return
			}

			conn, rw, err := hj.Hijack()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			defer func() { _ = conn.Close() }()

			// Write upgrade response.
			_, _ = rw.WriteString("HTTP/1.1 101 Switching Protocols\r\n")
			_, _ = rw.WriteString("Upgrade: websocket\r\n")
			_, _ = rw.WriteString("Connection: Upgrade\r\n")
			_, _ = fmt.Fprintf(rw, "Sec-WebSocket-Accept: %s\r\n\r\n", accept)
			_ = rw.Flush()

			// Read one frame from client (ignore content).
			_ = wsReadFrame(rw.Reader)

			// Send a text frame back.
			wsWriteTextFrame(rw, "echo-reply")
			_ = rw.Flush()

			// Wait briefly so browser can process.
			time.Sleep(100 * time.Millisecond)
		})
	})
}

// wsAcceptKey computes the Sec-WebSocket-Accept value per RFC 6455.
func wsAcceptKey(key string) string {
	const magic = "258EAFA5-E914-47DA-95CA-5AB5DC587D11"

	h := sha1.New()
	_, _ = h.Write([]byte(key + magic))

	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// wsReadFrame reads and discards one WebSocket frame.
func wsReadFrame(r *bufio.Reader) error {
	// Read first 2 bytes: FIN+opcode, MASK+length.
	header := make([]byte, 2)
	if _, err := r.Read(header); err != nil {
		return err
	}

	length := int(header[1] & 0x7F)
	masked := header[1]&0x80 != 0

	switch length {
	case 126:
		ext := make([]byte, 2)
		if _, err := r.Read(ext); err != nil {
			return err
		}

		length = int(binary.BigEndian.Uint16(ext))
	case 127:
		ext := make([]byte, 8)
		if _, err := r.Read(ext); err != nil {
			return err
		}

		length = int(binary.BigEndian.Uint64(ext))
	}

	if masked {
		// Read 4-byte mask key.
		mask := make([]byte, 4)
		if _, err := r.Read(mask); err != nil {
			return err
		}
	}

	// Read payload.
	payload := make([]byte, length)
	if _, err := r.Read(payload); err != nil {
		return err
	}

	return nil
}

// wsWriteTextFrame writes a simple unmasked text frame.
func wsWriteTextFrame(rw *bufio.ReadWriter, text string) {
	data := []byte(text)
	// FIN + text opcode
	_ = rw.WriteByte(0x81)

	if len(data) < 126 {
		_ = rw.WriteByte(byte(len(data)))
	} else {
		_ = rw.WriteByte(126)
		b := make([]byte, 2)
		binary.BigEndian.PutUint16(b, uint16(len(data)))
		_, _ = rw.Write(b)
	}

	_, _ = rw.Write(data)
}

func TestSessionHijackerCapturesHTTP(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage("about:blank")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	hijacker, err := page.NewSessionHijacker()
	if err != nil {
		t.Fatalf("NewSessionHijacker() error: %v", err)
	}
	defer hijacker.Stop()

	if err := page.Navigate(srv.URL + "/hijack-page"); err != nil {
		t.Fatalf("Navigate() error: %v", err)
	}

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	// Collect events with timeout.
	var events []HijackEvent

	timeout := time.After(3 * time.Second)

	for {
		select {
		case ev, ok := <-hijacker.Events():
			if !ok {
				goto done
			}

			events = append(events, ev)
			// We expect at least a request + response for the page and the API fetch.
			if len(events) >= 4 {
				goto done
			}
		case <-timeout:
			goto done
		}
	}

done:

	if len(events) == 0 {
		t.Fatal("expected at least one hijack event")
	}

	var hasRequest, hasResponse bool

	for _, ev := range events {
		switch ev.Type { //nolint:exhaustive
		case HijackEventRequest:
			hasRequest = true

			if ev.Request == nil {
				t.Error("request event has nil Request")
			}
		case HijackEventResponse:
			hasResponse = true

			if ev.Response == nil {
				t.Error("response event has nil Response")
			}
		}
	}

	if !hasRequest {
		t.Error("expected at least one request event")
	}

	if !hasResponse {
		t.Error("expected at least one response event")
	}
}

func TestSessionHijackerURLFilter(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage("about:blank")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	// Only capture requests matching *hijack-api*
	hijacker, err := page.NewSessionHijacker(WithHijackURLFilter("*hijack-api*"))
	if err != nil {
		t.Fatalf("NewSessionHijacker() error: %v", err)
	}
	defer hijacker.Stop()

	if err := page.Navigate(srv.URL + "/hijack-page"); err != nil {
		t.Fatalf("Navigate() error: %v", err)
	}

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	var events []HijackEvent

	timeout := time.After(3 * time.Second)

	for {
		select {
		case ev, ok := <-hijacker.Events():
			if !ok {
				goto done
			}

			events = append(events, ev)
		case <-timeout:
			goto done
		}
	}

done:

	// All captured events should be for the API URL only.
	for _, ev := range events {
		var url string
		if ev.Request != nil {
			url = ev.Request.URL
		} else if ev.Response != nil {
			url = ev.Response.URL
		}

		if url != "" && !strings.Contains(url, "hijack-api") {
			t.Errorf("filter should have excluded URL %q", url)
		}
	}
}

func TestSessionHijackerWebSocket(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage("about:blank")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	hijacker, err := page.NewSessionHijacker()
	if err != nil {
		t.Fatalf("NewSessionHijacker() error: %v", err)
	}
	defer hijacker.Stop()

	if err := page.Navigate(srv.URL + "/hijack-ws-page"); err != nil {
		t.Fatalf("Navigate() error: %v", err)
	}

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	// Wait for title to change (indicates WS exchange completed).
	time.Sleep(2 * time.Second)

	var events []HijackEvent
	// Drain the channel.
drain:
	for {
		select {
		case ev, ok := <-hijacker.Events():
			if !ok {
				break drain
			}

			events = append(events, ev)
		default:
			break drain
		}
	}

	var hasWSOpened, hasWSSent, hasWSReceived, hasWSClosed bool

	for _, ev := range events {
		switch ev.Type { //nolint:exhaustive
		case HijackWSOpened:
			hasWSOpened = true
		case HijackWSSent:
			hasWSSent = true
		case HijackWSReceived:
			hasWSReceived = true
		case HijackWSClosed:
			hasWSClosed = true
		}
	}

	if !hasWSOpened {
		t.Error("expected ws.opened event")
	}

	if !hasWSSent {
		t.Error("expected ws.sent event")
	}

	if !hasWSReceived {
		t.Error("expected ws.received event")
	}

	if !hasWSClosed {
		t.Error("expected ws.closed event")
	}
}

func TestSessionHijackerStopIdempotent(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	hijacker, err := page.NewSessionHijacker()
	if err != nil {
		t.Fatalf("NewSessionHijacker() error: %v", err)
	}

	// Stop multiple times should not panic.
	hijacker.Stop()
	hijacker.Stop()
	hijacker.Stop()
}

func TestSessionHijackerNilSafety(t *testing.T) {
	var hijacker *SessionHijacker

	// Should not panic.
	hijacker.Stop()
}

func TestSessionHijackerNilPage(t *testing.T) {
	var page *Page

	hijacker, err := page.NewSessionHijacker()
	if err == nil {
		t.Error("expected error for nil page")
	}

	if hijacker != nil {
		t.Error("expected nil hijacker for nil page")
	}
}

func TestSessionHijackerBodyCapture(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage("about:blank")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	hijacker, err := page.NewSessionHijacker(WithHijackBodyCapture())
	if err != nil {
		t.Fatalf("NewSessionHijacker() error: %v", err)
	}
	defer hijacker.Stop()

	if err := page.Navigate(srv.URL + "/json"); err != nil {
		t.Fatalf("Navigate() error: %v", err)
	}

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	var events []HijackEvent

	timeout := time.After(3 * time.Second)

	for {
		select {
		case ev, ok := <-hijacker.Events():
			if !ok {
				goto done
			}

			events = append(events, ev)
		case <-timeout:
			goto done
		}
	}

done:

	foundBody := false

	for _, ev := range events {
		if ev.Type == HijackEventResponse && ev.Response != nil && ev.Response.Body != "" {
			foundBody = true
			break
		}
	}

	if !foundBody {
		t.Error("WithHijackBodyCapture should produce response events with non-empty Body")
	}
}

func TestSessionHijackerWithAutoAttach(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b, err := New(
		WithHeadless(true),
		WithNoSandbox(),
		WithTimeout(30e9),
		WithSessionHijack(),
	)
	if err != nil {
		t.Skipf("skipping: browser unavailable: %v", err)
	}

	defer func() { _ = b.Close() }()

	page, err := b.NewPage(srv.URL + "/hijack-page")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	hijacker := page.Hijacker()
	if hijacker == nil {
		t.Fatal("WithSessionHijack() should auto-attach a hijacker")
	}
	defer hijacker.Stop()
}

func TestMatchFilter(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		url      string
		want     bool
	}{
		{"no patterns matches all", nil, "https://example.com/api", true},
		{"contains match", []string{"*api*"}, "https://example.com/api/v1", true},
		{"no match", []string{"*api*"}, "https://example.com/page", false},
		{"multiple patterns", []string{"*api*", "*page*"}, "https://example.com/page", true},
		{"exact glob", []string{"*.json"}, "/data.json", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &SessionHijacker{
				opts: &hijackOptions{
					filter: HijackFilter{URLPatterns: tt.patterns},
				},
			}
			if got := h.matchFilter(tt.url); got != tt.want {
				t.Errorf("matchFilter(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}
