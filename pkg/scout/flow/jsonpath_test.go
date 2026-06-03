package flow

import "testing"

func TestExtractPath(t *testing.T) {
	data := []byte(`{"data":{"cart":{"id":"c-1","total":42}},"items":[{"sku":"A"},{"sku":"B"}]}`)
	cases := []struct {
		path, want string
		wantErr    bool
	}{
		{"$.data.cart.id", "c-1", false},
		{"$.data.cart.total", "42", false},
		{"$.items.1.sku", "B", false},
		{"$.data.cart", `{"id":"c-1","total":42}`, false}, // object → compact JSON
		{"$.missing.key", "", true},
		{"$.items.9.sku", "", true},
	}
	for _, c := range cases {
		got, err := ExtractPath(data, c.path)
		if c.wantErr {
			if err == nil {
				t.Errorf("%s: expected error", c.path)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", c.path, err)
		}
		if got != c.want {
			t.Errorf("%s = %q, want %q", c.path, got, c.want)
		}
	}
}
