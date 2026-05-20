package session

import "strings"

// verifyProcess reports whether pid is alive AND the recorded start token
// matches the live process. Used to guard browser-process kills against PID
// reuse: between recording a session's BrowserPID and killing it, the
// kernel may reassign the PID to an unrelated process.
//
// If startToken is empty (legacy session metadata from before H2, or a
// platform where ProcessStartToken returns false) the check degrades to
// ProcessAlive-only. This preserves prior behavior on upgrade.
//
// Hardening H2 — see docs/quality/SESSION_HARDENING.md.
func verifyProcess(pid int, startToken string) bool {
	if !ProcessAlive(pid) {
		return false
	}

	if startToken == "" {
		return true
	}

	actual, ok := ProcessStartToken(pid)
	if !ok {
		return true
	}

	return actual == startToken
}

// isScoutExec reports whether the given executable path belongs to a scout
// binary, based on basename match. Case-insensitive. Matches "scout" or
// "scout.exe" only — NOT substring matches like "boyscout" or "scoutsdk".
//
// Hardening H1 — see docs/quality/SESSION_HARDENING.md. The previous check
// (strings.Contains(exec, "scout")) was over-permissive: any binary whose
// name happened to contain the substring "scout" could be misclassified as
// a live scout process, causing CleanOrphans to skip dead-scout sessions
// indefinitely and leaving browser PIDs to leak.
func isScoutExec(exec string) bool {
	if exec == "" {
		return false
	}

	// Split on both POSIX and Windows separators so the check is portable
	// across the OS where Scout runs and any host that inspects it.
	lower := strings.ToLower(exec)

	if i := strings.LastIndexAny(lower, "/\\"); i >= 0 {
		lower = lower[i+1:]
	}

	return lower == "scout" || lower == "scout.exe"
}
