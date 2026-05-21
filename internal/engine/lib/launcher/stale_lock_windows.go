//go:build windows

package launcher

import "golang.org/x/sys/windows"

// processAlive reports whether pid is a live process on Windows.
// Uses OpenProcess with PROCESS_QUERY_LIMITED_INFORMATION which works
// even for protected processes we cannot otherwise inspect.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)

	var code uint32
	if err := windows.GetExitCodeProcess(h, &code); err != nil {
		return false
	}
	const stillActive = 259 // STILL_ACTIVE
	return code == stillActive
}
