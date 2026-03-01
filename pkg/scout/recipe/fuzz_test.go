package recipe

import (
	"testing"
)

// FuzzParse exercises the recipe JSON parser with arbitrary input.
func FuzzParse(f *testing.F) {
	// Seed corpus: valid extract recipe.
	f.Add([]byte(`{
		"version": "1",
		"name": "test",
		"type": "extract",
		"url": "http://example.com",
		"items": {
			"container": ".row",
			"fields": {"title": "h2"}
		}
	}`))

	// Valid automate recipe.
	f.Add([]byte(`{
		"version": "1",
		"name": "auto",
		"type": "automate",
		"steps": [
			{"action": "navigate", "url": "http://example.com"},
			{"action": "click", "selector": "#btn"},
			{"action": "type", "selector": "#input", "text": "hello"}
		]
	}`))

	// Recipe with selectors and references.
	f.Add([]byte(`{
		"version": "1",
		"name": "refs",
		"type": "extract",
		"url": "http://example.com",
		"selectors": {"row": "tr.data", "title": "td.name"},
		"items": {
			"container": "$row",
			"fields": {"name": "$title", "link": "$title@href"}
		}
	}`))

	// Recipe with sibling prefix.
	f.Add([]byte(`{
		"version": "1",
		"name": "sibling",
		"type": "extract",
		"url": "http://example.com",
		"selectors": {"cell": "td.price"},
		"items": {
			"container": "tr",
			"fields": {"price": "+$cell"}
		}
	}`))

	// Recipe with pagination.
	f.Add([]byte(`{
		"version": "1",
		"name": "paged",
		"type": "extract",
		"url": "http://example.com",
		"items": {"container": ".item", "fields": {"name": ".name"}},
		"pagination": {
			"strategy": "click",
			"next_selector": ".next",
			"max_pages": 5,
			"delay_ms": 500
		}
	}`))

	// Empty object.
	f.Add([]byte(`{}`))

	// Invalid JSON.
	f.Add([]byte(`{not json`))

	// Empty input.
	f.Add([]byte(``))

	// Null.
	f.Add([]byte(`null`))

	// Missing required fields.
	f.Add([]byte(`{"version":"1","name":"x","type":"extract"}`))

	// Unknown type.
	f.Add([]byte(`{"version":"1","name":"x","type":"magic"}`))

	// Broken selector reference.
	f.Add([]byte(`{
		"version": "1",
		"name": "broken",
		"type": "extract",
		"url": "http://example.com",
		"items": {"container": "$missing", "fields": {"a": "b"}}
	}`))

	// Deep nesting via fields.
	f.Add([]byte(`{
		"version": "1",
		"name": "deep",
		"type": "extract",
		"url": "http://x.com",
		"selectors": {"a": ".a", "b": ".b", "c": ".c"},
		"items": {
			"container": "div",
			"fields": {"f1": "$a@href", "f2": "+$b", "f3": "$c@data-id"}
		}
	}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Parse must not panic regardless of input.
		r, err := Parse(data)
		if err != nil {
			return
		}

		// If parsing succeeded, the recipe should be valid.
		if r.Version == "" {
			t.Error("parsed recipe has empty version")
		}
		if r.Name == "" {
			t.Error("parsed recipe has empty name")
		}
		if r.Type != "extract" && r.Type != "automate" {
			t.Errorf("parsed recipe has unexpected type %q", r.Type)
		}
	})
}

// FuzzResolveSelector exercises selector reference resolution.
func FuzzResolveSelector(f *testing.F) {
	f.Add("$row", "row", ".data-row")
	f.Add("+$cell", "cell", "td.price")
	f.Add("$name@href", "name", "a.link")
	f.Add("+$x@data-id", "x", "span.id")
	f.Add(".plain-selector", "", "")
	f.Add("$", "", "")
	f.Add("+", "", "")
	f.Add("", "", "")
	f.Add("$missing", "other", ".other")
	f.Add("$a@", "a", ".cls")

	f.Fuzz(func(t *testing.T, sel, key, value string) {
		selectors := map[string]string{}
		if key != "" {
			selectors[key] = value
		}

		// Must not panic.
		_, _ = resolveSelector(sel, selectors)
	})
}
