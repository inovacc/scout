package b3pipe

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type SectionResult struct {
	ID     string `json:"id"`
	Status int    `json:"status"`
	Rows   int    `json:"rows"`
}

type Manifest struct {
	Timestamp    string          `json:"timestamp"`
	ScoutVersion string          `json:"scout_version"`
	Engine       string          `json:"engine"`
	Sections     []SectionResult `json:"sections"`
}

type Run struct {
	dir  string
	root string
}

// NewRun creates root/b3-data/<RFC3339-compact>/ with 0o700 perms.
func NewRun(root string, ts time.Time) (*Run, error) {
	stamp := ts.UTC().Format("2006-01-02T150405")
	dir := filepath.Join(root, "b3-data", stamp)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("b3: run dir: %w", err)
	}
	return &Run{dir: dir, root: root}, nil
}

func (r *Run) Dir() string { return r.dir }

func (r *Run) WriteSection(id string, raw []byte, header []string, rows [][]string) error {
	jsonPath := filepath.Join(r.dir, id+".json")
	f, err := os.OpenFile(jsonPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("b3: write %s.json: %w", id, err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Write(raw); err != nil {
		return fmt.Errorf("b3: write %s.json: %w", id, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("b3: close %s.json: %w", id, err)
	}

	csvPath := filepath.Join(r.dir, id+".csv")
	f, err = os.OpenFile(csvPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("b3: write %s.csv: %w", id, err)
	}
	defer func() { _ = f.Close() }()
	w := csv.NewWriter(f)
	if err := w.Write(header); err != nil {
		return fmt.Errorf("b3: csv header: %w", err)
	}
	if err := w.WriteAll(rows); err != nil {
		return fmt.Errorf("b3: csv rows: %w", err)
	}
	w.Flush()
	return w.Error()
}

func (r *Run) WriteManifest(m Manifest) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("b3: manifest marshal: %w", err)
	}
	manifestPath := filepath.Join(r.dir, "_run.json")
	f, err := os.OpenFile(manifestPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("b3: write manifest: %w", err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Write(b); err != nil {
		return fmt.Errorf("b3: write manifest: %w", err)
	}
	return nil
}

// UpdateLatest refreshes root/b3-data/latest/ as a copy of this run's files.
func (r *Run) UpdateLatest() error {
	latest := filepath.Join(r.root, "b3-data", "latest")
	if err := os.RemoveAll(latest); err != nil {
		return fmt.Errorf("b3: clear latest: %w", err)
	}
	if err := os.MkdirAll(latest, 0o700); err != nil {
		return fmt.Errorf("b3: latest dir: %w", err)
	}
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return fmt.Errorf("b3: read run dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(r.dir, e.Name()))
		if err != nil {
			return fmt.Errorf("b3: copy %s: %w", e.Name(), err)
		}
		destPath := filepath.Join(latest, e.Name())
		f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			return fmt.Errorf("b3: write latest %s: %w", e.Name(), err)
		}
		_, err = f.Write(b)
		_ = f.Close()
		if err != nil {
			return fmt.Errorf("b3: write latest %s: %w", e.Name(), err)
		}
	}
	return nil
}
