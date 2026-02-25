package recipe

import (
	"strings"
	"testing"
)

func TestSelectorRef_Resolution(t *testing.T) {
	tests := []struct {
		name      string
		sel       string
		selectors map[string]string
		want      string
	}{
		{
			name:      "simple ref",
			sel:       "$card",
			selectors: map[string]string{"card": ".product-card"},
			want:      ".product-card",
		},
		{
			name:      "ref resolves full value",
			sel:       "$card-heading",
			selectors: map[string]string{"card-heading": ".product-card h2"},
			want:      ".product-card h2",
		},
		{
			name:      "no ref passthrough",
			sel:       ".plain-selector",
			selectors: map[string]string{"card": ".product-card"},
			want:      ".plain-selector",
		},
		{
			name:      "empty selectors map",
			sel:       "$card",
			selectors: nil,
			want:      "$card",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveSelector(tt.sel, tt.selectors)
			if err != nil {
				t.Fatalf("resolveSelector(%q): %v", tt.sel, err)
			}
			if got != tt.want {
				t.Errorf("resolveSelector(%q) = %q, want %q", tt.sel, got, tt.want)
			}
		})
	}
}

func TestSelectorRef_FullRecipeResolution(t *testing.T) {
	input := `{
		"version": "1",
		"name": "ref-test",
		"type": "extract",
		"url": "http://example.com",
		"selectors": {"card": ".product-card", "heading": ".product-card h2"},
		"items": {
			"container": "$card",
			"fields": {
				"title": "$heading"
			}
		}
	}`

	r, err := Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if r.Items.Container != ".product-card" {
		t.Errorf("container = %q, want .product-card", r.Items.Container)
	}
	if r.Items.Fields["title"] != ".product-card h2" {
		t.Errorf("field title = %q, want '.product-card h2'", r.Items.Fields["title"])
	}
}

func TestSelectorRef_UnknownRef(t *testing.T) {
	_, err := resolveSelector("$unknown", map[string]string{"card": ".c"})
	if err == nil {
		t.Fatal("expected error for unknown ref")
	}
	if !strings.Contains(err.Error(), "unknown selector reference $unknown") {
		t.Errorf("error = %q, want to contain 'unknown selector reference $unknown'", err.Error())
	}
}

func TestSelectorRef_UnknownRefInRecipe(t *testing.T) {
	input := `{
		"version": "1",
		"name": "bad-ref",
		"type": "extract",
		"url": "http://example.com",
		"selectors": {"card": ".product-card"},
		"items": {
			"container": "$card",
			"fields": {
				"title": "$nonexistent"
			}
		}
	}`

	_, err := Parse([]byte(input))
	if err == nil {
		t.Fatal("expected error for unknown ref in recipe")
	}
	if !strings.Contains(err.Error(), "$nonexistent") {
		t.Errorf("error = %q, want to contain '$nonexistent'", err.Error())
	}
}

func TestSelectorRef_NestedRef(t *testing.T) {
	// If a selector value itself contains $, it should NOT be recursively resolved.
	// resolveSelector only looks up in the map once.
	selectors := map[string]string{
		"outer": "$inner",
		"inner": ".real",
	}

	got, err := resolveSelector("$outer", selectors)
	if err != nil {
		t.Fatalf("resolveSelector: %v", err)
	}

	// Should resolve to the literal value "$inner", not ".real".
	if got != "$inner" {
		t.Errorf("got %q, want %q (no recursive resolution)", got, "$inner")
	}
}

func TestSelectorRef_AttrPreserved(t *testing.T) {
	tests := []struct {
		name      string
		sel       string
		selectors map[string]string
		want      string
	}{
		{
			name:      "href suffix",
			sel:       "$card@href",
			selectors: map[string]string{"card": "a.link"},
			want:      "a.link@href",
		},
		{
			name:      "src suffix",
			sel:       "$img@src",
			selectors: map[string]string{"img": "img.thumbnail"},
			want:      "img.thumbnail@src",
		},
		{
			name:      "attr with space in ref name",
			sel:       "$card@data-id",
			selectors: map[string]string{"card": ".card"},
			want:      ".card@data-id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveSelector(tt.sel, tt.selectors)
			if err != nil {
				t.Fatalf("resolveSelector(%q): %v", tt.sel, err)
			}
			if got != tt.want {
				t.Errorf("resolveSelector(%q) = %q, want %q", tt.sel, got, tt.want)
			}
		})
	}
}

func TestSelectorRef_SiblingPrefix(t *testing.T) {
	tests := []struct {
		name      string
		sel       string
		selectors map[string]string
		want      string
	}{
		{
			name:      "sibling with ref",
			sel:       "+$card",
			selectors: map[string]string{"card": ".product-card"},
			want:      "+.product-card",
		},
		{
			name:      "sibling with ref and attr",
			sel:       "+$card@href",
			selectors: map[string]string{"card": "a.link"},
			want:      "+a.link@href",
		},
		{
			name:      "sibling without ref",
			sel:       "+.plain",
			selectors: map[string]string{"card": ".c"},
			want:      "+.plain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveSelector(tt.sel, tt.selectors)
			if err != nil {
				t.Fatalf("resolveSelector(%q): %v", tt.sel, err)
			}
			if got != tt.want {
				t.Errorf("resolveSelector(%q) = %q, want %q", tt.sel, got, tt.want)
			}
		})
	}
}
