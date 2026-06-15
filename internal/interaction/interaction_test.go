package interaction

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestGate(t *testing.T) {
	// Assert only the env-enable path; the persisted feature-flag state is
	// machine-dependent and not controllable from a unit test. The disabled
	// behaviour is covered by TestNilRecorderNoOp at the recorder level.
	t.Setenv("SCOUT_INTERACTIONS", "1")
	if !Enabled() {
		t.Fatal("SCOUT_INTERACTIONS=1 should enable")
	}
}

func TestDirDefault(t *testing.T) {
	t.Setenv("SCOUT_HOME", t.TempDir())
	d, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(d) != "captures" {
		t.Fatalf("Dir()=%s, want .../captures", d)
	}
}

func TestNilRecorderNoOp(t *testing.T) {
	var r *Recorder
	r.Emit(Event{Kind: "x"}) // must not panic
	if err := r.Close("ok"); err != nil {
		t.Fatalf("nil Close: %v", err)
	}
}

func TestRecorderRoundtrip(t *testing.T) {
	t.Setenv("SCOUT_INTERACTIONS", "1")
	t.Setenv("SCOUT_HOME", t.TempDir())

	rec, err := Open("cli", "cli-test")
	if err != nil || rec == nil {
		t.Fatalf("Open: rec=%v err=%v", rec, err)
	}

	ok := true
	rec.Emit(Event{Kind: "cli", Source: "cli", Name: "gather", OK: &ok})
	if err := rec.Close("ok"); err != nil {
		t.Fatalf("Close: %v", err)
	}

	dir, _ := Dir()
	f, err := os.Open(filepath.Join(dir, "cli-test.jsonl"))
	if err != nil {
		t.Fatalf("open capture: %v", err)
	}
	defer f.Close()

	var kinds []string
	var lastSeq int
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e Event
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			t.Fatalf("bad jsonl: %v", err)
		}
		if e.Seq != lastSeq {
			t.Fatalf("non-monotonic seq: got %d want %d", e.Seq, lastSeq)
		}
		lastSeq++
		kinds = append(kinds, e.Kind)
	}

	if len(kinds) != 3 || kinds[0] != "session_start" || kinds[1] != "cli" || kinds[2] != "session_end" {
		t.Fatalf("unexpected kinds: %v", kinds)
	}
}

// TestConcurrentClose guards the Close concurrency contract: many goroutines
// emitting and closing at once must produce exactly one session_end and no race
// (run with -race). Verifies the single-lock Close.
func TestConcurrentClose(t *testing.T) {
	t.Setenv("SCOUT_INTERACTIONS", "1")
	t.Setenv("SCOUT_HOME", t.TempDir())

	rec, err := Open("cli", "concurrent")
	if err != nil || rec == nil {
		t.Fatalf("Open: rec=%v err=%v", rec, err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec.Emit(Event{Kind: "browser_action"})
			_ = rec.Close("ok")
		}()
	}
	wg.Wait()

	dir, _ := Dir()
	f, err := os.Open(filepath.Join(dir, "concurrent.jsonl"))
	if err != nil {
		t.Fatalf("open capture: %v", err)
	}
	defer f.Close()

	ends := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e Event
		if json.Unmarshal(sc.Bytes(), &e) == nil && e.Kind == "session_end" {
			ends++
		}
	}

	if ends != 1 {
		t.Fatalf("session_end count = %d, want exactly 1", ends)
	}
}

// TestInitSupersedesAndClosesPrevious guards against a file-handle leak: a
// second Init (e.g. "mcp"/"repl" after the root "cli") must close the recorder
// it replaces, so the first file is finalized with a session_end rather than
// orphaned open.
func TestInitSupersedesAndClosesPrevious(t *testing.T) {
	t.Setenv("SCOUT_INTERACTIONS", "1")
	t.Setenv("SCOUT_HOME", t.TempDir())

	if first := Init("cli"); first == nil {
		t.Fatal("first Init returned nil")
	}

	second := Init("mcp")
	if second == nil {
		t.Fatal("second Init returned nil")
	}
	if Default() != second {
		t.Fatal("Default should be the second recorder")
	}

	dir, _ := Dir()
	entries, _ := os.ReadDir(dir)

	var cliFile string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "cli-") {
			cliFile = filepath.Join(dir, e.Name())
		}
	}
	if cliFile == "" {
		t.Fatal("no cli-*.jsonl created")
	}

	data, _ := os.ReadFile(cliFile)
	if !strings.Contains(string(data), `"kind":"session_end"`) {
		t.Fatalf("superseded recorder missing session_end:\n%s", data)
	}

	_ = Close("ok") // finalize the second recorder; don't leak across tests
}

// TestSessionEndDuration verifies session_end carries a first-class duration_ms
// so analysis tools need no timestamp math.
func TestSessionEndDuration(t *testing.T) {
	t.Setenv("SCOUT_INTERACTIONS", "1")
	t.Setenv("SCOUT_HOME", t.TempDir())

	rec, err := Open("cli", "dur-test")
	if err != nil || rec == nil {
		t.Fatalf("Open: rec=%v err=%v", rec, err)
	}

	time.Sleep(5 * time.Millisecond)

	if err := rec.Close("ok"); err != nil {
		t.Fatalf("Close: %v", err)
	}

	dir, _ := Dir()
	data, err := os.ReadFile(filepath.Join(dir, "dur-test.jsonl"))
	if err != nil {
		t.Fatal(err)
	}

	var end Event
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var e Event
		if json.Unmarshal([]byte(line), &e) == nil && e.Kind == "session_end" {
			end = e
		}
	}

	if end.Kind != "session_end" {
		t.Fatal("no session_end event found")
	}
	if end.DurationMS < 1 {
		t.Fatalf("session_end duration_ms = %d, want >= 1 after a 5ms session", end.DurationMS)
	}
}
