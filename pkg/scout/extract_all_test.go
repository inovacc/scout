package scout

import (
	"testing"
)

func TestExtractAll(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/extract")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatal(err)
	}

	result := page.ExtractAll(&ExtractionRequest{
		Selectors: []string{"h1", ".nonexistent"},
		Attrs:     []string{"a@href", "invalid"},
		Links:     true,
		Meta:      true,
	})

	if result.URL == "" {
		t.Error("URL should not be empty")
	}

	if result.Selectors == nil {
		t.Error("Selectors should not be nil")
	}

	if result.Links == nil {
		t.Error("Links should not be nil when requested")
	}

	if result.Meta == nil {
		t.Error("Meta should not be nil when requested")
	}

	// Invalid attr spec should produce an error
	foundInvalidError := false
	for _, e := range result.Errors {
		if e == "invalid attr spec invalid (use selector@attr)" {
			foundInvalidError = true
		}
	}
	if !foundInvalidError {
		t.Error("expected error for invalid attr spec")
	}
}

func TestExtractAllWithTable(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/table")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatal(err)
	}

	result := page.ExtractAll(&ExtractionRequest{
		TableSelector: "table",
	})

	if result.Table == nil {
		t.Error("Table should not be nil")
	}
}

func TestParseAttrSpec(t *testing.T) {
	tests := []struct {
		spec   string
		sel    string
		attr   string
		wantOK bool
	}{
		{"a@href", "a", "href", true},
		{"div.class@data-id", "div.class", "data-id", true},
		{"img.hero@src", "img.hero", "src", true},
		{"@href", "", "", false},
		{"a@", "", "", false},
		{"nope", "", "", false},
		{"", "", "", false},
	}

	for _, tt := range tests {
		sel, attr, ok := ParseAttrSpec(tt.spec)
		if ok != tt.wantOK {
			t.Errorf("ParseAttrSpec(%q) ok = %v, want %v", tt.spec, ok, tt.wantOK)
			continue
		}
		if ok {
			if sel != tt.sel {
				t.Errorf("ParseAttrSpec(%q) sel = %q, want %q", tt.spec, sel, tt.sel)
			}
			if attr != tt.attr {
				t.Errorf("ParseAttrSpec(%q) attr = %q, want %q", tt.spec, attr, tt.attr)
			}
		}
	}
}
