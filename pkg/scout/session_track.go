package scout

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
)

// SessionsDir returns the base directory for session data: ~/.scout/sessions.
// This is cross-platform: it uses os.UserHomeDir which resolves to
// %USERPROFILE% on Windows, $HOME on Unix/macOS.
func SessionsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback to temp dir if home is unavailable.
		return filepath.Join(os.TempDir(), "scout", "sessions")
	}
	return filepath.Join(home, ".scout", "sessions")
}

// SessionUserDataDir returns the user-data subdirectory under SessionsDir.
func SessionUserDataDir() string {
	return filepath.Join(SessionsDir(), "user-data")
}

// SessionPIDDir returns the pids subdirectory under SessionsDir.
func SessionPIDDir() string {
	return filepath.Join(SessionsDir(), "pids")
}

// SessionEntry represents a tracked browser session.
type SessionEntry struct {
	ID        string    `json:"id"`
	DataDir   string    `json:"data_dir"`
	CreatedAt time.Time `json:"created_at"`
	LastUsed  time.Time `json:"last_used"`
	Browser   string    `json:"browser"`
	Headless  bool      `json:"headless"`
	URLs      []string  `json:"urls,omitempty"`
	Reusable  bool      `json:"reusable"`
}

// SessionTracker manages session entries persisted to track.json.
type SessionTracker struct {
	Sessions []SessionEntry `json:"sessions"`
	path     string
	mu       sync.Mutex
}

// defaultTrackPath returns the default path for track.json.
func defaultTrackPath() string {
	return filepath.Join(SessionsDir(), "track.json")
}

// LoadTracker reads or creates the session tracker from the default track.json location.
func LoadTracker() (*SessionTracker, error) {
	return LoadTrackerFrom(defaultTrackPath())
}

// LoadTrackerFrom reads or creates a session tracker from the given path.
func LoadTrackerFrom(path string) (*SessionTracker, error) {
	t := &SessionTracker{path: path}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return t, nil
		}
		return nil, fmt.Errorf("scout: read track.json: %w", err)
	}

	if err := json.Unmarshal(data, t); err != nil {
		return nil, fmt.Errorf("scout: parse track.json: %w", err)
	}

	return t, nil
}

// Register adds a new session entry and saves to disk.
func (t *SessionTracker) Register(entry SessionEntry) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.Sessions = append(t.Sessions, entry)
	return t.save()
}

// Update modifies an existing session entry by ID.
func (t *SessionTracker) Update(id string, fn func(*SessionEntry)) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	for i := range t.Sessions {
		if t.Sessions[i].ID == id {
			fn(&t.Sessions[i])
			return t.save()
		}
	}

	return fmt.Errorf("scout: session %s not found", id)
}

// FindReusable returns a matching reusable session, or nil if none found.
func (t *SessionTracker) FindReusable(browser string, headless bool) *SessionEntry {
	t.mu.Lock()
	defer t.mu.Unlock()

	for i := range t.Sessions {
		e := &t.Sessions[i]
		if e.Reusable && e.Browser == browser && e.Headless == headless {
			return e
		}
	}

	return nil
}

// Remove deletes a session entry by ID and saves to disk.
func (t *SessionTracker) Remove(id string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	for i := range t.Sessions {
		if t.Sessions[i].ID == id {
			t.Sessions = append(t.Sessions[:i], t.Sessions[i+1:]...)
			return t.save()
		}
	}

	return nil
}

// Save writes the tracker to disk.
func (t *SessionTracker) Save() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.save()
}

func (t *SessionTracker) save() error {
	dir := filepath.Dir(t.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("scout: create track dir: %w", err)
	}

	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return fmt.Errorf("scout: marshal track.json: %w", err)
	}

	if err := os.WriteFile(t.path, data, 0o644); err != nil {
		return fmt.Errorf("scout: write track.json: %w", err)
	}

	return nil
}

// Prune removes entries whose DataDir no longer exists on disk.
func (t *SessionTracker) Prune() (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	var kept []SessionEntry
	pruned := 0

	for _, e := range t.Sessions {
		if _, err := os.Stat(e.DataDir); err == nil {
			kept = append(kept, e)
		} else {
			pruned++
		}
	}

	t.Sessions = kept
	if pruned > 0 {
		if err := t.save(); err != nil {
			return pruned, err
		}
	}

	return pruned, nil
}

// Scan discovers UUID v7 directories in the user-data dir that are not yet
// tracked and adds them to the tracker. It also reads the PID file if present.
func (t *SessionTracker) Scan() (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	userDataDir := SessionUserDataDir()
	entries, err := os.ReadDir(userDataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("scout: scan user-data: %w", err)
	}

	// Build set of already-tracked IDs.
	tracked := make(map[string]struct{}, len(t.Sessions))
	for _, s := range t.Sessions {
		tracked[s.ID] = struct{}{}
	}

	added := 0
	pidDir := SessionPIDDir()

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()

		// Validate it's a UUID.
		if _, err := uuid.Parse(name); err != nil {
			continue
		}

		if _, exists := tracked[name]; exists {
			continue
		}

		dataDir := filepath.Join(userDataDir, name)
		info, _ := e.Info()
		created := time.Now()
		if info != nil {
			created = info.ModTime()
		}

		entry := SessionEntry{
			ID:        name,
			DataDir:   dataDir,
			CreatedAt: created,
			LastUsed:  created,
			Browser:   "chrome",
			Headless:  true,
		}

		// Read PID if available.
		if pidData, err := os.ReadFile(filepath.Join(pidDir, name)); err == nil {
			if pid, err := strconv.Atoi(string(pidData)); err == nil {
				// Check if process is still running.
				if p, err := os.FindProcess(pid); err == nil {
					_ = p
					entry.Reusable = false
				}
			}
		}

		t.Sessions = append(t.Sessions, entry)
		added++
	}

	if added > 0 {
		if err := t.save(); err != nil {
			return added, err
		}
	}

	return added, nil
}
