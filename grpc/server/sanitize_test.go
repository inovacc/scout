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

	for i := 0; i < maxEvents+10; i++ {
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
