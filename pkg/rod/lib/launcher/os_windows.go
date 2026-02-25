//go:build windows

package launcher

import (
	"fmt"
	"os/exec"
	"syscall"
)

func killGroup(pid int) {
	// Use Windows Job Objects or taskkill to terminate the entire process tree (rod#865).
	// taskkill /T /F terminates the process and all child processes.
	cmd := exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", pid))
	_ = cmd.Run()

	// Fallback: directly terminate the process if taskkill failed.
	terminateProcess(pid)
}

func (l *Launcher) osSetupCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

func terminateProcess(pid int) {
	handle, err := syscall.OpenProcess(syscall.PROCESS_TERMINATE, true, uint32(pid))
	if err != nil {
		return
	}

	_ = syscall.TerminateProcess(handle, 0)
	_ = syscall.CloseHandle(handle)
}
