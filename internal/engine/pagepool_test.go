package engine

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestNewManagedPagePool(t *testing.T) {
	b := newTestBrowser(t)
	defer func() { _ = b.Close() }()

	pp, err := NewManagedPagePool(b, 3)
	if err != nil {
		t.Fatalf("NewManagedPagePool: %v", err)
	}
	defer pp.Close()

	if pp.Size() != 3 {
		t.Errorf("Size() = %d, want 3", pp.Size())
	}

	if pp.Available() != 3 {
		t.Errorf("Available() = %d, want 3", pp.Available())
	}
}

func TestNewManagedPagePoolNilBrowser(t *testing.T) {
	_, err := NewManagedPagePool(nil, 3)
	if err == nil {
		t.Fatal("expected error for nil browser")
	}
}

func TestNewManagedPagePoolInvalidSize(t *testing.T) {
	b := newTestBrowser(t)
	defer func() { _ = b.Close() }()

	_, err := NewManagedPagePool(b, 0)
	if err == nil {
		t.Fatal("expected error for size 0")
	}
}

func TestManagedPoolAcquireRelease(t *testing.T) {
	b := newTestBrowser(t)
	defer func() { _ = b.Close() }()

	pp, err := NewManagedPagePool(b, 2)
	if err != nil {
		t.Fatalf("NewManagedPagePool: %v", err)
	}
	defer pp.Close()

	ctx := context.Background()

	// Acquire a page.
	page, err := pp.Acquire(ctx)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	if page == nil {
		t.Fatal("Acquire returned nil page")
	}

	if pp.Available() != 1 {
		t.Errorf("Available() = %d, want 1 after acquire", pp.Available())
	}

	// Release the page.
	pp.Release(page)

	if pp.Available() != 2 {
		t.Errorf("Available() = %d, want 2 after release", pp.Available())
	}

	// Acquire the same slot again — should succeed.
	page2, err := pp.Acquire(ctx)
	if err != nil {
		t.Fatalf("Acquire (second): %v", err)
	}

	if page2 == nil {
		t.Fatal("Acquire (second) returned nil page")
	}

	pp.Release(page2)
}

func TestManagedPoolAcquireContextCancelled(t *testing.T) {
	b := newTestBrowser(t)
	defer func() { _ = b.Close() }()

	pp, err := NewManagedPagePool(b, 1)
	if err != nil {
		t.Fatalf("NewManagedPagePool: %v", err)
	}
	defer pp.Close()

	ctx := context.Background()

	// Drain the pool.
	page, err := pp.Acquire(ctx)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	// Now the pool is empty; acquire with a cancelled context should fail.
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = pp.Acquire(cancelCtx)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}

	pp.Release(page)
}

func TestManagedPoolConcurrency(t *testing.T) {
	b := newTestBrowser(t)
	defer func() { _ = b.Close() }()

	const poolSize = 3
	const goroutines = 10

	pp, err := NewManagedPagePool(b, poolSize)
	if err != nil {
		t.Fatalf("NewManagedPagePool: %v", err)
	}
	defer pp.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			page, acqErr := pp.Acquire(ctx)
			if acqErr != nil {
				t.Errorf("Acquire: %v", acqErr)
				return
			}

			// Simulate some work.
			time.Sleep(10 * time.Millisecond)

			pp.Release(page)
		}()
	}

	wg.Wait()

	if pp.Available() != poolSize {
		t.Errorf("Available() = %d, want %d after all goroutines done", pp.Available(), poolSize)
	}
}

func TestManagedPoolClose(t *testing.T) {
	b := newTestBrowser(t)
	defer func() { _ = b.Close() }()

	pp, err := NewManagedPagePool(b, 2)
	if err != nil {
		t.Fatalf("NewManagedPagePool: %v", err)
	}

	pp.Close()

	// Acquire after close should fail.
	ctx := context.Background()

	_, err = pp.Acquire(ctx)
	if err == nil {
		t.Fatal("expected error after pool close")
	}

	// Double close should not panic.
	pp.Close()
}

func TestManagedPoolCloseReleasedAfter(t *testing.T) {
	b := newTestBrowser(t)
	defer func() { _ = b.Close() }()

	pp, err := NewManagedPagePool(b, 1)
	if err != nil {
		t.Fatalf("NewManagedPagePool: %v", err)
	}

	ctx := context.Background()

	page, err := pp.Acquire(ctx)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}

	// Close pool while page is acquired.
	pp.Close()

	// Releasing after close should not panic — page gets closed instead.
	pp.Release(page)
}
