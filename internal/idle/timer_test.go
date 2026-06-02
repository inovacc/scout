package idle

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestTimerFiresOnIdle(t *testing.T) {
	var fired atomic.Int32
	done := make(chan struct{})

	tm := New(20*time.Millisecond, func() {
		fired.Add(1)
		close(done)
	})
	defer tm.Stop()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("onIdle did not fire within 2s")
	}

	if fired.Load() != 1 {
		t.Fatalf("expected onIdle to fire once, got %d", fired.Load())
	}
}

func TestTimerZeroTimeoutNeverFires(t *testing.T) {
	var fired atomic.Int32

	tm := New(0, func() { fired.Add(1) })
	defer tm.Stop()

	time.Sleep(50 * time.Millisecond)

	if fired.Load() != 0 {
		t.Fatalf("expected zero-timeout timer never to fire, got %d", fired.Load())
	}
}

func TestTimerStopBeforeFire(t *testing.T) {
	var fired atomic.Int32

	tm := New(100*time.Millisecond, func() { fired.Add(1) })
	tm.Stop()

	time.Sleep(200 * time.Millisecond)

	if fired.Load() != 0 {
		t.Fatalf("expected stopped timer never to fire, got %d", fired.Load())
	}
}

func TestTimerResetExtendsCountdown(t *testing.T) {
	var fired atomic.Int32

	tm := New(80*time.Millisecond, func() { fired.Add(1) })
	defer tm.Stop()

	// Reset twice within the window — total elapsed ~120ms but each Reset
	// restarts the 80ms countdown, so it must not have fired yet.
	time.Sleep(40 * time.Millisecond)
	tm.Reset()
	time.Sleep(40 * time.Millisecond)
	tm.Reset()

	if got := fired.Load(); got != 0 {
		t.Fatalf("expected no fire after resets, got %d", got)
	}
}
