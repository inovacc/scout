package utils

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSleep(t *testing.T) {
	start := time.Now()
	Sleep(0.01)
	if elapsed := time.Since(start); elapsed < 5*time.Millisecond {
		t.Fatalf("Sleep(0.01) returned too early: %v", elapsed)
	}
}

func TestMaxSleepCountError(t *testing.T) {
	err := &MaxSleepCountError{Max: 3}
	if err.Error() != "max sleep count 3 exceeded" {
		t.Fatalf("unexpected message: %q", err.Error())
	}

	t.Run("errors.As matches", func(t *testing.T) {
		wrapped := errors.New("outer")
		var target *MaxSleepCountError
		// errors.Is against another instance via custom Is method.
		if !errors.Is(err, &MaxSleepCountError{Max: 99}) {
			t.Fatal("errors.Is should match any MaxSleepCountError")
		}
		if errors.As(wrapped, &target) {
			t.Fatal("errors.As should not match a plain error")
		}
	})
}

func TestCountSleeper(t *testing.T) {
	s := CountSleeper(2)
	ctx := context.Background()

	if err := s(ctx); err != nil {
		t.Fatalf("first call should succeed: %v", err)
	}
	if err := s(ctx); err != nil {
		t.Fatalf("second call should succeed: %v", err)
	}

	err := s(ctx)
	var maxErr *MaxSleepCountError
	if !errors.As(err, &maxErr) {
		t.Fatalf("expected *MaxSleepCountError on overflow, got %v", err)
	}
	if maxErr.Max != 2 {
		t.Fatalf("expected Max 2, got %d", maxErr.Max)
	}

	t.Run("cancelled context returns ctx error", func(t *testing.T) {
		s2 := CountSleeper(5)
		cancelled, cancel := context.WithCancel(context.Background())
		cancel()
		if err := s2(cancelled); !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	})
}

func TestDefaultBackoff(t *testing.T) {
	base := time.Second
	for i := 0; i < 100; i++ {
		got := DefaultBackoff(base)
		// scale in [1.9, 2.1) => duration in [1.9s, 2.1s)
		if got < time.Duration(1.9*float64(base)) || got >= time.Duration(2.1*float64(base)) {
			t.Fatalf("DefaultBackoff out of range: %v", got)
		}
	}
}

func TestBackoffSleeper(t *testing.T) {
	t.Run("wakes immediately when maxInterval<=0", func(t *testing.T) {
		s := BackoffSleeper(time.Hour, 0, nil)
		start := time.Now()
		if err := s(context.Background()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
			t.Fatalf("expected immediate wake, took %v", elapsed)
		}
	})

	t.Run("uses custom algorithm and grows", func(t *testing.T) {
		calls := 0
		algo := func(d time.Duration) time.Duration {
			calls++
			return time.Millisecond // tiny so the test is fast
		}
		s := BackoffSleeper(time.Millisecond, time.Hour, algo)
		if err := s(context.Background()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if calls != 1 {
			t.Fatalf("expected custom algorithm called once, got %d", calls)
		}
	})

	t.Run("uses maxInterval when init>=max", func(t *testing.T) {
		algoCalled := false
		algo := func(d time.Duration) time.Duration {
			algoCalled = true
			return time.Hour
		}
		s := BackoffSleeper(time.Hour, time.Millisecond, algo)
		if err := s(context.Background()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if algoCalled {
			t.Fatal("algorithm must not be called when initInterval >= maxInterval")
		}
	})

	t.Run("context cancellation returns error", func(t *testing.T) {
		s := BackoffSleeper(time.Hour, time.Hour, func(time.Duration) time.Duration { return time.Hour })
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(10 * time.Millisecond)
			cancel()
		}()
		if err := s(ctx); !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	})
}

func TestEachSleepers(t *testing.T) {
	t.Run("all succeed", func(t *testing.T) {
		var order []int
		mk := func(n int) Sleeper {
			return func(context.Context) error { order = append(order, n); return nil }
		}
		s := EachSleepers(mk(1), mk(2), mk(3))
		if err := s(context.Background()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(order) != 3 || order[0] != 1 || order[2] != 3 {
			t.Fatalf("sleepers ran out of order: %v", order)
		}
	})

	t.Run("stops at first error", func(t *testing.T) {
		boom := errors.New("boom")
		ran := 0
		mk := func(err error) Sleeper {
			return func(context.Context) error { ran++; return err }
		}
		s := EachSleepers(mk(nil), mk(boom), mk(nil))
		err := s(context.Background())
		if !errors.Is(err, boom) {
			t.Fatalf("expected boom, got %v", err)
		}
		if ran != 2 {
			t.Fatalf("expected to stop after 2 sleepers, ran %d", ran)
		}
	})
}

func TestRaceSleepers(t *testing.T) {
	fast := func(ctx context.Context) error {
		t := time.NewTimer(5 * time.Millisecond)
		defer t.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			return nil
		}
	}
	slow := func(ctx context.Context) error {
		t := time.NewTimer(time.Hour)
		defer t.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			return nil
		}
	}

	s := RaceSleepers(fast, slow)
	if err := s(context.Background()); err != nil {
		t.Fatalf("RaceSleepers should win with the fast sleeper, got %v", err)
	}
}

func TestRetry(t *testing.T) {
	t.Run("stops when fn returns stop", func(t *testing.T) {
		calls := 0
		fn := func() (bool, error) {
			calls++
			return calls >= 3, nil
		}
		err := Retry(context.Background(), CountSleeper(10), fn)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if calls != 3 {
			t.Fatalf("expected fn called 3 times, got %d", calls)
		}
	})

	t.Run("propagates fn error on stop", func(t *testing.T) {
		boom := errors.New("fn boom")
		fn := func() (bool, error) { return true, boom }
		err := Retry(context.Background(), CountSleeper(10), fn)
		if !errors.Is(err, boom) {
			t.Fatalf("expected boom, got %v", err)
		}
	})

	t.Run("returns sleeper error when max exceeded", func(t *testing.T) {
		fn := func() (bool, error) { return false, nil } // never stops
		err := Retry(context.Background(), CountSleeper(2), fn)
		var maxErr *MaxSleepCountError
		if !errors.As(err, &maxErr) {
			t.Fatalf("expected *MaxSleepCountError, got %v", err)
		}
	})
}
