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

func TestIsIP(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"192.168.1.1", true},
		{"10.0.0.1", true},
		{"0.0.0.0", true},
		{"255.255.255.255", true},
		{"1.2.3.4", true},
		{"localhost", false},
		{"example.com", false},
		{"", false},
		{"abc", false},
		{"192.168.1", true},  // technically valid per function's simple heuristic
		{"1234", false},      // no dot
		{"12.ab.34.cd", false},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := IsIP(tc.input)
			if got != tc.want {
				t.Errorf("IsIP(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestGetSessionsDir(t *testing.T) {
	dir := GetSessionsDir()
	if dir == "" {
		t.Fatal("GetSessionsDir() returned empty")
	}

	// Override and verify.
	origFunc := SessionsDir
	SessionsDir = func() string { return "/custom/path" }

	defer func() { SessionsDir = origFunc }()

	if got := GetSessionsDir(); got != "/custom/path" {
		t.Errorf("GetSessionsDir() = %q after override, want /custom/path", got)
	}
}

func TestDir(t *testing.T) {
	dir := t.TempDir()
	origFunc := SessionsDir
	SessionsDir = func() string { return dir }

	defer func() { SessionsDir = origFunc }()

	got := Dir("test-session")
	want := filepath.Join(dir, "test-session")

	if got != want {
		t.Errorf("Dir(test-session) = %q, want %q", got, want)
	}
}

func TestReset(t *testing.T) {
	dir := t.TempDir()
	origFunc := SessionsDir
	SessionsDir = func() string { return dir }

	defer func() { SessionsDir = origFunc }()

	// Create a session.
	id := "test-reset-session"
	now := time.Now()

	_ = WriteInfo(id, &SessionInfo{
		Browser:   "chrome",
		CreatedAt: now,
		LastUsed:  now,
	})

	// Add extra files to simulate real session data.
	_ = os.WriteFile(filepath.Join(Dir(id), "extra.dat"), []byte("data"), 0o644)

	// Reset should remove entire directory.
	if err := Reset(id); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	if _, err := os.Stat(Dir(id)); !os.IsNotExist(err) {
		t.Fatalf("session dir still exists after Reset")
	}

	// Reset nonexistent session should error.
	if err := Reset("nonexistent"); err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestResetAll(t *testing.T) {
	dir := t.TempDir()
	origFunc := SessionsDir
	SessionsDir = func() string { return dir }

	defer func() { SessionsDir = origFunc }()

	now := time.Now()

	_ = WriteInfo("s1", &SessionInfo{Browser: "chrome", CreatedAt: now, LastUsed: now})
	_ = WriteInfo("s2", &SessionInfo{Browser: "edge", CreatedAt: now, LastUsed: now})
	_ = WriteInfo("s3", &SessionInfo{Browser: "brave", CreatedAt: now, LastUsed: now})

	removed, err := ResetAll()
	if err != nil {
		t.Fatalf("ResetAll: %v", err)
	}

	if removed != 3 {
		t.Errorf("ResetAll removed %d, want 3", removed)
	}

	// Verify all gone.
	sessions, _ := List()
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions after ResetAll, got %d", len(sessions))
	}
}

func TestCleanOrphans(t *testing.T) {
	dir := t.TempDir()
	origFunc := SessionsDir
	SessionsDir = func() string { return dir }

	defer func() { SessionsDir = origFunc }()

	// Create a session with PIDs that don't exist (orphan scenario).
	now := time.Now()

	_ = WriteInfo("orphan", &SessionInfo{
		ScoutPID:   999999999, // non-existent
		BrowserPID: 999999998, // non-existent
		Browser:    "chrome",
		CreatedAt:  now,
		LastUsed:   now,
	})

	// Session with zero PIDs should be skipped.
	_ = WriteInfo("zero-pids", &SessionInfo{
		ScoutPID:   0,
		BrowserPID: 0,
		Browser:    "chrome",
		CreatedAt:  now,
		LastUsed:   now,
	})

	killed, err := CleanOrphans()
	if err != nil {
		t.Fatalf("CleanOrphans: %v", err)
	}

	// The orphan's scout PID is dead, so its info should be removed.
	// Browser PID is also dead, so killed count depends on ProcessAlive.
	t.Logf("CleanOrphans killed=%d", killed)

	// The "orphan" info file should be removed (scout PID is dead).
	if _, err := ReadInfo("orphan"); !os.IsNotExist(err) {
		t.Error("expected orphan info to be removed")
	}

	// The "zero-pids" session should still exist (skipped).
	if _, err := ReadInfo("zero-pids"); err != nil {
		t.Error("zero-pids session should not be touched")
	}
}

func TestStartOrphanWatchdog(t *testing.T) {
	done := make(chan struct{})

	// Start with short interval, verify it doesn't panic.
	StartOrphanWatchdog(10*time.Millisecond, done)

	// Let it tick a couple times.
	time.Sleep(50 * time.Millisecond)

	// Stop it.
	close(done)

	// Give goroutine time to exit.
	time.Sleep(20 * time.Millisecond)
}

func TestProcessAlive(t *testing.T) {
	// Current process should be alive.
	if !ProcessAlive(os.Getpid()) {
		t.Error("ProcessAlive(os.Getpid()) should be true")
	}

	// Non-existent PID should not be alive.
	if ProcessAlive(999999999) {
		t.Error("ProcessAlive(999999999) should be false")
	}
}

func TestHashEmptyLabel(t *testing.T) {
	// Empty label should default to "default".
	h1 := Hash("https://example.com", "")
	h2 := Hash("https://example.com", "default")

	if h1 != h2 {
		t.Errorf("Hash with empty label should equal Hash with 'default': %s vs %s", h1, h2)
	}
}

func TestDomainHashEmpty(t *testing.T) {
	if got := DomainHash(""); got != "" {
		t.Errorf("DomainHash('') = %q, want empty", got)
	}

	if got := DomainHash("not a url %%"); got != "" {
		t.Errorf("DomainHash(invalid) = %q, want empty", got)
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
