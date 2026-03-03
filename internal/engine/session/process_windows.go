package session

import "syscall"

const processQueryLimitedInformation = 0x1000

// ProcessAlive checks if a process with the given PID is still running.
// On Windows, os.FindProcess always succeeds, so we open the process handle.
func ProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	h, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		return false
	}

	_ = syscall.CloseHandle(h)

	return true
}
