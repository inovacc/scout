package interaction

import (
	"bufio"
	"encoding/json"
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

// Close writes a session_end event and closes the file. Safe on a nil receiver.
func (r *Recorder) Close(status string) error {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil
	}
	events := r.seq
	r.mu.Unlock()

	r.Emit(Event{Kind: "session_end", Extra: map[string]any{"status": status, "events": events}})

	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
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
		return nil
	}

	defMu.Lock()
	defRec = r
	defMu.Unlock()

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
