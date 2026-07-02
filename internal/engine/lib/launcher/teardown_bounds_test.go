package launcher

import (
	"testing"
	"time"
)

// TestKill_NoProcessReturnsFast proves Kill() no longer pays an unconditional
// 1s sleep on the no-process path (PID()==0). Regression for the "+1s on every
// command exit" defect.
func TestKill_NoProcessReturnsFast(t *testing.T) {
	l := New() // nothing launched → PID()==0

	done := make(chan struct{})
	go func() { l.Kill(); close(done) }()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Kill() with no process did not return promptly (unconditional sleep?)")
	}
}

// TestCleanup_BoundedWhenExitNeverCloses proves Cleanup() no longer blocks
// forever on l.exit. A launcher that never launched has an exit channel that
// never closes; Cleanup must still return within its bound.
func TestCleanup_BoundedWhenExitNeverCloses(t *testing.T) {
	l := New() // exit channel never closes

	done := make(chan struct{})
	go func() { l.Cleanup(); close(done) }()

	select {
	case <-done:
	case <-time.After(9 * time.Second):
		t.Fatal("Cleanup() blocked past its bound on a never-closing exit channel")
	}
}
