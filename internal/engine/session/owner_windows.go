//go:build windows

package session

import (
	"golang.org/x/sys/windows"
)

// validateDirOwner reports whether the session directory at the given path is
// owned by the current user's SID. Used to reject session directories planted
// by another local user — the H4 attack vector where the dir name is fully
// predictable.
//
// Returns false on error (treat as untrusted). Returns true when ownership
// matches the current process token's user SID.
//
// Note: %USERPROFILE% is conventionally only writable by its owning user, but
// in roaming-profile and shared-system configurations this is not guaranteed;
// the explicit check is worth keeping.
//
// Hardening H4 — see docs/quality/SESSION_HARDENING.md.
func validateDirOwner(dir string) bool {
	// Get owner SID of the directory's security descriptor.
	sd, err := windows.GetNamedSecurityInfo(
		dir,
		windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION,
	)
	if err != nil {
		return false
	}

	dirOwner, _, err := sd.Owner()
	if err != nil {
		return false
	}

	// Get current process token's user SID.
	var token windows.Token

	if err := windows.OpenProcessToken(
		windows.CurrentProcess(),
		windows.TOKEN_QUERY,
		&token,
	); err != nil {
		return false
	}

	defer func() { _ = token.Close() }()

	tokenUser, err := token.GetTokenUser()
	if err != nil {
		return false
	}

	return windows.EqualSid(dirOwner, tokenUser.User.Sid)
}
