package logger

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func resetGlobal() {
	instance = nil
	once = sync.Once{}
}

func TestLoggingWriter(t *testing.T) {
	var buf bytes.Buffer
	lw := NewLoggingWriter(&buf)

	_, err := lw.Write([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}

	if lw.String() != "hello" {
		t.Fatalf("expected hello, got %q", lw.String())
	}

	if buf.String() != "hello" {
		t.Fatalf("underlying writer missing data: %q", buf.String())
	}

	if lw.IsTruncated() {
		t.Fatal("should not be truncated")
	}
}

func TestLoggingWriterTruncation(t *testing.T) {
	var buf bytes.Buffer
	lw := &LoggingWriter{
		underlying: &buf,
		maxSize:    10,
	}

	_, _ = lw.Write([]byte("12345678901234"))

	if !lw.IsTruncated() {
		t.Fatal("should be truncated")
	}

	if lw.Len() != 10 {
		t.Fatalf("expected 10 bytes captured, got %d", lw.Len())
	}
}

func TestNewLogger(t *testing.T) {
	resetGlobal()

	dir := t.TempDir()
	l, err := New(dir, "test-cmd")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()

	if !l.IsActive() {
		t.Fatal("expected logger to be active")
	}

	// Verify log file was created
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 log file, got %d", len(entries))
	}

	name := entries[0].Name()
	if !strings.HasSuffix(name, "-test-cmd.log") {
		t.Fatalf("unexpected filename: %s", name)
	}
}

func TestStartEndExecution(t *testing.T) {
	resetGlobal()

	dir := t.TempDir()
	l, err := New(dir, "test-cmd")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()

	var stdout, stderr bytes.Buffer
	outW, errW := l.StartExecution("test-cmd", []string{"--flag"}, &stdout, &stderr)

	_, _ = outW.Write([]byte("output data"))
	_, _ = errW.Write([]byte("error data"))

	l.EndExecution(nil)

	if stdout.String() != "output data" {
		t.Fatalf("stdout passthrough failed: %q", stdout.String())
	}

	if stderr.String() != "error data" {
		t.Fatalf("stderr passthrough failed: %q", stderr.String())
	}

	// Verify JSON was written to log file
	entries, _ := os.ReadDir(dir)
	content, _ := os.ReadFile(filepath.Join(dir, entries[0].Name()))

	if !strings.Contains(string(content), "command_start") {
		t.Fatal("log file missing command_start")
	}

	if !strings.Contains(string(content), "command_end") {
		t.Fatal("log file missing command_end")
	}
}

func TestNilLoggerSafety(t *testing.T) {
	var l *Logger

	if l.IsActive() {
		t.Fatal("nil logger should not be active")
	}

	l.EndExecution(nil)

	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
}
