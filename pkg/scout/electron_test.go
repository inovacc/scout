package scout

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveElectronWithVersion(t *testing.T) {
	// This only tests the resolution path — it will try to download
	// so we skip if network is unavailable or too slow.
	_, err := resolveElectron(context.Background(), "v33.2.0")
	if err != nil {
		t.Skipf("skipping: electron download unavailable: %v", err)
	}
}

func TestResolveElectronFromPATH(t *testing.T) {
	// resolveElectron with empty version checks PATH first.
	path, err := resolveElectron(context.Background(), "")
	if err != nil {
		t.Skipf("skipping: no electron in PATH and download unavailable: %v", err)
	}

	assert.NotEmpty(t, path)
}

func TestLookupElectronCDP(t *testing.T) {
	// Direct devtools URL should pass through.
	u, err := lookupElectronCDP("ws://127.0.0.1:9222/devtools/browser/abc")
	assert.NoError(t, err)
	assert.Equal(t, "ws://127.0.0.1:9222/devtools/browser/abc", u)
}

func TestLookupElectronCDPFallback(t *testing.T) {
	// Non-devtools URL that can't be resolved should fall back to raw endpoint.
	u, err := lookupElectronCDP("ws://127.0.0.1:99999")
	assert.NoError(t, err)
	assert.Equal(t, "ws://127.0.0.1:99999", u)
}
