package auth

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/scraper/crypt"
)

// TestSaveLoadEncrypted_RichRoundTrip exercises every populated Session field
// through the marshal -> encrypt -> write -> read -> decrypt -> unmarshal path
// and asserts each value survives the round trip.
func TestSaveLoadEncrypted_RichRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rich.enc")

	ts := time.Date(2026, 6, 10, 12, 30, 0, 0, time.UTC)
	exp := ts.Add(24 * time.Hour)
	want := &Session{
		Provider:  "slack",
		Version:   "2",
		Timestamp: ts,
		URL:       "https://app.slack.com",
		Cookies: []scout.Cookie{
			{Name: "d", Value: "xoxd-secret", Domain: ".slack.com"},
			{Name: "lc", Value: "1700000000"},
		},
		Tokens:         map[string]string{"xoxc": "token-c", "xoxd": "token-d"},
		LocalStorage:   map[string]string{"theme": "dark"},
		SessionStorage: map[string]string{"tab": "channels"},
		Extra:          map[string]string{"team": "T123"},
		ExpiresAt:      exp,
	}

	if err := SaveEncrypted(want, path, "pa$$phrase"); err != nil {
		t.Fatalf("SaveEncrypted() error = %v", err)
	}

	got, err := LoadEncrypted(path, "pa$$phrase")
	if err != nil {
		t.Fatalf("LoadEncrypted() error = %v", err)
	}

	if got.Provider != want.Provider || got.Version != want.Version {
		t.Errorf("Provider/Version = %q/%q, want %q/%q", got.Provider, got.Version, want.Provider, want.Version)
	}
	if !got.Timestamp.Equal(want.Timestamp) {
		t.Errorf("Timestamp = %v, want %v", got.Timestamp, want.Timestamp)
	}
	if !got.ExpiresAt.Equal(want.ExpiresAt) {
		t.Errorf("ExpiresAt = %v, want %v", got.ExpiresAt, want.ExpiresAt)
	}
	if len(got.Cookies) != 2 || got.Cookies[0].Name != "d" || got.Cookies[0].Value != "xoxd-secret" {
		t.Errorf("Cookies = %+v, want first cookie d=xoxd-secret", got.Cookies)
	}
	if got.Tokens["xoxc"] != "token-c" || got.Tokens["xoxd"] != "token-d" {
		t.Errorf("Tokens = %v, want xoxc/xoxd populated", got.Tokens)
	}
	if got.LocalStorage["theme"] != "dark" {
		t.Errorf("LocalStorage[theme] = %q, want dark", got.LocalStorage["theme"])
	}
	if got.SessionStorage["tab"] != "channels" {
		t.Errorf("SessionStorage[tab] = %q, want channels", got.SessionStorage["tab"])
	}
	if got.Extra["team"] != "T123" {
		t.Errorf("Extra[team] = %q, want T123", got.Extra["team"])
	}
}

// TestSaveEncrypted_CiphertextIsNotPlaintext verifies the on-disk blob is
// genuinely encrypted: it must carry the crypt header version byte and must not
// contain any plaintext field value.
func TestSaveEncrypted_CiphertextIsNotPlaintext(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "enc.bin")

	s := &Session{Provider: "secret-provider", Tokens: map[string]string{"k": "SUPER-SECRET-VALUE"}}
	if err := SaveEncrypted(s, path, "pw"); err != nil {
		t.Fatalf("SaveEncrypted() error = %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	if len(raw) == 0 {
		t.Fatal("encrypted blob is empty")
	}
	if raw[0] != crypt.Version() {
		t.Errorf("first byte = %d, want crypt version %d", raw[0], crypt.Version())
	}
	if strings.Contains(string(raw), "SUPER-SECRET-VALUE") {
		t.Error("plaintext token leaked into encrypted file")
	}
	if strings.Contains(string(raw), "secret-provider") {
		t.Error("plaintext provider leaked into encrypted file")
	}
}

// TestSaveEncrypted_MkdirFails forces the MkdirAll branch to fail by making a
// parent path component a regular file, so creating a directory under it errors.
func TestSaveEncrypted_MkdirFails(t *testing.T) {
	dir := t.TempDir()
	// Create a regular file, then try to save "under" it as if it were a dir.
	blocker := filepath.Join(dir, "iamafile")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("setup write: %v", err)
	}

	path := filepath.Join(blocker, "sub", "session.enc")

	err := SaveEncrypted(&Session{Provider: "p"}, path, "pw")
	if err == nil {
		t.Fatal("expected error when parent path is a file")
	}
	if !strings.Contains(err.Error(), "auth: save session") {
		t.Errorf("error = %q, want prefix 'auth: save session'", err.Error())
	}
}

// TestLoadEncrypted_DecryptsButNotJSON pins the unmarshal-failure branch: the
// blob decrypts cleanly (correct passphrase) but the plaintext is not valid
// JSON, so json.Unmarshal must fail with the documented wrap.
func TestLoadEncrypted_DecryptsButNotJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "notjson.enc")

	// Encrypt raw non-JSON bytes directly with the same crypt the loader uses.
	enc, err := crypt.EncryptData([]byte("this-is-not-json{{{"), "pw")
	if err != nil {
		t.Fatalf("EncryptData() error = %v", err)
	}
	if err := os.WriteFile(path, enc, 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err = LoadEncrypted(path, "pw")
	if err == nil {
		t.Fatal("expected unmarshal error for non-JSON plaintext")
	}
	if !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("error = %q, want to mention 'unmarshal'", err.Error())
	}

	// The wrapped error must be a *json.SyntaxError (errors.As contract).
	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) {
		t.Errorf("error chain does not contain *json.SyntaxError: %v", err)
	}
}

// TestLoadEncrypted_TooShort pins the decrypt error path for a truncated blob
// that is shorter than the crypt header.
func TestLoadEncrypted_TooShort(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "short.enc")

	if err := os.WriteFile(path, []byte{0x01, 0x02}, 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := LoadEncrypted(path, "pw")
	if err == nil {
		t.Fatal("expected error for truncated blob")
	}
	if !strings.Contains(err.Error(), "auth: load session") {
		t.Errorf("error = %q, want prefix 'auth: load session'", err.Error())
	}
}

// TestSaveEncrypted_NoTempLeftover verifies the atomic rename cleans up after
// itself: after a successful save only the final file exists in the dir, no
// leftover *.tmp.* temp files.
func TestSaveEncrypted_NoTempLeftover(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.enc")

	if err := SaveEncrypted(&Session{Provider: "p"}, path, "pw"); err != nil {
		t.Fatalf("SaveEncrypted() error = %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 1 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("dir has %d entries %v, want exactly 1 (the renamed file)", len(entries), names)
	}
	if entries[0].Name() != "session.enc" {
		t.Errorf("leftover/unexpected file %q, want session.enc", entries[0].Name())
	}
}

// TestSaveEncrypted_Overwrite verifies an existing target is atomically replaced
// and the new content is what is read back (not the stale one).
func TestSaveEncrypted_Overwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.enc")

	if err := SaveEncrypted(&Session{Provider: "first"}, path, "pw"); err != nil {
		t.Fatalf("first SaveEncrypted() error = %v", err)
	}
	if err := SaveEncrypted(&Session{Provider: "second"}, path, "pw"); err != nil {
		t.Fatalf("second SaveEncrypted() error = %v", err)
	}

	got, err := LoadEncrypted(path, "pw")
	if err != nil {
		t.Fatalf("LoadEncrypted() error = %v", err)
	}
	if got.Provider != "second" {
		t.Errorf("Provider = %q, want %q (overwrite did not take)", got.Provider, "second")
	}
}
