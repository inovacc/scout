package session

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestBinaryRoundtrip(t *testing.T) {
	in := &SessionInfo{
		ScoutPID:          12345,
		BrowserPID:        67890,
		BrowserParentPID:  12345,
		BrowserStartToken: "tok-abc-12345",
		Reusable:          true,
		Headless:          true,
		CreatedAt:         time.Unix(1700000000, 0).UTC(),
		LastUsed:          time.Unix(1700001000, 500).UTC(),
		ExpiresAt:         time.Unix(1700604800, 0).UTC(),
		Browser:           "chrome",
		DomainHash:        "abcd1234ef567890",
		Domain:            "example.com",
		Exec:              `C:\Program Files\Chrome\chrome.exe`,
		BuildVersion:      "v1.2.3-build5",
	}

	buf := marshalBinary(in)
	if got := len(buf); got != binSize {
		t.Fatalf("marshalled size = %d, want %d", got, binSize)
	}

	out, err := unmarshalBinary(buf)
	if err != nil {
		t.Fatalf("unmarshalBinary: %v", err)
	}

	cases := []struct {
		name      string
		got, want any
	}{
		{"ScoutPID", out.ScoutPID, in.ScoutPID},
		{"BrowserPID", out.BrowserPID, in.BrowserPID},
		{"BrowserParentPID", out.BrowserParentPID, in.BrowserParentPID},
		{"Reusable", out.Reusable, in.Reusable},
		{"Headless", out.Headless, in.Headless},
		{"CreatedAt", out.CreatedAt.UnixNano(), in.CreatedAt.UnixNano()},
		{"LastUsed", out.LastUsed.UnixNano(), in.LastUsed.UnixNano()},
		{"ExpiresAt", out.ExpiresAt.UnixNano(), in.ExpiresAt.UnixNano()},
		{"Browser", out.Browser, in.Browser},
		{"DomainHash", out.DomainHash, in.DomainHash},
		{"Domain", out.Domain, in.Domain},
		{"Exec", out.Exec, in.Exec},
		{"BuildVersion", out.BuildVersion, in.BuildVersion},
		{"BrowserStartToken", out.BrowserStartToken, in.BrowserStartToken},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestBinaryZeroTimes(t *testing.T) {
	in := &SessionInfo{ScoutPID: 1, Browser: "chrome"} // no times set
	buf := marshalBinary(in)
	out, err := unmarshalBinary(buf)
	if err != nil {
		t.Fatalf("unmarshalBinary: %v", err)
	}
	if !out.CreatedAt.IsZero() || !out.LastUsed.IsZero() || !out.ExpiresAt.IsZero() {
		t.Fatalf("zero times not preserved: created=%v last=%v expires=%v",
			out.CreatedAt, out.LastUsed, out.ExpiresAt)
	}
}

func TestBinaryLegacyDetect(t *testing.T) {
	// JSON-style content starts with `{`, no SCT1 magic.
	if _, err := unmarshalBinary([]byte(`{"scout_pid":1}`)); !errors.Is(err, ErrLegacyFormat) {
		t.Fatalf("legacy JSON should return ErrLegacyFormat; got %v", err)
	}
}

func TestBinaryTruncated(t *testing.T) {
	// Magic OK but body short.
	short := []byte("SCT1____")
	if _, err := unmarshalBinary(short); !errors.Is(err, ErrCorruptInfo) {
		t.Fatalf("truncated buffer should return ErrCorruptInfo; got %v", err)
	}
}

func TestBinaryStringTruncation(t *testing.T) {
	long := strings.Repeat("x", 256) // larger than Exec cap (128)
	in := &SessionInfo{ScoutPID: 1, Browser: "chrome", Exec: long}
	out, err := unmarshalBinary(marshalBinary(in))
	if err != nil {
		t.Fatalf("unmarshalBinary: %v", err)
	}
	if len(out.Exec) != binExecCap {
		t.Fatalf("Exec truncated to %d, want %d", len(out.Exec), binExecCap)
	}
}
