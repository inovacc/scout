package slack

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/inovacc/scout"
)

func TestSaveLoadEncryptedRoundTrip(t *testing.T) {
	session := &CapturedSession{
		Version:      "1.0",
		Timestamp:    time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
		WorkspaceURL: "https://myteam.slack.com/",
		Token:        "xoxc-test-token-12345",
		DCookie:      "xoxd-test-d-cookie",
		Cookies: []scout.Cookie{
			{Name: "d", Value: "xoxd-test-d-cookie", Domain: ".slack.com", Path: "/"},
			{Name: "b", Value: "session-id", Domain: ".slack.com", Path: "/"},
		},
		LocalStorage: map[string]string{
			"localConfig_v2": `{"teams":{"T01":{"token":"xoxc-test-token-12345"}}}`,
		},
		SessionStorage: map[string]string{
			"some_key": "some_value",
		},
		UserID:   "U01TEST",
		TeamName: "My Team",
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "session.enc")
	passphrase := "test-passphrase-123"

	if err := SaveEncrypted(session, path, passphrase); err != nil {
		t.Fatalf("SaveEncrypted: %v", err)
	}

	// Verify file exists and has restricted permissions on non-Windows
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat encrypted file: %v", err)
	}

	if info.Size() == 0 {
		t.Fatal("encrypted file is empty")
	}

	loaded, err := LoadEncrypted(path, passphrase)
	if err != nil {
		t.Fatalf("LoadEncrypted: %v", err)
	}

	if loaded.Version != session.Version {
		t.Errorf("version = %q, want %q", loaded.Version, session.Version)
	}

	if loaded.Token != session.Token {
		t.Errorf("token = %q, want %q", loaded.Token, session.Token)
	}

	if loaded.DCookie != session.DCookie {
		t.Errorf("d_cookie = %q, want %q", loaded.DCookie, session.DCookie)
	}

	if loaded.WorkspaceURL != session.WorkspaceURL {
		t.Errorf("workspace_url = %q, want %q", loaded.WorkspaceURL, session.WorkspaceURL)
	}

	if len(loaded.Cookies) != len(session.Cookies) {
		t.Errorf("cookies count = %d, want %d", len(loaded.Cookies), len(session.Cookies))
	}

	if loaded.LocalStorage["localConfig_v2"] != session.LocalStorage["localConfig_v2"] {
		t.Error("localStorage mismatch")
	}

	if loaded.SessionStorage["some_key"] != session.SessionStorage["some_key"] {
		t.Error("sessionStorage mismatch")
	}

	if loaded.UserID != session.UserID {
		t.Errorf("user_id = %q, want %q", loaded.UserID, session.UserID)
	}

	if loaded.TeamName != session.TeamName {
		t.Errorf("team_name = %q, want %q", loaded.TeamName, session.TeamName)
	}
}

func TestLoadEncryptedWrongPassphrase(t *testing.T) {
	session := &CapturedSession{
		Version:   "1.0",
		Timestamp: time.Now().UTC(),
		Token:     "xoxc-secret",
		DCookie:   "xoxd-secret",
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "session.enc")

	if err := SaveEncrypted(session, path, "right-passphrase"); err != nil {
		t.Fatalf("SaveEncrypted: %v", err)
	}

	_, err := LoadEncrypted(path, "wrong-passphrase")
	if err == nil {
		t.Fatal("LoadEncrypted with wrong passphrase should fail")
	}
}

func TestLoadEncryptedMissingFile(t *testing.T) {
	_, err := LoadEncrypted("/nonexistent/path/session.enc", "passphrase")
	if err == nil {
		t.Fatal("LoadEncrypted with missing file should fail")
	}
}
