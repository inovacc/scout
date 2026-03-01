package runbook

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRunbookPipeline_GenerateAndValidateSelectors(t *testing.T) {
	analysis := &SiteAnalysis{
		URL:      "http://example.com/products",
		PageType: "listing",
		Metadata: map[string]string{"title": "Products"},
		Containers: []ContainerCandidate{
			{
				Selector: "[data-testid=\"product\"]",
				Count:    5,
				Fields: []FieldCandidate{
					{Name: "title", Selector: "h2.name", Attr: ""},
					{Name: "link", Selector: "a", Attr: "href"},
					{Name: "price", Selector: "[data-price]", Attr: ""},
				},
			},
		},
	}

	r, err := GenerateRunbook(analysis)
	if err != nil {
		t.Fatalf("GenerateRunbook: %v", err)
	}

	scores := ScoreRunbookSelectors(r)

	// Container uses data-testid, should be excellent.
	if s := scores["container"]; s.Tier != "excellent" {
		t.Errorf("container tier = %q, want excellent", s.Tier)
	}

	// data-price field should be excellent.
	if s := scores["field:price"]; s.Tier != "excellent" {
		t.Errorf("field:price tier = %q, want excellent", s.Tier)
	}

	// No fragile warnings expected for container or data-* fields.
	for _, w := range r.Warnings {
		if strings.Contains(w, "container") || strings.Contains(w, "field:price") {
			t.Errorf("unexpected fragile warning: %s", w)
		}
	}
}

func TestRunbookPipeline_GenerateWithFragileSelectors(t *testing.T) {
	analysis := &SiteAnalysis{
		URL:      "http://example.com/items",
		PageType: "listing",
		Metadata: map[string]string{"title": "Items"},
		Containers: []ContainerCandidate{
			{
				Selector: "div",
				Count:    5,
				Fields: []FieldCandidate{
					{Name: "title", Selector: "span", Attr: ""},
				},
			},
		},
	}

	r, err := GenerateRunbook(analysis)
	if err != nil {
		t.Fatalf("GenerateRunbook: %v", err)
	}

	// Both container "div" and field "span" are tag-only = fragile.
	if len(r.Warnings) == 0 {
		t.Fatal("expected fragile warnings, got none")
	}

	foundContainer := false
	foundField := false
	for _, w := range r.Warnings {
		if strings.Contains(w, "container") {
			foundContainer = true
		}
		if strings.Contains(w, "field:title") {
			foundField = true
		}
	}
	if !foundContainer {
		t.Error("expected fragile warning for container")
	}
	if !foundField {
		t.Error("expected fragile warning for field:title")
	}
}

func TestRunbookPipeline_FixRunbookNoURL(t *testing.T) {
	r := &Runbook{
		Version: "1",
		Name:    "no-url",
		Type:    "extract",
		Items:   &ItemSpec{Container: ".c", Fields: map[string]string{"a": "b"}},
	}

	_, _, err := FixRunbook(nil, r)
	if err == nil {
		t.Fatal("expected error for missing URL")
	}
	if !strings.Contains(err.Error(), "no URL") {
		t.Errorf("error = %q, want to contain 'no URL'", err.Error())
	}
}

func TestRunbookPipeline_SampleExtractNoItems(t *testing.T) {
	r := &Runbook{
		Version: "1",
		Name:    "no-items",
		Type:    "extract",
		URL:     "http://example.com",
	}

	_, err := SampleExtract(nil, r)
	if err == nil {
		t.Fatal("expected error for missing items")
	}
	if !strings.Contains(err.Error(), "no items spec") {
		t.Errorf("error = %q, want to contain 'no items spec'", err.Error())
	}
}

func TestRunbookPipeline_LoadSaveRoundTrip(t *testing.T) {
	original := &Runbook{
		Version: "1",
		Name:    "roundtrip-test",
		Type:    "extract",
		URL:     "http://example.com/data",
		WaitFor: ".container",
		Selectors: map[string]string{
			"card": ".product-card",
		},
		Items: &ItemSpec{
			Container: ".product-card",
			Fields: map[string]string{
				"title": "h2",
				"link":  "a@href",
			},
		},
		Pagination: &Pagination{
			Strategy:     "click",
			NextSelector: ".next",
			MaxPages:     3,
			DelayMs:      500,
			DedupField:   "title",
		},
		Output: Output{Format: "json"},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	loaded, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if loaded.Version != original.Version {
		t.Errorf("Version = %q, want %q", loaded.Version, original.Version)
	}
	if loaded.Name != original.Name {
		t.Errorf("Name = %q, want %q", loaded.Name, original.Name)
	}
	if loaded.Type != original.Type {
		t.Errorf("Type = %q, want %q", loaded.Type, original.Type)
	}
	if loaded.URL != original.URL {
		t.Errorf("URL = %q, want %q", loaded.URL, original.URL)
	}
	if loaded.Items.Container != original.Items.Container {
		t.Errorf("Items.Container = %q, want %q", loaded.Items.Container, original.Items.Container)
	}
	if len(loaded.Items.Fields) != len(original.Items.Fields) {
		t.Errorf("Items.Fields count = %d, want %d", len(loaded.Items.Fields), len(original.Items.Fields))
	}
	for k, v := range original.Items.Fields {
		if loaded.Items.Fields[k] != v {
			t.Errorf("Items.Fields[%q] = %q, want %q", k, loaded.Items.Fields[k], v)
		}
	}
	if loaded.Pagination.Strategy != original.Pagination.Strategy {
		t.Errorf("Pagination.Strategy = %q, want %q", loaded.Pagination.Strategy, original.Pagination.Strategy)
	}
	if loaded.Pagination.MaxPages != original.Pagination.MaxPages {
		t.Errorf("Pagination.MaxPages = %d, want %d", loaded.Pagination.MaxPages, original.Pagination.MaxPages)
	}
	if loaded.Pagination.DedupField != original.Pagination.DedupField {
		t.Errorf("Pagination.DedupField = %q, want %q", loaded.Pagination.DedupField, original.Pagination.DedupField)
	}
	if loaded.Output.Format != original.Output.Format {
		t.Errorf("Output.Format = %q, want %q", loaded.Output.Format, original.Output.Format)
	}
}

func TestRunbookPipeline_ValidateRunbookFormat(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name:    "invalid JSON",
			input:   `{not valid json`,
			wantErr: "runbook: parse:",
		},
		{
			name:    "missing version",
			input:   `{"name":"test","type":"extract","url":"http://x","items":{"container":".c","fields":{"a":"b"}}}`,
			wantErr: "missing version",
		},
		{
			name:    "missing name",
			input:   `{"version":"1","type":"extract","url":"http://x","items":{"container":".c","fields":{"a":"b"}}}`,
			wantErr: "missing name",
		},
		{
			name:    "missing type",
			input:   `{"version":"1","name":"test"}`,
			wantErr: "unknown type",
		},
		{
			name:    "unknown type",
			input:   `{"version":"1","name":"test","type":"bogus"}`,
			wantErr: "unknown type",
		},
		{
			name:    "extract missing url",
			input:   `{"version":"1","name":"test","type":"extract","items":{"container":".c","fields":{"a":"b"}}}`,
			wantErr: "requires url",
		},
		{
			name:    "extract missing items",
			input:   `{"version":"1","name":"test","type":"extract","url":"http://x"}`,
			wantErr: "requires items",
		},
		{
			name:    "automate missing steps",
			input:   `{"version":"1","name":"test","type":"automate"}`,
			wantErr: "requires steps",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.input))
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestRunbookPipeline_InteractiveWithNilBrowser(t *testing.T) {
	_, err := InteractiveCreate(InteractiveConfig{
		Browser: nil,
		URL:     "http://example.com",
	})
	if err == nil {
		t.Fatal("expected error for nil browser")
	}
	if !strings.Contains(err.Error(), "nil browser") {
		t.Errorf("error = %q, want to contain 'nil browser'", err.Error())
	}
}

func TestRunbookPipeline_ScoreAndWarn(t *testing.T) {
	analysis := &SiteAnalysis{
		URL:      "http://example.com/mixed",
		PageType: "listing",
		Metadata: map[string]string{"title": "Mixed Selectors"},
		Containers: []ContainerCandidate{
			{
				Selector: "[data-testid=\"item\"]",
				Count:    10,
				Fields: []FieldCandidate{
					{Name: "title", Selector: "span", Attr: ""},           // tag-only = fragile
					{Name: "link", Selector: "[data-link]", Attr: "href"}, // data-* = excellent
					{Name: "price", Selector: ".price", Attr: ""},         // class = fair
				},
			},
		},
	}

	r, err := GenerateRunbook(analysis)
	if err != nil {
		t.Fatalf("GenerateRunbook: %v", err)
	}

	scores := ScoreRunbookSelectors(r)

	// Verify tiers for the mixed selectors.
	tests := []struct {
		key      string
		wantTier string
	}{
		{"container", "excellent"},
		{"field:title", "fragile"},
		{"field:link", "excellent"},
		{"field:price", "fair"},
	}

	for _, tt := range tests {
		s, ok := scores[tt.key]
		if !ok {
			t.Errorf("missing score for %q", tt.key)
			continue
		}
		if s.Tier != tt.wantTier {
			t.Errorf("scores[%q].Tier = %q, want %q (score=%.2f)", tt.key, s.Tier, tt.wantTier, s.Score)
		}
	}

	// Warnings should mention fragile selectors but not excellent/fair ones.
	hasFragileTitle := false
	for _, w := range r.Warnings {
		if strings.Contains(w, "field:title") {
			hasFragileTitle = true
		}
		if strings.Contains(w, "field:link") {
			t.Error("unexpected warning for excellent field:link")
		}
		if strings.Contains(w, "field:price") {
			t.Error("unexpected warning for fair field:price")
		}
	}
	if !hasFragileTitle {
		t.Error("expected fragile warning for field:title")
	}
}
