package main

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/inovacc/scout/pkg/scout/capture"
	"github.com/inovacc/scout/pkg/scout/vault"
)

func TestRunCaptureHostStreams_EndToEnd(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SCOUT_HOME", home)

	// Set up a vault + capture key + nonce + persisted ext-id, exactly as the
	// one-time operator setup would.
	pass := []byte("correct horse battery staple")
	v, err := vault.Create(pass, vaultFileFor(home))
	if err != nil {
		t.Fatal(err)
	}
	pubPath := filepath.Join(home, "captures", "capture.pub")
	if _, err := capture.InitKeypair(v, pubPath, false); err != nil {
		t.Fatal(err)
	}
	_ = v.Close()

	nonce, err := capture.EnsureNonce(filepath.Join(home, "captures", "pairing.nonce"))
	if err != nil {
		t.Fatal(err)
	}
	const id = "abcdefghijklmnopabcdefghijklmnop"
	if err := saveExtID(id); err != nil {
		t.Fatal(err)
	}

	// Build a hello + capture_session frame stream from the "extension".
	var in bytes.Buffer
	mustFrame(t, &in, capture.Msg{V: 1, Type: "hello", ExtID: id, Nonce: nonce})
	mustFrame(t, &in, capture.Msg{V: 1, Type: "capture_session", Site: "example.com",
		Cookies: []capture.WireCookie{{Name: "sid", Value: "x", Domain: "example.com", Path: "/"}},
		Storage: map[string]capture.WireOriginStorage{"https://example.com": {Local: map[string]string{"k": "v"}}},
		At:      "2026-06-10T00:00:00Z"})

	var out bytes.Buffer
	if err := runCaptureHostStreams(&in, &out, "chrome-extension://"+id+"/"); err != nil {
		t.Fatalf("runCaptureHostStreams: %v", err)
	}

	// One sealed capture must have landed in the spool.
	spool, _ := capture.SpoolDir()
	files, _ := capture.ListSpool(spool)
	if len(files) != 1 {
		t.Fatalf("spool files = %d, want 1", len(files))
	}
	// And the host must have acked (never echoed a secret).
	if !bytes.Contains(out.Bytes(), []byte("hello_ack")) || !bytes.Contains(out.Bytes(), []byte("ack")) {
		t.Fatalf("missing acks in host output: %q", out.String())
	}
	if bytes.Contains(out.Bytes(), []byte("\"sid\"")) || bytes.Contains(out.Bytes(), []byte("\"x\"")) {
		t.Fatal("host echoed secret material")
	}
}

func TestRunCaptureHostStreams_OriginMismatch(t *testing.T) {
	home := t.TempDir()
	t.Setenv("SCOUT_HOME", home)

	pass := []byte("correct horse battery staple")
	v, err := vault.Create(pass, vaultFileFor(home))
	if err != nil {
		t.Fatal(err)
	}
	pubPath := filepath.Join(home, "captures", "capture.pub")
	if _, err := capture.InitKeypair(v, pubPath, false); err != nil {
		t.Fatal(err)
	}
	_ = v.Close()

	if _, err := capture.EnsureNonce(filepath.Join(home, "captures", "pairing.nonce")); err != nil {
		t.Fatal(err)
	}

	const installed = "abcdefghijklmnopabcdefghijklmnop"
	if err := saveExtID(installed); err != nil {
		t.Fatal(err)
	}

	spool, err := capture.SpoolDir()
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		origin string
	}{
		{
			name:   "different valid ext id",
			origin: "chrome-extension://pabcdefghijklmnopabcdefghijklmno/",
		},
		{
			name:   "malformed origin",
			origin: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var in, out bytes.Buffer
			err := runCaptureHostStreams(&in, &out, tc.origin)
			if err == nil {
				t.Fatalf("origin %q: expected error, got nil", tc.origin)
			}
			// No spool file must have been written.
			files, _ := capture.ListSpool(spool)
			if len(files) != 0 {
				t.Fatalf("origin %q: spool has %d file(s), want 0", tc.origin, len(files))
			}
		})
	}
}

func mustFrame(t *testing.T, w *bytes.Buffer, m capture.Msg) {
	t.Helper()
	if err := capture.WriteFrame(w, m); err != nil {
		t.Fatal(err)
	}
}
