package scout

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func init() {
	registerTestRoutes(storageTestRoutes)
}

func storageTestRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/storage", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Storage Test</title></head>
<body>
<h1>Storage Page</h1>
<script>
localStorage.setItem("preloaded", "yes");
sessionStorage.setItem("session_pre", "active");
</script>
</body></html>`)
	})
}

func TestLocalStorageSetGet(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/storage")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	if err := page.LocalStorageSet("foo", "bar"); err != nil {
		t.Fatalf("LocalStorageSet() error: %v", err)
	}

	val, err := page.LocalStorageGet("foo")
	if err != nil {
		t.Fatalf("LocalStorageGet() error: %v", err)
	}

	if val != "bar" {
		t.Errorf("LocalStorageGet() = %q, want %q", val, "bar")
	}
}

func TestLocalStorageRemove(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/storage")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	if err := page.LocalStorageSet("remove_me", "value"); err != nil {
		t.Fatalf("LocalStorageSet() error: %v", err)
	}

	if err := page.LocalStorageRemove("remove_me"); err != nil {
		t.Fatalf("LocalStorageRemove() error: %v", err)
	}

	val, err := page.LocalStorageGet("remove_me")
	if err != nil {
		t.Fatalf("LocalStorageGet() error: %v", err)
	}

	if val != "" {
		t.Errorf("LocalStorageGet() after remove = %q, want empty", val)
	}
}

func TestLocalStorageClear(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/storage")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	if err := page.LocalStorageSet("a", "1"); err != nil {
		t.Fatalf("LocalStorageSet() error: %v", err)
	}

	if err := page.LocalStorageSet("b", "2"); err != nil {
		t.Fatalf("LocalStorageSet() error: %v", err)
	}

	if err := page.LocalStorageClear(); err != nil {
		t.Fatalf("LocalStorageClear() error: %v", err)
	}

	length, err := page.LocalStorageLength()
	if err != nil {
		t.Fatalf("LocalStorageLength() error: %v", err)
	}

	if length != 0 {
		t.Errorf("LocalStorageLength() = %d, want 0", length)
	}
}

func TestLocalStorageGetAll(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/storage")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	// Clear first to have a clean slate
	if err := page.LocalStorageClear(); err != nil {
		t.Fatalf("LocalStorageClear() error: %v", err)
	}

	if err := page.LocalStorageSet("x", "10"); err != nil {
		t.Fatalf("LocalStorageSet() error: %v", err)
	}

	if err := page.LocalStorageSet("y", "20"); err != nil {
		t.Fatalf("LocalStorageSet() error: %v", err)
	}

	all, err := page.LocalStorageGetAll()
	if err != nil {
		t.Fatalf("LocalStorageGetAll() error: %v", err)
	}

	if len(all) != 2 {
		t.Fatalf("LocalStorageGetAll() returned %d items, want 2", len(all))
	}

	if all["x"] != "10" {
		t.Errorf("all[\"x\"] = %q, want %q", all["x"], "10")
	}

	if all["y"] != "20" {
		t.Errorf("all[\"y\"] = %q, want %q", all["y"], "20")
	}
}

func TestLocalStorageLength(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/storage")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	// The page pre-sets "preloaded" in localStorage
	length, err := page.LocalStorageLength()
	if err != nil {
		t.Fatalf("LocalStorageLength() error: %v", err)
	}

	if length < 1 {
		t.Errorf("LocalStorageLength() = %d, want >= 1 (preloaded key)", length)
	}
}

func TestSessionStorageSetGet(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/storage")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	if err := page.SessionStorageSet("sess_key", "sess_val"); err != nil {
		t.Fatalf("SessionStorageSet() error: %v", err)
	}

	val, err := page.SessionStorageGet("sess_key")
	if err != nil {
		t.Fatalf("SessionStorageGet() error: %v", err)
	}

	if val != "sess_val" {
		t.Errorf("SessionStorageGet() = %q, want %q", val, "sess_val")
	}
}

func TestSessionStorageGetAll(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/storage")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	// Clear first to have a clean slate
	if err := page.SessionStorageClear(); err != nil {
		t.Fatalf("SessionStorageClear() error: %v", err)
	}

	if err := page.SessionStorageSet("s1", "v1"); err != nil {
		t.Fatalf("SessionStorageSet() error: %v", err)
	}

	if err := page.SessionStorageSet("s2", "v2"); err != nil {
		t.Fatalf("SessionStorageSet() error: %v", err)
	}

	all, err := page.SessionStorageGetAll()
	if err != nil {
		t.Fatalf("SessionStorageGetAll() error: %v", err)
	}

	if len(all) != 2 {
		t.Fatalf("SessionStorageGetAll() returned %d items, want 2", len(all))
	}

	if all["s1"] != "v1" {
		t.Errorf("all[\"s1\"] = %q, want %q", all["s1"], "v1")
	}

	if all["s2"] != "v2" {
		t.Errorf("all[\"s2\"] = %q, want %q", all["s2"], "v2")
	}
}

func TestSaveSession(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/storage")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	if err := page.SetCookies(Cookie{
		Name:  "session_id",
		Value: "abc123",
		URL:   srv.URL,
	}); err != nil {
		t.Fatalf("SetCookies() error: %v", err)
	}

	if err := page.LocalStorageSet("token", "xyz"); err != nil {
		t.Fatalf("LocalStorageSet() error: %v", err)
	}

	if err := page.SessionStorageSet("temp", "data"); err != nil {
		t.Fatalf("SessionStorageSet() error: %v", err)
	}

	state, err := page.SaveSession()
	if err != nil {
		t.Fatalf("SaveSession() error: %v", err)
	}

	if state.URL == "" {
		t.Error("SaveSession() URL is empty")
	}

	if len(state.Cookies) == 0 {
		t.Error("SaveSession() Cookies is empty")
	}

	foundCookie := false

	for _, c := range state.Cookies {
		if c.Name == "session_id" && c.Value == "abc123" {
			foundCookie = true
			break
		}
	}

	if !foundCookie {
		t.Error("SaveSession() missing session_id cookie")
	}

	if state.LocalStorage["token"] != "xyz" {
		t.Errorf("SaveSession() LocalStorage[\"token\"] = %q, want %q", state.LocalStorage["token"], "xyz")
	}

	if state.SessionStorage["temp"] != "data" {
		t.Errorf("SaveSession() SessionStorage[\"temp\"] = %q, want %q", state.SessionStorage["temp"], "data")
	}
}

func TestLoadSession(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	// Create initial page and save session
	page1, err := b.NewPage(srv.URL + "/storage")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	if err := page1.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	if err := page1.SetCookies(Cookie{
		Name:  "auth",
		Value: "token123",
		URL:   srv.URL,
	}); err != nil {
		t.Fatalf("SetCookies() error: %v", err)
	}

	if err := page1.LocalStorageSet("user", "alice"); err != nil {
		t.Fatalf("LocalStorageSet() error: %v", err)
	}

	if err := page1.SessionStorageSet("tab_id", "t1"); err != nil {
		t.Fatalf("SessionStorageSet() error: %v", err)
	}

	state, err := page1.SaveSession()
	if err != nil {
		t.Fatalf("SaveSession() error: %v", err)
	}

	_ = page1.Close()

	// Open new page and load session
	page2, err := b.NewPage("")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page2.Close() }()

	if err := page2.LoadSession(state); err != nil {
		t.Fatalf("LoadSession() error: %v", err)
	}

	// Verify cookies
	cookies, err := page2.GetCookies()
	if err != nil {
		t.Fatalf("GetCookies() error: %v", err)
	}

	foundAuth := false

	for _, c := range cookies {
		if c.Name == "auth" && c.Value == "token123" {
			foundAuth = true
			break
		}
	}

	if !foundAuth {
		t.Error("LoadSession() did not restore auth cookie")
	}

	// Verify localStorage
	user, err := page2.LocalStorageGet("user")
	if err != nil {
		t.Fatalf("LocalStorageGet() error: %v", err)
	}

	if user != "alice" {
		t.Errorf("LocalStorageGet(\"user\") = %q, want %q", user, "alice")
	}

	// Verify sessionStorage
	tabID, err := page2.SessionStorageGet("tab_id")
	if err != nil {
		t.Fatalf("SessionStorageGet() error: %v", err)
	}

	if tabID != "t1" {
		t.Errorf("SessionStorageGet(\"tab_id\") = %q, want %q", tabID, "t1")
	}
}

func TestSaveLoadSessionFile(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/storage")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	if err := page.LocalStorageSet("file_key", "file_val"); err != nil {
		t.Fatalf("LocalStorageSet() error: %v", err)
	}

	state, err := page.SaveSession()
	if err != nil {
		t.Fatalf("SaveSession() error: %v", err)
	}

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "session.json")

	if err := SaveSessionToFile(state, path); err != nil {
		t.Fatalf("SaveSessionToFile() error: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("session file does not exist: %v", err)
	}

	loaded, err := LoadSessionFromFile(path)
	if err != nil {
		t.Fatalf("LoadSessionFromFile() error: %v", err)
	}

	if loaded.URL != state.URL {
		t.Errorf("loaded URL = %q, want %q", loaded.URL, state.URL)
	}

	if loaded.LocalStorage["file_key"] != "file_val" {
		t.Errorf("loaded LocalStorage[\"file_key\"] = %q, want %q", loaded.LocalStorage["file_key"], "file_val")
	}

	if len(loaded.Cookies) != len(state.Cookies) {
		t.Errorf("loaded Cookies count = %d, want %d", len(loaded.Cookies), len(state.Cookies))
	}
}

func TestLoadSessionFileNotFound(t *testing.T) {
	_, err := LoadSessionFromFile("/nonexistent/path/session.json")
	if err == nil {
		t.Fatal("LoadSessionFromFile() expected error for missing file, got nil")
	}
}

func TestSessionStorageRemove(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/storage")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	if err := page.SessionStorageSet("del_me", "value"); err != nil {
		t.Fatalf("SessionStorageSet() error: %v", err)
	}

	if err := page.SessionStorageRemove("del_me"); err != nil {
		t.Fatalf("SessionStorageRemove() error: %v", err)
	}

	val, err := page.SessionStorageGet("del_me")
	if err != nil {
		t.Fatalf("SessionStorageGet() error: %v", err)
	}

	if val != "" {
		t.Errorf("SessionStorageGet() after remove = %q, want empty", val)
	}
}

func TestSessionStorageLength(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/storage")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	if err := page.SessionStorageClear(); err != nil {
		t.Fatalf("SessionStorageClear() error: %v", err)
	}

	if err := page.SessionStorageSet("k1", "v1"); err != nil {
		t.Fatalf("SessionStorageSet() error: %v", err)
	}

	if err := page.SessionStorageSet("k2", "v2"); err != nil {
		t.Fatalf("SessionStorageSet() error: %v", err)
	}

	length, err := page.SessionStorageLength()
	if err != nil {
		t.Fatalf("SessionStorageLength() error: %v", err)
	}

	if length != 2 {
		t.Errorf("SessionStorageLength() = %d, want 2", length)
	}
}

func TestSaveAndLoadSession(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/set-cookie")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	if err := page.LocalStorageSet("lsk", "lsv"); err != nil {
		t.Fatalf("LocalStorageSet() error: %v", err)
	}

	if err := page.SessionStorageSet("ssk", "ssv"); err != nil {
		t.Fatalf("SessionStorageSet() error: %v", err)
	}

	state, err := page.SaveSession()
	if err != nil {
		t.Fatalf("SaveSession() error: %v", err)
	}

	if state.URL == "" {
		t.Error("SaveSession URL should not be empty")
	}

	// Test file save/load
	tmpFile := filepath.Join(os.TempDir(), "scout_test_session.json")
	defer func() { _ = os.Remove(tmpFile) }()

	if err := SaveSessionToFile(state, tmpFile); err != nil {
		t.Fatalf("SaveSessionToFile() error: %v", err)
	}

	loaded, err := LoadSessionFromFile(tmpFile)
	if err != nil {
		t.Fatalf("LoadSessionFromFile() error: %v", err)
	}

	if loaded.URL != state.URL {
		t.Errorf("loaded URL = %q, want %q", loaded.URL, state.URL)
	}

	// Test LoadSession on a new page
	page2, err := b.NewPage("")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page2.Close() }()

	if err := page2.LoadSession(loaded); err != nil {
		t.Fatalf("LoadSession() error: %v", err)
	}
}
