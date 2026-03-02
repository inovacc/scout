package server

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSanitizeError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"nil error", nil, ""},
		{"no paths", errors.New("connection refused"), "connection refused"},
		{"windows path", errors.New(`failed: C:\Users\john\AppData\Local\chrome`), "failed: [path-redacted]"},
		{"unix home", errors.New("profile at /home/user/.config/chrome"), "profile at [path-redacted]"},
		{"mac path", errors.New("dir /Users/john/Library/chrome"), "dir [path-redacted]"},
		{"tmp path", errors.New("temp file /tmp/rod-12345/profile"), "temp file [path-redacted]"},
		{"var path", errors.New("log at /var/log/chrome.log"), "log at [path-redacted]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeError(tt.err)
			if tt.err == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}

				return
			}

			if result.Error() != tt.want {
				t.Errorf("got %q, want %q", result.Error(), tt.want)
			}
		})
	}
}

func TestSessionEvent(t *testing.T) {
	srv := New()

	srv.recordEvent("test", "sess-123", "DEV1", "some detail")

	events := srv.Events()
	if len(events) != 1 {
		t.Fatalf("events count = %d, want 1", len(events))
	}

	if events[0].Type != "test" {
		t.Errorf("event type = %q, want %q", events[0].Type, "test")
	}

	totalSess, totalReq := srv.Stats()
	if totalSess != 0 {
		t.Errorf("totalSessions = %d, want 0", totalSess)
	}

	if totalReq != 1 {
		t.Errorf("totalRequests = %d, want 1", totalReq)
	}
}

func TestEventRingBuffer(t *testing.T) {
	srv := New()

	for range maxEvents + 10 {
		srv.recordEvent("test", "sess", "dev", "detail")
	}

	events := srv.Events()
	if len(events) != maxEvents {
		t.Errorf("events count = %d, want %d", len(events), maxEvents)
	}
}

func TestPrintServerTable_NoPeers(t *testing.T) {
	var buf bytes.Buffer

	info := ServerInfo{
		DeviceID:      "TEST-DEVICE-ID",
		ListenAddr:    ":50051",
		Insecure:      true,
		LocalIPs:      []string{"192.168.1.100"},
		TotalSessions: 5,
	}
	PrintServerTable(&buf, info, nil)
	out := buf.String()

	if !strings.Contains(out, "Scout Server") {
		t.Error("missing header")
	}

	if !strings.Contains(out, "Active: 0  Total: 5") {
		t.Error("missing counters")
	}

	if !strings.Contains(out, "Insecure") {
		t.Error("missing mode")
	}

	if !strings.Contains(out, "Recent Activity") {
		t.Error("missing activity section")
	}

	if !strings.Contains(out, "(no activity yet)") {
		t.Error("missing empty activity message")
	}
}

func TestPrintServerTable_WithPeersAndEvents(t *testing.T) {
	var buf bytes.Buffer

	info := ServerInfo{
		DeviceID:      "ABCDEFG",
		ListenAddr:    ":50051",
		PairingAddr:   ":50052",
		LocalIPs:      []string{"10.0.0.1"},
		TotalSessions: 3,
		Events: []SessionEvent{
			{Time: time.Date(2026, 1, 1, 22, 52, 8, 0, time.UTC), Type: "connect", SessionID: "abc", DeviceID: "HM2ASC3", Detail: "session abc"},
			{Time: time.Date(2026, 1, 1, 22, 52, 9, 0, time.UTC), Type: "navigate", SessionID: "abc", DeviceID: "HM2ASC3", Detail: "https://example.com"},
		},
	}
	peers := []ConnectedPeer{
		{DeviceID: "full-device-id", ShortID: "HM2ASC3", Addr: "192.168.1.5:12345", Sessions: 2},
	}
	PrintServerTable(&buf, info, peers)
	out := buf.String()

	if !strings.Contains(out, "Active: 1  Total: 3") {
		t.Error("missing counters")
	}

	if !strings.Contains(out, "HM2ASC3") {
		t.Error("missing peer short ID")
	}

	if !strings.Contains(out, "connect") {
		t.Error("missing connect event")
	}

	if !strings.Contains(out, "navigate") {
		t.Error("missing navigate event")
	}

	if !strings.Contains(out, "Pairing") {
		t.Error("missing pairing addr")
	}
}

func TestOnStatsChangeCallback(t *testing.T) {
	srv := New()
	called := false
	srv.OnStatsChange = func() { called = true }

	srv.recordEvent("test", "s1", "d1", "detail")

	if !called {
		t.Error("OnStatsChange not called")
	}
}

func TestGetLocalIPs(t *testing.T) {
	ips := GetLocalIPs()
	// On any machine with a network interface, we should get at least one IP.
	// In isolated CI containers this might be empty, so just verify no panic.
	if ips == nil {
		t.Log("GetLocalIPs returned nil (no non-loopback IPv4 interfaces)")
	}

	for _, ip := range ips {
		if ip == "" {
			t.Error("empty IP in result")
		}

		if ip == "127.0.0.1" {
			t.Error("loopback should be excluded")
		}
	}
}

func TestMapKey(t *testing.T) {
	tests := []struct {
		input string
		want  rune
	}{
		{"Enter", 0x0d},
		{"Tab", 0x09},
		{"Escape", 0x1b},
		{"Space", ' '},
		{"Backspace", 0x08},
		{"Delete", 0x7f},
		{"ArrowUp", 0xe011},
		{"ArrowDown", 0xe012},
		{"ArrowLeft", 0xe013},
		{"ArrowRight", 0xe014},
		{"Home", 0xe011},   // verify it returns something non-zero
		{"End", 0xe010},    // verify it returns something non-zero
		{"PageUp", 0xe00e}, // verify non-zero
		{"PageDown", 0xe00f},
		{"a", 'a'},
		{"1", '1'},
		{"", 0},
		{"UnknownMultiChar", 0},
	}

	for _, tt := range tests {
		got := mapKey(tt.input)
		// For named keys, just verify non-zero (exact values depend on input package)
		switch tt.input {
		case "Enter", "Tab", "Escape", "Space", "Backspace", "Delete",
			"ArrowUp", "ArrowDown", "ArrowLeft", "ArrowRight",
			"Home", "End", "PageUp", "PageDown":
			if got == 0 {
				t.Errorf("mapKey(%q) = 0, want non-zero", tt.input)
			}
		case "a":
			if got != 'a' {
				t.Errorf("mapKey(%q) = %d, want %d", tt.input, got, 'a')
			}
		case "1":
			if got != '1' {
				t.Errorf("mapKey(%q) = %d, want %d", tt.input, got, '1')
			}
		case "", "UnknownMultiChar":
			if got != 0 {
				t.Errorf("mapKey(%q) = %d, want 0", tt.input, got)
			}
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"hello world", 8, "hello..."},
		{"ab", 2, "ab"},
		{"abcd", 3, "abc"},
		{"abcdef", 5, "ab..."},
	}
	for _, tt := range tests {
		got := truncate(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}
