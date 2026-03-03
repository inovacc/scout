package engine

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestAutoFreeOption(t *testing.T) {
	o := defaults()
	WithAutoFree(5 * time.Minute)(o)

	if o.autoFreeInterval != 5*time.Minute {
		t.Fatalf("expected 5m, got %v", o.autoFreeInterval)
	}
}

func TestAutoFreeCallbackOption(t *testing.T) {
	o := defaults()
	fn := func() {}
	WithAutoFreeCallback(fn)(o)

	if o.autoFreeCallback == nil {
		t.Fatal("expected callback to be set")
	}
}

func TestAutoFreeGoroutineLifecycle(t *testing.T) {
	// Verify the goroutine starts and the callback fires, then stops on close.
	var count atomic.Int32

	b := &Browser{
		opts: defaults(),
		done: make(chan struct{}),
	}

	b.startAutoFree(AutoFreeConfig{
		Interval: 50 * time.Millisecond,
		OnRecycle: func() {
			count.Add(1)
		},
	})

	// Wait enough for at least one tick. recycleBrowser will fail (nil browser)
	// but the callback should still fire.
	time.Sleep(200 * time.Millisecond)

	// Signal stop.
	close(b.done)

	// Give goroutine time to exit.
	time.Sleep(100 * time.Millisecond)

	c := count.Load()
	if c < 1 {
		t.Fatalf("expected callback to fire at least once, got %d", c)
	}

	t.Logf("callback fired %d times", c)
}

func TestAutoFreeStopsOnClose(t *testing.T) {
	var count atomic.Int32

	b := &Browser{
		opts: defaults(),
		done: make(chan struct{}),
	}

	b.startAutoFree(AutoFreeConfig{
		Interval: 50 * time.Millisecond,
		OnRecycle: func() {
			count.Add(1)
		},
	})

	// Let it tick a couple times.
	time.Sleep(150 * time.Millisecond)
	close(b.done)
	time.Sleep(50 * time.Millisecond)

	snapshot := count.Load()
	// After closing, no more increments should happen.
	time.Sleep(150 * time.Millisecond)

	final := count.Load()

	// Allow at most 1 extra (race between close and in-flight tick).
	if final > snapshot+1 {
		t.Fatalf("goroutine did not stop: snapshot=%d, final=%d", snapshot, final)
	}
}
