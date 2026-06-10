package flags

import (
	"os"
	"path/filepath"
	"testing"
)

// isolateAppDir points appDir at a fresh temp directory and resets the
// caches. It first drains initOnce (via ensureInit) so that a later
// LoadFeatureFlags call cannot fire the sync.Once and clobber appDir with the
// real user cache dir. This makes the test independent of execution order.
func isolateAppDir(t *testing.T) {
	t.Helper()

	// Drain the init Once if it has not fired yet; ignore the error because we
	// immediately override appDir afterwards with our own temp dir.
	_ = ensureInit()

	resetState(t)
}

// writeFlagFile creates a raw flag file directly in appDir (bypassing the
// public API) so tests can exercise the on-disk parsing branches in isolation.
func writeFlagFile(t *testing.T, name, contents string) {
	t.Helper()

	path := filepath.Join(appDir, name)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write flag file %q: %v", name, err)
	}
}

func TestLoadFeatureFlags_ParsingBranches(t *testing.T) {
	isolateAppDir(t)

	// _ENABLED -> true.
	writeFlagFile(t, "SCOUT_ALPHA_ENABLED", "data")
	// _DISABLED with no matching ENABLED -> false.
	writeFlagFile(t, "SCOUT_BETA_DISABLED", "")
	// Both files present: _ENABLED wins and the _DISABLED branch must NOT
	// overwrite the existing true value (exercises the `!exists` guard).
	writeFlagFile(t, "SCOUT_GAMMA_ENABLED", "")
	writeFlagFile(t, "SCOUT_GAMMA_DISABLED", "")
	// Missing prefix -> skipped entirely.
	writeFlagFile(t, "NOPREFIX_DELTA_ENABLED", "")
	// Has prefix but neither suffix -> skipped.
	writeFlagFile(t, "SCOUT_EPSILON_OTHER", "")
	// A subdirectory must be skipped (entry.IsDir() branch).
	if err := os.MkdirAll(filepath.Join(appDir, "SCOUT_ZETA_ENABLED"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	flags, err := LoadFeatureFlags()
	if err != nil {
		t.Fatalf("LoadFeatureFlags: %v", err)
	}

	want := map[string]bool{
		"ALPHA": true,
		"BETA":  false,
		"GAMMA": true,
	}

	if len(flags) != len(want) {
		t.Fatalf("flag count = %d, want %d (flags=%v)", len(flags), len(want), flags)
	}

	for feature, wantVal := range want {
		got, ok := flags[feature]
		if !ok {
			t.Errorf("feature %q missing from result", feature)
			continue
		}

		if got != wantVal {
			t.Errorf("feature %q = %v, want %v", feature, got, wantVal)
		}
	}

	// The directory entry must not have produced a ZETA flag.
	if _, ok := flags["ZETA"]; ok {
		t.Error("directory entry SCOUT_ZETA_ENABLED leaked into flags")
	}

	// NOPREFIX/EPSILON must not appear.
	for _, skip := range []string{"DELTA", "EPSILON", "NOPREFIX_DELTA", "EPSILON_OTHER"} {
		if _, ok := flags[skip]; ok {
			t.Errorf("unexpected feature %q present", skip)
		}
	}
}

func TestLoadFeatureFlags_PopulatesCache(t *testing.T) {
	isolateAppDir(t)

	if GetCachedFlags() != nil {
		t.Fatal("cache should be nil before any load")
	}

	writeFlagFile(t, "SCOUT_CACHED_ENABLED", "")

	if _, err := LoadFeatureFlags(); err != nil {
		t.Fatalf("LoadFeatureFlags: %v", err)
	}

	cached := GetCachedFlags()
	if cached == nil {
		t.Fatal("cache should be populated after LoadFeatureFlags")
	}

	if !cached["CACHED"] {
		t.Fatalf("expected CACHED=true in cache, got %v", cached)
	}
}

func TestGetCachedFlags_ReturnsCopy(t *testing.T) {
	isolateAppDir(t)

	writeFlagFile(t, "SCOUT_COPY_ENABLED", "")

	if _, err := LoadFeatureFlags(); err != nil {
		t.Fatalf("LoadFeatureFlags: %v", err)
	}

	first := GetCachedFlags()
	if first == nil {
		t.Fatal("expected populated cache")
	}

	// Mutating the returned map must not affect subsequent reads.
	first["COPY"] = false
	first["INJECTED"] = true

	second := GetCachedFlags()
	if !second["COPY"] {
		t.Error("mutation of returned map leaked into cache (COPY flipped)")
	}

	if _, ok := second["INJECTED"]; ok {
		t.Error("mutation of returned map leaked into cache (INJECTED added)")
	}
}

func TestGetCachedFlags_NilWhenUnpopulated(t *testing.T) {
	isolateAppDir(t)

	if got := GetCachedFlags(); got != nil {
		t.Fatalf("expected nil cache, got %v", got)
	}
}

func TestEnableFeature_OverwritesAndRemovesDisabled(t *testing.T) {
	isolateAppDir(t)

	// Pre-seed a DISABLED file; EnableFeature must remove it.
	disabled := filepath.Join(appDir, fixDisableName("OVR"))
	if err := os.WriteFile(disabled, nil, 0o644); err != nil {
		t.Fatalf("seed disabled: %v", err)
	}

	if err := EnableFeature("ovr", "first"); err != nil {
		t.Fatalf("EnableFeature(first): %v", err)
	}

	if _, err := os.Stat(disabled); !os.IsNotExist(err) {
		t.Fatalf("disabled file should have been removed, stat err = %v", err)
	}

	if data := GetFeatureData("ovr"); data != "first" {
		t.Fatalf("data = %q, want %q", data, "first")
	}

	// Re-enabling with new data must overwrite (truncate), not append.
	if err := EnableFeature("OVR", "second"); err != nil {
		t.Fatalf("EnableFeature(second): %v", err)
	}

	if data := GetFeatureData("ovr"); data != "second" {
		t.Fatalf("after overwrite data = %q, want %q", data, "second")
	}
}

func TestEnableFeature_EmptyDataCreatesEmptyFile(t *testing.T) {
	isolateAppDir(t)

	if err := EnableFeature("nodata", ""); err != nil {
		t.Fatalf("EnableFeature: %v", err)
	}

	if !IsFeatureEnabled("nodata") {
		t.Fatal("expected nodata enabled")
	}

	enabled := filepath.Join(appDir, fixEnableName("NODATA"))
	info, err := os.Stat(enabled)
	if err != nil {
		t.Fatalf("stat enabled file: %v", err)
	}

	if info.Size() != 0 {
		t.Fatalf("expected empty file, size = %d", info.Size())
	}

	if data := GetFeatureData("nodata"); data != "" {
		t.Fatalf("expected empty data, got %q", data)
	}
}

func TestDisableFeature_RenamesExistingEnabled(t *testing.T) {
	isolateAppDir(t)

	if err := EnableFeature("ren", "payload"); err != nil {
		t.Fatalf("EnableFeature: %v", err)
	}

	if err := DisableFeature("ren"); err != nil {
		t.Fatalf("DisableFeature: %v", err)
	}

	enabled := filepath.Join(appDir, fixEnableName("REN"))
	if _, err := os.Stat(enabled); !os.IsNotExist(err) {
		t.Fatalf("enabled file should be gone after rename, err = %v", err)
	}

	disabled := filepath.Join(appDir, fixDisableName("REN"))
	if _, err := os.Stat(disabled); err != nil {
		t.Fatalf("disabled file should exist after rename, err = %v", err)
	}

	if IsFeatureEnabled("ren") {
		t.Fatal("feature should be disabled")
	}
}

func TestDisableFeature_CreatesDisabledWhenNoEnabled(t *testing.T) {
	isolateAppDir(t)

	// No enabled file exists -> DisableFeature takes the os.Create branch.
	if err := DisableFeature("fresh"); err != nil {
		t.Fatalf("DisableFeature: %v", err)
	}

	disabled := filepath.Join(appDir, fixDisableName("FRESH"))
	if _, err := os.Stat(disabled); err != nil {
		t.Fatalf("disabled file should exist, err = %v", err)
	}

	if IsFeatureEnabled("fresh") {
		t.Fatal("feature should not be enabled")
	}
}

func TestGetFeatureData_TrimsLeadingAndTrailingNewlines(t *testing.T) {
	isolateAppDir(t)

	// Write directly so we control the exact bytes (with surrounding newlines).
	writeFlagFile(t, fixEnableName("TRIM"), "\nhello world\n")

	if data := GetFeatureData("trim"); data != "hello world" {
		t.Fatalf("data = %q, want %q", data, "hello world")
	}
}

func TestGetFeatureData_MissingFileReturnsEmpty(t *testing.T) {
	isolateAppDir(t)

	if data := GetFeatureData("ghost"); data != "" {
		t.Fatalf("expected empty string for missing feature, got %q", data)
	}
}

func TestIsFeatureEnabled_CaseInsensitiveAndCacheBypass(t *testing.T) {
	isolateAppDir(t)

	// EnableFeature invalidates the cache (deferred), so IsFeatureEnabled
	// must reload from disk and still find the flag via case-folding.
	if err := EnableFeature("MyFeature", ""); err != nil {
		t.Fatalf("EnableFeature: %v", err)
	}

	if !IsFeatureEnabled("myfeature") {
		t.Error("expected lower-case lookup to match")
	}

	if !IsFeatureEnabled("MYFEATURE") {
		t.Error("expected upper-case lookup to match")
	}

	if IsFeatureEnabled("absent") {
		t.Error("absent feature must report disabled")
	}
}

func TestExportFlagsToEnv(t *testing.T) {
	isolateAppDir(t)

	// Snapshot and restore env vars we may touch (exportFlagsToEnv uses the
	// real os.Setenv rather than t.Setenv).
	keys := []string{"SCOUT_EXPON", "SCOUT_EXPOFF"}

	saved := make(map[string]*string, len(keys))
	for _, k := range keys {
		if v, ok := os.LookupEnv(k); ok {
			vv := v
			saved[k] = &vv
		} else {
			saved[k] = nil
		}
	}

	t.Cleanup(func() {
		for _, k := range keys {
			if v := saved[k]; v != nil {
				_ = os.Setenv(k, *v)
			} else {
				_ = os.Unsetenv(k)
			}
		}
	})

	writeFlagFile(t, "SCOUT_EXPON_ENABLED", "")
	writeFlagFile(t, "SCOUT_EXPOFF_DISABLED", "")

	if err := ExportFlagsToEnv(); err != nil {
		t.Fatalf("ExportFlagsToEnv: %v", err)
	}

	if got := os.Getenv("SCOUT_EXPON"); got != "1" {
		t.Errorf("SCOUT_EXPON = %q, want \"1\"", got)
	}

	if got := os.Getenv("SCOUT_EXPOFF"); got != "0" {
		t.Errorf("SCOUT_EXPOFF = %q, want \"0\"", got)
	}
}

func TestExportFlagsToEnv_RunsOnlyOnce(t *testing.T) {
	isolateAppDir(t)

	// exportMu is a sync.Once; a second call after a successful first call
	// must be a no-op even if the underlying flags change on disk.
	key := "SCOUT_ONCEFLAG"

	orig, had := os.LookupEnv(key)

	t.Cleanup(func() {
		if had {
			_ = os.Setenv(key, orig)
		} else {
			_ = os.Unsetenv(key)
		}
	})

	_ = os.Unsetenv(key)

	if err := ExportFlagsToEnv(); err != nil {
		t.Fatalf("first ExportFlagsToEnv: %v", err)
	}

	// First call had no flags, so nothing was set.
	if _, ok := os.LookupEnv(key); ok {
		t.Fatalf("did not expect %s to be set after first call", key)
	}

	// Now add a flag and call again: the Once guard means it stays unset.
	writeFlagFile(t, "SCOUT_ONCEFLAG_ENABLED", "")

	if err := ExportFlagsToEnv(); err != nil {
		t.Fatalf("second ExportFlagsToEnv: %v", err)
	}

	if _, ok := os.LookupEnv(key); ok {
		t.Errorf("second ExportFlagsToEnv should be a no-op (sync.Once), but %s was set", key)
	}
}

func TestIgnoreCommand(t *testing.T) {
	// IgnoreCommand mutates the package-level ignoredCommands map; restore it.
	original := make(map[string]bool, len(ignoredCommands))
	for k, v := range ignoredCommands {
		original[k] = v
	}

	t.Cleanup(func() {
		ignoredCommands = original
	})

	if ShouldIgnoreCommand("Custom") {
		t.Fatal("Custom should not be ignored before registration")
	}

	// Mixed case input must be folded to lower-case on store.
	if err := IgnoreCommand("CUSTOM"); err != nil {
		t.Fatalf("IgnoreCommand: %v", err)
	}

	if !ShouldIgnoreCommand("custom") {
		t.Error("lower-case lookup should match registered command")
	}

	if !ShouldIgnoreCommand("Custom") {
		t.Error("mixed-case lookup should match (case-folded)")
	}

	if !ShouldIgnoreCommand("CUSTOM") {
		t.Error("upper-case lookup should match")
	}
}

func TestGetFlags_PrefersCacheOverDisk(t *testing.T) {
	isolateAppDir(t)

	// Seed the cache directly; getFlags must return it without touching disk.
	cacheMu.Lock()
	flagCache = map[string]bool{"FROMCACHE": true}
	cacheMu.Unlock()

	if !IsFeatureEnabled("fromcache") {
		t.Fatal("expected getFlags to serve FROMCACHE from cache")
	}

	// Ensure a stray disk file is NOT consulted while cache is warm.
	writeFlagFile(t, "SCOUT_DISKONLY_ENABLED", "")

	if IsFeatureEnabled("diskonly") {
		t.Error("warm cache should shadow disk; DISKONLY must not resolve")
	}
}

func TestInvalidateCache(t *testing.T) {
	isolateAppDir(t)

	cacheMu.Lock()
	flagCache = map[string]bool{"X": true}
	cacheMu.Unlock()

	if GetCachedFlags() == nil {
		t.Fatal("precondition: cache should be populated")
	}

	invalidateCache()

	if GetCachedFlags() != nil {
		t.Fatal("invalidateCache should clear the cache")
	}
}
