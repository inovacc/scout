package recipe

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/inovacc/scout/pkg/scout"
)

// Recipe defines a declarative scraping or automation playbook.
type Recipe struct {
	Version    string      `json:"version"`
	Name       string      `json:"name"`
	Type       string      `json:"type"` // "extract" or "automate"
	URL        string      `json:"url,omitempty"`
	WaitFor    string      `json:"wait_for,omitempty"`
	Items      *ItemSpec   `json:"items,omitempty"`
	Pagination *Pagination `json:"pagination,omitempty"`
	Steps      []Step      `json:"steps,omitempty"`
	Output     Output      `json:"output,omitempty"`
}

// ItemSpec defines how to extract structured data from a page.
type ItemSpec struct {
	Container string            `json:"container"`
	Fields    map[string]string `json:"fields"` // name → "selector" or "selector@attr"; "+" prefix = sibling row
}

// Pagination configures multi-page extraction.
type Pagination struct {
	Strategy     string `json:"strategy"` // "click", "url", "scroll", "load_more"
	NextSelector string `json:"next_selector,omitempty"`
	URLTemplate  string `json:"url_template,omitempty"` // with {page}
	MaxPages     int    `json:"max_pages"`
	DelayMs      int    `json:"delay_ms"`
	DedupField   string `json:"dedup_field,omitempty"`
}

// Step is a single action in an automation recipe.
type Step struct {
	Action   string `json:"action"` // navigate, click, type, screenshot, extract, eval, wait, key
	URL      string `json:"url,omitempty"`
	Selector string `json:"selector,omitempty"`
	Text     string `json:"text,omitempty"`
	Script   string `json:"script,omitempty"`
	Name     string `json:"name,omitempty"` // screenshot name
	As       string `json:"as,omitempty"`   // variable name for result
	FullPage bool   `json:"full_page,omitempty"`
}

// Output configures the result format.
type Output struct {
	Format string `json:"format"` // "json", "csv"
}

// Result holds the output of a recipe execution.
type Result struct {
	Items       []map[string]string `json:"items,omitempty"`
	Variables   map[string]any      `json:"variables,omitempty"`
	Screenshots map[string][]byte   `json:"-"`
}

// LoadFile reads and parses a recipe JSON file.
func LoadFile(path string) (*Recipe, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("recipe: read %s: %w", path, err)
	}

	return Parse(data)
}

// Parse decodes a recipe from JSON bytes.
func Parse(data []byte) (*Recipe, error) {
	var r Recipe
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("recipe: parse: %w", err)
	}

	if err := r.Validate(); err != nil {
		return nil, err
	}

	return &r, nil
}

// Run executes a recipe against the given browser.
func Run(ctx context.Context, browser *scout.Browser, r *Recipe) (*Result, error) {
	switch r.Type {
	case "extract":
		return runExtract(ctx, browser, r)
	case "automate":
		return runAutomate(ctx, browser, r)
	default:
		return nil, fmt.Errorf("recipe: unknown type %q", r.Type)
	}
}

// Validate checks that a recipe has all required fields.
func (r *Recipe) Validate() error {
	if r.Version == "" {
		return fmt.Errorf("recipe: missing version")
	}

	if r.Name == "" {
		return fmt.Errorf("recipe: missing name")
	}

	switch r.Type {
	case "extract":
		if r.URL == "" {
			return fmt.Errorf("recipe: extract recipe requires url")
		}
		if r.Items == nil {
			return fmt.Errorf("recipe: extract recipe requires items")
		}
		if r.Items.Container == "" {
			return fmt.Errorf("recipe: items.container is required")
		}
		if len(r.Items.Fields) == 0 {
			return fmt.Errorf("recipe: items.fields is required")
		}
	case "automate":
		if len(r.Steps) == 0 {
			return fmt.Errorf("recipe: automate recipe requires steps")
		}
		for i, step := range r.Steps {
			if step.Action == "" {
				return fmt.Errorf("recipe: step %d missing action", i)
			}
		}
	default:
		return fmt.Errorf("recipe: unknown type %q (must be \"extract\" or \"automate\")", r.Type)
	}

	return nil
}
