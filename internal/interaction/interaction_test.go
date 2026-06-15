package interaction

import (
	"path/filepath"
	"testing"
)

func TestGate(t *testing.T) {
	// Assert only the env-enable path; the persisted feature-flag state is
	// machine-dependent and not controllable from a unit test. The disabled
	// behaviour is covered by TestNilRecorderNoOp at the recorder level.
	t.Setenv("SCOUT_INTERACTIONS", "1")
	if !Enabled() {
		t.Fatal("SCOUT_INTERACTIONS=1 should enable")
	}
}

func TestDirDefault(t *testing.T) {
	t.Setenv("SCOUT_HOME", t.TempDir())
	d, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(d) != "captures" {
		t.Fatalf("Dir()=%s, want .../captures", d)
	}
}
