//go:build !windows

package launcher

import (
	"os/exec"
	"syscall"
	"time"

	"github.com/inovacc/scout/internal/engine/lib/launcher/flags"
)

func killGroup(pid int) {
	// Try graceful SIGTERM first, then SIGKILL after a short delay.
	// This gives Chrome a chance to clean up child processes (rod#865).
	_ = syscall.Kill(-pid, syscall.SIGTERM)

	// Give the process group a moment to exit gracefully.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 20 { // up to ~200ms
			if err := syscall.Kill(pid, 0); err != nil {
				return // process already gone
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	select {
	case <-done:
		// Process exited gracefully, nothing more to do.
	case <-time.After(300 * time.Millisecond):
		// Force kill the entire process group.
		_ = syscall.Kill(-pid, syscall.SIGKILL)
	}
}

func (l *Launcher) osSetupCmd(cmd *exec.Cmd) {
	if flags, has := l.GetFlags(flags.XVFB); has {
		var command []string
		// flags must append before cmd.Args
		command = append(command, flags...)
		command = append(command, cmd.Args...)

		*cmd = *exec.Command("xvfb-run", command...)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
