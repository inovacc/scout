package engine

import (
	"testing"
	"time"
)

func TestBoundedCleanupReturnsOnTimeout(t *testing.T) {
	start := time.Now()
	// fn blocks forever; boundedCleanup must still return after the timeout.
	ok := boundedCleanup(func() { select {} }, 100*time.Millisecond)
	elapsed := time.Since(start)

	if ok {
		t.Fatalf("boundedCleanup returned ok=true for a hung cleanup, want false")
	}
	if elapsed < 100*time.Millisecond {
		t.Fatalf("boundedCleanup returned too early: %v", elapsed)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("boundedCleanup did not honor timeout: %v", elapsed)
	}
}

func TestBoundedCleanupReturnsOnCompletion(t *testing.T) {
	done := make(chan struct{})
	ok := boundedCleanup(func() { close(done) }, 3*time.Second)
	if !ok {
		t.Fatalf("boundedCleanup returned ok=false for a fast cleanup, want true")
	}
	select {
	case <-done:
	default:
		t.Fatalf("cleanup fn did not run")
	}
}
