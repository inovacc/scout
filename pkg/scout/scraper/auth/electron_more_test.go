package auth

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestGetDebuggerWebSocketURL_Success verifies the happy path: a well-formed
// /json/version document yields the embedded webSocketDebuggerUrl. The handler
// is served by httptest (no real network, no browser).
func TestGetDebuggerWebSocketURL_Success(t *testing.T) {
	const wantWS = "ws://127.0.0.1:9222/devtools/browser/abc-123"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/json/version" {
			t.Errorf("requested path = %q, want /json/version", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Browser":"Chrome/120","webSocketDebuggerUrl":"` + wantWS + `"}`))
	}))
	defer srv.Close()

	got, err := getDebuggerWebSocketURL(srv.URL)
	if err != nil {
		t.Fatalf("getDebuggerWebSocketURL() error = %v", err)
	}
	if got != wantWS {
		t.Errorf("ws url = %q, want %q", got, wantWS)
	}
}

// TestGetDebuggerWebSocketURL_ErrorPaths is table-driven over the documented
// failure modes that do not require a browser: a JSON document missing the
// websocket field, a body that is not valid JSON, and an empty body.
func TestGetDebuggerWebSocketURL_ErrorPaths(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantErrSub string
	}{
		{
			name:       "missing websocket field",
			body:       `{"Browser":"Chrome/120"}`,
			wantErrSub: "no WebSocket URL",
		},
		{
			name:       "empty websocket field",
			body:       `{"webSocketDebuggerUrl":""}`,
			wantErrSub: "no WebSocket URL",
		},
		{
			name:       "not json",
			body:       `<<<not-json>>>`,
			wantErrSub: "", // decode error message is not pinned; any non-nil error is fine
		},
		{
			name:       "empty body",
			body:       ``,
			wantErrSub: "", // EOF from the JSON decoder
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			got, err := getDebuggerWebSocketURL(srv.URL)
			if err == nil {
				t.Fatalf("expected error, got ws url = %q", got)
			}
			if got != "" {
				t.Errorf("ws url = %q, want empty on error", got)
			}
			if tt.wantErrSub != "" && !strings.Contains(err.Error(), tt.wantErrSub) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErrSub)
			}
		})
	}
}

// TestGetDebuggerWebSocketURL_ConnRefused pins the transport-error branch: when
// nothing is listening, http.Get returns an error which is propagated verbatim.
// We obtain a definitely-closed port by opening then immediately closing a
// listener, so no real external network is touched.
func TestGetDebuggerWebSocketURL_ConnRefused(t *testing.T) {
	addr := closedLoopbackAddr(t)

	got, err := getDebuggerWebSocketURL("http://" + addr)
	if err == nil {
		t.Fatalf("expected connection error, got ws url = %q", got)
	}
	if got != "" {
		t.Errorf("ws url = %q, want empty on error", got)
	}
}

// TestElectronSession_MissingPort verifies the early guard: a zero DebugPort is
// rejected before any network or browser work, with an actionable message.
func TestElectronSession_MissingPort(t *testing.T) {
	_, err := ElectronSession(context.Background(), ElectronOptions{})
	if err == nil {
		t.Fatal("expected error for missing debug port")
	}
	if !strings.Contains(err.Error(), "debug port is required") {
		t.Errorf("error = %q, want to mention 'debug port is required'", err.Error())
	}
	if !strings.Contains(err.Error(), "auth: electron:") {
		t.Errorf("error = %q, want 'auth: electron:' prefix", err.Error())
	}
}

// TestElectronSession_ContextCancelled verifies that a cancelled context short
// circuits the discovery loop and returns the context error wrapped as
// "auth: electron: ...". The debug port points at a closed loopback port so the
// first getDebuggerWebSocketURL fails immediately and the ctx.Done() branch is
// taken — engine.New is never reached (no browser).
func TestElectronSession_ContextCancelled(t *testing.T) {
	port := closedLoopbackPort(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already done before the call

	done := make(chan struct{})
	var (
		gotErr error
	)
	go func() {
		_, gotErr = ElectronSession(ctx, ElectronOptions{
			DebugPort: port,
			Timeout:   5 * time.Second, // generous; the cancelled ctx must end it first
		})
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("ElectronSession did not honor a cancelled context promptly")
	}

	if gotErr == nil {
		t.Fatal("expected error from cancelled context")
	}
	if !strings.Contains(gotErr.Error(), "auth: electron:") {
		t.Errorf("error = %q, want 'auth: electron:' prefix", gotErr.Error())
	}
	if !strings.Contains(gotErr.Error(), "context canceled") {
		t.Errorf("error = %q, want to wrap 'context canceled'", gotErr.Error())
	}
}

// TestElectronSession_ConnectTimeout verifies the deadline branch: with a tiny
// timeout and a port where no debug endpoint answers, ElectronSession exhausts
// its deadline and returns the "could not connect" error without ever launching
// a browser.
func TestElectronSession_ConnectTimeout(t *testing.T) {
	port := closedLoopbackPort(t)

	start := time.Now()
	_, err := ElectronSession(context.Background(), ElectronOptions{
		DebugPort: port,
		Timeout:   200 * time.Millisecond,
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "could not connect to debug port") {
		t.Errorf("error = %q, want to mention 'could not connect to debug port'", err.Error())
	}
	// Sanity: it should not hang well past the configured timeout.
	if elapsed > 5*time.Second {
		t.Errorf("took %v, far longer than the 200ms timeout", elapsed)
	}
}

// closedLoopbackAddr binds a loopback TCP port, closes it, and returns the
// host:port string. The port is then (almost certainly) refusing connections,
// giving us a deterministic connection error without touching real network.
func closedLoopbackAddr(t *testing.T) string {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := l.Addr().String()
	if err := l.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}

	return addr
}

// closedLoopbackPort is closedLoopbackAddr reduced to just the integer port.
func closedLoopbackPort(t *testing.T) int {
	t.Helper()

	addr := closedLoopbackAddr(t)
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split host port: %v", err)
	}

	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		t.Fatalf("resolve %q: %v", portStr, err)
	}

	return tcpAddr.Port
}
