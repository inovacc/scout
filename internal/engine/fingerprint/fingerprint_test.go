package fingerprint

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// GenerateFingerprint
// ---------------------------------------------------------------------------

func TestGenerateFingerprint_Default(t *testing.T) {
	fp := GenerateFingerprint()
	if fp == nil {
		t.Fatal("expected non-nil fingerprint")
	}
	if fp.UserAgent == "" {
		t.Error("expected non-empty UserAgent")
	}
	if fp.Platform == "" {
		t.Error("expected non-empty Platform")
	}
	if fp.Vendor == "" {
		t.Error("expected non-empty Vendor")
	}
	if len(fp.Languages) == 0 {
		t.Error("expected non-empty Languages")
	}
	if fp.Timezone == "" {
		t.Error("expected non-empty Timezone")
	}
	if fp.ScreenWidth == 0 || fp.ScreenHeight == 0 {
		t.Error("expected non-zero screen dimensions")
	}
	if fp.ColorDepth == 0 {
		t.Error("expected non-zero ColorDepth")
	}
	if fp.PixelRatio == 0 {
		t.Error("expected non-zero PixelRatio")
	}
	if fp.WebGLVendor == "" {
		t.Error("expected non-empty WebGLVendor")
	}
	if fp.WebGLRenderer == "" {
		t.Error("expected non-empty WebGLRenderer")
	}
	if fp.HardwareConcurrency == 0 {
		t.Error("expected non-zero HardwareConcurrency")
	}
	if fp.DeviceMemory == 0 {
		t.Error("expected non-zero DeviceMemory")
	}
}

func TestGenerateFingerprint_OSOptions(t *testing.T) {
	tests := []struct {
		name     string
		os       string
		platform string
		uaHint   string // substring expected in UserAgent
	}{
		{"windows", "windows", "Win32", "Windows NT"},
		{"mac", "mac", "MacIntel", "Macintosh"},
		{"linux", "linux", "Linux x86_64", "X11; Linux"},
		{"windows_uppercase", "WINDOWS", "Win32", "Windows NT"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fp := GenerateFingerprint(WithFingerprintOS(tt.os))
			if fp.Platform != tt.platform {
				t.Errorf("platform = %q, want %q", fp.Platform, tt.platform)
			}
			if !strings.Contains(fp.UserAgent, tt.uaHint) {
				t.Errorf("UserAgent %q missing hint %q", fp.UserAgent, tt.uaHint)
			}
			if fp.Vendor != "Google Inc." {
				t.Errorf("vendor = %q, want %q", fp.Vendor, "Google Inc.")
			}
		})
	}
}

func TestGenerateFingerprint_Mobile(t *testing.T) {
	fp := GenerateFingerprint(WithFingerprintMobile(true))

	if fp.MaxTouchPoints != 5 {
		t.Errorf("MaxTouchPoints = %d, want 5", fp.MaxTouchPoints)
	}
	if fp.DoNotTrack != "" {
		t.Errorf("DoNotTrack = %q, want empty for mobile", fp.DoNotTrack)
	}
	if fp.ColorDepth != 24 {
		t.Errorf("ColorDepth = %d, want 24 for mobile", fp.ColorDepth)
	}

	// Platform should be either iPhone or Linux armv81
	if fp.Platform != "iPhone" && fp.Platform != "Linux armv81" {
		t.Errorf("unexpected mobile platform: %q", fp.Platform)
	}

	if strings.Contains(fp.UserAgent, "iPhone") {
		if fp.Vendor != "Apple Computer, Inc." {
			t.Errorf("iPhone vendor = %q, want %q", fp.Vendor, "Apple Computer, Inc.")
		}
	} else {
		if fp.Vendor != "Google Inc." {
			t.Errorf("Android vendor = %q, want %q", fp.Vendor, "Google Inc.")
		}
	}
}

func TestGenerateFingerprint_Desktop_MaxTouchPoints(t *testing.T) {
	fp := GenerateFingerprint(WithFingerprintMobile(false))
	if fp.MaxTouchPoints != 0 {
		t.Errorf("desktop MaxTouchPoints = %d, want 0", fp.MaxTouchPoints)
	}
}

func TestGenerateFingerprint_Locale(t *testing.T) {
	tests := []struct {
		name     string
		locale   string
		wantTZ   string
		wantLang string
	}{
		{"pt-BR", "pt-BR", "America/Sao_Paulo", "pt-BR"},
		{"ja-JP", "ja-JP", "Asia/Tokyo", "ja-JP"},
		{"en-US", "en-US", "America/New_York", "en-US"},
		{"de-DE", "de-DE", "Europe/Berlin", "de-DE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fp := GenerateFingerprint(WithFingerprintLocale(tt.locale))
			if fp.Timezone != tt.wantTZ {
				t.Errorf("Timezone = %q, want %q", fp.Timezone, tt.wantTZ)
			}
			if len(fp.Languages) == 0 || fp.Languages[0] != tt.wantLang {
				t.Errorf("Languages[0] = %v, want %q", fp.Languages, tt.wantLang)
			}
		})
	}
}

func TestGenerateFingerprint_UnknownLocale(t *testing.T) {
	fp := GenerateFingerprint(WithFingerprintLocale("xx-YY"))
	if fp.Timezone != "America/New_York" {
		t.Errorf("unknown locale Timezone = %q, want America/New_York", fp.Timezone)
	}
	if len(fp.Languages) < 2 || fp.Languages[0] != "xx-YY" || fp.Languages[1] != "en" {
		t.Errorf("unknown locale Languages = %v, want [xx-YY en]", fp.Languages)
	}
}

func TestGenerateFingerprint_Randomness(t *testing.T) {
	// Generate several and verify we get at least some variation
	seen := make(map[string]bool)
	for i := 0; i < 20; i++ {
		fp := GenerateFingerprint()
		seen[fp.UserAgent] = true
	}
	if len(seen) < 2 {
		t.Error("expected at least 2 different UserAgents in 20 generations")
	}
}

// ---------------------------------------------------------------------------
// Fingerprint.JSON
// ---------------------------------------------------------------------------

func TestFingerprint_JSON(t *testing.T) {
	fp := GenerateFingerprint(WithFingerprintOS("windows"))
	js, err := fp.JSON()
	if err != nil {
		t.Fatalf("JSON() error: %v", err)
	}

	var decoded Fingerprint
	if err := json.Unmarshal([]byte(js), &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if decoded.UserAgent != fp.UserAgent {
		t.Errorf("round-trip UserAgent mismatch: got %q, want %q", decoded.UserAgent, fp.UserAgent)
	}
	if decoded.Platform != fp.Platform {
		t.Error("round-trip Platform mismatch")
	}
	if decoded.ScreenWidth != fp.ScreenWidth {
		t.Error("round-trip ScreenWidth mismatch")
	}
}

// ---------------------------------------------------------------------------
// Fingerprint.ToJS
// ---------------------------------------------------------------------------

func TestFingerprint_ToJS(t *testing.T) {
	fp := GenerateFingerprint(WithFingerprintOS("mac"), WithFingerprintLocale("en-US"))
	js := fp.ToJS()

	if !strings.Contains(js, fp.UserAgent) {
		t.Error("ToJS missing UserAgent")
	}
	if !strings.Contains(js, fp.Platform) {
		t.Error("ToJS missing Platform")
	}
	if !strings.Contains(js, fp.Timezone) {
		t.Error("ToJS missing Timezone")
	}
	if !strings.Contains(js, fp.WebGLVendor) {
		t.Error("ToJS missing WebGLVendor")
	}
	if !strings.Contains(js, fp.WebGLRenderer) {
		t.Error("ToJS missing WebGLRenderer")
	}
	if !strings.Contains(js, "navigator") {
		t.Error("ToJS missing navigator override block")
	}
	if !strings.Contains(js, "devicePixelRatio") {
		t.Error("ToJS missing devicePixelRatio override")
	}
}

func TestFingerprint_ToJS_DNT(t *testing.T) {
	fp := &Fingerprint{
		UserAgent:           "test-ua",
		Platform:            "Win32",
		Vendor:              "Google Inc.",
		Languages:           []string{"en-US"},
		Timezone:            "America/New_York",
		ScreenWidth:         1920,
		ScreenHeight:        1080,
		ColorDepth:          24,
		PixelRatio:          1.0,
		WebGLVendor:         "test-vendor",
		WebGLRenderer:       "test-renderer",
		HardwareConcurrency: 8,
		DeviceMemory:        16,
		DoNotTrack:          "1",
	}
	js := fp.ToJS()
	if !strings.Contains(js, `"1"`) {
		t.Error("ToJS should contain quoted '1' for DoNotTrack")
	}

	fp.DoNotTrack = ""
	js = fp.ToJS()
	if !strings.Contains(js, "null") {
		t.Error("ToJS should contain null for empty DoNotTrack")
	}
}

// ---------------------------------------------------------------------------
// Helper functions
// ---------------------------------------------------------------------------

func TestQuotedOrNull(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", "null"},
		{"1", `"1"`},
		{"hello", `"hello"`},
	}
	for _, tt := range tests {
		got := quotedOrNull(tt.input)
		if got != tt.want {
			t.Errorf("quotedOrNull(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFirstOrEmpty(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  string
	}{
		{"nil", nil, ""},
		{"empty", []string{}, ""},
		{"single", []string{"a"}, "a"},
		{"multi", []string{"a", "b"}, "a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := firstOrEmpty(tt.input)
			if got != tt.want {
				t.Errorf("firstOrEmpty(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// DomainFromURL
// ---------------------------------------------------------------------------

func TestDomainFromURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{"empty", "", ""},
		{"full_url", "https://example.com/path?q=1", "example.com"},
		{"with_port", "https://example.com:8080/path", "example.com"},
		{"http", "http://sub.example.org", "sub.example.org"},
		{"no_scheme", "://bad", ""},
		{"bare_host", "https://localhost", "localhost"},
		{"ip_address", "http://192.168.1.1:3000/api", "192.168.1.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DomainFromURL(tt.url)
			if got != tt.want {
				t.Errorf("DomainFromURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// FingerprintStore
// ---------------------------------------------------------------------------

func newTestStore(t *testing.T) *FingerprintStore {
	t.Helper()
	dir := t.TempDir()
	store, err := NewFingerprintStore(dir)
	if err != nil {
		t.Fatalf("NewFingerprintStore: %v", err)
	}
	return store
}

func TestFingerprintStore_SaveAndLoad(t *testing.T) {
	store := newTestStore(t)
	fp := GenerateFingerprint(WithFingerprintOS("linux"))

	saved, err := store.Save(fp, "test-tag", "another")
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if saved.ID == "" {
		t.Error("expected non-empty ID")
	}
	if saved.Fingerprint != fp {
		t.Error("saved fingerprint pointer mismatch")
	}
	if saved.UseCount != 0 {
		t.Errorf("UseCount = %d, want 0", saved.UseCount)
	}
	if len(saved.Tags) != 2 || saved.Tags[0] != "test-tag" {
		t.Errorf("Tags = %v, want [test-tag another]", saved.Tags)
	}
	if saved.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}

	loaded, err := store.Load(saved.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Fingerprint.UserAgent != fp.UserAgent {
		t.Error("loaded UserAgent mismatch")
	}
	if loaded.Fingerprint.Platform != "Linux x86_64" {
		t.Errorf("loaded Platform = %q, want Linux x86_64", loaded.Fingerprint.Platform)
	}
}

func TestFingerprintStore_Load_NotFound(t *testing.T) {
	store := newTestStore(t)
	_, err := store.Load("nonexistent-id")
	if err == nil {
		t.Error("expected error loading nonexistent fingerprint")
	}
}

func TestFingerprintStore_Delete(t *testing.T) {
	store := newTestStore(t)
	fp := GenerateFingerprint()

	saved, err := store.Save(fp)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := store.Delete(saved.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = store.Load(saved.ID)
	if err == nil {
		t.Error("expected error loading deleted fingerprint")
	}
}

func TestFingerprintStore_Delete_NotFound(t *testing.T) {
	store := newTestStore(t)
	err := store.Delete("does-not-exist")
	if err == nil {
		t.Error("expected error deleting nonexistent fingerprint")
	}
}

func TestFingerprintStore_List(t *testing.T) {
	store := newTestStore(t)

	// Empty list.
	list, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("empty store List len = %d, want 0", len(list))
	}

	// Add three fingerprints.
	for i := 0; i < 3; i++ {
		_, err := store.Save(GenerateFingerprint())
		if err != nil {
			t.Fatalf("Save[%d]: %v", i, err)
		}
		// Small sleep to ensure distinct CreatedAt ordering.
		time.Sleep(10 * time.Millisecond)
	}

	list, err = store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("List len = %d, want 3", len(list))
	}

	// Should be sorted newest first.
	for i := 1; i < len(list); i++ {
		if list[i].CreatedAt.After(list[i-1].CreatedAt) {
			t.Error("List not sorted newest first")
		}
	}
}

func TestFingerprintStore_List_SkipsNonJSON(t *testing.T) {
	store := newTestStore(t)

	// Save a valid fingerprint.
	_, err := store.Save(GenerateFingerprint())
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Write a non-JSON file and a subdirectory.
	_ = os.WriteFile(filepath.Join(store.dir, "readme.txt"), []byte("not json"), 0600)
	_ = os.Mkdir(filepath.Join(store.dir, "subdir"), 0700)
	// Write an invalid JSON file.
	_ = os.WriteFile(filepath.Join(store.dir, "bad.json"), []byte("{invalid"), 0600)

	list, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("List len = %d, want 1 (should skip non-JSON and invalid)", len(list))
	}
}

func TestFingerprintStore_MarkUsed(t *testing.T) {
	store := newTestStore(t)
	fp := GenerateFingerprint()

	saved, err := store.Save(fp)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Mark used with a domain.
	if err := store.MarkUsed(saved.ID, "example.com"); err != nil {
		t.Fatalf("MarkUsed: %v", err)
	}

	loaded, err := store.Load(saved.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.UseCount != 1 {
		t.Errorf("UseCount = %d, want 1", loaded.UseCount)
	}
	if len(loaded.Domains) != 1 || loaded.Domains[0] != "example.com" {
		t.Errorf("Domains = %v, want [example.com]", loaded.Domains)
	}

	// Mark used again with same domain — should not duplicate.
	if err := store.MarkUsed(saved.ID, "example.com"); err != nil {
		t.Fatalf("MarkUsed 2: %v", err)
	}
	loaded, _ = store.Load(saved.ID)
	if loaded.UseCount != 2 {
		t.Errorf("UseCount = %d, want 2", loaded.UseCount)
	}
	if len(loaded.Domains) != 1 {
		t.Errorf("Domains should still be 1, got %d", len(loaded.Domains))
	}

	// Mark used with a different domain.
	if err := store.MarkUsed(saved.ID, "other.com"); err != nil {
		t.Fatalf("MarkUsed 3: %v", err)
	}
	loaded, _ = store.Load(saved.ID)
	if len(loaded.Domains) != 2 {
		t.Errorf("Domains len = %d, want 2", len(loaded.Domains))
	}

	// Mark used with empty domain — should not add to domains.
	if err := store.MarkUsed(saved.ID, ""); err != nil {
		t.Fatalf("MarkUsed empty: %v", err)
	}
	loaded, _ = store.Load(saved.ID)
	if len(loaded.Domains) != 2 {
		t.Errorf("Domains len = %d after empty domain, want 2", len(loaded.Domains))
	}
}

func TestFingerprintStore_MarkUsed_NotFound(t *testing.T) {
	store := newTestStore(t)
	err := store.MarkUsed("nonexistent", "example.com")
	if err == nil {
		t.Error("expected error marking nonexistent fingerprint")
	}
}

func TestFingerprintStore_Generate(t *testing.T) {
	store := newTestStore(t)

	saved, err := store.Generate(WithFingerprintOS("mac"))
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if saved.ID == "" {
		t.Error("expected non-empty ID")
	}
	if saved.Fingerprint.Platform != "MacIntel" {
		t.Errorf("Platform = %q, want MacIntel", saved.Fingerprint.Platform)
	}

	// Should be loadable.
	loaded, err := store.Load(saved.ID)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Fingerprint.Platform != "MacIntel" {
		t.Errorf("loaded Platform = %q, want MacIntel", loaded.Fingerprint.Platform)
	}
}

func TestNewFingerprintStore_CreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "deep", "fingerprints")
	store, err := NewFingerprintStore(dir)
	if err != nil {
		t.Fatalf("NewFingerprintStore: %v", err)
	}

	info, err := os.Stat(store.dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected directory")
	}
}

// ---------------------------------------------------------------------------
// Rotator
// ---------------------------------------------------------------------------

func TestRotator_PerSession(t *testing.T) {
	r := NewRotator(FingerprintRotationConfig{
		Strategy: FingerprintRotatePerSession,
	})

	fp1 := r.ForPage("example.com")
	fp2 := r.ForPage("other.com")
	fp3 := r.ForPage("example.com")

	if fp1 != fp2 || fp2 != fp3 {
		t.Error("PerSession should return the same fingerprint for all pages")
	}
}

func TestRotator_PerPage(t *testing.T) {
	r := NewRotator(FingerprintRotationConfig{
		Strategy: FingerprintRotatePerPage,
	})

	fp1 := r.ForPage("example.com")
	fp2 := r.ForPage("example.com")

	// They should be different objects (freshly generated each time).
	if fp1 == fp2 {
		t.Error("PerPage should return different fingerprint pointers for each call")
	}
}

func TestRotator_PerDomain(t *testing.T) {
	r := NewRotator(FingerprintRotationConfig{
		Strategy: FingerprintRotatePerDomain,
	})

	fpA1 := r.ForPage("example.com")
	fpA2 := r.ForPage("example.com")
	fpB := r.ForPage("other.com")

	if fpA1 != fpA2 {
		t.Error("PerDomain should return same fingerprint for same domain")
	}
	if fpA1 == fpB {
		t.Error("PerDomain should return different fingerprints for different domains")
	}
}

func TestRotator_Interval(t *testing.T) {
	r := NewRotator(FingerprintRotationConfig{
		Strategy: FingerprintRotateInterval,
		Interval: 50 * time.Millisecond,
	})

	fp1 := r.ForPage("example.com")
	fp2 := r.ForPage("example.com")
	if fp1 != fp2 {
		t.Error("should return same fingerprint before interval elapses")
	}

	time.Sleep(60 * time.Millisecond)
	fp3 := r.ForPage("example.com")
	if fp1 == fp3 {
		t.Error("should return different fingerprint after interval elapses")
	}
}

func TestRotator_Pool(t *testing.T) {
	pool := []*Fingerprint{
		{UserAgent: "ua-0"},
		{UserAgent: "ua-1"},
		{UserAgent: "ua-2"},
	}

	r := NewRotator(FingerprintRotationConfig{
		Strategy: FingerprintRotatePerPage,
		Pool:     pool,
	})

	// Initial fingerprint consumed pool[0].
	// ForPage PerPage generates a new one each time, consuming pool[1], pool[2], then wrapping.
	got1 := r.ForPage("a")
	got2 := r.ForPage("b")
	got3 := r.ForPage("c")

	if got1.UserAgent != "ua-1" {
		t.Errorf("pool[1] expected ua-1, got %q", got1.UserAgent)
	}
	if got2.UserAgent != "ua-2" {
		t.Errorf("pool[2] expected ua-2, got %q", got2.UserAgent)
	}
	// Wraps around to pool[0].
	if got3.UserAgent != "ua-0" {
		t.Errorf("pool wrap expected ua-0, got %q", got3.UserAgent)
	}
}

func TestRotator_NilSafe(t *testing.T) {
	var r *Rotator
	fp := r.ForPage("example.com")
	if fp != nil {
		t.Error("nil Rotator.ForPage should return nil")
	}
}

// ---------------------------------------------------------------------------
// FingerprintRotation constants
// ---------------------------------------------------------------------------

func TestFingerprintRotation_Values(t *testing.T) {
	tests := []struct {
		name string
		val  FingerprintRotation
		want int
	}{
		{"PerSession", FingerprintRotatePerSession, 0},
		{"PerPage", FingerprintRotatePerPage, 1},
		{"PerDomain", FingerprintRotatePerDomain, 2},
		{"Interval", FingerprintRotateInterval, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if int(tt.val) != tt.want {
				t.Errorf("%s = %d, want %d", tt.name, tt.val, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Data pool sanity checks
// ---------------------------------------------------------------------------

func TestDataPools_NonEmpty(t *testing.T) {
	pools := []struct {
		name string
		size int
	}{
		{"userAgentsWindows", len(userAgentsWindows)},
		{"userAgentsMac", len(userAgentsMac)},
		{"userAgentsLinux", len(userAgentsLinux)},
		{"userAgentsMobile", len(userAgentsMobile)},
		{"screenResolutionsWindows", len(screenResolutionsWindows)},
		{"screenResolutionsMac", len(screenResolutionsMac)},
		{"screenResolutionsLinux", len(screenResolutionsLinux)},
		{"screenResolutionsMobile", len(screenResolutionsMobile)},
		{"webglProfilesWindows", len(webglProfilesWindows)},
		{"webglProfilesMac", len(webglProfilesMac)},
		{"webglProfilesLinux", len(webglProfilesLinux)},
		{"webglProfilesMobile", len(webglProfilesMobile)},
		{"timezoneLocales", len(timezoneLocales)},
		{"hardwareConcurrencies", len(hardwareConcurrencies)},
		{"deviceMemories", len(deviceMemories)},
		{"colorDepths", len(colorDepths)},
		{"pixelRatiosDesktop", len(pixelRatiosDesktop)},
		{"pixelRatiosMobile", len(pixelRatiosMobile)},
	}

	for _, p := range pools {
		t.Run(p.name, func(t *testing.T) {
			if p.size == 0 {
				t.Errorf("%s is empty", p.name)
			}
		})
	}
}

func TestTimezoneLocales_HaveRequiredFields(t *testing.T) {
	for _, tz := range timezoneLocales {
		if tz.Timezone == "" {
			t.Errorf("timezoneLocale with empty Timezone: %+v", tz)
		}
		if tz.Locale == "" {
			t.Errorf("timezoneLocale with empty Locale: %+v", tz)
		}
		if len(tz.Langs) == 0 {
			t.Errorf("timezoneLocale with empty Langs: %+v", tz)
		}
	}
}

func TestScreenResolutions_Positive(t *testing.T) {
	allRes := make([]screenResolution, 0)
	allRes = append(allRes, screenResolutionsWindows...)
	allRes = append(allRes, screenResolutionsMac...)
	allRes = append(allRes, screenResolutionsLinux...)
	allRes = append(allRes, screenResolutionsMobile...)

	for _, r := range allRes {
		if r.Width <= 0 || r.Height <= 0 {
			t.Errorf("invalid resolution: %dx%d", r.Width, r.Height)
		}
	}
}

// ---------------------------------------------------------------------------
// containsString
// ---------------------------------------------------------------------------

func TestContainsString(t *testing.T) {
	tests := []struct {
		name string
		ss   []string
		s    string
		want bool
	}{
		{"found", []string{"a", "b", "c"}, "b", true},
		{"not_found", []string{"a", "b", "c"}, "d", false},
		{"empty_slice", []string{}, "a", false},
		{"nil_slice", nil, "a", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsString(tt.ss, tt.s)
			if got != tt.want {
				t.Errorf("containsString(%v, %q) = %v, want %v", tt.ss, tt.s, got, tt.want)
			}
		})
	}
}
