package browser

import (
	"runtime"
	"strings"
	"testing"
)

func TestLoadManifest(t *testing.T) {
	m := LoadManifest()
	if m == nil {
		t.Fatal("LoadManifest() returned nil")
	}

	if len(m.Chromium.Platforms) == 0 {
		t.Fatal("expected at least one platform in chromium config")
	}
}

func TestManifestDefaultRevision(t *testing.T) {
	rev := LoadManifest().DefaultRevision()
	if rev == 0 {
		t.Fatal("DefaultRevision() should not be 0")
	}
}

func TestManifestHostURLs(t *testing.T) {
	m := LoadManifest()

	hosts := m.HostURLs(m.DefaultRevision())
	if len(hosts) == 0 {
		t.Skip("no host URLs for current platform")
	}

	for i, h := range hosts {
		u := h(m.DefaultRevision())
		if u == "" {
			t.Errorf("host[%d] returned empty URL", i)
		}

		if strings.Contains(u, "{revision}") {
			t.Errorf("host[%d] URL still contains {revision}: %s", i, u)
		}
	}
}

func TestManifestPlatform(t *testing.T) {
	p := LoadManifest().Platform()
	if p == nil {
		t.Skipf("no platform config for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	if p.Prefix == "" {
		t.Error("platform Prefix is empty")
	}

	if p.Zip == "" {
		t.Error("platform Zip is empty")
	}
}

func TestManifestLatestAPI(t *testing.T) {
	url := LoadManifest().LatestAPI()
	if url == "" {
		t.Skipf("no LatestAPI for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	if !strings.HasPrefix(url, "https://") {
		t.Errorf("LatestAPI should start with https://, got %q", url)
	}
}

func TestPlatformKey(t *testing.T) {
	key := PlatformKey()
	if key == "" {
		t.Fatal("PlatformKey() returned empty")
	}

	expected := runtime.GOOS + "_" + runtime.GOARCH
	if key != expected {
		t.Errorf("PlatformKey() = %q, want %q", key, expected)
	}
}

func TestHostConf(t *testing.T) {
	conf := HostConf()
	if conf == nil {
		t.Skipf("no HostConf for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	if conf.Prefix == "" {
		t.Error("HostConf().Prefix is empty")
	}

	if conf.Zip == "" {
		t.Error("HostConf().Zip is empty")
	}

	if conf.Binary == "" {
		t.Error("HostConf().Binary is empty")
	}

	if len(conf.URLs) == 0 {
		t.Error("HostConf().URLs is empty")
	}
}

func TestBrowserDefBrowserAPI(t *testing.T) {
	m := LoadManifest()

	tests := []struct {
		name   string
		def    *BrowserDef
		key    string
		exists bool
	}{
		{"chrome known_versions", &m.Chrome, "known_versions", true},
		{"chrome latest_stable", &m.Chrome, "latest_stable", true},
		{"chrome missing_key", &m.Chrome, "nonexistent", false},
		{"brave latest_release", &m.Brave, "latest_release", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.def.BrowserAPI(tc.key)
			if tc.exists && got == "" {
				t.Errorf("BrowserAPI(%q) returned empty, expected non-empty", tc.key)
			}

			if !tc.exists && got != "" {
				t.Errorf("BrowserAPI(%q) = %q, expected empty", tc.key, got)
			}

			if tc.exists && !strings.HasPrefix(got, "https://") {
				t.Errorf("BrowserAPI(%q) = %q, expected https:// prefix", tc.key, got)
			}
		})
	}

	// Test with nil API map.
	empty := &BrowserDef{}
	if got := empty.BrowserAPI("key"); got != "" {
		t.Errorf("BrowserAPI on empty def = %q, want empty", got)
	}
}

func TestBrowserDefBrowserPlatform(t *testing.T) {
	m := LoadManifest()

	for _, tc := range []struct {
		name string
		def  *BrowserDef
	}{
		{"chrome", &m.Chrome},
		{"brave", &m.Brave},
		{"edge", &m.Edge},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := tc.def.BrowserPlatform()
			if p == nil {
				t.Skipf("no platform for %s on %s", tc.name, PlatformKey())
			}

			if p.Binary == "" {
				t.Error("BrowserPlatform().Binary is empty")
			}
		})
	}
}

func TestBrowserDefBinaryPath(t *testing.T) {
	m := LoadManifest()

	// Chrome should have a platform-specific binary.
	got := m.Chrome.BinaryPath("fallback")
	if got == "" {
		t.Error("BinaryPath returned empty")
	}

	if got == "fallback" {
		t.Skipf("no chrome platform for %s, got fallback", PlatformKey())
	}

	// Empty def should return fallback.
	empty := &BrowserDef{}
	if got := empty.BinaryPath("default-bin"); got != "default-bin" {
		t.Errorf("BinaryPath on empty def = %q, want %q", got, "default-bin")
	}
}

func TestBrowserDefZipName(t *testing.T) {
	m := LoadManifest()

	tests := []struct {
		name    string
		def     *BrowserDef
		version string
	}{
		{"chrome", &m.Chrome, "120.0.6099.109"},
		{"brave", &m.Brave, "1.87.188"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.def.ZipName(tc.version)
			if got == "" {
				t.Skipf("no ZipName for %s on %s", tc.name, PlatformKey())
			}

			if strings.Contains(got, "{version}") {
				t.Errorf("ZipName still contains {version}: %s", got)
			}

			if !strings.HasSuffix(got, ".zip") {
				t.Errorf("ZipName = %q, expected .zip suffix", got)
			}
		})
	}

	// Empty def should return empty.
	empty := &BrowserDef{}
	if got := empty.ZipName("1.0"); got != "" {
		t.Errorf("ZipName on empty def = %q, want empty", got)
	}
}

func TestBrowserDefDownloadURL(t *testing.T) {
	m := LoadManifest()

	tests := []struct {
		name    string
		def     *BrowserDef
		version string
	}{
		{"chrome", &m.Chrome, "120.0.6099.109"},
		{"brave", &m.Brave, "1.87.188"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.def.DownloadURL(tc.version)
			if got == "" {
				t.Skipf("no DownloadURL for %s on %s", tc.name, PlatformKey())
			}

			if strings.Contains(got, "{version}") {
				t.Errorf("DownloadURL still contains {version}: %s", got)
			}

			if !strings.HasPrefix(got, "https://") {
				t.Errorf("DownloadURL = %q, expected https:// prefix", got)
			}
		})
	}

	// Empty def should return empty.
	empty := &BrowserDef{}
	if got := empty.DownloadURL("1.0"); got != "" {
		t.Errorf("DownloadURL on empty def = %q, want empty", got)
	}
}
