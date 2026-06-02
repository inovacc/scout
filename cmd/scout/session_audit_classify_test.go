package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/inovacc/scout/internal/engine/session"
)

// TestClassifySessionExpiredWithLiveBrowser proves that a reusable+expired
// session whose browser PID is still alive is classified as ZOMBIE (not
// EXPIRED). This is the "false-clean doctor" regression: previously the
// expired arm fired before the zombie arm, so a live orphan browser was
// invisible to `scout session doctor`.
func TestClassifySessionExpiredWithLiveBrowser(t *testing.T) {
	dir := withTempSessions(t)

	id := "test-expired-zombie"
	sessDir := filepath.Join(dir, id)
	if err := os.MkdirAll(sessDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Write a session that is:
	//  - Reusable = true
	//  - ExpiresAt in the past (expired)
	//  - ScoutPID = 0 (scout dead / not recorded)
	//  - BrowserPID = os.Getpid() (current process is "alive" for this test
	//    without needing a real browser process — ProcessAlive(os.Getpid())
	//    returns true on all platforms)
	info := &session.SessionInfo{
		ScoutPID:   0,
		BrowserPID: os.Getpid(),
		Reusable:   true,
		CreatedAt:  time.Now().Add(-2 * time.Hour),
		ExpiresAt:  time.Now().Add(-1 * time.Hour), // already expired
		Browser:    "chrome",
	}
	if err := session.WriteInfo(id, info); err != nil {
		t.Fatalf("WriteInfo: %v", err)
	}

	e := classifySession(id)

	if e.Status != statusZombie {
		t.Errorf("classifySession: got status %q, want %q (expired+live-orphan must be ZOMBIE)", e.Status, statusZombie)
	}
}

// TestClassifySessionExpiredNoLiveBrowser confirms that a reusable+expired
// session whose browser is NOT alive (both processes dead) stays EXPIRED, not
// ZOMBIE. Ensures the zombie-first reorder doesn't over-classify dead sessions.
func TestClassifySessionExpiredNoLiveBrowser(t *testing.T) {
	dir := withTempSessions(t)

	id := "test-expired-dead"
	sessDir := filepath.Join(dir, id)
	if err := os.MkdirAll(sessDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	info := &session.SessionInfo{
		ScoutPID:   0,
		BrowserPID: 0, // no browser recorded → BrowserAlive = false
		Reusable:   true,
		CreatedAt:  time.Now().Add(-2 * time.Hour),
		ExpiresAt:  time.Now().Add(-1 * time.Hour),
		Browser:    "chrome",
	}
	if err := session.WriteInfo(id, info); err != nil {
		t.Fatalf("WriteInfo: %v", err)
	}

	e := classifySession(id)

	if e.Status != statusExpired {
		t.Errorf("classifySession: got status %q, want %q (expired+dead-browser must be EXPIRED)", e.Status, statusExpired)
	}
}

// TestClassifySessionReusableNotExpiredDeadOwner confirms that a reusable
// session that has NOT expired, with both ScoutPID=0 and BrowserPID=0 (both
// dead / not recorded), is classified as REUSABLE — available for a new daemon
// to adopt. Pins the "owner died but still within expiry window" path and
// confirms the zombie reorder does not over-classify sessions with no live
// browser.
func TestClassifySessionReusableNotExpiredDeadOwner(t *testing.T) {
	dir := withTempSessions(t)

	id := "test-reusable-dead-owner"
	sessDir := filepath.Join(dir, id)
	if err := os.MkdirAll(sessDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	info := &session.SessionInfo{
		ScoutPID:   0, // owner gone
		BrowserPID: 0, // browser not recorded (dead)
		Reusable:   true,
		CreatedAt:  time.Now().Add(-30 * time.Minute),
		ExpiresAt:  time.Now().Add(30 * time.Minute), // still within window
		Browser:    "chrome",
	}
	if err := session.WriteInfo(id, info); err != nil {
		t.Fatalf("WriteInfo: %v", err)
	}

	e := classifySession(id)

	// ScoutAlive=false, BrowserAlive=false → zombie arm does NOT fire.
	// Reusable + not expired + no live browser → REUSABLE.
	if e.Status == statusZombie {
		t.Errorf("classifySession: got ZOMBIE for dead-owner reusable session within expiry window — must not over-classify")
	}
	if e.Status != statusReusable {
		t.Errorf("classifySession: got %q, want REUSABLE for dead-owner within expiry window", e.Status)
	}
}
