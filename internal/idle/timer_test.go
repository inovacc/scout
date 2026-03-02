package idle

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestTimerFires(t *testing.T) {
	var fired atomic.Bool

	timer := New(50*time.Millisecond, func() { fired.Store(true) })
	defer timer.Stop()

	time.Sleep(100 * time.Millisecond)

	if !fired.Load() {
		t.Fatal("expected timer to fire after timeout")
	}
}

func TestTimerResetPostpones(t *testing.T) {
	var fired atomic.Bool

	timer := New(80*time.Millisecond, func() { fired.Store(true) })
	defer timer.Stop()

	time.Sleep(50 * time.Millisecond)
	timer.Reset()

	time.Sleep(50 * time.Millisecond)

	if fired.Load() {
		t.Fatal("expected timer NOT to fire yet after reset")
	}

	time.Sleep(50 * time.Millisecond)

	if !fired.Load() {
		t.Fatal("expected timer to fire after reset + timeout")
	}
}

func TestTimerStopPrevents(t *testing.T) {
	var fired atomic.Bool

	timer := New(50*time.Millisecond, func() { fired.Store(true) })
	timer.Stop()

	time.Sleep(100 * time.Millisecond)

	if fired.Load() {
		t.Fatal("expected timer NOT to fire after Stop")
	}
}

func TestZeroTimeoutDisabled(t *testing.T) {
	var fired atomic.Bool

	timer := New(0, func() { fired.Store(true) })
	defer timer.Stop()

	time.Sleep(50 * time.Millisecond)

	if fired.Load() {
		t.Fatal("zero timeout timer should never fire")
	}
}

func TestTimerTimeout(t *testing.T) {
	timer := New(5*time.Minute, func() {})
	defer timer.Stop()

	if timer.Timeout() != 5*time.Minute {
		t.Fatalf("expected 5m, got %v", timer.Timeout())
	}
}
