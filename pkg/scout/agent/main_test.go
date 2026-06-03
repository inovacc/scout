package agent

import (
	"os"
	"testing"
)

// TestMain opts the agent test binary into local SSRF targets: the
// browser-integration tests (TestHandleNavigate, TestHandleScreenshot, ...)
// drive a Provider built via NewProvider → urlpolicy.FromEnv against loopback
// httptest servers, which the default-deny policy would otherwise block. The
// agent package has no block-by-default assertion test, so a process-wide
// opt-in is safe and avoids repeating t.Setenv in every browser test.
func TestMain(m *testing.M) {
	_ = os.Setenv("SCOUT_ALLOW_LOCAL_TARGETS", "true")

	os.Exit(m.Run())
}
