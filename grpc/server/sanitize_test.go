package server

import (
	"errors"
	"testing"
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
