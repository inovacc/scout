package capture

import (
	"bytes"
	"path/filepath"
	"testing"
)

// drive runs the host against a sequence of inbound frames, returning the spool
// dir and the host's outbound frames.
func drive(t *testing.T, cfg HostConfig, msgs ...Msg) ([]string, []Msg) {
	t.Helper()
	var in, out bytes.Buffer
	for _, m := range msgs {
		if err := WriteFrame(&in, m); err != nil {
			t.Fatalf("WriteFrame: %v", err)
		}
	}
	if err := RunHost(&in, &out, cfg); err != nil {
		t.Fatalf("RunHost: %v", err)
	}
	var replies []Msg
	for {
		m, err := ReadFrame(&out)
		if err != nil {
			break
		}
		replies = append(replies, m)
	}
	files, _ := ListSpool(cfg.SpoolDir)
	return files, replies
}

func baseCfg(t *testing.T) (HostConfig, string) {
	t.Helper()
	v, dir := newTempVault(t)
	pub, err := InitKeypair(v, filepath.Join(dir, "capture.pub"), false)
	if err != nil {
		t.Fatalf("InitKeypair: %v", err)
	}
	noncePath := filepath.Join(dir, "pairing.nonce")
	nonce, err := EnsureNonce(noncePath)
	if err != nil {
		t.Fatalf("EnsureNonce: %v", err)
	}
	return HostConfig{
		Pub:          pub,
		SpoolDir:     filepath.Join(dir, "spool"),
		AllowedExtID: "abc123",
		NoncePath:    noncePath,
	}, nonce
}

func TestHostHappyPath(t *testing.T) {
	cfg, nonce := baseCfg(t)
	files, replies := drive(t, cfg,
		Msg{V: 1, Type: "hello", ExtID: "abc123", Nonce: nonce},
		Msg{V: 1, Type: "capture_login", Site: "example.com", Username: "alice", Password: "hunter2"},
	)
	if len(files) != 1 {
		t.Fatalf("want 1 spool file, got %d", len(files))
	}
	if len(replies) != 2 || replies[0].Type != "hello_ack" || replies[1].Type != "ack" {
		t.Fatalf("unexpected replies: %+v", replies)
	}
}

func TestHostRejectsWrongOrigin(t *testing.T) {
	cfg, nonce := baseCfg(t)
	files, replies := drive(t, cfg,
		Msg{V: 1, Type: "hello", ExtID: "WRONG", Nonce: nonce},
		Msg{V: 1, Type: "capture_login", Site: "x", Username: "u", Password: "topsecret"},
	)
	if len(files) != 0 {
		t.Fatal("spooled despite wrong origin")
	}
	if len(replies) == 0 || replies[0].Type != "error" {
		t.Fatalf("expected error reply, got %+v", replies)
	}
	for _, r := range replies {
		if bytes.Contains([]byte(r.Message), []byte("topsecret")) {
			t.Fatal("error reply leaked the password")
		}
	}
}

func TestHostRejectsMissingNonce(t *testing.T) {
	cfg, _ := baseCfg(t)
	files, replies := drive(t, cfg,
		Msg{V: 1, Type: "hello", ExtID: "abc123", Nonce: "bad"},
		Msg{V: 1, Type: "capture_session", Site: "x"},
	)
	if len(files) != 0 || replies[0].Type != "error" {
		t.Fatalf("missing/bad nonce not rejected: files=%d replies=%+v", len(files), replies)
	}
}

func TestHostRejectsCaptureBeforeHello(t *testing.T) {
	cfg, _ := baseCfg(t)
	files, replies := drive(t, cfg,
		Msg{V: 1, Type: "capture_session", Site: "x"},
	)
	if len(files) != 0 || len(replies) == 0 || replies[0].Type != "error" {
		t.Fatalf("capture before hello not rejected: files=%d replies=%+v", len(files), replies)
	}
}
