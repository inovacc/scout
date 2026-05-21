package id

import (
	"strings"
	"testing"
)

func TestRoundtrip(t *testing.T) {
	cases := []Attrs{
		{Browser: "chrome", Headless: true, Reusable: false, Stealth: false, Bridge: true, VPN: false},
		{Browser: "brave", Headless: false, Reusable: true, Stealth: true, Bridge: false, VPN: true},
		{Browser: "edge", Headless: true, Reusable: true, Stealth: false, Bridge: true, VPN: false},
		{Browser: "electron", Headless: false, Reusable: false, Stealth: true, Bridge: false, VPN: false},
		{Browser: "chromium", Headless: true, Reusable: false, Stealth: false, Bridge: false, VPN: false},
	}

	for _, in := range cases {
		s, err := New(in)
		if err != nil {
			t.Fatalf("New: %v", err)
		}

		if len(s) != Len {
			t.Fatalf("id len %d != %d", len(s), Len)
		}

		out, err := Parse(s)
		if err != nil {
			t.Fatalf("Parse(%q): %v", s, err)
		}

		if out.Browser != in.Browser ||
			out.Headless != in.Headless ||
			out.Reusable != in.Reusable ||
			out.Stealth != in.Stealth ||
			out.Bridge != in.Bridge ||
			out.VPN != in.VPN {
			t.Errorf("roundtrip mismatch:\n in=%+v\nout=%+v\n id=%s", in, out, s)
		}
	}
}

func TestUnknownBrowserDefaultsToChrome(t *testing.T) {
	s, err := New(Attrs{Browser: "firefox"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if s[posBrowser] != browserCodeChrome {
		t.Fatalf("unknown browser should default to chrome code, got %q", s[posBrowser])
	}
}

func TestInvalidLength(t *testing.T) {
	if _, err := Parse("short"); err == nil {
		t.Fatal("expected error for short id")
	}
}

func TestInvalidVersion(t *testing.T) {
	bad := "2CHPSBN00000" + strings.Repeat("A", 24)
	if _, err := Parse(bad); err == nil {
		t.Fatal("expected error for unsupported version")
	}
}

func TestInvalidBrowser(t *testing.T) {
	bad := "1ZHPSBN00000" + strings.Repeat("A", 24)
	if _, err := Parse(bad); err == nil {
		t.Fatal("expected error for unknown browser code")
	}
}

func TestRandomEntropy(t *testing.T) {
	seen := make(map[string]struct{}, 5000)
	for range 5000 {
		s, err := New(Attrs{Browser: "chrome"})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		r := s[attrLen:]
		if _, dup := seen[r]; dup {
			t.Fatalf("random suffix collision in 5k samples: %s", r)
		}
		seen[r] = struct{}{}
	}
}

func TestValid(t *testing.T) {
	s, _ := New(Attrs{Browser: "chrome"})
	if !Valid(s) {
		t.Fatal("freshly minted id should be valid")
	}
	if Valid("not-an-id") {
		t.Fatal("garbage should not validate")
	}
}
