package flags

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func resetState(t *testing.T) {
	t.Helper()

	tmpDir := t.TempDir()
	appDir = tmpDir
	errInit = nil
	exportMu = sync.Once{}
	cacheMu = sync.RWMutex{}
	flagCache = nil
	// Don't reset initOnce — appDir is already set to tmpDir,
	// and re-running ensureInit would overwrite it with the real cache dir.
}

func TestEnableDisableFeature(t *testing.T) {
	resetState(t)

	if err := EnableFeature("logger", "/tmp/logs"); err != nil {
		t.Fatalf("EnableFeature: %v", err)
	}

	if !IsFeatureEnabled("logger") {
		t.Fatal("expected logger to be enabled")
	}

	data := GetFeatureData("logger")
	if data != "/tmp/logs" {
		t.Fatalf("expected /tmp/logs, got %q", data)
	}

	if err := DisableFeature("logger"); err != nil {
		t.Fatalf("DisableFeature: %v", err)
	}

	if IsFeatureEnabled("logger") {
		t.Fatal("expected logger to be disabled")
	}
}

func TestLoadFeatureFlags(t *testing.T) {
	resetState(t)

	// Create an enabled flag file
	path := filepath.Join(appDir, "SCOUT_TESTFLAG_ENABLED")
	if err := os.WriteFile(path, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	flags, err := LoadFeatureFlags()
	if err != nil {
		t.Fatal(err)
	}

	if !flags["TESTFLAG"] {
		t.Fatal("expected TESTFLAG to be enabled")
	}
}

func TestShouldIgnoreCommand(t *testing.T) {
	if !ShouldIgnoreCommand("logger") {
		t.Fatal("expected logger to be ignored")
	}

	if ShouldIgnoreCommand("navigate") {
		t.Fatal("expected navigate to not be ignored")
	}
}
