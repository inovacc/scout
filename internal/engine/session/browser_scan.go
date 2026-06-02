package session

import (
	"os"
	"runtime"
	"strings"
)

// FindBrowsersUsingDataDir scans running processes for chrome/brave/msedge/
// chromium binaries whose command line references the given user-data-dir.
// Returns the PIDs that match.
//
// Used by `scout session audit` to detect orphan browsers holding a CORRUPT
// session dir (no scout.pid) so they can be killed before removing the dir.
//
// Linux: reads /proc/<pid>/cmdline.
// Windows: reads command lines via gops + win32 ProcessQuery.
// Darwin/BSD: returns nil — sysctl-based cmdline read not implemented.
func FindBrowsersUsingDataDir(dataDir string) []int {
	if dataDir == "" {
		return nil
	}

	switch runtime.GOOS {
	case "linux":
		return findBrowsersLinux(dataDir)
	case "windows":
		return findBrowsersWindows(dataDir)
	default:
		// SAFETY-FLOOR BLIND SPOT: Darwin/BSD have no /proc and we do not
		// implement sysctl-based cmdline reading. Callers must accept nil here.
		return nil
	}
}

// matchDataDirCmdline returns true iff the --user-data-dir value extracted from
// cmdline resolves under both GetSessionsDir() and the specific dataDir the
// caller is reaping. Substring matching is deliberately NOT used: a foreign
// profile path that merely contains the session path as a substring must never
// match.
func matchDataDirCmdline(cmdline, dataDir string) bool {
	if dataDir == "" {
		return false
	}

	udd := extractUserDataDir(cmdline)
	if udd == "" {
		return false
	}

	return isUnderSessions(udd) && isUnderDir(udd, dataDir)
}

func findBrowsersLinux(dataDir string) []int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}

	var pids []int

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		var pid int

		if _, err := fmtSscanf(e.Name(), &pid); err != nil {
			continue
		}

		data, err := os.ReadFile("/proc/" + e.Name() + "/cmdline")
		if err != nil {
			continue
		}

		cmdline := strings.ReplaceAll(string(data), "\x00", " ")
		if !isBrowserCmdline(cmdline) {
			continue
		}

		if matchDataDirCmdline(cmdline, dataDir) {
			pids = append(pids, pid)
		}
	}

	return pids
}

func isBrowserCmdline(s string) bool {
	lower := strings.ToLower(s)

	return strings.Contains(lower, "chrome.exe") ||
		strings.Contains(lower, "brave.exe") ||
		strings.Contains(lower, "msedge.exe") ||
		strings.Contains(lower, "chromium.exe") ||
		strings.Contains(lower, "/chrome ") ||
		strings.Contains(lower, "/chrome\x00") ||
		strings.Contains(lower, "/brave ") ||
		strings.Contains(lower, "/brave\x00") ||
		strings.Contains(lower, "/chromium ") ||
		strings.Contains(lower, "/chromium\x00")
}

// fmtSscanf wraps fmt.Sscanf for the single-int parse case to avoid an
// import on this otherwise-cheap file.
func fmtSscanf(s string, p *int) (int, error) {
	*p = 0

	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, errNotInt
		}

		*p = *p*10 + int(c-'0')
	}

	if *p == 0 {
		return 0, errNotInt
	}

	return 1, nil
}

var errNotInt = &parseError{msg: "not an int"}

type parseError struct{ msg string }

func (e *parseError) Error() string { return e.msg }
