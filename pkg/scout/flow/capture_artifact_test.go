package flow

import (
	"path/filepath"
	"testing"
)

func TestCaptureSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.json")
	in := &Capture{
		Version: "1",
		Name:    "demo",
		Entries: []CaptureEntry{{
			Method: "POST", URL: "https://api.example.com/login",
			ReqHeaders: map[string]string{"Content-Type": "application/json"},
			ReqBody:    `{"u":"a"}`,
			Status:     200,
			RespHeaders: map[string]string{"X-CSRF-Token": "csrf-1"},
			RespBody:   `{"access_token":"tok-1"}`,
			MimeType:   "application/json",
		}},
	}
	if err := SaveCapture(path, in); err != nil {
		t.Fatalf("SaveCapture: %v", err)
	}
	out, err := LoadCapture(path)
	if err != nil {
		t.Fatalf("LoadCapture: %v", err)
	}
	if len(out.Entries) != 1 || out.Entries[0].RespBody != `{"access_token":"tok-1"}` {
		t.Fatalf("round-trip mismatch: %+v", out.Entries)
	}
}

func TestLoadCaptureMissing(t *testing.T) {
	if _, err := LoadCapture(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Fatal("expected error for missing capture")
	}
}
