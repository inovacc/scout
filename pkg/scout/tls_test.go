package scout

import "testing"

func TestWithTLSProfile(t *testing.T) {
	tests := []struct {
		name    string
		profile string
		want    string
	}{
		{"default empty", "", ""},
		{"chrome", "chrome", "chrome"},
		{"randomized", "randomized", "randomized"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := defaults()
			if tt.profile != "" {
				WithTLSProfile(tt.profile)(o)
			}

			if o.tlsProfile != tt.want {
				t.Errorf("tlsProfile = %q, want %q", o.tlsProfile, tt.want)
			}
		})
	}
}
