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
