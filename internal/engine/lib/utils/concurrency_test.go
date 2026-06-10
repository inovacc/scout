package utils

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestAll(t *testing.T) {
	var counter int32
	wait := All(
		func() { atomic.AddInt32(&counter, 1) },
		func() { atomic.AddInt32(&counter, 1) },
		func() { atomic.AddInt32(&counter, 1) },
	)
	wait()
	if got := atomic.LoadInt32(&counter); got != 3 {
		t.Fatalf("expected 3 actions run, got %d", got)
	}

	t.Run("no actions returns immediately", func(t *testing.T) {
		done := make(chan struct{})
		go func() { All()(); close(done) }()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("All() with no actions should return immediately")
		}
	})
}

func TestIdleCounter(t *testing.T) {
	t.Run("waits for idle duration when no jobs", func(t *testing.T) {
		ic := NewIdleCounter(20 * time.Millisecond)
		start := time.Now()
		ic.Wait(context.Background())
		if elapsed := time.Since(start); elapsed < 15*time.Millisecond {
			t.Fatalf("Wait returned too early: %v", elapsed)
		}
	})

	t.Run("resolves after last job done", func(t *testing.T) {
		ic := NewIdleCounter(15 * time.Millisecond)
		ic.Add()
		ic.Add()

		done := make(chan struct{})
		go func() {
			ic.Wait(context.Background())
			close(done)
		}()

		// While jobs outstanding, Wait should not resolve.
		select {
		case <-done:
			t.Fatal("Wait resolved while jobs were outstanding")
		case <-time.After(30 * time.Millisecond):
		}

		ic.Done()
		ic.Done() // last job done -> idle timer starts

		select {
		case <-done:
		case <-time.After(500 * time.Millisecond):
			t.Fatal("Wait did not resolve after jobs completed")
		}
	})

	t.Run("context cancellation stops wait", func(t *testing.T) {
		ic := NewIdleCounter(time.Hour)
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			ic.Wait(ctx)
			close(done)
		}()
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("Wait did not return after context cancellation")
		}
	})

	t.Run("over-done panics", func(t *testing.T) {
		ic := NewIdleCounter(time.Millisecond)
		ic.Add()
		ic.Done()
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic on extra Done()")
			}
		}()
		ic.Done()
	})
}
