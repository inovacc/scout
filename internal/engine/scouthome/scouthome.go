// Package scouthome resolves the per-user Scout state root.
//
// All persisted Scout state — sessions, browser cache, plugins, fingerprints,
// reports, OAuth tokens, extensions, electron runtime — lives under a single
// per-user root directory. This package is the one place that resolves that
// path, so callers don't bypass overrides like SCOUT_HOME.
//
// Resolution order:
//  1. SCOUT_HOME env var → use as-is
//  2. Platform-conventional default (preferred):
//     - Windows: %LOCALAPPDATA%\Scout
//     - Darwin:  ~/Library/Application Support/Scout
//     - Linux:   $XDG_DATA_HOME/scout, else ~/.local/share/scout
//  3. Legacy fallback: ~/.scout (for back-compat with installations that
//     pre-date the platform-default change). Returned only if the legacy
//     directory already exists AND the preferred default does not yet —
//     so old deployments keep working, while new installs use the
//     conventional path.
//  4. Error.
//
// Hardening H5 — previously 25+ call sites each called os.UserHomeDir and
// joined ".scout/<subdir>" directly. SCOUT_HOME affected only the sessions
// subdir, so browser binaries and OAuth tokens still leaked to the real
// home directory even when the test harness pointed SCOUT_HOME elsewhere.
package scouthome

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Root returns the per-user Scout state root.
func Root() (string, error) {
	if env := os.Getenv("SCOUT_HOME"); env != "" {
		return env, nil
	}

	preferred, perr := platformDefault()
	legacy, lerr := legacyDir()

	// Back-compat: prefer the legacy ~/.scout dir only if it exists AND
	// has real contents AND the new platform-default does not yet exist.
	// Empty ~/.scout (left behind from an aborted run or by curious users)
	// does not trigger back-compat — new installs go to the conventional
	// platform path.
	if lerr == nil && legacyHasContent(legacy) {
		if perr != nil {
			return legacy, nil
		}

		if _, err := os.Stat(preferred); err != nil {
			return legacy, nil
		}
	}

	if perr != nil {
		return "", perr
	}

	return preferred, nil
}

// platformDefault returns the conventional per-user state path for the
// current OS.
func platformDefault() (string, error) {
	switch runtime.GOOS {
	case "windows":
		if base := os.Getenv("LOCALAPPDATA"); base != "" {
			return filepath.Join(base, "Scout"), nil
		}
	case "darwin":
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, "Library", "Application Support", "Scout"), nil
		}
	default: // linux + BSDs
		if base := os.Getenv("XDG_DATA_HOME"); base != "" {
			return filepath.Join(base, "scout"), nil
		}

		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, ".local", "share", "scout"), nil
		}
	}

	return "", fmt.Errorf("scout: cannot resolve platform-default state dir")
}

// legacyDir returns the pre-H5 path (~/.scout) for back-compat lookup.
func legacyDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".scout"), nil
}

// legacyHasContent reports whether the legacy dir exists and contains at
// least one entry. Empty dirs do not trigger back-compat.
func legacyHasContent(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}

	return len(entries) > 0
}

// MustRoot panics with a clear message if Root errors. Use only in init or
// process startup paths where there is no sensible fallback.
func MustRoot() string {
	r, err := Root()
	if err != nil {
		panic("scout: cannot resolve home directory: " + err.Error())
	}

	return r
}

// Sub returns <root>/<subdir>. Helper for the common path-join idiom.
func Sub(subdir string) (string, error) {
	root, err := Root()
	if err != nil {
		return "", err
	}

	return filepath.Join(root, subdir), nil
}

// MustSub is the panic-on-error variant of Sub.
func MustSub(subdir string) string {
	return filepath.Join(MustRoot(), subdir)
}
