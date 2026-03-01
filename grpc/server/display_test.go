package server

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestTruncateDisplay(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short_string", "hello", 10, "hello"},
		{"exact_length", "hello", 5, "hello"},
		{"truncated", "hello world", 8, "hello..."},
		{"maxLen_3", "hello", 3, "hel"},
		{"maxLen_2", "hello", 2, "he"},
		{"maxLen_1", "hello", 1, "h"},
		{"empty_string", "", 5, ""},
		{"maxLen_4", "hello", 4, "h..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestPrintKV(t *testing.T) {
	var buf bytes.Buffer
	printKV(&buf, 82, "Listen", ":9551")
	got := buf.String()

	if !strings.Contains(got, "Listen") {
		t.Error("printKV output should contain key")
	}
	if !strings.Contains(got, ":9551") {
		t.Error("printKV output should contain value")
	}
	if !strings.HasPrefix(got, "│") {
		t.Error("printKV output should start with box-drawing border")
	}
}

func TestPrintServerTable_Basic(t *testing.T) {
	var buf bytes.Buffer

	info := ServerInfo{
		DeviceID:      "ABC123DEF456",
		ListenAddr:    ":9551",
		PairingAddr:   ":9552",
		Insecure:      false,
		LocalIPs:      []string{"192.168.1.10", "10.0.0.5"},
		TotalSessions: 5,
		Events:        nil,
	}

	PrintServerTable(&buf, info, nil)
	out := buf.String()

	tests := []struct {
		name    string
		pattern string
	}{
		{"header", "Scout Server"},
		{"device_id", "ABC123DEF456"},
		{"listen_addr", ":9551"},
		{"pairing_addr", ":9552"},
		{"local_ips", "192.168.1.10"},
		{"mode_mTLS", "mTLS"},
		{"active_count", "Active: 0"},
		{"total_count", "Total: 5"},
		{"no_activity", "(no activity yet)"},
		{"top_border", "┌"},
		{"bottom_border", "└"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(out, tt.pattern) {
				t.Errorf("output missing %q", tt.pattern)
			}
		})
	}
}

func TestPrintServerTable_InsecureMode(t *testing.T) {
	var buf bytes.Buffer
	info := ServerInfo{Insecure: true}
	PrintServerTable(&buf, info, nil)
	if !strings.Contains(buf.String(), "Insecure") {
		t.Error("insecure mode should show 'Insecure'")
	}
}

func TestPrintServerTable_EmptyDeviceID(t *testing.T) {
	var buf bytes.Buffer
	info := ServerInfo{DeviceID: ""}
	PrintServerTable(&buf, info, nil)
	if !strings.Contains(buf.String(), "(none)") {
		t.Error("empty device ID should show '(none)'")
	}
}

func TestPrintServerTable_NoLocalIPs(t *testing.T) {
	var buf bytes.Buffer
	info := ServerInfo{LocalIPs: nil}
	PrintServerTable(&buf, info, nil)
	if !strings.Contains(buf.String(), "(none)") {
		t.Error("no local IPs should show '(none)'")
	}
}

func TestPrintServerTable_WithPeers(t *testing.T) {
	var buf bytes.Buffer
	info := ServerInfo{
		ListenAddr:    ":9551",
		TotalSessions: 2,
	}
	peers := []ConnectedPeer{
		{DeviceID: "device-abc-123", ShortID: "HM2ASC3", Addr: "192.168.1.5:50001", Sessions: 3},
		{DeviceID: "device-xyz-789", ShortID: "KP4QRZ7", Addr: "10.0.0.2:50002", Sessions: 1},
	}

	PrintServerTable(&buf, info, peers)
	out := buf.String()

	if !strings.Contains(out, "HM2ASC3") {
		t.Error("output should contain peer short ID")
	}
	if !strings.Contains(out, "device-abc-123") {
		t.Error("output should contain peer device ID")
	}
	if !strings.Contains(out, "Active: 2") {
		t.Error("output should show 2 active peers")
	}
	if !strings.Contains(out, "Short ID") {
		t.Error("output should contain column header")
	}
}

func TestPrintServerTable_WithEvents(t *testing.T) {
	var buf bytes.Buffer
	now := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	info := ServerInfo{
		Events: []SessionEvent{
			{Time: now, Type: "connect", DeviceID: "HM2ASC3", Detail: "session abc"},
		},
	}
	PrintServerTable(&buf, info, nil)
	out := buf.String()

	if !strings.Contains(out, "Recent Activity") {
		t.Error("output should contain Recent Activity header")
	}
	if !strings.Contains(out, "connect") {
		t.Error("output should contain event type")
	}
	if !strings.Contains(out, "10:30:00") {
		t.Error("output should contain formatted event time")
	}
}

func TestPrintServerTable_MoreThan10Events(t *testing.T) {
	var buf bytes.Buffer
	events := make([]SessionEvent, 15)
	for i := range events {
		events[i] = SessionEvent{
			Time:   time.Date(2026, 1, 1, 0, 0, i, 0, time.UTC),
			Type:   "connect",
			Detail: "event",
		}
	}
	info := ServerInfo{Events: events}
	PrintServerTable(&buf, info, nil)
	out := buf.String()

	// Should show events starting from index 5 (last 10)
	if !strings.Contains(out, "00:00:05") {
		t.Error("should show event at index 5")
	}
	if strings.Contains(out, "00:00:04") {
		t.Error("should NOT show event at index 4 (only last 10)")
	}
}

func TestPrintServerTable_NoPairingAddr(t *testing.T) {
	var buf bytes.Buffer
	info := ServerInfo{PairingAddr: ""}
	PrintServerTable(&buf, info, nil)
	if strings.Contains(buf.String(), "Pairing") {
		t.Error("should not show Pairing row when empty")
	}
}

func TestGetLocalIPsDisplay(t *testing.T) {
	ips := GetLocalIPs()
	// Can't assert specific IPs, but verify structure
	for _, ip := range ips {
		if strings.HasPrefix(ip, "127.") {
			t.Errorf("GetLocalIPs should not return loopback: %s", ip)
		}
		if strings.Contains(ip, ":") {
			t.Errorf("GetLocalIPs should not return IPv6: %s", ip)
		}
	}
}
