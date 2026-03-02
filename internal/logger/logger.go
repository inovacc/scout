package logger

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/inovacc/scout/internal/flags"
	"github.com/segmentio/ksuid"
)

const (
	// EnvLogEnabled is the environment variable that enables logging.
	EnvLogEnabled = "SCOUT_LOGGER_ENABLED"

	// MaxOutputSize is the maximum output size to log (1MB)
	MaxOutputSize = 1024 * 1024
)

var (
	instance *Logger
	once     sync.Once
)

// Logger handles command logging for scout.
type Logger struct {
	slog      *slog.Logger
	file      *os.File
	active    bool
	command   string
	execution *CommandExecution
	mu        sync.Mutex
}

// CommandExecution tracks a single command execution.
type CommandExecution struct {
	Command   string
	Args      []string
	StartTime time.Time
	Stdout    *LoggingWriter
	Stderr    *LoggingWriter
}

// LoggingWriter wraps an io.Writer to capture output while passing it through.
type LoggingWriter struct {
	underlying io.Writer
	buffer     bytes.Buffer
	mu         sync.Mutex
	maxSize    int
	truncated  bool
}

// NewLoggingWriter creates a writer that captures output while passing it through.
func NewLoggingWriter(w io.Writer) *LoggingWriter {
	return &LoggingWriter{
		underlying: w,
		maxSize:    MaxOutputSize,
	}
}

// Write implements io.Writer, capturing output while passing it through.
func (lw *LoggingWriter) Write(p []byte) (n int, err error) {
	lw.mu.Lock()
	defer lw.mu.Unlock()

	// Write to underlying writer first
	n, err = lw.underlying.Write(p)
	if err != nil {
		return n, err
	}

	// Capture to buffer if not truncated
	if !lw.truncated {
		if lw.buffer.Len()+len(p) > lw.maxSize {
			remaining := lw.maxSize - lw.buffer.Len()
			if remaining > 0 {
				_, _ = lw.buffer.Write(p[:remaining])
			}

			lw.truncated = true
		} else {
			_, _ = lw.buffer.Write(p)
		}
	}

	return n, nil
}

// String returns the captured output.
func (lw *LoggingWriter) String() string {
	lw.mu.Lock()
	defer lw.mu.Unlock()

	return lw.buffer.String()
}

// Bytes returns the captured output as bytes.
func (lw *LoggingWriter) Bytes() []byte {
	lw.mu.Lock()
	defer lw.mu.Unlock()

	return lw.buffer.Bytes()
}

// Len returns the length of captured output.
func (lw *LoggingWriter) Len() int {
	lw.mu.Lock()
	defer lw.mu.Unlock()

	return lw.buffer.Len()
}

// IsTruncated returns true if output was truncated due to size limits.
func (lw *LoggingWriter) IsTruncated() bool {
	lw.mu.Lock()
	defer lw.mu.Unlock()

	return lw.truncated
}

// Reset clears the captured output.
func (lw *LoggingWriter) Reset() {
	lw.mu.Lock()
	defer lw.mu.Unlock()

	lw.buffer.Reset()
	lw.truncated = false
}

// Init initializes the global logger instance with the command name.
// Log files are created as: logDir/ksuid-command.log
// Safe to call multiple times; initialization happens only once.
func Init(command string) *Logger {
	once.Do(func() {
		instance = initLogger(command)
	})

	return instance
}

// initLogger creates a new logger based on feature flag configuration.
func initLogger(command string) *Logger {
	l := &Logger{
		command: command,
	}

	enabled := flags.IsFeatureEnabled("logger")
	if !enabled {
		return l
	}

	logDir := flags.GetFeatureData("logger")
	if logDir == "" {
		_, _ = os.Stderr.WriteString("scout: SCOUT_LOGGER_ENABLED set but empty logger path\n")
		return l
	}

	// Ensure log directory exists
	if err := os.MkdirAll(logDir, 0755); err != nil {
		_, _ = os.Stderr.WriteString("scout: failed to create log directory: " + err.Error() + "\n")
		return l
	}

	// Generate unique log file path: dir/ksuid-command.log
	logPath, err := generateLogPath(logDir, command)
	if err != nil {
		_, _ = os.Stderr.WriteString("scout: failed to generate log path: " + err.Error() + "\n")
		return l
	}

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		_, _ = os.Stderr.WriteString("scout: failed to open log file: " + err.Error() + "\n")
		return l
	}

	l.file = file
	l.slog = slog.New(slog.NewJSONHandler(file, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	l.active = true

	return l
}

// generateLogPath creates a unique log file path using ksuid and command name.
func generateLogPath(logDir, command string) (string, error) {
	id, err := ksuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("failed to generate ksuid: %w", err)
	}

	if command == "" {
		command = "scout"
	}

	filename := fmt.Sprintf("%s-%s.log", id.String(), command)

	return filepath.Join(logDir, filename), nil
}

// New creates a new logger that writes to a unique file in the specified directory.
func New(logDir, command string) (*Logger, error) {
	l := &Logger{
		command: command,
	}

	if logDir == "" {
		return l, nil
	}

	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	logPath, err := generateLogPath(logDir, command)
	if err != nil {
		return nil, err
	}

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	l.file = file
	l.slog = slog.New(slog.NewJSONHandler(file, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	l.active = true

	return l, nil
}

// NewWithExactPath creates a logger that writes to the exact path specified.
func NewWithExactPath(logPath string) (*Logger, error) {
	l := &Logger{}

	if logPath == "" {
		return l, nil
	}

	if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	l.file = file
	l.slog = slog.New(slog.NewJSONHandler(file, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	l.active = true

	return l, nil
}

// Get returns the global logger instance.
// Returns nil if Init has not been called.
func Get() *Logger {
	return instance
}

// IsActive returns true if logging is enabled.
func (l *Logger) IsActive() bool {
	if l == nil {
		return false
	}

	return l.active
}

// StartExecution begins tracking a command execution.
// Returns wrapped stdout and stderr writers that capture output.
func (l *Logger) StartExecution(command string, args []string, stdout, stderr io.Writer) (io.Writer, io.Writer) {
	if l == nil || !l.active {
		return stdout, stderr
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.execution = &CommandExecution{
		Command:   command,
		Args:      args,
		StartTime: time.Now(),
		Stdout:    NewLoggingWriter(stdout),
		Stderr:    NewLoggingWriter(stderr),
	}

	l.slog.Info("command_start",
		"cmd", command,
		"args", args,
		"timestamp", l.execution.StartTime.Format(time.RFC3339),
		"pid", os.Getpid(),
	)

	return l.execution.Stdout, l.execution.Stderr
}

// EndExecution completes tracking and logs the full execution result.
func (l *Logger) EndExecution(err error) {
	if l == nil || !l.active {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.execution == nil {
		return
	}

	endTime := time.Now()
	duration := endTime.Sub(l.execution.StartTime)

	status := "success"

	var errMsg string

	if err != nil {
		status = "error"
		errMsg = err.Error()
	}

	stdout := l.execution.Stdout.String()
	stderr := l.execution.Stderr.String()
	stdoutTruncated := l.execution.Stdout.IsTruncated()
	stderrTruncated := l.execution.Stderr.IsTruncated()

	attrs := []any{
		"cmd", l.execution.Command,
		"args", l.execution.Args,
		"status", status,
		"duration_ms", duration.Milliseconds(),
		"start_time", l.execution.StartTime.Format(time.RFC3339),
		"end_time", endTime.Format(time.RFC3339),
		"pid", os.Getpid(),
	}

	if errMsg != "" {
		attrs = append(attrs, "error", errMsg)
	}

	if len(stdout) > 0 {
		attrs = append(attrs, "stdout", stdout)

		if stdoutTruncated {
			attrs = append(attrs, "stdout_truncated", true)
		}
	}

	if len(stderr) > 0 {
		attrs = append(attrs, "stderr", stderr)

		if stderrTruncated {
			attrs = append(attrs, "stderr_truncated", true)
		}
	}

	attrs = append(attrs, "stdout_bytes", l.execution.Stdout.Len())
	attrs = append(attrs, "stderr_bytes", l.execution.Stderr.Len())

	l.slog.Info("command_end", attrs...)

	l.execution = nil
}

// Close closes the log file if open.
func (l *Logger) Close() error {
	if l == nil || l.file == nil {
		return nil
	}

	return l.file.Close()
}

// Writer returns the underlying io.Writer for the logger.
func (l *Logger) Writer() io.Writer {
	if l == nil || l.file == nil {
		return io.Discard
	}

	return l.file
}

// FormatArgs formats command arguments as a single string.
func FormatArgs(args []string) string {
	return strings.Join(args, " ")
}
