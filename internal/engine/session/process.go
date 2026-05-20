package session

import "strings"

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
