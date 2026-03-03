package engine

import (
	"time"

	"github.com/inovacc/scout/internal/engine/session"
)

// sessionsDir is kept as a package-level variable for backward compatibility.
// It delegates to session.SessionsDir.
var sessionsDir = session.SessionsDir

// Type aliases for backward compatibility.
type SessionInfo = session.SessionInfo
type SessionListing = session.SessionListing

// DefaultOrphanCheckInterval is the default interval for periodic orphan checks.
const DefaultOrphanCheckInterval = session.DefaultOrphanCheckInterval

// SessionsDir returns the base directory for session data.
func SessionsDir() string { return session.GetSessionsDir() }

// SessionDir returns the directory for a given session ID.
func SessionDir(id string) string { return session.Dir(id) }

// WriteSessionInfo writes the session info as JSON to <SessionsDir>/<id>/scout.pid.
func WriteSessionInfo(id string, info *SessionInfo) error { return session.WriteInfo(id, info) }

// ReadSessionInfo reads the session info from <SessionsDir>/<id>/scout.pid.
func ReadSessionInfo(id string) (*SessionInfo, error) { return session.ReadInfo(id) }

// RemoveSessionInfo removes the scout.pid file from a session directory.
func RemoveSessionInfo(id string) { session.RemoveInfo(id) }

// ListSessions reads all <dir>/scout.pid files under SessionsDir.
func ListSessions() ([]SessionListing, error) { return session.List() }

// FindSessionByDomain looks up a session by domain hash directory name.
func FindSessionByDomain(rawURL string) *SessionListing { return session.FindByDomain(rawURL) }

// FindReusableSession scans session dirs for a matching reusable session.
func FindReusableSession(browser string, headless bool) *SessionListing {
	return session.FindReusable(browser, headless)
}

// CleanOrphans scans for orphaned browser processes and kills them.
func CleanOrphans() (int, error) { return session.CleanOrphans() }

// ResetSession removes an entire session directory.
func ResetSession(id string) error { return session.Reset(id) }

// ResetAllSessions removes all session directories.
func ResetAllSessions() (int, error) { return session.ResetAll() }

// StartOrphanWatchdog starts a background goroutine for periodic orphan cleanup.
func StartOrphanWatchdog(interval time.Duration, done <-chan struct{}) {
	session.StartOrphanWatchdog(interval, done)
}

// RootDomain extracts the root domain from a URL.
func RootDomain(rawURL string) string { return session.RootDomain(rawURL) }

// DomainHash returns a short SHA-256 hash of the root domain.
func DomainHash(rawURL string) string { return session.DomainHash(rawURL) }

// SessionHash returns a deterministic hash for a session directory name.
func SessionHash(rawURL, label string) string { return session.Hash(rawURL, label) }
