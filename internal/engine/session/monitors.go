package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// MonitorConfig is the per-session sidecar describing which monitors are
// active for a given browser session. Persisted as <session>/monitors.json.
// Read by session-destroy paths to know which artifacts (HAR, hijack
// stream, console log, ws stream) need finalizing before the dir is
// reaped, and by audit / list tooling to surface monitor status.
type MonitorConfig struct {
	HAR     MonitorSink   `json:"har"`
	Hijack  MonitorSink   `json:"hijack"`
	Console MonitorSink   `json:"console"`
	WS      MonitorSink   `json:"ws"`
	Blocks  []MonitorRule `json:"blocks,omitempty"`
}

// MonitorSink describes one monitor's enabled state + on-disk artifact path.
type MonitorSink struct {
	Enabled     bool   `json:"enabled"`
	Path        string `json:"path,omitempty"` // relative to session dir, e.g. "har.json"
	WithBodies  bool   `json:"with_bodies,omitempty"`
}

// MonitorRule mirrors engine.BlockRule for on-disk persistence.
type MonitorRule struct {
	Pattern string `json:"pattern"`
	Method  string `json:"method,omitempty"`
}

const monitorsFilename = "monitors.json"

// MonitorsPath returns the absolute path to the sidecar for session id.
func MonitorsPath(id string) string {
	return filepath.Join(Dir(id), monitorsFilename)
}

// WriteMonitors persists cfg to <session>/monitors.json. Pretty-printed for
// human readability (this file is operator-facing, not hot-path).
func WriteMonitors(id string, cfg *MonitorConfig) error {
	if id == "" || cfg == nil {
		return fmt.Errorf("scout: WriteMonitors: empty id or nil cfg")
	}

	dir := Dir(id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("scout: monitors: mkdir: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("scout: monitors: marshal: %w", err)
	}

	return writeFileAtomic(filepath.Join(dir, monitorsFilename), data, 0o600)
}

// ReadMonitors loads monitors.json. Returns a zero MonitorConfig (not an
// error) when the file is absent — sessions created before this sidecar
// existed silently report "no monitors" rather than crashing destroy.
func ReadMonitors(id string) (*MonitorConfig, error) {
	path := filepath.Join(Dir(id), monitorsFilename)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &MonitorConfig{}, nil
		}
		return nil, fmt.Errorf("scout: monitors: read: %w", err)
	}

	var cfg MonitorConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("scout: monitors: parse: %w", err)
	}

	return &cfg, nil
}

// DefaultHARPath returns "har.json" inside the session dir.
func DefaultHARPath(id string) string {
	return filepath.Join(Dir(id), "har.json")
}

// DefaultHijackPath returns "hijack.jsonl" inside the session dir.
func DefaultHijackPath(id string) string {
	return filepath.Join(Dir(id), "hijack.jsonl")
}

// DefaultConsolePath returns "console.log" inside the session dir.
func DefaultConsolePath(id string) string {
	return filepath.Join(Dir(id), "console.log")
}

// DefaultWSPath returns "ws.jsonl" inside the session dir.
func DefaultWSPath(id string) string {
	return filepath.Join(Dir(id), "ws.jsonl")
}
