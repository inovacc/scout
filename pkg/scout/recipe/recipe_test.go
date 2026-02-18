package recipe

import (
	"testing"
)

func TestParse_ExtractRecipe(t *testing.T) {
	data := []byte(`{
		"version": "1",
		"name": "test_extract",
		"type": "extract",
		"url": "https://example.com",
		"wait_for": ".item",
		"items": {
			"container": ".item",
			"fields": {
				"title": "h2",
				"link": "a@href"
			}
		}
	}`)

	r, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if r.Name != "test_extract" {
		t.Errorf("name = %q, want %q", r.Name, "test_extract")
	}
	if r.Type != "extract" {
		t.Errorf("type = %q, want %q", r.Type, "extract")
	}
	if r.Items.Container != ".item" {
		t.Errorf("container = %q, want %q", r.Items.Container, ".item")
	}
	if len(r.Items.Fields) != 2 {
		t.Errorf("fields count = %d, want 2", len(r.Items.Fields))
	}
}

func TestParse_AutomateRecipe(t *testing.T) {
	data := []byte(`{
		"version": "1",
		"name": "test_automate",
		"type": "automate",
		"steps": [
			{"action": "navigate", "url": "https://example.com"},
			{"action": "click", "selector": "#btn"},
			{"action": "screenshot", "name": "result"}
		]
	}`)

	r, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if r.Type != "automate" {
		t.Errorf("type = %q, want %q", r.Type, "automate")
	}
	if len(r.Steps) != 3 {
		t.Errorf("steps count = %d, want 3", len(r.Steps))
	}
	if r.Steps[0].Action != "navigate" {
		t.Errorf("step 0 action = %q, want %q", r.Steps[0].Action, "navigate")
	}
}

func TestValidate_MissingVersion(t *testing.T) {
	r := &Recipe{Name: "test", Type: "extract", URL: "https://x.com",
		Items: &ItemSpec{Container: ".c", Fields: map[string]string{"a": "b"}}}
	if err := r.Validate(); err == nil {
		t.Error("expected error for missing version")
	}
}

func TestValidate_MissingName(t *testing.T) {
	r := &Recipe{Version: "1", Type: "extract", URL: "https://x.com",
		Items: &ItemSpec{Container: ".c", Fields: map[string]string{"a": "b"}}}
	if err := r.Validate(); err == nil {
		t.Error("expected error for missing name")
	}
}

func TestValidate_UnknownType(t *testing.T) {
	r := &Recipe{Version: "1", Name: "test", Type: "unknown"}
	if err := r.Validate(); err == nil {
		t.Error("expected error for unknown type")
	}
}

func TestValidate_ExtractMissingURL(t *testing.T) {
	r := &Recipe{Version: "1", Name: "test", Type: "extract",
		Items: &ItemSpec{Container: ".c", Fields: map[string]string{"a": "b"}}}
	if err := r.Validate(); err == nil {
		t.Error("expected error for extract missing url")
	}
}

func TestValidate_ExtractMissingItems(t *testing.T) {
	r := &Recipe{Version: "1", Name: "test", Type: "extract", URL: "https://x.com"}
	if err := r.Validate(); err == nil {
		t.Error("expected error for extract missing items")
	}
}

func TestValidate_AutomateMissingSteps(t *testing.T) {
	r := &Recipe{Version: "1", Name: "test", Type: "automate"}
	if err := r.Validate(); err == nil {
		t.Error("expected error for automate missing steps")
	}
}

func TestValidate_AutomateStepMissingAction(t *testing.T) {
	r := &Recipe{Version: "1", Name: "test", Type: "automate",
		Steps: []Step{{URL: "https://x.com"}}}
	if err := r.Validate(); err == nil {
		t.Error("expected error for step missing action")
	}
}

func TestParse_InvalidJSON(t *testing.T) {
	_, err := Parse([]byte(`{invalid`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParse_Pagination(t *testing.T) {
	data := []byte(`{
		"version": "1",
		"name": "paginated",
		"type": "extract",
		"url": "https://example.com",
		"items": {
			"container": ".item",
			"fields": {"title": "h2"}
		},
		"pagination": {
			"strategy": "click",
			"next_selector": "a.next",
			"max_pages": 3,
			"delay_ms": 500
		}
	}`)

	r, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if r.Pagination == nil {
		t.Fatal("pagination should not be nil")
	}
	if r.Pagination.Strategy != "click" {
		t.Errorf("strategy = %q, want %q", r.Pagination.Strategy, "click")
	}
	if r.Pagination.MaxPages != 3 {
		t.Errorf("max_pages = %d, want 3", r.Pagination.MaxPages)
	}
}
