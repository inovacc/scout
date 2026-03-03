package browser

import "testing"

func TestDetectBrowsers(t *testing.T) {
	browsers := DetectBrowsers()
	if browsers == nil {
		t.Log("DetectBrowsers returned nil (expected on systems without browsers)")
	} else {
		t.Logf("DetectBrowsers found %d browser(s)", len(browsers))

		for _, b := range browsers {
			t.Logf("  %s (%s) at %s version=%s", b.Name, b.Type, b.Path, b.Version)
		}
	}

	for i := 1; i < len(browsers); i++ {
		prev := browserTypePriority[browsers[i-1].Type]

		curr := browserTypePriority[browsers[i].Type]
		if prev > curr {
			t.Errorf("browsers not sorted by priority: %s (%d) before %s (%d)",
				browsers[i-1].Type, prev, browsers[i].Type, curr)
		}
	}
}

func TestBestDetected(t *testing.T) {
	path, bt, err := BestDetected()
	if err != nil {
		t.Logf("BestDetected() error (expected on CI): %v", err)
		return
	}

	if path == "" {
		t.Error("BestDetected() returned empty path with no error")
	}

	if bt == "" {
		t.Error("BestDetected() returned empty BrowserType")
	}

	t.Logf("BestDetected: %s (%s)", path, bt)
}

func TestBrowserTypePriority(t *testing.T) {
	p := BrowserTypePriority()
	if len(p) == 0 {
		t.Fatal("BrowserTypePriority() returned empty map")
	}

	if _, ok := p[Chrome]; !ok {
		t.Error("Chrome not in priority map")
	}

	if _, ok := p[Brave]; !ok {
		t.Error("Brave not in priority map")
	}

	if _, ok := p[Edge]; !ok {
		t.Error("Edge not in priority map")
	}

	if p[Chrome] >= p[Brave] {
		t.Error("Chrome should have higher priority (lower value) than Brave")
	}

	if p[Brave] >= p[Edge] {
		t.Error("Brave should have higher priority (lower value) than Edge")
	}
}

func TestParseBrowserVersion(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{"chrome linux", "Google Chrome 120.0.6099.109", "120.0.6099.109"},
		{"brave linux", "Brave Browser 121.1.62.153", "121.1.62.153"},
		{"edge", "Microsoft Edge 120.0.2210.91", "120.0.2210.91"},
		{"chromium with prefix", "Chromium 120.0.6099.109 built on Debian", "120.0.6099.109"},
		{"no version", "something without version", ""},
		{"three-part version", "Browser 1.2.3", "1.2.3"},
		{"empty", "", ""},
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
