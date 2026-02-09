package scout

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestRateLimiterWait(t *testing.T) {
	rl := NewRateLimiter(WithRateLimit(100), WithBurstSize(10))

	start := time.Now()
	for i := 0; i < 10; i++ {
		rl.Wait()
	}
	elapsed := time.Since(start)

	// 10 requests with burst of 10 should be nearly instant
	if elapsed > 500*time.Millisecond {
		t.Errorf("10 burst requests took %v, expected < 500ms", elapsed)
	}
}

func TestRateLimiterDo(t *testing.T) {
	rl := NewRateLimiter(WithRateLimit(100), WithBurstSize(10))

	var count atomic.Int32
	err := rl.Do(func() error {
		count.Add(1)
		return nil
	})
	if err != nil {
		t.Fatalf("Do() error: %v", err)
	}
	if count.Load() != 1 {
		t.Errorf("fn called %d times, want 1", count.Load())
	}
}

func TestRateLimiterDoRetry(t *testing.T) {
	rl := NewRateLimiter(
		WithRateLimit(100),
		WithBurstSize(10),
		WithMaxRetries(3),
		WithBackoff(10*time.Millisecond),
		WithMaxBackoff(50*time.Millisecond),
	)

	var count atomic.Int32
	errTest := errors.New("transient error")

	err := rl.Do(func() error {
		n := count.Add(1)
		if n < 3 {
			return errTest
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Do() error: %v", err)
	}
	if count.Load() != 3 {
		t.Errorf("fn called %d times, want 3 (2 failures + 1 success)", count.Load())
	}
}

func TestRateLimiterDoMaxRetries(t *testing.T) {
	rl := NewRateLimiter(
		WithRateLimit(100),
		WithBurstSize(10),
		WithMaxRetries(2),
		WithBackoff(10*time.Millisecond),
		WithMaxBackoff(50*time.Millisecond),
	)

	errTest := errors.New("always fails")
	err := rl.Do(func() error {
		return errTest
	})
	if err == nil {
		t.Fatal("Do() should return error after max retries")
	}
	if !errors.Is(err, errTest) {
		t.Errorf("error should wrap original: %v", err)
	}
}

func TestRateLimiterMaxConcurrent(t *testing.T) {
	rl := NewRateLimiter(
		WithRateLimit(100),
		WithBurstSize(20),
		WithMaxConcurrent(2),
	)

	var running atomic.Int32
	var maxRunning atomic.Int32

	done := make(chan struct{}, 10)

	for i := 0; i < 5; i++ {
		go func() {
			err := rl.Do(func() error {
				cur := running.Add(1)
				// Track max concurrent
				for {
					old := maxRunning.Load()
					if cur <= old {
						break
					}
					if maxRunning.CompareAndSwap(old, cur) {
						break
					}
				}
				time.Sleep(50 * time.Millisecond)
				running.Add(-1)
				return nil
			})
			if err != nil {
				t.Errorf("Do() error: %v", err)
			}
			done <- struct{}{}
		}()
	}

	for i := 0; i < 5; i++ {
		<-done
	}

	if maxRunning.Load() > 2 {
		t.Errorf("max concurrent = %d, want <= 2", maxRunning.Load())
	}
}

func TestRateLimiterOptions(t *testing.T) {
	o := rateLimitDefaults()

	if o.rps != 2.0 {
		t.Errorf("default rps = %v, want 2.0", o.rps)
	}
	if o.burst != 5 {
		t.Errorf("default burst = %d, want 5", o.burst)
	}
	if o.maxRetries != 3 {
		t.Errorf("default maxRetries = %d, want 3", o.maxRetries)
	}
	if o.initialBackoff != time.Second {
		t.Errorf("default initialBackoff = %v, want 1s", o.initialBackoff)
	}
	if o.maxBackoff != 30*time.Second {
		t.Errorf("default maxBackoff = %v, want 30s", o.maxBackoff)
	}

	WithRateLimit(10)(o)
	if o.rps != 10 {
		t.Errorf("rps = %v, want 10", o.rps)
	}
	WithBurstSize(20)(o)
	if o.burst != 20 {
		t.Errorf("burst = %d, want 20", o.burst)
	}
	WithMaxConcurrent(5)(o)
	if o.maxConcurrent != 5 {
		t.Errorf("maxConcurrent = %d, want 5", o.maxConcurrent)
	}
	WithMaxRetries(5)(o)
	if o.maxRetries != 5 {
		t.Errorf("maxRetries = %d, want 5", o.maxRetries)
	}
	WithBackoff(2 * time.Second)(o)
	if o.initialBackoff != 2*time.Second {
		t.Errorf("initialBackoff = %v, want 2s", o.initialBackoff)
	}
	WithMaxBackoff(60 * time.Second)(o)
	if o.maxBackoff != 60*time.Second {
		t.Errorf("maxBackoff = %v, want 60s", o.maxBackoff)
	}
}

func TestNavigateWithRetry(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)
	page, err := b.NewPage("")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	rl := NewRateLimiter(WithRateLimit(100), WithBurstSize(10))

	if err := page.NavigateWithRetry(srv.URL, rl); err != nil {
		t.Fatalf("NavigateWithRetry() error: %v", err)
	}

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	title, err := page.Title()
	if err != nil {
		t.Fatalf("Title() error: %v", err)
	}
	if title != "Test Page" {
		t.Errorf("Title() = %q, want %q", title, "Test Page")
	}
}
