package interaction

import (
	"bufio"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/segmentio/ksuid"
)

// Recorder is a per-session append-only JSONL writer. All methods are safe on a
// nil *Recorder (no-op), so callers never branch on Enabled().
type Recorder struct {
	mu     sync.Mutex
	f      *os.File
	w      *bufio.Writer
	seq    int
	closed bool
}

// Open returns a Recorder writing <Dir>/<id>.jsonl, or (nil, nil) when capture
// is disabled. It writes a session_start header. entrypoint is one of cli, mcp,
// grpc, agent.
func Open(entrypoint, id string) (*Recorder, error) {
	if !Enabled() {
		return nil, nil
	}

	dir, err := Dir()
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(filepath.Join(dir, id+".jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}

	r := &Recorder{f: f, w: bufio.NewWriter(f)}
	r.Emit(Event{Kind: "session_start", Source: entrypoint, Extra: map[string]any{"entrypoint": entrypoint, "id": id}})

	return r, nil
}

// Emit appends an event, stamping Seq and TS. Safe on a nil receiver.
func (r *Recorder) Emit(e Event) {
	if r == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return
	}

	e.Seq = r.seq
	r.seq++

	if e.TS == "" {
		e.TS = time.Now().UTC().Format(time.RFC3339Nano)
	}

	b, err := json.Marshal(e)
	if err != nil {
		return
	}

	_, _ = r.w.Write(b)
	_ = r.w.WriteByte('\n')
	_ = r.w.Flush()
}

// Close writes a session_end event and closes the file. Safe on a nil receiver
// and idempotent: the whole operation holds the lock so concurrent callers
// cannot double-write session_end or double-close the file.
func (r *Recorder) Close(status string) error {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.closed {
		return nil
	}

	r.closed = true

	// Write session_end inline (not via Emit, which would re-acquire the lock).
	end := Event{
		Seq:   r.seq,
		TS:    time.Now().UTC().Format(time.RFC3339Nano),
		Kind:  "session_end",
		Extra: map[string]any{"status": status, "events": r.seq},
	}
	r.seq++

	if b, err := json.Marshal(end); err == nil {
		_, _ = r.w.Write(b)
		_ = r.w.WriteByte('\n')
	}

	_ = r.w.Flush()

	return r.f.Close()
}

// --- process-global default recorder (single-session processes) ---

var (
	defMu  sync.Mutex
	defRec *Recorder
)

// Init opens the process-global default recorder with a generated id
// "<entrypoint>-<ksuid>". Returns nil when disabled.
func Init(entrypoint string) *Recorder {
	r, err := Open(entrypoint, entrypoint+"-"+ksuid.New().String())
	if err != nil {
		slog.Warn("scout: interaction capture disabled", "error", err)
		return nil
	}

	defMu.Lock()
	old := defRec
	defRec = r
	defMu.Unlock()

	// Close any recorder we just replaced so its file handle is not leaked and
	// it still gets a session_end — e.g. the short-lived "cli" recorder that
	// root PersistentPreRunE opens before a long-lived "scout mcp"/"scout repl"
	// replaces it with its own. Close is nil-safe.
	_ = old.Close("superseded")

	return r
}

// Default returns the process-global recorder (nil if not initialised/disabled).
func Default() *Recorder {
	defMu.Lock()
	defer defMu.Unlock()

	return defRec
}

// Emit appends to the default recorder (no-op if none).
func Emit(e Event) { Default().Emit(e) }

// Close closes the default recorder (no-op if none).
func Close(status string) error { return Default().Close(status) }
