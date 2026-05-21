package engine

import (
	"os"
	"time"

	"github.com/inovacc/scout/internal/engine/session"
	"github.com/inovacc/scout/pkg/id"
)

// SessionInfo re-exports session.SessionInfo from sub-package.
type SessionInfo = session.SessionInfo

// SessionAttrs re-exports pkg/id.Attrs (encoded session-ID attribute prefix).
type SessionAttrs = id.Attrs

// NewSessionID builds an encoded session ID — see pkg/id.New.
func NewSessionID(a SessionAttrs) (string, error) { return id.New(a) }

// ParseSessionID decodes the attribute prefix — see pkg/id.Parse.
func ParseSessionID(s string) (SessionAttrs, error) { return id.Parse(s) }

// IsValidSessionID reports whether s parses without error.
func IsValidSessionID(s string) bool { return id.Valid(s) }
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

// WriteSessionInfo writes the session info to <SessionsDir>/<id>/scout.pid.
func WriteSessionInfo(id string, info *SessionInfo) error { return session.WriteInfo(id, info) }

// ProcessStartToken returns an opaque token uniquely identifying the process
// instance at the given PID. Used to detect PID reuse between recording a
// browser PID and killing it. Returns ("", false) when unsupported.
func ProcessStartToken(pid int) (string, bool) { return session.ProcessStartToken(pid) }

// ProcessParentPID returns the parent PID of the given process, or (0,false)
// when unsupported / not found.
func ProcessParentPID(pid int) (int, bool) { return session.ProcessParentPID(pid) }

// ProcessAlive reports whether the given PID is currently a running process.
func ProcessAlive(pid int) bool { return session.ProcessAlive(pid) }

// IsScoutProcess reports whether the given PID belongs to a live scout binary
// (basename-exact match via gops).
func IsScoutProcess(pid int) bool { return session.IsScoutProcess(pid) }

// FindBrowsersUsingDataDir scans running chrome/brave/msedge/chromium for
// processes whose --user-data-dir matches the given path. Returns the PIDs.
func FindBrowsersUsingDataDir(dataDir string) []int {
	return session.FindBrowsersUsingDataDir(dataDir)
}

// SessionLockGuard is re-exported so engine consumers can hold a session lock
// without importing the internal/engine/session subpackage directly.
type SessionLockGuard = session.LockGuard

// AcquireSessionLock takes a cross-process exclusive lock on a session via
// the M3 mkdir-based mutex. Caller must Release() to drop the lock.
func AcquireSessionLock(id string) (*SessionLockGuard, error) {
	return session.AcquireLock(id)
}

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

// preWriteStubInfo writes a minimal scout.pid immediately after the session ID
// is assigned in New(). If New() then fails before registerSession runs (e.g.
// launcher errors, bridge init fails), the dir is left with a valid scout.pid
// whose ScoutPID points to THIS process — which will exit shortly after the
// failure, marking the session as a normal orphan that CleanStaleSessions
// removes on the next startup. Without this, partial session dirs have no
// scout.pid and the cleanup logic cannot reason about them on retries.
//
// Best-effort: write errors are ignored. The real registerSession call (after
// launcher PID is known) overwrites this stub via the atomic-write path.
func preWriteStubInfo(id, browserName string, reusable, headless bool) {
	if id == "" {
		return
	}

	// Skip if a valid scout.pid already exists — reusing a session must
	// NOT clobber the existing metadata (BrowserStartToken, CreatedAt,
	// BrowserParentPID). registerSession runs the proper update path.
	if _, err := ReadSessionInfo(id); err == nil {
		return
	}

	if browserName == "" {
		browserName = "chrome"
	}

	now := time.Now()
	info := &SessionInfo{
		ScoutPID:   os.Getpid(),
		BrowserPID: 0,
		Reusable:   reusable,
		CreatedAt:  now,
		LastUsed:   now,
		Headless:   headless,
		Browser:    browserName,
	}

	EnrichSessionInfo(info)
	_ = WriteSessionInfo(id, info)
}
