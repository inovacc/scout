package browser

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	enginebrowser "github.com/inovacc/scout/internal/engine/browser"
)

// TestChromiumDownloadURLs verifies that both the primary Google snapshot URL
// and the npmmirror fallback URL are produced, in order, with the revision and
// host-config values interpolated correctly.
func TestChromiumDownloadURLs(t *testing.T) {
	tests := []struct {
		name      string
		revision  int
		urlPrefix string
		zipName   string
		want      []string
	}{
		{
			name:      "windows x64",
			revision:  1592198,
			urlPrefix: "Win_x64",
			zipName:   "chrome-win.zip",
			want: []string{
				"https://storage.googleapis.com/chromium-browser-snapshots/Win_x64/1592198/chrome-win.zip",
				"https://registry.npmmirror.com/-/binary/chromium-browser-snapshots/Win_x64/1592198/chrome-win.zip",
			},
		},
		{
			name:      "mac arm",
			revision:  100,
			urlPrefix: "Mac_Arm",
			zipName:   "chrome-mac.zip",
			want: []string{
				"https://storage.googleapis.com/chromium-browser-snapshots/Mac_Arm/100/chrome-mac.zip",
				"https://registry.npmmirror.com/-/binary/chromium-browser-snapshots/Mac_Arm/100/chrome-mac.zip",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conf := struct{ urlPrefix, zipName string }{tt.urlPrefix, tt.zipName}

			got := chromiumDownloadURLs(tt.revision, conf)
			if len(got) != len(tt.want) {
				t.Fatalf("chromiumDownloadURLs returned %d urls, want %d", len(got), len(tt.want))
			}

			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("url[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestChromiumDownloadURLs_PrimaryFirst guards the contract that the Google
// snapshot mirror is always attempted before the npmmirror fallback.
func TestChromiumDownloadURLs_PrimaryFirst(t *testing.T) {
	conf := struct{ urlPrefix, zipName string }{"Linux_x64", "chrome-linux.zip"}

	got := chromiumDownloadURLs(1, conf)
	if len(got) < 2 {
		t.Fatalf("expected at least 2 urls, got %d", len(got))
	}

	if !strings.Contains(got[0], "storage.googleapis.com") {
		t.Errorf("primary url should target storage.googleapis.com, got %q", got[0])
	}

	if !strings.Contains(got[1], "npmmirror.com") {
		t.Errorf("fallback url should target npmmirror.com, got %q", got[1])
	}
}

// TestChromiumBinPath asserts that the relative binary path matches the
// running platform's expected layout. The function is keyed on runtime.GOOS,
// so the test adapts its expectation accordingly.
func TestChromiumBinPath(t *testing.T) {
	got := chromiumBinPath()

	want := map[string]string{
		"darwin":  filepath.Join("Chromium.app", "Contents", "MacOS", "Chromium"),
		"linux":   "chrome",
		"windows": "chrome.exe",
	}[runtime.GOOS]

	if want == "" {
		// Unmapped platform: function returns "" from the map lookup.
		if got != "" {
			t.Errorf("chromiumBinPath() = %q on unmapped GOOS %q, want empty", got, runtime.GOOS)
		}

		return
	}

	if got != want {
		t.Errorf("chromiumBinPath() = %q, want %q", got, want)
	}
}

// TestBraveAssetName checks the GitHub release asset filename for every
// supported platform key, plus the unsupported-platform path that yields "".
func TestBraveAssetName(t *testing.T) {
	got := braveAssetName("1.62.156")

	key := runtime.GOOS + "_" + runtime.GOARCH

	pattern, ok := braveAssets[key]
	if !ok {
		// Unsupported platform must return empty.
		if got != "" {
			t.Errorf("braveAssetName on unsupported %q = %q, want empty", key, got)
		}

		return
	}

	want := strings.Replace(pattern, "%s", "1.62.156", 1)
	if got != want {
		t.Errorf("braveAssetName(1.62.156) = %q, want %q", got, want)
	}

	// The version must be embedded in the produced asset name.
	if !strings.Contains(got, "1.62.156") {
		t.Errorf("asset name %q does not embed version", got)
	}
}

// TestBraveAssetMapCoverage validates the static asset/version patterns so a
// future edit that drops a platform or breaks the %s placeholder is caught.
func TestBraveAssetMapCoverage(t *testing.T) {
	wantKeys := []string{
		"windows_amd64", "windows_arm64",
		"darwin_amd64", "darwin_arm64",
		"linux_amd64", "linux_arm64",
	}

	for _, k := range wantKeys {
		pattern, ok := braveAssets[k]
		if !ok {
			t.Errorf("braveAssets missing key %q", k)

			continue
		}

		if !strings.Contains(pattern, "%s") {
			t.Errorf("braveAssets[%q] = %q missing %%s version placeholder", k, pattern)
		}

		if !strings.HasSuffix(pattern, ".zip") {
			t.Errorf("braveAssets[%q] = %q should be a .zip asset", k, pattern)
		}
	}
}

// TestBraveBinPath confirms the per-GOOS executable path within the extracted
// archive, including the documented "brave" fallback for unmapped platforms.
func TestBraveBinPath(t *testing.T) {
	got := braveBinPath()

	want, ok := braveBins[runtime.GOOS]
	if !ok {
		want = "brave" // documented fallback
	}

	if got != want {
		t.Errorf("braveBinPath() = %q, want %q", got, want)
	}

	// Spot-check the static map values directly.
	cases := map[string]string{
		"windows": "brave.exe",
		"darwin":  "Brave Browser.app/Contents/MacOS/Brave Browser",
		"linux":   "brave",
	}

	for goos, exp := range cases {
		if braveBins[goos] != exp {
			t.Errorf("braveBins[%q] = %q, want %q", goos, braveBins[goos], exp)
		}
	}
}

// TestDownloadEdge verifies the stub returns ErrNotFound wrapped with a
// human-readable manual-download hint, regardless of the cache dir argument.
func TestDownloadEdge(t *testing.T) {
	path, err := DownloadEdge("/any/cache/dir")
	if err == nil {
		t.Fatal("DownloadEdge should always return an error (no programmatic API)")
	}

	if path != "" {
		t.Errorf("DownloadEdge path = %q, want empty", path)
	}

	if !errors.Is(err, ErrNotFound) {
		t.Errorf("DownloadEdge error should wrap ErrNotFound, got: %v", err)
	}

	if !strings.Contains(err.Error(), "microsoft.com/edge") {
		t.Errorf("DownloadEdge error should mention the manual download URL, got: %v", err)
	}
}

// TestPatch confirms the reserved no-op stub never errors for any input.
func TestPatch(t *testing.T) {
	inputs := []string{"", "/usr/bin/chrome", "C:/Program Files/chrome.exe"}

	for _, in := range inputs {
		if err := Patch(in); err != nil {
			t.Errorf("Patch(%q) = %v, want nil", in, err)
		}
	}
}

// TestStripFirstDir_SingleTopLevelDir covers the common case: a single
// top-level directory whose children get promoted up one level and the now
// empty inner directory removed.
func TestStripFirstDir_SingleTopLevelDir(t *testing.T) {
	root := t.TempDir()

	inner := filepath.Join(root, "chrome-win")
	if err := os.MkdirAll(filepath.Join(inner, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(inner, "chrome.exe"), []byte("bin"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(inner, "sub", "lib.dll"), []byte("lib"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := stripFirstDir(root); err != nil {
		t.Fatalf("stripFirstDir error: %v", err)
	}

	// chrome.exe should now live directly under root.
	if _, err := os.Stat(filepath.Join(root, "chrome.exe")); err != nil {
		t.Errorf("expected chrome.exe promoted to root: %v", err)
	}

	// The nested "sub" dir should have moved up too.
	if _, err := os.Stat(filepath.Join(root, "sub", "lib.dll")); err != nil {
		t.Errorf("expected sub/lib.dll promoted to root: %v", err)
	}

	// The original inner wrapper dir must be gone.
	if _, err := os.Stat(inner); !os.IsNotExist(err) {
		t.Errorf("inner wrapper dir should be removed, stat err = %v", err)
	}
}

// TestStripFirstDir_NoOpCases covers the branches where stripping must NOT
// happen: zero entries, multiple entries, or a single non-directory entry.
func TestStripFirstDir_NoOpCases(t *testing.T) {
	t.Run("empty dir", func(t *testing.T) {
		root := t.TempDir()

		if err := stripFirstDir(root); err != nil {
			t.Fatalf("stripFirstDir on empty dir: %v", err)
		}

		entries, _ := os.ReadDir(root)
		if len(entries) != 0 {
			t.Errorf("empty dir should stay empty, got %d entries", len(entries))
		}
	})

	t.Run("multiple top-level entries", func(t *testing.T) {
		root := t.TempDir()

		if err := os.MkdirAll(filepath.Join(root, "a"), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.MkdirAll(filepath.Join(root, "b"), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := stripFirstDir(root); err != nil {
			t.Fatalf("stripFirstDir: %v", err)
		}

		// Both dirs must remain untouched.
		if _, err := os.Stat(filepath.Join(root, "a")); err != nil {
			t.Errorf("dir a should remain: %v", err)
		}

		if _, err := os.Stat(filepath.Join(root, "b")); err != nil {
			t.Errorf("dir b should remain: %v", err)
		}
	})

	t.Run("single file not dir", func(t *testing.T) {
		root := t.TempDir()

		file := filepath.Join(root, "only.txt")
		if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}

		if err := stripFirstDir(root); err != nil {
			t.Fatalf("stripFirstDir: %v", err)
		}

		if _, err := os.Stat(file); err != nil {
			t.Errorf("single file should remain untouched: %v", err)
		}
	})
}

// TestStripFirstDir_MissingDir verifies the error path when the target dir
// cannot be read (does not exist).
func TestStripFirstDir_MissingDir(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")

	if err := stripFirstDir(missing); err == nil {
		t.Fatal("stripFirstDir on missing dir should return an error")
	}
}

// TestEngineTypeToPkg maps every engine BrowserType onto this package's string
// type, asserting the Chromium/Electron -> Chrome folding behavior and the
// default fallthrough for unknown types.
func TestEngineTypeToPkg(t *testing.T) {
	tests := []struct {
		name string
		in   enginebrowser.BrowserType
		want string
	}{
		{"chrome", enginebrowser.Chrome, TypeChrome},
		{"chromium folds to chrome", enginebrowser.Chromium, TypeChrome},
		{"electron folds to chrome", enginebrowser.Electron, TypeChrome},
		{"brave", enginebrowser.Brave, TypeBrave},
		{"edge", enginebrowser.Edge, TypeEdge},
		{"unknown defaults to chrome", enginebrowser.BrowserType("firefox"), TypeChrome},
		{"empty defaults to chrome", enginebrowser.BrowserType(""), TypeChrome},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := engineTypeToPkg(tt.in); got != tt.want {
				t.Errorf("engineTypeToPkg(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestGuessCachedBinPath confirms the path-building for cached browsers: Brave
// composes the platform brave binary path under the entry dir, while every
// non-Brave type returns "" (the default branch).
func TestGuessCachedBinPath(t *testing.T) {
	cacheDir := filepath.Join("home", "user", ".scout", "browsers")

	t.Run("brave", func(t *testing.T) {
		got := guessCachedBinPath(cacheDir, "brave-1.62.156", TypeBrave)

		want := filepath.Join(cacheDir, "brave-1.62.156", braveBinPath())
		if got != want {
			t.Errorf("guessCachedBinPath brave = %q, want %q", got, want)
		}
	})

	t.Run("chrome returns empty", func(t *testing.T) {
		if got := guessCachedBinPath(cacheDir, "chrome-120", TypeChrome); got != "" {
			t.Errorf("guessCachedBinPath chrome = %q, want empty", got)
		}
	})

	t.Run("edge returns empty", func(t *testing.T) {
		if got := guessCachedBinPath(cacheDir, "edge-120", TypeEdge); got != "" {
			t.Errorf("guessCachedBinPath edge = %q, want empty", got)
		}
	})

	t.Run("unknown returns empty", func(t *testing.T) {
		if got := guessCachedBinPath(cacheDir, "whatever", "opera"); got != "" {
			t.Errorf("guessCachedBinPath unknown = %q, want empty", got)
		}
	})
}

// TestDownloadFile_BadURL exercises the request-construction error path of
// downloadFile without performing any real network I/O: a control character in
// the URL makes http.NewRequestWithContext fail before any dial occurs.
func TestDownloadFile_BadURL(t *testing.T) {
	_, err := downloadFile(context.Background(), "http://exa\x7fmple.com/\x00")
	if err == nil {
		t.Fatal("downloadFile with malformed URL should return an error")
	}

	if !strings.Contains(err.Error(), "scout: browser:") {
		t.Errorf("error should carry scout prefix, got: %v", err)
	}
}
