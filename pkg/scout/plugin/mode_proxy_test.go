package plugin

import (
	"testing"
)

func TestModeProxy_Name(t *testing.T) {
	p := &ModeProxy{
		entry: ModeEntry{Name: "test-mode", Description: "Test mode"},
	}

	if p.Name() != "test-mode" {
		t.Errorf("Name() = %q, want %q", p.Name(), "test-mode")
	}

	if p.Description() != "Test mode" {
		t.Errorf("Description() = %q, want %q", p.Description(), "Test mode")
	}

	if p.AuthProvider() != nil {
		t.Error("expected nil AuthProvider")
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"debug", "DEBUG"},
		{"info", "INFO"},
		{"warn", "WARN"},
		{"error", "ERROR"},
		{"unknown", "INFO"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseLevel(tt.input)
			if got.String() != tt.want {
				t.Errorf("parseLevel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
