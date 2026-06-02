package main

import (
	"os"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestInstallSignalCleanup_RunsCleanupAndExits(t *testing.T) {
	// Stub the exit hook so the test process is not terminated.
	var (
		mu       sync.Mutex
		gotCode  int
		exited   = make(chan struct{})
		cleaned  = make(chan struct{})
		origExit = signalExitFunc
	)
	t.Cleanup(func() { signalExitFunc = origExit })

	signalExitFunc = func(code int) {
		mu.Lock()
		gotCode = code
		mu.Unlock()
		close(exited)
		// Block forever to emulate os.Exit never returning; the test's
		// select below proceeds without waiting on this goroutine.
		select {}
	}

	cleanup := func() { close(cleaned) }

	sigCh := installSignalCleanup(cleanup)

	// Deliver the signal directly on the channel returned by the helper.
	sigCh <- syscall.SIGINT

	select {
	case <-cleaned:
	case <-time.After(2 * time.Second):
		t.Fatal("cleanup was not invoked after SIGINT")
	}

	select {
	case <-exited:
	case <-time.After(2 * time.Second):
		t.Fatal("signalExitFunc was not invoked after SIGINT")
	}

	mu.Lock()
	defer mu.Unlock()
	if gotCode != 130 {
		t.Errorf("exit code = %d, want 130", gotCode)
	}
}

func TestInstallSignalCleanup_ReturnsBufferedChannel(t *testing.T) {
	origExit := signalExitFunc
	t.Cleanup(func() { signalExitFunc = origExit })
	signalExitFunc = func(int) { select {} }

	sigCh := installSignalCleanup(func() {})
	if cap(sigCh) < 1 {
		t.Errorf("signal channel cap = %d, want >= 1 (buffered)", cap(sigCh))
	}

	// Ensure the channel actually carries os.Signal values.
	var _ chan os.Signal = sigCh
}
