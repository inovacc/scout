package session

import (
	"sync"
	"testing"
	"time"

	idpkg "github.com/inovacc/scout/pkg/id"
)

// writeReusableSession creates a reusable session whose ID encodes Browser and
// Headless (which ReadInfo treats as authoritative, overwriting the binary
// body), so FindReusable's Browser/Headless match works as in production.
func writeReusableSession(t *testing.T, browser string, headless bool, expiresAt time.Time) string {
	t.Helper()

	sid, err := idpkg.New(idpkg.Attrs{Browser: browser, Headless: headless, Reusable: true})
	if err != nil {
		t.Fatalf("idpkg.New: %v", err)
	}

	now := time.Now()
	if err := WriteInfo(sid, &SessionInfo{
		Reusable:  true,
		ExpiresAt: expiresAt,
		CreatedAt: now,
		LastUsed:  now,
		Browser:   browser,
		Headless:  headless,
	}); err != nil {
		t.Fatalf("WriteInfo: %v", err)
	}

	return sid
}

// TestFindReusable_SkipsExpired proves a launch never adopts an expired reusable
// session (which the reaper is about to kill — adopting it would kill the
// launching browser).
func TestFindReusable_SkipsExpired(t *testing.T) {
	dir := t.TempDir()
	orig := SessionsDir
	SessionsDir = func() string { return dir }
	t.Cleanup(func() { SessionsDir = orig })

	now := time.Now()

	// Expired reusable chrome session: must NOT be returned.
	writeReusableSession(t, "chrome", true, now.Add(-1*time.Hour))

	if got := FindReusable("chrome", true); got != nil {
		t.Fatalf("FindReusable returned an expired session: %+v", got.Info)
	}

	// A non-expired one IS found.
	writeReusableSession(t, "chrome", true, now.Add(1*time.Hour))

	if got := FindReusable("chrome", true); got == nil {
		t.Fatal("FindReusable did not return a live (unexpired) reusable session")
	}
}

// TestWriteInfo_AtomicNoTornRead hammers WriteInfo concurrently with ReadInfo.
// With the in-place O_TRUNC write a reader could observe a truncated/partial
// record; the atomic temp+rename write means a reader always sees a complete
// file. Run under -race to also catch unsynchronized access.
func TestWriteInfo_AtomicNoTornRead(t *testing.T) {
	dir := t.TempDir()
	orig := SessionsDir
	SessionsDir = func() string { return dir }
	t.Cleanup(func() { SessionsDir = orig })

	sid, err := idpkg.New(idpkg.Attrs{})
	if err != nil {
		t.Fatalf("idpkg.New: %v", err)
	}

	// Seed a complete record so the reader always has a file to read.
	if err := WriteInfo(sid, &SessionInfo{ScoutPID: 1234, Browser: "chrome", CreatedAt: time.Now(), LastUsed: time.Now()}); err != nil {
		t.Fatalf("seed WriteInfo: %v", err)
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 400; i++ {
			// Fresh struct each iteration to avoid racing on shared fields.
			_ = WriteInfo(sid, &SessionInfo{ScoutPID: 1234, BrowserPID: i, Browser: "chrome", CreatedAt: time.Now(), LastUsed: time.Now()})
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 400; i++ {
			out, err := ReadInfo(sid)
			if err != nil {
				t.Errorf("ReadInfo saw a torn/corrupt record: %v", err)
				return
			}
			if out.ScoutPID != 1234 {
				t.Errorf("ReadInfo returned wrong ScoutPID %d (torn read?)", out.ScoutPID)
				return
			}
		}
	}()

	wg.Wait()
}
