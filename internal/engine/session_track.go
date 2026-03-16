package engine

import (
	"time"

	"github.com/inovacc/scout/internal/engine/session"
)

// SessionInfo re-exports session.SessionInfo from sub-package.
type SessionInfo = session.SessionInfo
type SessionListing = session.SessionListing
type SessionJob = session.Job
type SessionJobStep = session.JobStep
type SessionJobStatus = session.JobStatus
type SessionJobProgress = session.Progress

// DefaultOrphanCheckInterval is the default interval for periodic orphan checks.
const DefaultOrphanCheckInterval = session.DefaultOrphanCheckInterval

// SessionsDir returns the base directory for session data.
func SessionsDir() string { return session.GetSessionsDir() }

// SessionDir returns the directory for a given session ID.
func SessionDir(id string) string { return session.Dir(id) }

// SessionDataDir returns the browser user-data directory for a given session ID.
func SessionDataDir(id string) string { return session.DataDir(id) }

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

// CleanStaleSessions removes leftover session directories on startup.
func CleanStaleSessions() (int, error) { return session.CleanStaleSessions() }

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

// NewSessionJob creates a new Job with a KSUID, pending status, and timestamps.
func NewSessionJob(jobType string, targetURLs []string, command string) *SessionJob {
	return session.NewJob(jobType, targetURLs, command)
}

// WriteSessionJob writes the job as JSON to <SessionsDir>/<sessionID>/job.json.
func WriteSessionJob(sessionID string, job *SessionJob) error {
	return session.WriteJob(sessionID, job)
}

// ReadSessionJob reads the job from <SessionsDir>/<sessionID>/job.json.
func ReadSessionJob(sessionID string) (*SessionJob, error) { return session.ReadJob(sessionID) }

// RemoveSessionJob removes the job.json file from a session directory.
func RemoveSessionJob(sessionID string) error { return session.RemoveJob(sessionID) }

// StartSessionJob transitions a job from pending to running.
func StartSessionJob(sessionID string) error { return session.StartJob(sessionID) }

// CompleteSessionJob marks a job as completed with output and timestamp.
func CompleteSessionJob(sessionID string, output string) error {
	return session.CompleteJob(sessionID, output)
}

// FailSessionJob marks a job as failed with an error message and timestamp.
func FailSessionJob(sessionID string, errMsg string) error {
	return session.FailJob(sessionID, errMsg)
}

// AddSessionJobStep appends a step to the job and auto-updates progress.
func AddSessionJobStep(sessionID string, step SessionJobStep) error {
	return session.AddJobStep(sessionID, step)
}

// UpdateSessionJobProgress updates the progress fields on a job.
func UpdateSessionJobProgress(sessionID string, current, total int, message string) error {
	return session.UpdateJobProgress(sessionID, current, total, message)
}

// EnrichSessionInfo populates Exec and BuildVersion from gops if available.
func EnrichSessionInfo(info *SessionInfo) {
	if p := session.ScoutProcessInfo(info.ScoutPID); p != nil {
		info.Exec = p.Exec
		info.BuildVersion = p.BuildVersion
	}
}
