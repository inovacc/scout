package session

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/net/publicsuffix"

	"github.com/inovacc/scout/internal/engine/scouthome"
	idpkg "github.com/inovacc/scout/pkg/id"
)

const (
	// removeRetries is the number of attempts to remove a session directory.
	// Chrome holds file handles briefly after exit on Windows, and Windows
	// Defender / Search Indexer can hold LevelDB files for 5–15 seconds while
	// scanning. This budget outlasts both.
	removeRetries = 8
	// removeRetryWait is the initial backoff between retries. Exponential
	// (×2) with a per-step cap of removeRetryWaitMax.
	removeRetryWait    = 50 * time.Millisecond
	removeRetryWaitMax = 5 * time.Second
)

// retryRemoveAll retries os.RemoveAll with exponential backoff (capped).
// Returns the final error after exhausting removeRetries attempts (nil on
// success). Total budget ≈ 50+100+200+400+800+1600+3200+5000 ms ≈ 11 s.
// Hardening L1 (extended for Windows AV interaction).
func retryRemoveAll(path string) error {
	var lastErr error

	wait := removeRetryWait
	for range removeRetries {
		lastErr = os.RemoveAll(path)
		if lastErr == nil {
			return nil
		}

		time.Sleep(wait)

		wait *= 2
		if wait > removeRetryWaitMax {
			wait = removeRetryWaitMax
		}
	}

	return lastErr
}

// SessionsDir is the function that returns the base directory for session data.
// It is a variable so tests can override it.
var SessionsDir = defaultSessionsDir

// dirOwnerCheck is the function used to validate session-dir ownership.
// Exposed as a package var so tests can inject a stub. Defaults to the
// platform implementation in owner_{unix,windows}.go.
var dirOwnerCheck = validateDirOwner

// defaultSessionsDir resolves the per-user sessions directory via the
// centralized scouthome resolver (M2 + H5). Returns "" on resolve failure so
// downstream MkdirAll surfaces a clear error rather than silently writing to
// a world-readable shared location.
func defaultSessionsDir() string {
	sub, err := scouthome.Sub("sessions")
	if err != nil {
		return ""
	}

	return sub
}

// GetSessionsDir returns the base directory for session data: ~/.scout/sessions.
// This is cross-platform: it uses os.UserHomeDir which resolves to
// %USERPROFILE% on Windows, $HOME on Unix/macOS.
func GetSessionsDir() string {
	return SessionsDir()
}

// SessionInfo holds all metadata for a browser session, stored as scout.pid
// inside the session's directory using the fixed-width binary v1 layout
// defined in info_binary.go. JSON tags are retained for ad-hoc tooling
// (e.g. `scout session audit --json` still emits JSON) but on-disk storage
// is binary.
type SessionInfo struct {
	ScoutPID          int       `json:"scout_pid"`
	BrowserPID        int       `json:"browser_pid"`
	BrowserParentPID  int       `json:"browser_parent_pid,omitempty"`
	BrowserStartToken string    `json:"browser_start_token,omitempty"`
	Reusable          bool      `json:"reusable"`
	CreatedAt         time.Time `json:"created_at"`
	LastUsed          time.Time `json:"last_used"`
	// ExpiresAt is REQUIRED for Reusable sessions — persistent sessions
	// must have a bounded lifetime so they don't accumulate forever. Zero
	// for non-reusable (ephemeral) sessions.
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	Headless     bool      `json:"headless"`
	Browser      string    `json:"browser"`
	DomainHash   string    `json:"domain_hash,omitempty"`
	Domain       string    `json:"domain,omitempty"`
	Exec         string    `json:"exec,omitempty"`
	BuildVersion string    `json:"build_version,omitempty"`
}

// DefaultReusableLifetime is the default expiration window for reusable
// sessions when callers don't specify one. Seven days is long enough for
// typical multi-day workflows but short enough that abandoned sessions get
// cleaned eventually.
const DefaultReusableLifetime = 7 * 24 * time.Hour

// IsExpired reports whether a reusable session has passed its expiration.
// Returns false for non-reusable sessions (they're cleaned on Close) and
// for reusable sessions with no ExpiresAt set (legacy data — treated as
// not-yet-expired to avoid surprise deletion).
func (s *SessionInfo) IsExpired() bool {
	if s == nil || !s.Reusable {
		return false
	}

	if s.ExpiresAt.IsZero() {
		return false
	}

	return time.Now().After(s.ExpiresAt)
}

// SessionListing pairs a session ID with its directory and info.
type SessionListing struct {
	ID   string
	Dir  string
	Info *SessionInfo
	Job  *Job
}

// Dir returns the directory for a given session ID.
func Dir(id string) string {
	return filepath.Join(GetSessionsDir(), id)
}

// DataDir returns the browser user-data directory for a given session ID.
// This is the subdirectory where Chrome stores its profile data, separated
// from session metadata (scout.pid, job.json) at the parent level.
func DataDir(id string) string {
	return filepath.Join(GetSessionsDir(), id, "data")
}

// WriteInfo writes the session info as a fixed-width binary v1 record to
// <SessionsDir>/<id>/scout.pid. Writes happen in place (truncate + write)
// rather than via atomic rename so the same file can carry an OS-level
// advisory lock (LockFileEx / flock) for concurrent-access safety.
//
// Mode 0o600/0o700 keep other local users out of Chrome's cookie database
// or scraper auth tokens stored under the session's data/ subdir.
//
// Hardening M1 — see docs/quality/SESSION_HARDENING.md.
func WriteInfo(id string, info *SessionInfo) error {
	dir := Dir(id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("scout: create session dir: %w", err)
	}

	// Chmod after MkdirAll so the mode is umask-independent. No-op on
	// Windows (ACLs handle permissions there).
	_ = os.Chmod(dir, 0o700)

	path := filepath.Join(dir, "scout.pid")

	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("scout: open scout.pid: %w", err)
	}

	if _, err := f.Write(marshalBinary(info)); err != nil {
		_ = f.Close()
		return fmt.Errorf("scout: write scout.pid: %w", err)
	}

	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("scout: sync scout.pid: %w", err)
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("scout: close scout.pid: %w", err)
	}

	return nil
}


// ReadInfo reads the session info from <SessionsDir>/<id>/scout.pid in the
// v1 binary format. Legacy JSON-format files yield ErrLegacyFormat so the
// startup purge can identify and remove them under hard cutover.
//
// Refuses to return data from a session directory that is not owned by the
// current user — H4 hardening against pre-planted directories with predictable
// hash names. When the directory does not exist, returns the underlying
// os.IsNotExist error so callers (CleanOrphans, FindByDomain) can distinguish
// "absent" from "foreign".
func ReadInfo(id string) (*SessionInfo, error) {
	dir := Dir(id)

	// Stat first so a missing dir yields os.IsNotExist semantics rather than
	// the ownership-check error.
	if _, err := os.Stat(dir); err != nil {
		return nil, err
	}

	if !dirOwnerCheck(dir) {
		return nil, fmt.Errorf("scout: session %s: directory not owned by current user", id)
	}

	data, err := os.ReadFile(filepath.Join(dir, "scout.pid"))
	if err != nil {
		return nil, err
	}

	info, err := unmarshalBinary(data)
	if err != nil {
		return nil, err
	}

	// Browser, Reusable, Headless are authoritative in the session ID's
	// attribute prefix, not in the scout.pid body. Overwrite the binary's
	// values so they're consistent even if a future writer skips them or
	// records a stale snapshot. Legacy / non-v1 IDs leave info untouched.
	if attrs, perr := idpkg.Parse(id); perr == nil {
		info.Browser = attrs.Browser
		info.Reusable = attrs.Reusable
		info.Headless = attrs.Headless
	}

	return info, nil
}

// RemoveInfo removes the scout.pid file from a session directory.
func RemoveInfo(id string) {
	_ = os.Remove(filepath.Join(Dir(id), "scout.pid"))
}

// List reads all <dir>/scout.pid files under SessionsDir.
//
// Per M5: distinguishes missing scout.pid (orphan dir, silently skipped) from
// corrupt scout.pid (logged via slog so operators can detect tampering or
// crash damage) from foreign-owned directories (also logged — see H4).
func List() ([]SessionListing, error) {
	sessDir := GetSessionsDir()

	entries, err := os.ReadDir(sessDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("scout: read sessions dir: %w", err)
	}

	var result []SessionListing

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		name := e.Name()

		info, err := ReadInfo(name)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				slog.Warn("scout: session unreadable",
					"id", name,
					"err", err,
				)
			}

			continue
		}

		listing := SessionListing{
			ID:   name,
			Dir:  filepath.Join(sessDir, name),
			Info: info,
		}

		if job, err := ReadJob(name); err == nil {
			listing.Job = job
		}

		result = append(result, listing)
	}

	return result, nil
}

// FindByDomain looks up a session by domain hash directory name.
// Since the dir name IS the domain hash, this is a direct path check — no scanning.
func FindByDomain(rawURL string) *SessionListing {
	hash := DomainHash(rawURL)
	if hash == "" {
		return nil
	}

	info, err := ReadInfo(hash)
	if err != nil {
		return nil
	}

	return &SessionListing{
		ID:   hash,
		Dir:  Dir(hash),
		Info: info,
	}
}

// FindReusable scans session dirs for a matching reusable session.
func FindReusable(browser string, headless bool) *SessionListing {
	sessions, err := List()
	if err != nil {
		return nil
	}

	for i := range sessions {
		info := sessions[i].Info
		if info.Reusable && info.Browser == browser && info.Headless == headless {
			return &sessions[i]
		}
	}

	return nil
}

// CleanOrphans scans SessionsDir for sessions where the scout process is dead
// but the browser process is still running, and kills the orphaned browser.
// Returns the number of orphaned browsers killed.
func CleanOrphans() (int, error) {
	sessions, err := List()
	if err != nil {
		return 0, err
	}

	killed := 0

	for _, s := range sessions {
		if s.Info.ScoutPID == 0 || s.Info.BrowserPID == 0 {
			continue
		}

		if IsScoutProcess(s.Info.ScoutPID) {
			continue
		}

		// Reusable sessions persist by user opt-in UNTIL their ExpiresAt
		// passes. Don't kill their browser or remove their dir while
		// within the lifetime window. H6 + expiration enforcement.
		if s.Info.Reusable && !s.Info.IsExpired() {
			continue
		}

		if verifyProcess(s.Info.BrowserPID, s.Info.BrowserStartToken) {
			if p, err := os.FindProcess(s.Info.BrowserPID); err == nil {
				_ = p.Kill()
			}

			killed++
		}

		// Remove the full session directory (scout.pid + job.json + data/).
		// Exponential-backoff retry handles Windows file locks (L1).
		sessionDir := Dir(s.ID)
		if err := retryRemoveAll(sessionDir); err != nil {
			// M6: surface persistent removal failures (permission denied,
			// stuck file lock) instead of silently swallowing them.
			slog.Warn("scout: session cleanup failed after retries",
				"id", s.ID,
				"dir", sessionDir,
				"err", err,
			)
		}
	}

	return killed, nil
}

// Reset removes an entire session directory (all browser data + scout.pid).
// If the session's browser process is still running, it is killed first.
// On Windows, retries removal if files are still locked by Chrome.
func Reset(id string) error {
	dir := Dir(id)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("scout: session %s not found", id)
	}

	// Kill browser process if still alive AND identity matches. L2: only
	// sleep after an actual Kill() — skipping the 500ms when the process
	// is already gone shaves shutdown latency in the common case.
	if info, err := ReadInfo(id); err == nil && info.BrowserPID != 0 {
		if verifyProcess(info.BrowserPID, info.BrowserStartToken) {
			if p, err := os.FindProcess(info.BrowserPID); err == nil {
				if killErr := p.Kill(); killErr == nil {
					time.Sleep(500 * time.Millisecond)
				}
			}
		}
	}

	if err := retryRemoveAll(dir); err != nil {
		return fmt.Errorf("scout: reset session %s: %w", id, err)
	}

	return nil
}

// ResetAll removes all session directories under SessionsDir, including
// orphaned directories without scout.pid.
// Returns the number of sessions removed.
func ResetAll() (int, error) {
	sessDir := GetSessionsDir()

	entries, err := os.ReadDir(sessDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}

		return 0, fmt.Errorf("scout: read sessions dir: %w", err)
	}

	removed := 0

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		if err := Reset(e.Name()); err != nil {
			// L3: surface per-session failures (PID kill failed,
			// dir still locked) instead of silently swallowing
			// them. Operator running `scout session reset --all`
			// previously saw "removed N" with no visibility into
			// which session refused to die.
			slog.Warn("scout: session reset failed",
				"id", e.Name(),
				"err", err,
			)

			continue
		}

		removed++
	}

	return removed, nil
}

// CleanStaleSessions removes leftover session directories on startup.
// It removes:
//   - All non-reusable sessions unconditionally (kills browser if still alive)
//   - Reusable sessions where both scout and browser processes are dead
//   - Orphaned directories that have no scout.pid file at all
//
// Only explicitly reusable sessions with a live process are preserved.
// Returns the number of sessions cleaned.
func CleanStaleSessions() (int, error) {
	sessDir := GetSessionsDir()

	entries, err := os.ReadDir(sessDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}

		return 0, fmt.Errorf("scout: read sessions dir: %w", err)
	}

	cleaned := 0

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		id := e.Name()
		info, err := ReadInfo(id)

		// No scout.pid OR legacy JSON format (hard cutover) — remove it.
		// Use a single-shot RemoveAll for the fast path; on failure (the
		// usual case for legacy dirs whose Chrome LevelDB / SQLite files
		// are held by Defender / Search Indexer / OneDrive), enqueue for
		// the background retrier instead of burning 11 s of startup time
		// per dir. With 8 stuck dirs this cut startup latency from ~88 s
		// to <1 s.
		target := filepath.Join(sessDir, id)
		if err != nil {
			if errors.Is(err, ErrLegacyFormat) {
				slog.Info("scout: removing legacy JSON session", "id", id)
			}
			if removeErr := os.RemoveAll(target); removeErr == nil {
				cleaned++
			} else {
				recordCleanupFailure(target)
			}

			continue
		}

		// Reusable sessions are auto-cleaned ONLY when their ExpiresAt has
		// passed. Otherwise persistence overrides liveness (H6 contract).
		// Reusable sessions without ExpiresAt set (legacy data) are treated
		// as not-yet-expired to avoid surprise deletion on upgrade.
		if info.Reusable {
			if !info.IsExpired() {
				continue
			}

			slog.Info("scout: reusable session expired, cleaning",
				"id", id,
				"expires_at", info.ExpiresAt,
			)
		}

		// Non-reusable session or dead reusable — kill orphaned browser
		// only if we can confirm identity.
		if info.BrowserPID != 0 && verifyProcess(info.BrowserPID, info.BrowserStartToken) {
			if p, err := os.FindProcess(info.BrowserPID); err == nil {
				_ = p.Kill()
			}
		}

		// Single-shot RemoveAll for the fast startup path. On failure
		// (Windows AV / Search Indexer / OneDrive holding the LevelDB
		// files), enqueue for the background retrier rather than
		// blocking startup on retries.
		target2 := filepath.Join(sessDir, id)
		if err := os.RemoveAll(target2); err == nil {
			cleaned++
		} else {
			recordCleanupFailure(target2)
			slog.Warn("scout: stale session cleanup deferred to background retrier",
				"id", id,
				"err", err,
			)
		}
	}

	return cleaned, nil
}

// DefaultOrphanCheckInterval is the default interval for periodic orphan checks.
const DefaultOrphanCheckInterval = 2 * time.Minute

// StartOrphanWatchdog starts a background goroutine that periodically calls
// CleanOrphans to kill dangling browser processes whose scout owner has died.
// It stops when the done channel is closed. Returns immediately.
func StartOrphanWatchdog(interval time.Duration, done <-chan struct{}) {
	if interval <= 0 {
		interval = DefaultOrphanCheckInterval
	}

	go func() {
		// L6: panic recovery so a single bad iteration (e.g. a gops
		// internal race or filesystem race triggering nil deref)
		// doesn't kill the watchdog silently. Log and continue.
		defer func() {
			if r := recover(); r != nil {
				slog.Error("scout: orphan watchdog panic",
					"panic", r,
				)
			}
		}()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				_, _ = CleanOrphans()
			}
		}
	}()
}

// RootDomain extracts the registrable (eTLD+1) domain from a URL, using the
// Mozilla Public Suffix List via golang.org/x/net/publicsuffix.
//
// Examples:
//
//	"https://sub.admin.mysite.com/path"   → "mysite.com"
//	"https://app.mysite.co.uk/path"       → "mysite.co.uk"
//	"https://x.gov.uk/path"               → "x.gov.uk"
//	"https://192.168.1.1:8080/path"       → "192.168.1.1"
//
// Hardening M4 — replaces the previous hand-curated two-part-TLD map which
// missed entries like co.il, gov.uk, ac.uk and would have collapsed
// unrelated domains into the same session directory.
func RootDomain(rawURL string) string {
	if rawURL == "" {
		return ""
	}

	if !strings.Contains(rawURL, "://") {
		rawURL = "https://" + rawURL
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}

	host := u.Hostname()
	if host == "" {
		return ""
	}

	// Bypass PSL for IP literals — they have no registrable domain.
	if net := strings.TrimRight(host, "."); strings.ContainsAny(net, ":") || IsIP(net) {
		return host
	}

	etld1, err := publicsuffix.EffectiveTLDPlusOne(host)
	if err != nil {
		return host
	}

	return etld1
}

// IsIP checks whether s looks like an IPv4 address (digits and dots only).
func IsIP(s string) bool {
	for _, c := range s {
		if c != '.' && (c < '0' || c > '9') {
			return false
		}
	}

	return strings.Contains(s, ".")
}

// DomainHash returns a short SHA-256 hash (first 16 hex chars) of the root domain.
func DomainHash(rawURL string) string {
	root := RootDomain(rawURL)
	if root == "" {
		return ""
	}

	h := sha256.Sum256([]byte(root))

	return hex.EncodeToString(h[:12]) // 12 bytes = 16 hex chars, enough for a short unique ID with low collision risk.
}

// Hash returns a deterministic hash for a session directory name.
// The label (typically the browser name) is always included in the digest so
// that different browsers produce different session directories even for the
// same URL.
func Hash(rawURL, label string) string {
	if label == "" {
		label = "default"
	}

	if rawURL != "" {
		root := RootDomain(rawURL)
		if root != "" {
			h := sha256.Sum256([]byte(root + "\x00" + label))
			return hex.EncodeToString(h[:12])
		}
	}

	h := sha256.Sum256([]byte(label))

	return hex.EncodeToString(h[:12])
}
