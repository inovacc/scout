package runbook

import (
	"strings"
	"testing"
)

// These tests exercise the pure (browser-free) generation logic by constructing
// SiteAnalysis values directly, instead of routing through AnalyzeSite which
// requires a live browser.

func TestDetectRunbookType(t *testing.T) {
	tests := []struct {
		name     string
		analysis *SiteAnalysis
		want     string
	}{
		{
			name:     "containers with >=3 items -> extract",
			analysis: &SiteAnalysis{Containers: []ContainerCandidate{{Selector: ".item", Count: 5}}},
			want:     "extract",
		},
		{
			name:     "container with <3 items but forms -> automate",
			analysis: &SiteAnalysis{Containers: []ContainerCandidate{{Selector: ".item", Count: 2}}, Forms: []FormCandidate{{Selector: "form"}}},
			want:     "automate",
		},
		{
			name:     "forms only -> automate",
			analysis: &SiteAnalysis{Forms: []FormCandidate{{Selector: "form"}}},
			want:     "automate",
		},
		{
			name:     "nothing -> empty",
			analysis: &SiteAnalysis{},
			want:     "",
		},
		{
			name:     "container exactly 3 -> extract (boundary)",
			analysis: &SiteAnalysis{Containers: []ContainerCandidate{{Selector: ".item", Count: 3}}},
			want:     "extract",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := detectRunbookType(tt.analysis); got != tt.want {
				t.Errorf("detectRunbookType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInferName(t *testing.T) {
	long := strings.Repeat("a", 60)

	tests := []struct {
		name     string
		analysis *SiteAnalysis
		want     string
	}{
		{
			name:     "title sanitized to kebab-case lowercase",
			analysis: &SiteAnalysis{Metadata: map[string]string{"title": "My Cool Page"}},
			want:     "my-cool-page",
		},
		{
			name:     "title truncated to 40 chars",
			analysis: &SiteAnalysis{Metadata: map[string]string{"title": long}},
			want:     long[:40],
		},
		{
			name:     "empty title falls through to host",
			analysis: &SiteAnalysis{Metadata: map[string]string{"title": ""}, URL: "https://example.com/path"},
			want:     "example.com",
		},
		{
			name:     "no metadata, url host used",
			analysis: &SiteAnalysis{URL: "https://shop.example.org/items?page=2"},
			want:     "shop.example.org",
		},
		{
			name:     "no title and unparsable/empty host -> default",
			analysis: &SiteAnalysis{URL: ""},
			want:     "generated-runbook",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := inferName(tt.analysis); got != tt.want {
				t.Errorf("inferName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenerateExtract_AllFields(t *testing.T) {
	a := &SiteAnalysis{
		URL: "https://example.com/list",
		Containers: []ContainerCandidate{
			{
				Selector: ".product",
				Count:    4,
				Fields: []FieldCandidate{
					{Name: "title", Selector: "h3"},
					{Name: "link", Selector: "a", Attr: "href"},
				},
			},
		},
	}

	r, err := generateExtract(a, &generateOptions{maxPages: 1}, "products")
	if err != nil {
		t.Fatalf("generateExtract() error: %v", err)
	}

	if r.Type != "extract" || r.URL != "https://example.com/list" {
		t.Errorf("unexpected runbook header: type=%q url=%q", r.Type, r.URL)
	}

	if r.WaitFor != ".product" {
		t.Errorf("WaitFor = %q, want %q", r.WaitFor, ".product")
	}

	if r.Items == nil || r.Items.Container != ".product" {
		t.Fatalf("Items.Container wrong: %+v", r.Items)
	}

	if r.Items.Fields["title"] != "h3" {
		t.Errorf("title field = %q, want %q", r.Items.Fields["title"], "h3")
	}

	// Attr should be appended with "@".
	if r.Items.Fields["link"] != "a@href" {
		t.Errorf("link field = %q, want %q", r.Items.Fields["link"], "a@href")
	}
}

func TestGenerateExtract_FieldFilter(t *testing.T) {
	a := &SiteAnalysis{
		URL: "https://example.com/list",
		Containers: []ContainerCandidate{
			{
				Selector: ".card",
				Count:    3,
				Fields: []FieldCandidate{
					{Name: "title", Selector: "h2"},
					{Name: "price", Selector: ".price"},
					{Name: "image", Selector: "img", Attr: "src"},
				},
			},
		},
	}

	// Only request title + image; price should be excluded.
	r, err := generateExtract(a, &generateOptions{fields: []string{"title", "image"}}, "n")
	if err != nil {
		t.Fatalf("generateExtract() error: %v", err)
	}

	if _, ok := r.Items.Fields["price"]; ok {
		t.Error("price should have been filtered out")
	}

	if r.Items.Fields["title"] != "h2" {
		t.Errorf("title = %q", r.Items.Fields["title"])
	}

	if r.Items.Fields["image"] != "img@src" {
		t.Errorf("image = %q, want %q", r.Items.Fields["image"], "img@src")
	}

	if len(r.Items.Fields) != 2 {
		t.Errorf("field count = %d, want 2", len(r.Items.Fields))
	}
}

func TestGenerateExtract_Errors(t *testing.T) {
	// No containers.
	if _, err := generateExtract(&SiteAnalysis{}, &generateOptions{}, "n"); err == nil {
		t.Error("expected error for no containers")
	}

	// Container present but no discoverable fields.
	a := &SiteAnalysis{Containers: []ContainerCandidate{{Selector: ".x", Count: 3}}}
	if _, err := generateExtract(a, &generateOptions{}, "n"); err == nil {
		t.Error("expected error for no fields")
	}

	// Requested fields that don't exist -> resulting field map empty -> error.
	a2 := &SiteAnalysis{Containers: []ContainerCandidate{{
		Selector: ".x", Count: 3, Fields: []FieldCandidate{{Name: "title", Selector: "h1"}},
	}}}
	if _, err := generateExtract(a2, &generateOptions{fields: []string{"nonexistent"}}, "n"); err == nil {
		t.Error("expected error when requested fields don't match any discovered field")
	}
}

func TestGenerateExtract_PaginationClick(t *testing.T) {
	a := &SiteAnalysis{
		URL: "https://example.com/list",
		Containers: []ContainerCandidate{{
			Selector: ".item", Count: 3, Fields: []FieldCandidate{{Name: "title", Selector: "h3"}},
		}},
		Pagination: &PaginationCandidate{Strategy: "click", NextSelector: "a.next"},
	}

	// maxPages < 2 should be bumped to 5.
	r, err := generateExtract(a, &generateOptions{maxPages: 1}, "n")
	if err != nil {
		t.Fatalf("generateExtract() error: %v", err)
	}

	if r.Pagination == nil {
		t.Fatal("expected pagination on runbook")
	}

	if r.Pagination.Strategy != "click" || r.Pagination.NextSelector != "a.next" {
		t.Errorf("pagination wrong: %+v", r.Pagination)
	}

	if r.Pagination.MaxPages != 5 {
		t.Errorf("MaxPages = %d, want 5 (bumped from <2)", r.Pagination.MaxPages)
	}

	// Explicit maxPages >= 2 should be preserved.
	r2, _ := generateExtract(a, &generateOptions{maxPages: 9}, "n")
	if r2.Pagination.MaxPages != 9 {
		t.Errorf("MaxPages = %d, want 9 (preserved)", r2.Pagination.MaxPages)
	}

	// Non-click strategy should not produce pagination on the runbook.
	a.Pagination = &PaginationCandidate{Strategy: "url"}
	r3, _ := generateExtract(a, &generateOptions{}, "n")
	if r3.Pagination != nil {
		t.Errorf("expected no pagination for non-click strategy, got %+v", r3.Pagination)
	}
}

func TestGenerateAutomate(t *testing.T) {
	a := &SiteAnalysis{
		URL: "https://example.com/login",
		Forms: []FormCandidate{
			{
				Selector: "form#login",
				Fields: []FormFieldCandidate{
					{Name: "csrf", Type: "hidden", Selector: "[name=csrf]"},
					{Name: "username", Type: "text", Selector: "#username"},
					{Name: "password", Type: "password", Selector: "#password"},
					{Name: "noselector", Type: "text", Selector: ""},
					{Name: "go", Type: "submit", Selector: "[type=submit]"},
				},
			},
		},
	}

	r, err := generateAutomate(a, &generateOptions{}, "login")
	if err != nil {
		t.Fatalf("generateAutomate() error: %v", err)
	}

	if r.Type != "automate" {
		t.Errorf("type = %q, want automate", r.Type)
	}

	if r.Steps[0].Action != "navigate" || r.Steps[0].URL != "https://example.com/login" {
		t.Errorf("first step should navigate, got %+v", r.Steps[0])
	}

	// hidden, submit, and empty-selector fields are skipped: only username + password typed.
	typeSteps := 0
	for _, s := range r.Steps {
		if s.Action == "type" {
			typeSteps++
			if s.Selector != "#username" && s.Selector != "#password" {
				t.Errorf("unexpected type step selector: %q", s.Selector)
			}
		}
	}

	if typeSteps != 2 {
		t.Errorf("type steps = %d, want 2 (hidden/submit/empty skipped)", typeSteps)
	}

	// Last step is a submit click referencing the form selector.
	last := r.Steps[len(r.Steps)-1]
	if last.Action != "click" || !strings.Contains(last.Selector, "form#login") {
		t.Errorf("last step should be submit click, got %+v", last)
	}
}

func TestGenerateAutomate_NoForms(t *testing.T) {
	if _, err := generateAutomate(&SiteAnalysis{}, &generateOptions{}, "n"); err == nil {
		t.Error("expected error for no forms")
	}
}

func TestGenerateRunbook_NilAndUnknownType(t *testing.T) {
	if _, err := GenerateRunbook(nil); err == nil {
		t.Error("expected error for nil analysis")
	}

	// Analysis with neither containers nor forms -> detectRunbookType returns ""
	// -> default branch error.
	if _, err := GenerateRunbook(&SiteAnalysis{URL: "https://x.test"}); err == nil {
		t.Error("expected error when type cannot be determined")
	}
}

func TestGenerateRunbook_DispatchExtract(t *testing.T) {
	a := &SiteAnalysis{
		URL:      "https://example.com/list",
		Metadata: map[string]string{"title": "Cool Items"},
		Containers: []ContainerCandidate{{
			Selector: ".item", Count: 4,
			Fields: []FieldCandidate{{Name: "title", Selector: "h3"}},
		}},
	}

	r, err := GenerateRunbook(a)
	if err != nil {
		t.Fatalf("GenerateRunbook() error: %v", err)
	}

	if r.Type != "extract" {
		t.Errorf("type = %q, want extract", r.Type)
	}

	if r.Name != "cool-items" {
		t.Errorf("name = %q, want cool-items", r.Name)
	}
}

func TestGenerateRunbook_ForceTypeOverridesDetection(t *testing.T) {
	// Detection would say "extract" (containers >=3), but force "automate".
	a := &SiteAnalysis{
		URL: "https://example.com/list",
		Containers: []ContainerCandidate{{
			Selector: ".item", Count: 4,
			Fields: []FieldCandidate{{Name: "title", Selector: "h3"}},
		}},
		// No forms -> forced automate must error in generateAutomate.
	}

	if _, err := GenerateRunbook(a, WithGenerateType("automate")); err == nil {
		t.Error("expected error: forced automate with no forms")
	}
}

func TestGenerateRunbook_FragileWarnings(t *testing.T) {
	// A deeply-nested positional selector should score as fragile and emit a warning.
	a := &SiteAnalysis{
		URL: "https://example.com/list",
		Containers: []ContainerCandidate{{
			Selector: "body > div > div:nth-child(3) > ul > li",
			Count:    4,
			Fields: []FieldCandidate{
				{Name: "title", Selector: "div > div > span:nth-child(2)"},
			},
		}},
	}

	r, err := GenerateRunbook(a)
	if err != nil {
		t.Fatalf("GenerateRunbook() error: %v", err)
	}

	foundFragile := false
	for _, w := range r.Warnings {
		if strings.Contains(w, "fragile selector") {
			foundFragile = true
		}
	}

	if !foundFragile {
		t.Errorf("expected a fragile-selector warning, got warnings: %v", r.Warnings)
	}
}

func TestGenerateRunbook_OptionSetters(t *testing.T) {
	o := &generateOptions{}
	WithGenerateType("extract")(o)
	WithGenerateFields("a", "b")(o)
	WithGenerateMaxPages(7)(o)

	if o.forceType != "extract" {
		t.Errorf("forceType = %q", o.forceType)
	}

	if len(o.fields) != 2 || o.fields[0] != "a" || o.fields[1] != "b" {
		t.Errorf("fields = %v", o.fields)
	}

	if o.maxPages != 7 {
		t.Errorf("maxPages = %d, want 7", o.maxPages)
	}
}
