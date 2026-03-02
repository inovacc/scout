package scout

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteReadSessionInfo(t *testing.T) {
	dir := t.TempDir()
	origFunc := sessionsDir
	sessionsDir = func() string { return dir }

	defer func() { sessionsDir = origFunc }()

	id := DomainHash("https://example.com")

	info := &SessionInfo{
		ScoutPID:   1234,
		BrowserPID: 5678,
		Reusable:   true,
		CreatedAt:  time.Now().Truncate(time.Second),
		LastUsed:   time.Now().Truncate(time.Second),
		Headless:   true,
		Browser:    "chrome",
		DomainHash: id,
		Domain:     "example.com",
	}

	if err := WriteSessionInfo(id, info); err != nil {
		t.Fatalf("WriteSessionInfo: %v", err)
	}

	// Verify file exists.
	pidPath := filepath.Join(dir, id, "scout.pid")
	if _, err := os.Stat(pidPath); err != nil {
		t.Fatalf("scout.pid not found: %v", err)
	}

	// Read back.
	got, err := ReadSessionInfo(id)
	if err != nil {
		t.Fatalf("ReadSessionInfo: %v", err)
	}

	if got.ScoutPID != 1234 || got.BrowserPID != 5678 {
		t.Fatalf("PIDs mismatch: %+v", got)
	}

	if !got.Reusable || got.Browser != "chrome" || !got.Headless {
		t.Fatalf("metadata mismatch: %+v", got)
	}

	if got.Domain != "example.com" {
		t.Fatalf("domain mismatch: %s", got.Domain)
	}
}

func TestListSessions(t *testing.T) {
	dir := t.TempDir()
	origFunc := sessionsDir
	sessionsDir = func() string { return dir }

	defer func() { sessionsDir = origFunc }()

	// No sessions yet.
	sessions, err := ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}

	if len(sessions) != 0 {
		t.Fatalf("expected 0, got %d", len(sessions))
	}

	// Add two sessions using domain hashes.
	id1 := DomainHash("https://example.com")
	id2 := DomainHash("https://other.com")
	now := time.Now()

	_ = WriteSessionInfo(id1, &SessionInfo{Browser: "chrome", CreatedAt: now, LastUsed: now, Reusable: true, Headless: true, DomainHash: id1, Domain: "example.com"})
	_ = WriteSessionInfo(id2, &SessionInfo{Browser: "edge", CreatedAt: now, LastUsed: now, DomainHash: id2, Domain: "other.com"})

	sessions, err = ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}

	if len(sessions) != 2 {
		t.Fatalf("expected 2, got %d", len(sessions))
	}

	// FindReusableSession.
	found := FindReusableSession("chrome", true)
	if found == nil {
		t.Fatal("expected to find reusable session")
	}

	if found.ID != id1 {
		t.Fatalf("expected %s, got %s", id1, found.ID)
	}

	notFound := FindReusableSession("firefox", true)
	if notFound != nil {
		t.Fatal("expected no match for firefox")
	}
}

func TestRemoveSessionInfo(t *testing.T) {
	dir := t.TempDir()
	origFunc := sessionsDir
	sessionsDir = func() string { return dir }

	defer func() { sessionsDir = origFunc }()

	id := DomainHash("https://example.com")
	now := time.Now()
	_ = WriteSessionInfo(id, &SessionInfo{Browser: "chrome", CreatedAt: now, LastUsed: now})

	RemoveSessionInfo(id)

	if _, err := ReadSessionInfo(id); !os.IsNotExist(err) {
		t.Fatalf("expected not-exist error, got: %v", err)
	}
}

func TestRootDomain(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://www.seoptimer.com/page", "seoptimer.com"},
		{"https://sub.admin.mysite.com/path", "mysite.com"},
		{"https://admin.mysite.com", "mysite.com"},
		{"https://mysite.com", "mysite.com"},
		{"https://app.mysite.co.uk/path", "mysite.co.uk"},
		{"https://deep.sub.mysite.co.uk", "mysite.co.uk"},
		{"https://192.168.1.1:8080/path", "192.168.1.1"},
		{"", ""},
		{"mysite.com", "mysite.com"},
	}

	for _, tt := range tests {
		got := RootDomain(tt.input)
		if got != tt.want {
			t.Errorf("RootDomain(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestDomainHash(t *testing.T) {
	// Same root domain → same hash.
	h1 := DomainHash("https://sub.mysite.com")
	h2 := DomainHash("https://admin.mysite.com")
	h3 := DomainHash("https://mysite.com/path")

	if h1 != h2 {
		t.Errorf("sub.mysite.com and admin.mysite.com should have same hash: %s vs %s", h1, h2)
	}

	if h1 != h3 {
		t.Errorf("sub.mysite.com and mysite.com should have same hash: %s vs %s", h1, h3)
	}

	// Different domain → different hash.
	h4 := DomainHash("https://other.com")
	if h1 == h4 {
		t.Error("mysite.com and other.com should have different hashes")
	}
}

func TestFindByDomain(t *testing.T) {
	dir := t.TempDir()
	origFunc := sessionsDir
	sessionsDir = func() string { return dir }

	defer func() { sessionsDir = origFunc }()

	hash := DomainHash("https://mysite.com")
	now := time.Now()

	// Dir name IS the domain hash — direct lookup.
	_ = WriteSessionInfo(hash, &SessionInfo{
		Browser:    "chrome",
		CreatedAt:  now,
		LastUsed:   now,
		DomainHash: hash,
		Domain:     "mysite.com",
	})

	// Should find by subdomain of same root.
	found := FindSessionByDomain("https://admin.mysite.com/page")
	if found == nil {
		t.Fatal("expected to find session by domain")
	}

	if found.ID != hash {
		t.Fatalf("expected %s, got %s", hash, found.ID)
	}

	// Should not find for different domain.
	notFound := FindSessionByDomain("https://other.com")
	if notFound != nil {
		t.Fatal("expected no match for other.com")
	}
}
