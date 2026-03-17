package strategy

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/inovacc/scout/pkg/scout/scraper"
)

// Sink writes scraper results to an output destination.
type Sink interface {
	// Name returns the sink identifier.
	Name() string

	// Write sends a result to the sink.
	Write(result scraper.Result) error

	// Close flushes and closes the sink.
	Close() error
}

// NewSink creates a sink from configuration.
func NewSink(cfg SinkConfig) (Sink, error) {
	switch cfg.Type {
	case "json-file":
		return newJSONFileSink(cfg.Path)
	case "ndjson":
		return newNDJSONSink(cfg.Path)
	case "csv":
		return newCSVSink(cfg.Path)
	default:
		return nil, fmt.Errorf("strategy: unknown sink type %q", cfg.Type)
	}
}

// jsonFileSink collects all results and writes a single JSON array on close.
type jsonFileSink struct {
	path    string
	mu      sync.Mutex
	results []scraper.Result
}

func newJSONFileSink(path string) (*jsonFileSink, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("strategy: create sink dir: %w", err)
	}

	return &jsonFileSink{path: path}, nil
}

func (s *jsonFileSink) Name() string { return "json-file" }

func (s *jsonFileSink) Write(r scraper.Result) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.results = append(s.results, r)

	return nil
}

func (s *jsonFileSink) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(s.results, "", "  ")
	if err != nil {
		return fmt.Errorf("strategy: json sink marshal: %w", err)
	}

	if err := os.WriteFile(s.path, data, 0o600); err != nil {
		return fmt.Errorf("strategy: json sink write: %w", err)
	}

	return nil
}

// ndjsonSink writes one JSON object per line (newline-delimited JSON).
type ndjsonSink struct {
	path string
	mu   sync.Mutex
	f    *os.File
	enc  *json.Encoder
}

func newNDJSONSink(path string) (*ndjsonSink, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("strategy: create sink dir: %w", err)
	}

	f, err := os.Create(path) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("strategy: ndjson sink create: %w", err)
	}

	return &ndjsonSink{path: path, f: f, enc: json.NewEncoder(f)}, nil
}

func (s *ndjsonSink) Name() string { return "ndjson" }

func (s *ndjsonSink) Write(r scraper.Result) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.enc.Encode(r) //nolint:errchkjson
}

func (s *ndjsonSink) Close() error {
	return s.f.Close()
}

// csvSink writes results as CSV with headers: type, source, id, timestamp, author, content, url.
type csvSink struct {
	path string
	mu   sync.Mutex
	f    io.WriteCloser
	w    *csv.Writer
	hdrs bool
}

func newCSVSink(path string) (*csvSink, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("strategy: create sink dir: %w", err)
	}

	f, err := os.Create(path) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("strategy: csv sink create: %w", err)
	}

	return &csvSink{path: path, f: f, w: csv.NewWriter(f)}, nil
}

func (s *csvSink) Name() string { return "csv" }

func (s *csvSink) Write(r scraper.Result) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.hdrs {
		_ = s.w.Write([]string{"type", "source", "id", "timestamp", "author", "content", "url"})
		s.hdrs = true
	}

	return s.w.Write([]string{
		string(r.Type),
		r.Source,
		r.ID,
		r.Timestamp.Format("2006-01-02T15:04:05Z"),
		r.Author,
		r.Content,
		r.URL,
	})
}

func (s *csvSink) Close() error {
	s.w.Flush()

	return s.f.Close()
}
