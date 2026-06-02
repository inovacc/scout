package session

import (
	"path/filepath"
	"testing"
)

// TestMatchDataDirCmdline is the static safety-floor proof: a fabricated
// command line whose --user-data-dir is under the temp sessions dir matches;
// one outside sessions/ never matches. No browser is launched.
func TestMatchDataDirCmdline(t *testing.T) {
	sessionsRoot := t.TempDir()
	orig := SessionsDir
	SessionsDir = func() string { return sessionsRoot }
	t.Cleanup(func() { SessionsDir = orig })

	const sessID = "1CHPNBN00000ABTMCOGNDUHRXOOPVGAQGIGA"
	sessData := filepath.Join(sessionsRoot, sessID, "data")
	otherData := filepath.Join(sessionsRoot, "1OTHER0000000ABTMCOGNDUHRXOOPVGAQ", "data")
	realChrome := filepath.Join(t.TempDir(), "Chrome", "User Data")

	cases := []struct {
		name    string
		cmdline string
		dataDir string
		want    bool
	}{
		{
			name:    "under sessions and under target",
			cmdline: "chrome --user-data-dir=" + sessData + " --headless",
			dataDir: sessData,
			want:    true,
		},
		{
			name:    "under sessions but different session is not under target",
			cmdline: "chrome --user-data-dir=" + otherData + " --headless",
			dataDir: sessData,
			want:    false,
		},
		{
			name:    "real user chrome outside sessions is never matched",
			cmdline: "chrome --user-data-dir=" + realChrome,
			dataDir: realChrome, // even if caller passes it, the sessions floor rejects it
			want:    false,
		},
		{
			name:    "no user-data-dir flag",
			cmdline: "chrome --headless --remote-debugging-port=0",
			dataDir: sessData,
			want:    false,
		},
		{
			name:    "empty dataDir",
			cmdline: "chrome --user-data-dir=" + sessData,
			dataDir: "",
			want:    false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := matchDataDirCmdline(tc.cmdline, tc.dataDir); got != tc.want {
				t.Fatalf("matchDataDirCmdline(%q, %q) = %v, want %v", tc.cmdline, tc.dataDir, got, tc.want)
			}
		})
	}
}
