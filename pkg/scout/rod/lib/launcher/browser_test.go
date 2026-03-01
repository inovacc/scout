package launcher

import (
	"fmt"
	"net"
	"net/url"
	"sync"
	"testing"
	"time"
)

func TestLockPort(t *testing.T) {
	// Find a free port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	// lockPort should acquire the port.
	unlock := lockPort(port)

	// Verify port is occupied.
	_, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err == nil {
		t.Fatal("expected port to be locked")
	}

	// Release and verify.
	unlock()

	ln2, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatalf("expected port to be free after unlock: %v", err)
	}
	_ = ln2.Close()
}

func TestLockPortConcurrent(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	var mu sync.Mutex
	var order []int

	var wg sync.WaitGroup
	wg.Add(2)

	// First goroutine acquires lock.
	go func() {
		defer wg.Done()
		unlock := lockPort(port)
		mu.Lock()
		order = append(order, 1)
		mu.Unlock()
		time.Sleep(200 * time.Millisecond)
		unlock()
	}()

	// Small delay to ensure goroutine 1 gets the lock first.
	time.Sleep(50 * time.Millisecond)

	// Second goroutine waits for lock.
	go func() {
		defer wg.Done()
		unlock := lockPort(port)
		mu.Lock()
		order = append(order, 2)
		mu.Unlock()
		unlock()
	}()

	wg.Wait()

	if len(order) != 2 || order[0] != 1 || order[1] != 2 {
		t.Fatalf("expected order [1, 2], got %v", order)
	}
}

func TestNewBrowserDefaults(t *testing.T) {
	b := NewBrowser()
	if b.Revision != RevisionDefault {
		t.Fatalf("expected revision %d, got %d", RevisionDefault, b.Revision)
	}
	if len(b.Hosts) != 3 {
		t.Fatalf("expected 3 hosts, got %d", len(b.Hosts))
	}
	if b.LockPort == 0 {
		t.Fatal("expected non-zero lock port")
	}
}

func TestBrowserDir(t *testing.T) {
	b := NewBrowser()
	dir := b.Dir()
	expected := fmt.Sprintf("chromium-%d", RevisionDefault)
	if len(dir) == 0 {
		t.Fatal("expected non-empty dir")
	}
	if !contains(dir, expected) {
		t.Fatalf("expected dir to contain %q, got %q", expected, dir)
	}
}

func TestBrowserBinPath(t *testing.T) {
	b := NewBrowser()
	p := b.BinPath()
	if p == "" {
		t.Fatal("expected non-empty bin path")
	}
}

func TestURLParserWrite(t *testing.T) {
	p := NewURLParser()

	go func() {
		_, _ = p.Write([]byte("DevTools listening on ws://127.0.0.1:9222/devtools/browser/abc\n"))
	}()

	select {
	case u := <-p.URL:
		if u != "http://127.0.0.1:9222" {
			t.Fatalf("expected http://127.0.0.1:9222, got %q", u)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for URL")
	}
}

func TestURLParserErr(t *testing.T) {
	p := NewURLParser()
	_, _ = p.Write([]byte("some error output"))

	err := p.Err()
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestToHTTPAndToWS(t *testing.T) {
	tests := []struct {
		input  string
		toHTTP string
		toWS   string
	}{
		{"ws://host:9222", "http://host:9222", "ws://host:9222"},
		{"wss://host:9222", "https://host:9222", "wss://host:9222"},
		{"http://host:9222", "http://host:9222", "ws://host:9222"},
		{"https://host:9222", "https://host:9222", "wss://host:9222"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			u := mustParseURL(tt.input)
			h := toHTTP(*u)
			if h.String() != tt.toHTTP {
				t.Fatalf("toHTTP(%s) = %s, want %s", tt.input, h, tt.toHTTP)
			}
			w := toWS(*u)
			if w.String() != tt.toWS {
				t.Fatalf("toWS(%s) = %s, want %s", tt.input, w, tt.toWS)
			}
		})
	}
}

func mustParseURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

func TestExpandWindowsExePaths(t *testing.T) {
	result := expandWindowsExePaths("test.exe")
	if len(result) != 3 {
		t.Fatalf("expected 3 paths, got %d", len(result))
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
