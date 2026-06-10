package logger

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// errWriter is an io.Writer that always fails, used to exercise the
// underlying-writer error path in LoggingWriter.Write.
type errWriter struct{}

var errWrite = errors.New("write failed")

func (errWriter) Write(_ []byte) (int, error) { return 0, errWrite }

func TestLoggingWriterBytesAndReset(t *testing.T) {
	var buf bytes.Buffer

	lw := NewLoggingWriter(&buf)

	if _, err := lw.Write([]byte("payload")); err != nil {
		t.Fatalf("write: %v", err)
	}

	if got := string(lw.Bytes()); got != "payload" {
		t.Fatalf("Bytes() = %q, want %q", got, "payload")
	}

	// Bytes() must return the same content as String().
	if got, want := string(lw.Bytes()), lw.String(); got != want {
		t.Fatalf("Bytes()=%q != String()=%q", got, want)
	}

	lw.Reset()

	if lw.Len() != 0 {
		t.Fatalf("after Reset Len() = %d, want 0", lw.Len())
	}

	if lw.IsTruncated() {
		t.Fatal("after Reset IsTruncated() = true, want false")
	}

	if got := string(lw.Bytes()); got != "" {
		t.Fatalf("after Reset Bytes() = %q, want empty", got)
	}
}

func TestLoggingWriterResetClearsTruncation(t *testing.T) {
	var buf bytes.Buffer

	lw := &LoggingWriter{underlying: &buf, maxSize: 4}

	if _, err := lw.Write([]byte("abcdefgh")); err != nil {
		t.Fatalf("write: %v", err)
	}

	if !lw.IsTruncated() {
		t.Fatal("expected truncated before reset")
	}

	lw.Reset()

	if lw.IsTruncated() {
		t.Fatal("Reset should clear truncated flag")
	}

	// After reset, capture works again within the size limit.
	if _, err := lw.Write([]byte("xy")); err != nil {
		t.Fatalf("write after reset: %v", err)
	}

	if got := lw.String(); got != "xy" {
		t.Fatalf("after reset capture = %q, want %q", got, "xy")
	}
}

func TestLoggingWriterUnderlyingError(t *testing.T) {
	lw := NewLoggingWriter(errWriter{})

	n, err := lw.Write([]byte("data"))
	if !errors.Is(err, errWrite) {
		t.Fatalf("expected errWrite, got %v", err)
	}

	if n != 0 {
		t.Fatalf("expected n=0 on underlying error, got %d", n)
	}

	// Nothing must be captured when the underlying write fails.
	if lw.Len() != 0 {
		t.Fatalf("expected no captured bytes on error, got %d", lw.Len())
	}
}

func TestLoggingWriterExactBoundaryNotTruncated(t *testing.T) {
	var buf bytes.Buffer

	lw := &LoggingWriter{underlying: &buf, maxSize: 5}

	// Exactly maxSize: the boundary uses ">" so it must NOT truncate.
	if _, err := lw.Write([]byte("12345")); err != nil {
		t.Fatalf("write: %v", err)
	}

	if lw.IsTruncated() {
		t.Fatal("writing exactly maxSize should not truncate")
	}

	if lw.Len() != 5 {
		t.Fatalf("Len() = %d, want 5", lw.Len())
	}

	// One more byte tips it over the limit; the overflow byte is dropped
	// from capture but still passes through to the underlying writer.
	if _, err := lw.Write([]byte("6")); err != nil {
		t.Fatalf("write: %v", err)
	}

	if !lw.IsTruncated() {
		t.Fatal("writing past maxSize should truncate")
	}

	if lw.Len() != 5 {
		t.Fatalf("captured Len() = %d, want 5 (overflow dropped)", lw.Len())
	}

	if buf.String() != "123456" {
		t.Fatalf("underlying = %q, want full passthrough %q", buf.String(), "123456")
	}
}

func TestLoggingWriterTruncatedSubsequentWritesIgnored(t *testing.T) {
	var buf bytes.Buffer

	lw := &LoggingWriter{underlying: &buf, maxSize: 3}

	_, _ = lw.Write([]byte("abcdef")) // triggers truncation, captures "abc"
	_, _ = lw.Write([]byte("ghi"))    // ignored by capture, still passes through

	if lw.String() != "abc" {
		t.Fatalf("captured = %q, want %q", lw.String(), "abc")
	}

	if buf.String() != "abcdefghi" {
		t.Fatalf("underlying = %q, want %q", buf.String(), "abcdefghi")
	}
}

func TestFormatArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "empty", args: nil, want: ""},
		{name: "single", args: []string{"scrape"}, want: "scrape"},
		{name: "multiple", args: []string{"scrape", "--url", "x"}, want: "scrape --url x"},
		{name: "empty elements", args: []string{"a", "", "b"}, want: "a  b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatArgs(tt.args); got != tt.want {
				t.Fatalf("FormatArgs(%v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

func TestGet(t *testing.T) {
	resetGlobal()

	if Get() != nil {
		t.Fatal("Get() before Init should be nil")
	}

	got := Init("probe")
	if got == nil {
		t.Fatal("Init returned nil")
	}

	if Get() != got {
		t.Fatal("Get() must return the instance created by Init")
	}

	// Init is idempotent: a second call returns the same instance.
	if again := Init("other"); again != got {
		t.Fatal("Init should be idempotent (sync.Once)")
	}
}

func TestWriter(t *testing.T) {
	// nil logger returns io.Discard.
	var nilLogger *Logger
	if nilLogger.Writer() != io.Discard {
		t.Fatal("nil logger Writer() should be io.Discard")
	}

	// logger with no file returns io.Discard.
	inactive := &Logger{}
	if inactive.Writer() != io.Discard {
		t.Fatal("logger without file should return io.Discard")
	}

	// active logger returns its file handle.
	dir := t.TempDir()

	l, err := New(dir, "writer-cmd")
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = l.Close() }()

	w := l.Writer()
	if w == io.Discard {
		t.Fatal("active logger Writer() should not be io.Discard")
	}

	if w != l.file {
		t.Fatal("active logger Writer() should return its file handle")
	}
}

func TestNewEmptyDir(t *testing.T) {
	resetGlobal()

	l, err := New("", "no-dir")
	if err != nil {
		t.Fatalf("New with empty dir should not error, got %v", err)
	}

	if l == nil {
		t.Fatal("New should return a non-nil logger even with empty dir")
	}

	if l.IsActive() {
		t.Fatal("logger with empty dir must be inactive")
	}

	if l.file != nil {
		t.Fatal("logger with empty dir must have no file")
	}

	// Writer falls back to discard since the logger is inactive.
	if l.Writer() != io.Discard {
		t.Fatal("inactive logger Writer() should be io.Discard")
	}
}

func TestNewWithExactPathEmpty(t *testing.T) {
	l, err := NewWithExactPath("")
	if err != nil {
		t.Fatalf("empty path should not error, got %v", err)
	}

	if l == nil {
		t.Fatal("expected non-nil logger")
	}

	if l.IsActive() {
		t.Fatal("empty-path logger must be inactive")
	}
}

func TestNewWithExactPathCreatesNestedDir(t *testing.T) {
	base := t.TempDir()
	// The parent directory does not exist yet; NewWithExactPath must create it.
	logPath := filepath.Join(base, "nested", "deep", "scout.log")

	l, err := NewWithExactPath(logPath)
	if err != nil {
		t.Fatalf("NewWithExactPath: %v", err)
	}

	defer func() { _ = l.Close() }()

	if !l.IsActive() {
		t.Fatal("expected active logger")
	}

	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("expected log file at %s: %v", logPath, err)
	}

	// The file handle returned by Writer must be the exact path's file.
	if l.Writer() == io.Discard {
		t.Fatal("active logger should not return io.Discard")
	}
}

func TestNewWithExactPathWritesJSON(t *testing.T) {
	base := t.TempDir()
	logPath := filepath.Join(base, "exact.log")

	l, err := NewWithExactPath(logPath)
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer

	outW, _ := l.StartExecution("cmd", []string{"a"}, &stdout, &stderr)
	_, _ = outW.Write([]byte("hi"))
	l.EndExecution(nil)

	_ = l.Close()

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(content), "command_start") {
		t.Fatal("missing command_start entry")
	}

	if !strings.Contains(string(content), "command_end") {
		t.Fatal("missing command_end entry")
	}
}

func TestGenerateLogPathDefaultCommand(t *testing.T) {
	dir := t.TempDir()

	// Empty command defaults to "scout".
	path, err := generateLogPath(dir, "")
	if err != nil {
		t.Fatalf("generateLogPath: %v", err)
	}

	if filepath.Dir(path) != dir {
		t.Fatalf("path dir = %q, want %q", filepath.Dir(path), dir)
	}

	if !strings.HasSuffix(path, "-scout.log") {
		t.Fatalf("empty command should default to scout, got %q", path)
	}

	// Named command is embedded in the filename.
	path2, err := generateLogPath(dir, "crawl")
	if err != nil {
		t.Fatalf("generateLogPath: %v", err)
	}

	if !strings.HasSuffix(path2, "-crawl.log") {
		t.Fatalf("expected -crawl.log suffix, got %q", path2)
	}

	// Two calls must produce distinct (ksuid-prefixed) paths.
	if path == path2 {
		t.Fatal("expected unique paths from distinct calls")
	}
}

func TestStartExecutionInactivePassthrough(t *testing.T) {
	// An inactive logger must return the original writers unchanged.
	inactive := &Logger{}

	var stdout, stderr bytes.Buffer

	outW, errW := inactive.StartExecution("cmd", nil, &stdout, &stderr)

	if outW != io.Writer(&stdout) {
		t.Fatal("inactive logger should pass stdout through unchanged")
	}

	if errW != io.Writer(&stderr) {
		t.Fatal("inactive logger should pass stderr through unchanged")
	}

	// nil logger also passes through.
	var nilLogger *Logger

	o, e := nilLogger.StartExecution("cmd", nil, &stdout, &stderr)
	if o != io.Writer(&stdout) || e != io.Writer(&stderr) {
		t.Fatal("nil logger should pass writers through unchanged")
	}
}

func TestEndExecutionWithoutStart(t *testing.T) {
	dir := t.TempDir()

	l, err := New(dir, "cmd")
	if err != nil {
		t.Fatal(err)
	}

	defer func() { _ = l.Close() }()

	// EndExecution with no active execution must be a safe no-op
	// (l.execution is nil).
	l.EndExecution(errors.New("ignored"))

	// No command_end should have been written.
	entries, _ := os.ReadDir(dir)
	content, _ := os.ReadFile(filepath.Join(dir, entries[0].Name()))

	if strings.Contains(string(content), "command_end") {
		t.Fatal("EndExecution without StartExecution must not log command_end")
	}
}

func TestEndExecutionErrorStatus(t *testing.T) {
	dir := t.TempDir()

	l, err := New(dir, "cmd")
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer

	outW, errW := l.StartExecution("cmd", []string{"--x"}, &stdout, &stderr)
	_, _ = outW.Write([]byte("out"))
	_, _ = errW.Write([]byte("err"))

	wantErr := errors.New("boom")
	l.EndExecution(wantErr)

	_ = l.Close()

	entries, _ := os.ReadDir(dir)

	content, err := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}

	// Parse the command_end JSON line and assert concrete field values.
	end := findLogLine(t, content, "command_end")

	if end["status"] != "error" {
		t.Fatalf("status = %v, want error", end["status"])
	}

	if end["error"] != "boom" {
		t.Fatalf("error = %v, want boom", end["error"])
	}

	if end["stdout"] != "out" {
		t.Fatalf("stdout = %v, want out", end["stdout"])
	}

	if end["stderr"] != "err" {
		t.Fatalf("stderr = %v, want err", end["stderr"])
	}

	// stdout_bytes / stderr_bytes are always emitted.
	if end["stdout_bytes"] != float64(3) {
		t.Fatalf("stdout_bytes = %v, want 3", end["stdout_bytes"])
	}

	if end["stderr_bytes"] != float64(3) {
		t.Fatalf("stderr_bytes = %v, want 3", end["stderr_bytes"])
	}
}

func TestEndExecutionTruncatedFlags(t *testing.T) {
	dir := t.TempDir()

	l, err := New(dir, "cmd")
	if err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer

	outW, errW := l.StartExecution("cmd", nil, &stdout, &stderr)

	// Force truncation on both writers by shrinking their max size.
	l.execution.Stdout.maxSize = 2
	l.execution.Stderr.maxSize = 2

	_, _ = outW.Write([]byte("aaaaaa"))
	_, _ = errW.Write([]byte("bbbbbb"))

	l.EndExecution(nil)
	_ = l.Close()

	entries, _ := os.ReadDir(dir)
	content, _ := os.ReadFile(filepath.Join(dir, entries[0].Name()))

	end := findLogLine(t, content, "command_end")

	if end["status"] != "success" {
		t.Fatalf("status = %v, want success", end["status"])
	}

	if end["stdout_truncated"] != true {
		t.Fatalf("stdout_truncated = %v, want true", end["stdout_truncated"])
	}

	if end["stderr_truncated"] != true {
		t.Fatalf("stderr_truncated = %v, want true", end["stderr_truncated"])
	}

	// Only the first 2 bytes were captured.
	if end["stdout"] != "aa" {
		t.Fatalf("stdout = %v, want aa", end["stdout"])
	}

	// _, present check: when there is no error there must be no error field.
	if _, ok := end["error"]; ok {
		t.Fatal("success execution should not carry an error field")
	}
}

// findLogLine scans newline-delimited JSON log output and returns the first
// object whose "msg" field equals msg.
func findLogLine(t *testing.T, content []byte, msg string) map[string]any {
	t.Helper()

	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			continue
		}

		if m["msg"] == msg {
			return m
		}
	}

	t.Fatalf("log line with msg=%q not found in:\n%s", msg, content)

	return nil
}
