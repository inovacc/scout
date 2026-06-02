package session

import (
	"path/filepath"
	"runtime"
	"strings"
)

// isUnderSessions reports whether p resolves to a path under GetSessionsDir().
// This is the hard safety floor for the data-dir scan: the reaper must NEVER
// kill a browser whose --user-data-dir is the user's real Chrome profile, so
// every kill candidate is gated through here before any signal is sent.
//
// Returns false when the sessions dir cannot be resolved (GetSessionsDir
// returns ""), failing closed rather than matching everything.
func isUnderSessions(p string) bool {
	root := GetSessionsDir()
	if root == "" {
		return false
	}

	return isUnderDir(p, root)
}

// isUnderDir reports whether p is root itself or lives somewhere beneath root.
// Both paths are cleaned to absolute-ish form (filepath.Clean) so ".." escapes
// cannot smuggle a path out of root. Comparison is case-insensitive on Windows
// where the filesystem is case-insensitive.
func isUnderDir(p, root string) bool {
	if p == "" || root == "" {
		return false
	}

	cp := filepath.Clean(p)
	cr := filepath.Clean(root)

	if runtime.GOOS == "windows" {
		cp = strings.ToLower(cp)
		cr = strings.ToLower(cr)
	}

	// Exact match: p IS root.
	if cp == cr {
		return true
	}

	// Relative path from root to p. If p is not under root, filepath.Rel
	// still succeeds but the result starts with "..".
	rel, err := filepath.Rel(cr, cp)
	if err != nil {
		return false
	}

	// rel == "." means equal (handled above); any rel that starts with ".."
	// (or is "..") escapes root and is therefore NOT under it.
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}

	return true
}

// extractUserDataDir pulls the value of the --user-data-dir flag out of a raw
// process command line. Handles the three Chrome flag forms:
//
//	--user-data-dir=VALUE
//	--user-data-dir VALUE
//	--user-data-dir="VALUE WITH SPACES"
//
// Returns "" when the flag is absent. The returned value is NOT cleaned; the
// caller (isUnderSessions / isUnderDir) cleans it during comparison.
func extractUserDataDir(cmdline string) string {
	const flag = "--user-data-dir"

	_, rest, found := strings.Cut(cmdline, flag)
	if !found || rest == "" {
		return ""
	}

	// "=value" form (possibly quoted).
	if v, ok := strings.CutPrefix(rest, "="); ok {
		return unquoteValue(v)
	}

	// " value" form: skip the separating spaces, then take the next token.
	rest = strings.TrimLeft(rest, " ")
	if rest == "" {
		return ""
	}

	return unquoteValue(rest)
}

// unquoteValue returns the first whitespace-delimited token of s, honoring a
// leading double quote (everything up to the closing quote) so paths with
// spaces survive intact.
func unquoteValue(s string) string {
	if v, ok := strings.CutPrefix(s, `"`); ok {
		if before, _, found := strings.Cut(v, `"`); found {
			return before
		}

		return v
	}

	// Unquoted: read up to the next space (or end of string).
	if before, _, found := strings.Cut(s, " "); found {
		return before
	}

	return s
}
