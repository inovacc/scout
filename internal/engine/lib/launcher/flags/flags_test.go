package flags

import "testing"

func TestFlagCheck(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for flag with =")
		}
	}()

	Flag("bad=flag").Check()
}

func TestFlagCheckValid(t *testing.T) {
	// Should not panic.
	Flag("headless").Check()
	Flag("remote-debugging-port").Check()
}

func TestNormalizeFlag(t *testing.T) {
	tests := []struct {
		input    Flag
		expected Flag
	}{
		{"headless", "headless"},
		{"--headless", "headless"},
		{"-headless", "headless"},
		{"---headless", "headless"},
	}

	for _, tt := range tests {
		got := tt.input.NormalizeFlag()
		if got != tt.expected {
			t.Fatalf("NormalizeFlag(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
