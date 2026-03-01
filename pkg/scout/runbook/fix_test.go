package runbook

import "testing"

func TestFixRunbook_NilRunbook(t *testing.T) {
	_, _, err := FixRunbook(nil, nil)
	if err == nil {
		t.Fatal("expected error for nil runbook")
	}
	if got := err.Error(); got != "runbook: fix: nil runbook" {
		t.Fatalf("unexpected error: %s", got)
	}
}

func TestFixRunbook_NoURL(t *testing.T) {
	r := &Runbook{
		Version: "1",
		Name:    "test",
		Type:    "extract",
		Items:   &ItemSpec{Container: ".item", Fields: map[string]string{"title": "h2"}},
	}
	_, _, err := FixRunbook(nil, r)
	if err == nil {
		t.Fatal("expected error for missing URL")
	}
	if got := err.Error(); got != "runbook: fix: no URL to navigate to" {
		t.Fatalf("unexpected error: %s", got)
	}
}

func TestSampleExtract_NilRunbook(t *testing.T) {
	_, err := SampleExtract(nil, nil)
	if err == nil {
		t.Fatal("expected error for nil runbook")
	}
	if got := err.Error(); got != "runbook: sample: nil runbook" {
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

func TestCopyRunbook(t *testing.T) {
	orig := &Runbook{
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

	cp := copyRunbook(orig)

	// Mutate copy and verify original is unchanged.
	cp.Items.Fields["title"] = ".new-title"
	cp.Pagination.NextSelector = ".new-next"

	if orig.Items.Fields["title"] != "h2" {
		t.Fatal("copyRunbook did not deep-copy Items.Fields")
	}
	if orig.Pagination.NextSelector != ".next" {
		t.Fatal("copyRunbook did not deep-copy Pagination")
	}
}
