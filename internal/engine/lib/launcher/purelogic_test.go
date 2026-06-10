package launcher

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/inovacc/scout/internal/engine/lib/launcher/flags"
)

// --- pidAliveFromLockTarget ----------------------------------------------

func TestPidAliveFromLockTarget(t *testing.T) {
	self := strconv.Itoa(os.Getpid())

	// A PID we are confident is dead: a large value unlikely to be assigned.
	// Verify it really is dead via the platform processAlive before asserting.
	deadPID := 0x7FFFFFF0
	if processAlive(deadPID) {
		t.Skipf("chosen dead PID %d is unexpectedly alive on this host", deadPID)
	}

	tests := []struct {
		name string
		in   string
		want bool
	}{
		// Empty / unparseable content => assume alive (conservative, don't delete).
		{"empty", "", true},
		{"whitespace only", "   \n\t", true},
		{"no trailing digits", "hostname", true},
		{"zero pid treated as parse failure", "host\n0", true},

		// Live PID (this test process) in both observed formats => alive.
		{"linux newline format alive", "myhost\n" + self, true},
		{"macos dash format alive", "myhost-" + self, true},
		{"trailing whitespace trimmed alive", "myhost\n" + self + "  \n", true},

		// Dead PID => not alive.
		{"linux newline format dead", "myhost\n" + strconv.Itoa(deadPID), false},
		{"macos dash format dead", "myhost-" + strconv.Itoa(deadPID), false},
		{"bare dead pid", strconv.Itoa(deadPID), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pidAliveFromLockTarget(tt.in); got != tt.want {
				t.Fatalf("pidAliveFromLockTarget(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// --- clearStaleChromeLock -------------------------------------------------

func TestClearStaleChromeLockEmptyDir(t *testing.T) {
	// Empty dataDir is a no-op and must not panic.
	clearStaleChromeLock("")
}

func TestClearStaleChromeLockNoFiles(t *testing.T) {
	// Directory with none of the lock files present => no-op, no error.
	dir := t.TempDir()
	clearStaleChromeLock(dir)

	// Directory should remain untouched.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty dir, got %d entries", len(entries))
	}
}

func TestClearStaleChromeLockRemovesDeadLock(t *testing.T) {
	deadPID := 0x7FFFFFF0
	if processAlive(deadPID) {
		t.Skipf("chosen dead PID %d is unexpectedly alive on this host", deadPID)
	}

	dir := t.TempDir()
	lock := filepath.Join(dir, "lockfile")
	if err := os.WriteFile(lock, []byte("somehost\n"+strconv.Itoa(deadPID)), 0o600); err != nil {
		t.Fatal(err)
	}

	clearStaleChromeLock(dir)

	if _, err := os.Stat(lock); !os.IsNotExist(err) {
		t.Fatalf("expected stale lockfile to be removed, stat err = %v", err)
	}
}

func TestClearStaleChromeLockKeepsLiveLock(t *testing.T) {
	dir := t.TempDir()
	lock := filepath.Join(dir, "lockfile")
	// Point the lock at this live test process => must be kept.
	if err := os.WriteFile(lock, []byte("somehost\n"+strconv.Itoa(os.Getpid())), 0o600); err != nil {
		t.Fatal(err)
	}

	clearStaleChromeLock(dir)

	if _, err := os.Stat(lock); err != nil {
		t.Fatalf("expected live lockfile to be kept, stat err = %v", err)
	}
}

func TestClearStaleChromeLockKeepsUnparseableLock(t *testing.T) {
	dir := t.TempDir()
	lock := filepath.Join(dir, "SingletonCookie")
	// No trailing digits => pidAliveFromLockTarget returns true => keep.
	if err := os.WriteFile(lock, []byte("no-digits-here"), 0o600); err != nil {
		t.Fatal(err)
	}

	clearStaleChromeLock(dir)

	if _, err := os.Stat(lock); err != nil {
		t.Fatalf("expected unparseable lock to be kept, stat err = %v", err)
	}
}

// --- certSPKI / IgnoreCerts ----------------------------------------------

func TestCertSPKI(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	pin, err := certSPKI(key.Public())
	if err != nil {
		t.Fatalf("certSPKI returned error: %v", err)
	}

	// The pin is base64 of a SHA-256 digest: 32 raw bytes => 44 base64 chars.
	if len(pin) != base64.StdEncoding.EncodedLen(32) {
		t.Fatalf("unexpected pin length %d, want %d", len(pin), base64.StdEncoding.EncodedLen(32))
	}
	if _, derr := base64.StdEncoding.DecodeString(string(pin)); derr != nil {
		t.Fatalf("pin is not valid base64: %v", derr)
	}

	// Deterministic: same key => same pin.
	pin2, err := certSPKI(key.Public())
	if err != nil {
		t.Fatal(err)
	}
	if string(pin) != string(pin2) {
		t.Fatal("certSPKI is not deterministic for the same key")
	}

	// Different key => different pin.
	other, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pinOther, err := certSPKI(other.Public())
	if err != nil {
		t.Fatal(err)
	}
	if string(pin) == string(pinOther) {
		t.Fatal("certSPKI produced the same pin for different keys")
	}
}

func TestCertSPKIInvalidKey(t *testing.T) {
	// A non-public-key value cannot be marshaled by x509.MarshalPKIXPublicKey.
	if _, err := certSPKI("not a public key"); err == nil {
		t.Fatal("expected error for invalid public key")
	}
}

func TestIgnoreCerts(t *testing.T) {
	k1, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	k2, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	l := New()
	if err := l.IgnoreCerts([]crypto.PublicKey{k1.Public(), k2.Public()}); err != nil {
		t.Fatalf("IgnoreCerts returned error: %v", err)
	}

	vals, ok := l.GetFlags("ignore-certificate-errors-spki-list")
	if !ok {
		t.Fatal("expected ignore-certificate-errors-spki-list flag to be set")
	}
	if len(vals) != 2 {
		t.Fatalf("expected 2 spki entries, got %d", len(vals))
	}

	// Each entry must match the standalone certSPKI output.
	want1, _ := certSPKI(k1.Public())
	if vals[0] != string(want1) {
		t.Fatalf("first spki mismatch: got %q want %q", vals[0], want1)
	}
}

func TestIgnoreCertsEmpty(t *testing.T) {
	l := New()
	if err := l.IgnoreCerts(nil); err != nil {
		t.Fatalf("IgnoreCerts(nil) returned error: %v", err)
	}

	vals, ok := l.GetFlags("ignore-certificate-errors-spki-list")
	if !ok {
		t.Fatal("expected flag set even with empty list")
	}
	if len(vals) != 0 {
		t.Fatalf("expected 0 spki entries, got %d", len(vals))
	}
}

func TestIgnoreCertsInvalidKey(t *testing.T) {
	l := New()
	// A bogus key should propagate the certSPKI error.
	if err := l.IgnoreCerts([]crypto.PublicKey{"bogus"}); err == nil {
		t.Fatal("expected error from invalid public key")
	}
}

// --- stripFirstDir --------------------------------------------------------

func TestStripFirstDirPromotesSingleSubdir(t *testing.T) {
	root := t.TempDir()
	inner := filepath.Join(root, "chrome-win")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	// Files + a nested dir inside the single top-level dir.
	if err := os.WriteFile(filepath.Join(inner, "chrome.exe"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(inner, "locales"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := stripFirstDir(root); err != nil {
		t.Fatalf("stripFirstDir error: %v", err)
	}

	// chrome.exe and locales must now sit directly under root.
	if _, err := os.Stat(filepath.Join(root, "chrome.exe")); err != nil {
		t.Fatalf("expected chrome.exe promoted to root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "locales")); err != nil {
		t.Fatalf("expected locales promoted to root: %v", err)
	}
	// The original inner dir must be gone.
	if _, err := os.Stat(inner); !os.IsNotExist(err) {
		t.Fatalf("expected inner dir removed, stat err = %v", err)
	}
}

func TestStripFirstDirNoopMultipleEntries(t *testing.T) {
	root := t.TempDir()
	// Two top-level entries => no single dir to strip => no-op.
	if err := os.MkdirAll(filepath.Join(root, "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("y"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := stripFirstDir(root); err != nil {
		t.Fatalf("stripFirstDir error: %v", err)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected dir untouched (2 entries), got %d", len(entries))
	}
}

func TestStripFirstDirNoopSingleFile(t *testing.T) {
	root := t.TempDir()
	// Single top-level entry that is a file (not a dir) => no-op.
	if err := os.WriteFile(filepath.Join(root, "only.txt"), []byte("z"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := stripFirstDir(root); err != nil {
		t.Fatalf("stripFirstDir error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "only.txt")); err != nil {
		t.Fatalf("expected file untouched: %v", err)
	}
}

func TestStripFirstDirMissingDir(t *testing.T) {
	// Non-existent dir => ReadDir error propagated.
	if err := stripFirstDir(filepath.Join(t.TempDir(), "does-not-exist")); err == nil {
		t.Fatal("expected error for missing dir")
	}
}

// --- HostGoogle / HostNPM -------------------------------------------------

func TestHostGoogleAndNPM(t *testing.T) {
	rev := RevisionDefault

	g := HostGoogle(rev)
	if g == "" {
		t.Fatal("HostGoogle returned empty URL")
	}
	if !strings.HasPrefix(g, "http") {
		t.Fatalf("HostGoogle should be an http(s) URL, got %q", g)
	}
	if !strings.Contains(g, strconv.Itoa(rev)) {
		t.Fatalf("HostGoogle URL should embed the revision %d, got %q", rev, g)
	}

	n := HostNPM(rev)
	if n == "" {
		t.Fatal("HostNPM returned empty URL")
	}
	if !strings.Contains(n, strconv.Itoa(rev)) {
		t.Fatalf("HostNPM URL should embed the revision %d, got %q", rev, n)
	}

	// The two hosts must differ (different base mirrors).
	if g == n {
		t.Fatal("HostGoogle and HostNPM produced identical URLs")
	}

	// Revision is reflected: a different revision changes the URL.
	if HostGoogle(rev) == HostGoogle(rev+1) {
		t.Fatal("HostGoogle did not vary with revision")
	}
}

// --- Launcher flag setters: Bin / Revision / XVFB / Preferences / Logger --

func TestBinSetsFlag(t *testing.T) {
	l := New()
	ret := l.Bin("/opt/chrome/chrome")
	if ret != l {
		t.Fatal("Bin should return the launcher for chaining")
	}
	if got := l.Get(flags.Bin); got != "/opt/chrome/chrome" {
		t.Fatalf("expected bin path set, got %q", got)
	}
}

func TestRevisionSetsBrowserRevision(t *testing.T) {
	l := New()
	ret := l.Revision(424242)
	if ret != l {
		t.Fatal("Revision should return the launcher for chaining")
	}
	if l.browser.Revision != 424242 {
		t.Fatalf("expected browser revision 424242, got %d", l.browser.Revision)
	}
}

func TestXVFBSetsFlag(t *testing.T) {
	l := New()
	l.XVFB("-screen", "0", "1280x1024x16")

	vals, ok := l.GetFlags(flags.XVFB)
	if !ok {
		t.Fatal("expected XVFB flag set")
	}
	if len(vals) != 3 || vals[0] != "-screen" {
		t.Fatalf("unexpected XVFB values: %v", vals)
	}
}

func TestPreferencesSetsFlag(t *testing.T) {
	l := New()
	l.Preferences(`{"a":1}`)

	if got := l.Get(flags.Preferences); got != `{"a":1}` {
		t.Fatalf("expected preferences json, got %q", got)
	}
}

func TestLoggerSetsWriter(t *testing.T) {
	l := New()
	var buf strings.Builder
	ret := l.Logger(&buf)
	if ret != l {
		t.Fatal("Logger should return the launcher for chaining")
	}
	if l.logger != &buf {
		t.Fatal("expected logger writer to be set")
	}
}

// --- Get / Append edge cases ---------------------------------------------

func TestGetMissingFlagReturnsEmpty(t *testing.T) {
	l := New()
	if got := l.Get("definitely-not-set-flag"); got != "" {
		t.Fatalf("expected empty string for missing flag, got %q", got)
	}
}

func TestAppendCreatesFlagWhenMissing(t *testing.T) {
	l := New()
	// Append onto a flag that does not exist yet should create it.
	l.Append("brand-new-flag", "v1", "v2")

	vals, ok := l.GetFlags("brand-new-flag")
	if !ok {
		t.Fatal("expected flag created by Append")
	}
	if len(vals) != 2 || vals[0] != "v1" || vals[1] != "v2" {
		t.Fatalf("unexpected appended values: %v", vals)
	}
}

// --- FormatArgs: UserDataDir is absolutized, Env/scout- flags excluded ----

func TestFormatArgsAbsolutizesUserDataDir(t *testing.T) {
	l := New()
	l.UserDataDir("relative-data-dir")

	args := l.FormatArgs()

	var udd string
	for _, a := range args {
		if v, ok := strings.CutPrefix(a, "--user-data-dir="); ok {
			udd = v
		}
	}
	if udd == "" {
		t.Fatal("expected --user-data-dir in formatted args")
	}
	if !filepath.IsAbs(udd) {
		t.Fatalf("expected user-data-dir to be absolute, got %q", udd)
	}
}

func TestFormatArgsExcludesEnvFlag(t *testing.T) {
	l := New()
	l.Env("TZ=UTC")

	for _, a := range l.FormatArgs() {
		// flags.Env is "scout-env" so it must be filtered out.
		if strings.Contains(a, "scout-env") || strings.Contains(a, "TZ=UTC") {
			t.Fatalf("env flag leaked into CLI args: %s", a)
		}
	}
}

func TestFormatArgsSorted(t *testing.T) {
	l := New()
	l.Set("zzz-flag")
	l.Set("aaa-flag")

	args := l.FormatArgs()
	for i := 1; i < len(args); i++ {
		if args[i-1] > args[i] {
			t.Fatalf("FormatArgs output not sorted at index %d: %q > %q", i, args[i-1], args[i])
		}
	}
}

// --- Managed launcher: ClientHeader / JSON / mustManaged / KeepUserDataDir -

func TestJSONRoundTrip(t *testing.T) {
	l := New()
	l.Set(flags.ProxyServer, "http://proxy:8080")

	raw := l.JSON()
	if len(raw) == 0 {
		t.Fatal("JSON returned empty bytes")
	}

	// Must be valid JSON carrying the flags map.
	var decoded struct {
		Flags map[string][]string `json:"flags"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("JSON output is not valid: %v", err)
	}
	if got := decoded.Flags["proxy-server"]; len(got) != 1 || got[0] != "http://proxy:8080" {
		t.Fatalf("expected proxy-server in JSON, got %v", decoded.Flags["proxy-server"])
	}
}

func TestClientHeaderManaged(t *testing.T) {
	l := New()
	// Simulate a managed launcher without any network call.
	l.managed = true
	l.serviceURL = "ws://127.0.0.1:7317"

	u, h := l.ClientHeader()
	if u != "ws://127.0.0.1:7317" {
		t.Fatalf("expected service URL, got %q", u)
	}

	hdr := h.Get(string(HeaderName))
	if hdr == "" {
		t.Fatalf("expected %s header to be set", HeaderName)
	}
	// The header value must be the launcher serialized as JSON.
	var decoded map[string]any
	if err := json.Unmarshal([]byte(hdr), &decoded); err != nil {
		t.Fatalf("header is not valid JSON: %v", err)
	}
}

func TestKeepUserDataDirManaged(t *testing.T) {
	l := New()
	l.managed = true

	ret := l.KeepUserDataDir()
	if ret != l {
		t.Fatal("KeepUserDataDir should return the launcher for chaining")
	}
	if !l.Has(flags.KeepUserDataDir) {
		t.Fatal("expected keep-user-data-dir flag set")
	}
}

func TestMustManagedPanicsWhenUnmanaged(t *testing.T) {
	l := New() // not managed
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when calling managed-only method on unmanaged launcher")
		}
	}()
	// ClientHeader calls mustManaged internally.
	l.ClientHeader()
}

func TestMustManagedNoPanicWhenManaged(t *testing.T) {
	l := New()
	l.managed = true
	// Should not panic.
	l.mustManaged()
}

// --- URLParser.Err message selection --------------------------------------

func TestURLParserErrMessages(t *testing.T) {
	tests := []struct {
		name    string
		buffer  string
		wantSub string
	}{
		{
			name:    "generic failure",
			buffer:  "random noise",
			wantSub: "Failed to get the debug url",
		},
		{
			name:    "shared libraries hint",
			buffer:  "chrome: error while loading shared libraries: libX11.so",
			wantSub: "the doc might help",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewURLParser()
			p.Buffer = tt.buffer

			err := p.Err()
			if err == nil {
				t.Fatal("expected non-nil error")
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Fatalf("error %q does not contain %q", err.Error(), tt.wantSub)
			}
			// The original buffer content is always appended.
			if !strings.Contains(err.Error(), tt.buffer) {
				t.Fatalf("error %q should contain the buffer %q", err.Error(), tt.buffer)
			}
		})
	}
}
