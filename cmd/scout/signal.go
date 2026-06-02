package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/inovacc/scout/pkg/scout"
)

// signalExitFunc terminates the process after signal cleanup. It is a package
// var so tests can override it (os.Exit cannot be asserted directly).
var signalExitFunc = os.Exit

// installSignalCleanup registers a best-effort SIGINT/SIGTERM handler. On the
// first such signal it runs cleanup (e.g. closing every live browser so chrome
// processes and scout.lock files are not leaked) and then exits with code 130
// (128 + SIGINT). This is the "best-effort" tier from the session-hardening
// design: it catches the signals we can catch but is not the OS-guaranteed tier
// (Job Object / CTRL_CLOSE are out of scope). It mirrors the signal.Notify block
// in cmd/scout/server.go. The returned channel is buffered so a signal arriving
// before Notify's goroutine is scheduled is not dropped.
func installSignalCleanup(cleanup func()) chan os.Signal {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		if cleanup != nil {
			cleanup()
		}
		signalExitFunc(130)
	}()

	return sigCh
}

// closeAllLiveCleanup closes every live browser with a bounded per-browser
// timeout. It is the cleanup callback installed by main()'s signal handler.
// scout.CloseAllLive ranges the live-browser registry and Closes each (so chrome
// children + scout.lock are released) and returns the count closed.
func closeAllLiveCleanup() {
	_ = scout.CloseAllLive(5 * time.Second)
}
