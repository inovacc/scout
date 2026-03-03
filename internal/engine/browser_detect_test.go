package engine

import "testing"

func TestDetectBrowsers(t *testing.T) {
	// Should not panic; result may be empty in CI.
	browsers := DetectBrowsers()
	if browsers == nil {
		t.Log("DetectBrowsers returned nil (expected on systems without browsers)")
	} else {
		t.Logf("DetectBrowsers found %d browser(s)", len(browsers))

		for _, b := range browsers {
			t.Logf("  %s (%s) at %s version=%s", b.Name, b.Type, b.Path, b.Version)
		}
	}

	// Verify sorting: each browser should have priority <= next.
	for i := 1; i < len(browsers); i++ {
		prev := browserTypePriority[browsers[i-1].Type]

		curr := browserTypePriority[browsers[i].Type]
		if prev > curr {
			t.Errorf("browsers not sorted by priority: %s (%d) before %s (%d)",
				browsers[i-1].Type, prev, browsers[i].Type, curr)
		}
	}
}

func TestWithAutoDetect(t *testing.T) {
	o := defaults()
	if o.autoDetect {
		t.Fatal("autoDetect should be false by default")
	}

	WithAutoDetect()(o)

	if !o.autoDetect {
		t.Fatal("WithAutoDetect should set autoDetect to true")
	}
}

func TestParseBrowserVersion(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "chrome linux",
			input:  "Google Chrome 120.0.6099.109",
			expect: "120.0.6099.109",
		},
		{
			name:   "brave linux",
			input:  "Brave Browser 121.1.62.153",
			expect: "121.1.62.153",
		},
		{
			name:   "edge",
			input:  "Microsoft Edge 120.0.2210.91",
			expect: "120.0.2210.91",
		},
		{
			name:   "chromium with prefix",
			input:  "Chromium 120.0.6099.109 built on Debian",
			expect: "120.0.6099.109",
		},
		{
			name:   "no version",
			input:  "something without version",
			expect: "",
		},
		{
			name:   "three-part version",
			input:  "Browser 1.2.3",
			expect: "1.2.3",
		},
		{
			name:   "empty",
			input:  "",
			expect: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseBrowserVersion(tc.input)
			if got != tc.expect {
				t.Errorf("ParseBrowserVersion(%q) = %q, want %q", tc.input, got, tc.expect)
			}
		})
	}
}
