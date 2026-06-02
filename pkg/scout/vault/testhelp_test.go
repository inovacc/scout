package vault

import (
	"testing"

	"github.com/inovacc/scout/pkg/scout"
)

// newInjectTestBrowser returns an owned headless browser, skipping if Chromium
// is unavailable. Caller defers the returned cleanup.
func newInjectTestBrowser(t *testing.T) (*scout.Browser, func()) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping browser test in short mode")
	}
	b, err := scout.New() // headless is the default
	if err != nil {
		t.Skipf("browser unavailable: %v", err)
	}
	return b, func() { _ = b.Close() }
}
