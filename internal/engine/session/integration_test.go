package session

import (
	"os"
	"testing"
	"time"

	"github.com/inovacc/scout/pkg/id"
)

// TestSessionCreateBinaryRoundtrip exercises the full create→write→read
// path with an encoded session ID and verifies that the values land
// correctly across the binary wire format.
func TestSessionCreateBinaryRoundtrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SCOUT_HOME", dir)

	sid, err := id.New(id.Attrs{
		Browser:  "brave",
		Headless: false,
		Reusable: true,
		Stealth:  true,
		Bridge:   true,
		VPN:      false,
	})
	if err != nil {
		t.Fatalf("id.New: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Microsecond)
	expires := now.Add(24 * time.Hour)

	in := &SessionInfo{
		ScoutPID:          os.Getpid(),
		BrowserPID:        99999,
		BrowserParentPID:  os.Getpid(),
		BrowserStartToken: "tok-roundtrip",
		Reusable:          true,
		Headless:          false,
		Browser:           "brave",
		CreatedAt:         now,
		LastUsed:          now,
		ExpiresAt:         expires,
		DomainHash:        "abcd1234ef5678",
		Domain:            "example.com",
		Exec:              "/path/to/brave",
		BuildVersion:      "v1.2.3",
	}

	if err := WriteInfo(sid, in); err != nil {
		t.Fatalf("WriteInfo: %v", err)
	}

	out, err := ReadInfo(sid)
	if err != nil {
		t.Fatalf("ReadInfo: %v", err)
	}

	if out.ScoutPID != in.ScoutPID {
		t.Errorf("ScoutPID: got %d, want %d", out.ScoutPID, in.ScoutPID)
	}
	if out.BrowserPID != in.BrowserPID {
		t.Errorf("BrowserPID: got %d, want %d", out.BrowserPID, in.BrowserPID)
	}
	if !out.ExpiresAt.Equal(in.ExpiresAt) {
		t.Errorf("ExpiresAt: got %v, want %v", out.ExpiresAt, in.ExpiresAt)
	}
	if out.BrowserStartToken != in.BrowserStartToken {
		t.Errorf("BrowserStartToken: got %q, want %q", out.BrowserStartToken, in.BrowserStartToken)
	}
	if out.DomainHash != in.DomainHash {
		t.Errorf("DomainHash: got %q, want %q", out.DomainHash, in.DomainHash)
	}

	// Browser / Reusable / Headless come from the ID prefix, not the body.
	// They should reflect the encoded attrs even though the body's flags
	// were intentionally inconsistent with them in a real-world write.
	if out.Browser != "brave" {
		t.Errorf("Browser (from id): got %q, want %q", out.Browser, "brave")
	}
	if !out.Reusable {
		t.Errorf("Reusable (from id): got false, want true")
	}
	if out.Headless {
		t.Errorf("Headless (from id): got true, want false")
	}
}

// TestSessionCreateMonitorsRoundtrip verifies that the monitors.json
// sidecar round-trips with a representative configuration.
func TestSessionCreateMonitorsRoundtrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SCOUT_HOME", dir)

	sid, err := id.New(id.Attrs{Browser: "chrome", Headless: true, Reusable: true})
	if err != nil {
		t.Fatalf("id.New: %v", err)
	}

	want := &MonitorConfig{
		HAR:    MonitorSink{Enabled: true, Path: "har.json", WithBodies: true},
		Hijack: MonitorSink{Enabled: true, Path: "hijack.jsonl"},
		Blocks: []MonitorRule{
			{Pattern: "*/api/*/reembolsos*", Method: "POST"},
			{Pattern: "*/api/*/expenses*", Method: "POST"},
		},
	}

	if err := WriteMonitors(sid, want); err != nil {
		t.Fatalf("WriteMonitors: %v", err)
	}

	got, err := ReadMonitors(sid)
	if err != nil {
		t.Fatalf("ReadMonitors: %v", err)
	}

	if got.HAR.Path != want.HAR.Path || !got.HAR.Enabled || !got.HAR.WithBodies {
		t.Errorf("HAR roundtrip mismatch: %+v", got.HAR)
	}
	if got.Hijack.Path != want.Hijack.Path || !got.Hijack.Enabled {
		t.Errorf("Hijack roundtrip mismatch: %+v", got.Hijack)
	}
	if len(got.Blocks) != len(want.Blocks) {
		t.Fatalf("block count: got %d, want %d", len(got.Blocks), len(want.Blocks))
	}
	for i, r := range want.Blocks {
		if got.Blocks[i].Pattern != r.Pattern || got.Blocks[i].Method != r.Method {
			t.Errorf("block %d: got %+v, want %+v", i, got.Blocks[i], r)
		}
	}
}
