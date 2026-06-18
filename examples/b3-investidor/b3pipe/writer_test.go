package b3pipe

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunWriteSectionAndManifest(t *testing.T) {
	root := t.TempDir()
	ts := time.Date(2026, 6, 18, 14, 30, 0, 0, time.UTC)
	run, err := NewRun(root, ts)
	if err != nil {
		t.Fatalf("NewRun: %v", err)
	}
	raw := []byte(`{"data":[{"ticker":"PETR4"}]}`)
	if err := run.WriteSection("posicao", raw, []string{"ticker"}, [][]string{{"PETR4"}}); err != nil {
		t.Fatalf("WriteSection: %v", err)
	}
	// raw json present
	jsonPath := filepath.Join(run.Dir(), "posicao.json")
	if b, _ := os.ReadFile(jsonPath); string(b) != string(raw) {
		t.Errorf("posicao.json = %q", b)
	}
	// csv has header + row
	f, err := os.Open(filepath.Join(run.Dir(), "posicao.csv"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	recs, _ := csv.NewReader(f).ReadAll()
	if len(recs) != 2 || recs[0][0] != "ticker" || recs[1][0] != "PETR4" {
		t.Errorf("csv = %v", recs)
	}
	// mode 0o600
	info, _ := os.Stat(jsonPath)
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v", info.Mode().Perm())
	}
	// manifest + latest
	if err := run.WriteManifest(Manifest{Timestamp: ts.Format(time.RFC3339), Engine: "B",
		Sections: []SectionResult{{ID: "posicao", Status: 200, Rows: 1}}}); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}
	if err := run.UpdateLatest(); err != nil {
		t.Fatalf("UpdateLatest: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "b3-data", "latest", "_run.json")); err != nil {
		t.Errorf("latest/_run.json missing: %v", err)
	}
}
