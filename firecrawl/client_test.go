package firecrawl

import (
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		apiKey  string
		opts    []Option
		wantErr bool
	}{
		{name: "valid key", apiKey: "fc-xxx"},
		{name: "empty key", apiKey: "", wantErr: true},
		{name: "custom url", apiKey: "fc-xxx", opts: []Option{WithAPIURL("http://localhost:3000")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := New(tt.apiKey, tt.opts...)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if c.apiKey != tt.apiKey {
				t.Errorf("apiKey = %q, want %q", c.apiKey, tt.apiKey)
			}
		})
	}
}
