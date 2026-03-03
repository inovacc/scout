package session

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteReadSessionInfo(t *testing.T) {
	dir := t.TempDir()
	origFunc := SessionsDir
	SessionsDir = func() string { return dir }

	defer func() { SessionsDir = origFunc }()

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

	if err := WriteInfo(id, info); err != nil {
		t.Fatalf("WriteInfo: %v", err)
	}

	// Verify file exists.
	pidPath := filepath.Join(dir, id, "scout.pid")
	if _, err := os.Stat(pidPath); err != nil {
		t.Fatalf("scout.pid not found: %v", err)
	}

	// Read back.
	got, err := ReadInfo(id)
	if err != nil {
		t.Fatalf("ReadInfo: %v", err)
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
	origFunc := SessionsDir
	SessionsDir = func() string { return dir }

	defer func() { SessionsDir = origFunc }()

	// No sessions yet.
	sessions, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(sessions) != 0 {
		t.Fatalf("expected 0, got %d", len(sessions))
	}

	// Add two sessions using domain hashes.
	id1 := DomainHash("https://example.com")
	id2 := DomainHash("https://other.com")
	now := time.Now()

	_ = WriteInfo(id1, &SessionInfo{Browser: "chrome", CreatedAt: now, LastUsed: now, Reusable: true, Headless: true, DomainHash: id1, Domain: "example.com"})
	_ = WriteInfo(id2, &SessionInfo{Browser: "edge", CreatedAt: now, LastUsed: now, DomainHash: id2, Domain: "other.com"})

	sessions, err = List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(sessions) != 2 {
		t.Fatalf("expected 2, got %d", len(sessions))
	}

	// FindReusable.
	found := FindReusable("chrome", true)
	if found == nil {
		t.Fatal("expected to find reusable session")
	}

	if found.ID != id1 {
		t.Fatalf("expected %s, got %s", id1, found.ID)
	}

	notFound := FindReusable("firefox", true)
	if notFound != nil {
		t.Fatal("expected no match for firefox")
	}
}

func TestRemoveSessionInfo(t *testing.T) {
	dir := t.TempDir()
	origFunc := SessionsDir
	SessionsDir = func() string { return dir }

	defer func() { SessionsDir = origFunc }()

	id := DomainHash("https://example.com")
	now := time.Now()
	_ = WriteInfo(id, &SessionInfo{Browser: "chrome", CreatedAt: now, LastUsed: now})

	RemoveInfo(id)

	if _, err := ReadInfo(id); !os.IsNotExist(err) {
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

func TestHashIncludesBrowserName(t *testing.T) {
	url := "https://example.com"

	// Same URL, different browsers → different hashes.
	hChrome := Hash(url, "chrome")
	hBrave := Hash(url, "brave")
	hEdge := Hash(url, "edge")

	if hChrome == hBrave {
		t.Errorf("chrome and brave should have different hashes for same URL: %s", hChrome)
	}
	if hChrome == hEdge {
		t.Errorf("chrome and edge should have different hashes for same URL: %s", hChrome)
	}
	if hBrave == hEdge {
		t.Errorf("brave and edge should have different hashes for same URL: %s", hBrave)
	}

	// Same browser, same URL → stable hash.
	if h2 := Hash(url, "chrome"); h2 != hChrome {
		t.Errorf("Hash should be deterministic: %s vs %s", hChrome, h2)
	}

	// No URL → hash of label only; different labels → different hashes.
	hNoURL1 := Hash("", "chrome")
	hNoURL2 := Hash("", "brave")
	if hNoURL1 == hNoURL2 {
		t.Errorf("different labels without URL should produce different hashes: %s", hNoURL1)
	}
}

func TestFindByDomain(t *testing.T) {
	dir := t.TempDir()
	origFunc := SessionsDir
	SessionsDir = func() string { return dir }

	defer func() { SessionsDir = origFunc }()

	hash := DomainHash("https://mysite.com")
	now := time.Now()

	// Dir name IS the domain hash — direct lookup.
	_ = WriteInfo(hash, &SessionInfo{
		Browser:    "chrome",
		CreatedAt:  now,
		LastUsed:   now,
		DomainHash: hash,
		Domain:     "mysite.com",
	})

	// Should find by subdomain of same root.
	found := FindByDomain("https://admin.mysite.com/page")
	if found == nil {
		t.Fatal("expected to find session by domain")
	}

	if found.ID != hash {
		t.Fatalf("expected %s, got %s", hash, found.ID)
	}

	// Should not find for different domain.
	notFound := FindByDomain("https://other.com")
	if notFound != nil {
		t.Fatal("expected no match for other.com")
	}
}
