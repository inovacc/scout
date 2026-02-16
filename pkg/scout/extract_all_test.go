package scout

import "testing"

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
