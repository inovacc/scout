package session

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SessionsDir is the function that returns the base directory for session data.
// It is a variable so tests can override it.
var SessionsDir = defaultSessionsDir

func defaultSessionsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "scout", "sessions")
	}

	return filepath.Join(home, ".scout", "sessions")
}

// GetSessionsDir returns the base directory for session data: ~/.scout/sessions.
// This is cross-platform: it uses os.UserHomeDir which resolves to
// %USERPROFILE% on Windows, $HOME on Unix/macOS.
func GetSessionsDir() string {
	return SessionsDir()
}

// SessionInfo holds all metadata for a browser session, stored as scout.pid
// inside the session's data directory.
type SessionInfo struct {
	ScoutPID     int       `json:"scout_pid"`
	BrowserPID   int       `json:"browser_pid"`
	Reusable     bool      `json:"reusable"`
	CreatedAt    time.Time `json:"created_at"`
	LastUsed     time.Time `json:"last_used"`
	Headless     bool      `json:"headless"`
	Browser      string    `json:"browser"`
	DomainHash   string    `json:"domain_hash,omitempty"`
	Domain       string    `json:"domain,omitempty"`
	Exec         string    `json:"exec,omitempty"`
	BuildVersion string    `json:"build_version,omitempty"`
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

// WriteInfo writes the session info as JSON to <SessionsDir>/<id>/scout.pid.
func WriteInfo(id string, info *SessionInfo) error {
	dir := Dir(id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("scout: create session dir: %w", err)
	}

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("scout: marshal session info: %w", err)
	}

	return os.WriteFile(filepath.Join(dir, "scout.pid"), data, 0o644)
}

// ReadInfo reads the session info from <SessionsDir>/<id>/scout.pid.
func ReadInfo(id string) (*SessionInfo, error) {
	data, err := os.ReadFile(filepath.Join(Dir(id), "scout.pid"))
	if err != nil {
		return nil, err
	}

	var info SessionInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("scout: parse session info: %w", err)
	}

	return &info, nil
}

// RemoveInfo removes the scout.pid file from a session directory.
func RemoveInfo(id string) {
	_ = os.Remove(filepath.Join(Dir(id), "scout.pid"))
}

// List reads all <dir>/scout.pid files under SessionsDir.
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

		if ProcessAlive(s.Info.BrowserPID) {
			if p, err := os.FindProcess(s.Info.BrowserPID); err == nil {
				_ = p.Kill()
			}

			killed++
		}

		RemoveInfo(s.ID)
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

	// Kill browser process if still alive.
	if info, err := ReadInfo(id); err == nil && info.BrowserPID != 0 {
		if ProcessAlive(info.BrowserPID) {
			if p, err := os.FindProcess(info.BrowserPID); err == nil {
				_ = p.Kill()
				// Give the process time to exit and release file locks.
				time.Sleep(500 * time.Millisecond)
			}
		}
	}

	// Retry removal — Chrome may hold file locks briefly after exit.
	var err error
	for range 3 {
		if err = os.RemoveAll(dir); err == nil {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("scout: reset session %s: %w", id, err)
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

		// No scout.pid — orphaned directory, remove it.
		if err != nil {
			if removeErr := os.RemoveAll(filepath.Join(sessDir, id)); removeErr == nil {
				cleaned++
			}

			continue
		}

		// Reusable sessions are preserved if any owning process is still alive.
		if info.Reusable {
			if info.ScoutPID != 0 && ProcessAlive(info.ScoutPID) {
				continue
			}

			if info.BrowserPID != 0 && ProcessAlive(info.BrowserPID) {
				continue
			}
		}

		// Non-reusable session or dead reusable — kill orphaned browser.
		if info.BrowserPID != 0 && ProcessAlive(info.BrowserPID) {
			if p, err := os.FindProcess(info.BrowserPID); err == nil {
				_ = p.Kill()
			}
		}

		// Retry removal for Windows file locks.
		for range 3 {
			if err := os.RemoveAll(filepath.Join(sessDir, id)); err == nil {
				cleaned++

				break
			}

			time.Sleep(200 * time.Millisecond)
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

// RootDomain extracts the root domain from a URL, stripping subdomains.
// e.g. "https://sub.admin.mysite.com/path" → "mysite.com"
// e.g. "https://app.mysite.co.uk/path" → "mysite.co.uk"
func RootDomain(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	// Ensure scheme for url.Parse.
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

	// Handle IP addresses — no root domain extraction.
	if net := strings.TrimRight(host, "."); strings.ContainsAny(net, ":") || IsIP(net) {
		return host
	}

	parts := strings.Split(host, ".")
	if len(parts) <= 2 {
		return host
	}

	// Handle two-part TLDs: co.uk, com.br, co.jp, etc.
	twoPartTLDs := map[string]bool{
		"co.uk": true, "co.jp": true, "co.kr": true, "co.nz": true, "co.za": true,
		"com.br": true, "com.au": true, "com.cn": true, "com.mx": true, "com.ar": true,
		"com.tr": true, "com.tw": true, "com.sg": true, "com.hk": true, "com.my": true,
		"org.uk": true, "org.au": true, "net.au": true, "net.br": true,
		"co.in": true, "co.id": true, "co.th": true,
	}

	lastTwo := strings.Join(parts[len(parts)-2:], ".")
	if twoPartTLDs[lastTwo] && len(parts) >= 3 {
		return strings.Join(parts[len(parts)-3:], ".")
	}

	return strings.Join(parts[len(parts)-2:], ".")
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
