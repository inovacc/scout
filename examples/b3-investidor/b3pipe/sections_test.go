package b3pipe

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSections(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "sections.yaml")
	content := `base_url: "https://www.investidor.b3.com.br"
auth:
  mode: "bearer"
  token_storage_key: "accessToken"
sections:
  - id: "posicao"
    endpoint: "https://investidor.b3.com.br/api/posicao"
    output: "posicao"
    record_path: "data"
`
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadSections(p)
	if err != nil {
		t.Fatalf("LoadSections: %v", err)
	}
	if cfg.BaseURL != "https://www.investidor.b3.com.br" {
		t.Errorf("base_url = %q", cfg.BaseURL)
	}
	if cfg.Auth.Mode != "bearer" || cfg.Auth.TokenStorageKey != "accessToken" {
		t.Errorf("auth = %+v", cfg.Auth)
	}
	if len(cfg.Sections) != 1 || cfg.Sections[0].ID != "posicao" || cfg.Sections[0].RecordPath != "data" {
		t.Errorf("sections = %+v", cfg.Sections)
	}
}

func TestLoadSectionsRejectsEmpty(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "sections.yaml")
	if err := os.WriteFile(p, []byte("base_url: \"\"\nsections: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSections(p); err == nil {
		t.Fatal("expected error for empty base_url / sections")
	}
}
