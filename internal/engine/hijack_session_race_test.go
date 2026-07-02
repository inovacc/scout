package engine

import (
	"sync"
	"testing"
)

// TestSessionHijacker_EmitStopNoPanic hammers emit() while Stop() runs
// concurrently. Before the fix, emit checked stopCh unlocked and then sent to
// h.events, which Stop could close in between — a "send on closed channel"
// panic that crashed the whole process. Run under -race to also catch the
// unsynchronized access. The test passes iff neither a panic nor a data race
// occurs.
func TestSessionHijacker_EmitStopNoPanic(t *testing.T) {
	h := &SessionHijacker{
		events: make(chan HijackEvent, 4),
		stopCh: make(chan struct{}),
	}

	// Drain events until Stop closes the channel.
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		for range h.events { //nolint:revive // intentional drain
		}
	}()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				h.emit(HijackEvent{})
			}
		}()
	}

	// Stop concurrently with the emitters.
	go h.Stop()

	wg.Wait()
	h.Stop() // idempotent second call must not panic
	<-drained
}
