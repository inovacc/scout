package recipe

import "testing"

func TestFixRecipe_NilRecipe(t *testing.T) {
	_, _, err := FixRecipe(nil, nil)
	if err == nil {
		t.Fatal("expected error for nil recipe")
	}
	if got := err.Error(); got != "recipe: fix: nil recipe" {
		t.Fatalf("unexpected error: %s", got)
	}
}

func TestFixRecipe_NoURL(t *testing.T) {
	r := &Recipe{
		Version: "1",
		Name:    "test",
		Type:    "extract",
		Items:   &ItemSpec{Container: ".item", Fields: map[string]string{"title": "h2"}},
	}
	_, _, err := FixRecipe(nil, r)
	if err == nil {
		t.Fatal("expected error for missing URL")
	}
	if got := err.Error(); got != "recipe: fix: no URL to navigate to" {
		t.Fatalf("unexpected error: %s", got)
	}
}

func TestSampleExtract_NilRecipe(t *testing.T) {
	_, err := SampleExtract(nil, nil)
	if err == nil {
		t.Fatal("expected error for nil recipe")
	}
	if got := err.Error(); got != "recipe: sample: nil recipe" {
		t.Fatalf("unexpected error: %s", got)
	}
}

func TestGuessPurpose(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"product_title", "title"},
		{"heading", "title"},
		{"item_link", "link"},
		{"photo_url", "image"},
		{"thumbnail_img", "image"},
		{"sale_price", "price"},
		{"published_date", "date"},
		{"foobar", "foobar"},
	}

	for _, tt := range tests {
		got := guessPurpose(tt.input)
		if got != tt.want {
			t.Errorf("guessPurpose(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCopyRecipe(t *testing.T) {
	orig := &Recipe{
		Version: "1",
		Name:    "test",
		Type:    "extract",
		URL:     "http://example.com",
		Items: &ItemSpec{
			Container: ".item",
			Fields:    map[string]string{"title": "h2", "link": "a@href"},
		},
		Pagination: &Pagination{Strategy: "click", NextSelector: ".next"},
	}

	cp := copyRecipe(orig)

	// Mutate copy and verify original is unchanged.
	cp.Items.Fields["title"] = ".new-title"
	cp.Pagination.NextSelector = ".new-next"

	if orig.Items.Fields["title"] != "h2" {
		t.Fatal("copyRecipe did not deep-copy Items.Fields")
	}
	if orig.Pagination.NextSelector != ".next" {
		t.Fatal("copyRecipe did not deep-copy Pagination")
	}
}
