package capture

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestImportMergesPerSiteAndDeletes(t *testing.T) {
	v, dir := newTempVault(t)
	pub, _ := InitKeypair(v, filepath.Join(dir, "capture.pub"), false)
	spoolDir := filepath.Join(dir, "spool")

	login, _ := json.Marshal(Msg{V: 1, Type: "capture_login", Site: "example.com", Username: "alice", Password: "hunter2"})
	sess, _ := json.Marshal(Msg{V: 1, Type: "capture_session", Site: "example.com",
		Cookies: []WireCookie{{Name: "sid", Value: "v", Domain: "example.com", Path: "/"}}})
	if _, err := WriteSpool(spoolDir, pub, login); err != nil {
		t.Fatalf("WriteSpool login: %v", err)
	}
	if _, err := WriteSpool(spoolDir, pub, sess); err != nil {
		t.Fatalf("WriteSpool session: %v", err)
	}

	priv, _ := LoadPriv(v)
	report, err := ImportSpool(v, spoolDir, pub, priv)
	if err != nil {
		t.Fatalf("ImportSpool: %v", err)
	}
	if report.Imported != 2 {
		t.Fatalf("Imported = %d, want 2", report.Imported)
	}

	// One per-site profile named example.com with cookie + login secret.
	var id string
	for _, m := range v.List() {
		if m.Name == "example.com" {
			id = m.ID
		}
	}
	if id == "" {
		t.Fatal("no example.com profile created")
	}
	sp, _ := v.Get(id)
	defer sp.Close()
	if len(sp.Cookies) != 1 {
		t.Errorf("cookies = %d, want 1", len(sp.Cookies))
	}
	lb, ok := sp.Secrets["login:alice"]
	if !ok || !lb.Equal([]byte("hunter2")) {
		t.Errorf("login secret missing/wrong")
	}

	// Spool emptied.
	files, _ := ListSpool(spoolDir)
	if len(files) != 0 {
		t.Errorf("spool not drained: %v", files)
	}
}

func TestImportQuarantinesUndecryptable(t *testing.T) {
	v, dir := newTempVault(t)
	pub, _ := InitKeypair(v, filepath.Join(dir, "capture.pub"), false)
	spoolDir := filepath.Join(dir, "spool")
	_ = os.MkdirAll(spoolDir, 0o700)
	_ = os.WriteFile(filepath.Join(spoolDir, "junk.cap"), []byte("not a sealed box"), 0o600)

	priv, _ := LoadPriv(v)
	report, err := ImportSpool(v, spoolDir, pub, priv)
	if err != nil {
		t.Fatalf("ImportSpool: %v", err)
	}
	if report.Quarantined != 1 || report.Imported != 0 {
		t.Fatalf("report = %+v, want Quarantined=1 Imported=0", report)
	}
	if _, err := os.Stat(filepath.Join(spoolDir, "junk.cap.bad")); err != nil {
		t.Errorf("undecryptable file not quarantined: %v", err)
	}
}
