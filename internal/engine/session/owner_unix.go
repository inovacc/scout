//go:build !windows

package session

import (
	"os"
	"syscall"
)

// validateDirOwner reports whether the session directory at the given path is
// owned by the current user's UID. Used to reject session directories planted
// by a different local user — the H4 attack vector where the dir name (hash
// of domain + browser label) is fully predictable.
//
// Returns false if the dir does not exist or owner UID differs. Returns true
// when the platform's Stat_t cannot be cast (theoretical, since all targeted
// Unix variants implement it) — degrades to no-op rather than denying.
//
// Hardening H4 — see docs/quality/SESSION_HARDENING.md.
func validateDirOwner(dir string) bool {
	info, err := os.Stat(dir)
	if err != nil {
		return false
	}

	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return true
	}

	return st.Uid == uint32(os.Getuid())
}
