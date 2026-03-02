// Package idle provides an activity-based shutdown timer.
package idle

import (
	"sync"
	"time"
)

// Timer fires a callback after a period of inactivity. Each call to Reset
// restarts the countdown. A zero timeout disables the timer entirely.
type Timer struct {
	timeout time.Duration
	timer   *time.Timer
	onIdle  func()
	mu      sync.Mutex
	stopped bool
}

// New creates and starts an idle timer. When timeout elapses without a Reset
// call, onIdle is invoked in a separate goroutine. A zero timeout returns a
// no-op timer that never fires.
func New(timeout time.Duration, onIdle func()) *Timer {
	t := &Timer{
		timeout: timeout,
		onIdle:  onIdle,
	}

	if timeout <= 0 {
		t.stopped = true
		return t
	}

	t.timer = time.AfterFunc(timeout, func() {
		t.mu.Lock()
		if t.stopped {
			t.mu.Unlock()
			return
		}
		t.stopped = true
		t.mu.Unlock()
		onIdle()
	})

	return t
}

// Reset restarts the idle countdown to the full duration.
func (t *Timer) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.stopped || t.timer == nil {
		return
	}

	t.timer.Reset(t.timeout)
}

// Stop permanently disables the timer. It is safe to call multiple times.
func (t *Timer) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.stopped {
		return
	}

	t.stopped = true
	if t.timer != nil {
		t.timer.Stop()
	}
}

// Timeout returns the configured idle duration.
func (t *Timer) Timeout() time.Duration {
	return t.timeout
}
