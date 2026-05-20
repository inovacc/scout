package session

import "testing"

func TestIsScoutExec(t *testing.T) {
	tests := []struct {
		name string
		exec string
		want bool
	}{
		// True positives.
		{"unix bare", "/usr/local/bin/scout", true},
		{"windows bare", `C:\Program Files\scout\scout.exe`, true},
		{"windows lowercase drive", `c:\tools\scout.exe`, true},
		{"uppercase basename", "/usr/local/bin/SCOUT", true},
		{"mixed case .exe", `C:\bin\Scout.EXE`, true},

		// False positives the old strings.Contains check would have hit.
		{"substring boyscout", "/usr/local/bin/boyscout", false},
		{"substring scoutsdk", "/opt/scoutsdk-helper", false},
		{"substring whereisscout", "/tmp/whereisscout", false},
		{"substring scout-helper", "/usr/bin/scout-helper", false},
		{"substring myscout", "/home/user/bin/myscout", false},
		{"substring path contains scout", "/opt/scout-project/runner", false},
		{"directory named scout", "/etc/scout/tool", false},

		// Edge cases.
		{"empty", "", false},
		{"just slash", "/", false},
		{"unrelated", "/bin/bash", false},
		{"scout.exe substring", "/tmp/notscout.exe", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isScoutExec(tc.exec); got != tc.want {
				t.Errorf("isScoutExec(%q) = %v, want %v", tc.exec, got, tc.want)
			}
		})
	}
}
