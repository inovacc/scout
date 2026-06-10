package runbook

import (
	"testing"
	"time"
)

func TestDefaultAIConfig(t *testing.T) {
	cfg := defaultAIConfig()
	if cfg.timeout != 60*time.Second {
		t.Errorf("default timeout = %v, want 60s", cfg.timeout)
	}

	if cfg.provider != nil {
		t.Error("default provider should be nil")
	}

	if cfg.goal != "" || cfg.model != "" {
		t.Errorf("default goal/model should be empty, got goal=%q model=%q", cfg.goal, cfg.model)
	}
}

func TestAIRunbookOptions(t *testing.T) {
	provider := &mockLLMProvider{name: "mock"}

	cfg := defaultAIConfig()
	WithAI(provider)(cfg)
	WithGoal("scrape products")(cfg)
	WithAIModel("gpt-test")(cfg)
	WithAITimeout(5 * time.Second)(cfg)

	if cfg.provider == nil || cfg.provider.Name() != "mock" {
		t.Errorf("WithAI did not set provider")
	}

	if cfg.goal != "scrape products" {
		t.Errorf("goal = %q", cfg.goal)
	}

	if cfg.model != "gpt-test" {
		t.Errorf("model = %q", cfg.model)
	}

	if cfg.timeout != 5*time.Second {
		t.Errorf("timeout = %v, want 5s", cfg.timeout)
	}
}

func TestFallbackFromAnalysis(t *testing.T) {
	// fallbackFromAnalysis delegates to the pure rule-based GenerateRunbook,
	// so it is exercisable without a browser.
	a := &SiteAnalysis{
		URL:      "https://example.com/list",
		Metadata: map[string]string{"title": "Listing Page"},
		Containers: []ContainerCandidate{{
			Selector: ".item", Count: 4,
			Fields: []FieldCandidate{{Name: "title", Selector: "h3"}},
		}},
	}

	r, err := fallbackFromAnalysis(a)
	if err != nil {
		t.Fatalf("fallbackFromAnalysis() error: %v", err)
	}

	if r.Type != "extract" {
		t.Errorf("type = %q, want extract", r.Type)
	}

	if r.Name != "listing-page" {
		t.Errorf("name = %q, want listing-page", r.Name)
	}
}

func TestFallbackFromAnalysis_Error(t *testing.T) {
	// Analysis with no containers and no forms -> GenerateRunbook returns an error.
	_, err := fallbackFromAnalysis(&SiteAnalysis{URL: "https://x.test"})
	if err == nil {
		t.Error("expected error when analysis yields no runbook type")
	}
}
