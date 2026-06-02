package server

import (
	"sync"
	"testing"
	"time"
)

func TestIdleTimerDestroysAllSessionsThenShutsDown(t *testing.T) {
	s := New()

	// Two nil-browser stub sessions (Close is nil-safe → no Chromium).
	s.sessions.Store("a", &session{id: "a"})
	s.sessions.Store("b", &session{id: "b"})

	var (
		mu                 sync.Mutex
		shutdownCalled     bool
		sessionsAtShutdown int
	)

	s.IdleTimeout = 20 * time.Millisecond
	s.OnIdleShutdown = func() {
		mu.Lock()
		shutdownCalled = true
		// Count sessions remaining when shutdown fires — must be 0,
		// proving DestroyAllSessions ran first.
		s.sessions.Range(func(_, _ any) bool { sessionsAtShutdown++; return true })
		mu.Unlock()
	}

	s.StartIdleTimer()
	defer s.StopIdleTimer()

	deadline := time.After(2 * time.Second)
	for {
		mu.Lock()
		done := shutdownCalled
		mu.Unlock()
		if done {
			break
		}
		select {
		case <-deadline:
			t.Fatal("idle shutdown did not fire within 2s")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if sessionsAtShutdown != 0 {
		t.Fatalf("expected 0 sessions when OnIdleShutdown fired, got %d", sessionsAtShutdown)
	}
}

func TestIdleTimerCallsDestroyAllBeforeShutdown(t *testing.T) {
	s := New()
	s.sessions.Store("x", &session{id: "x", monitorCancel: func() {}})

	order := make([]string, 0, 2)
	var mu sync.Mutex

	// monitorCancel records that full teardown ran for session x.
	s.sessions.Range(func(_, v any) bool {
		sess := v.(*session)
		sess.monitorCancel = func() {
			mu.Lock()
			order = append(order, "destroy")
			mu.Unlock()
		}
		return true
	})

	s.IdleTimeout = 20 * time.Millisecond
	s.OnIdleShutdown = func() {
		mu.Lock()
		order = append(order, "shutdown")
		mu.Unlock()
	}

	s.StartIdleTimer()
	defer s.StopIdleTimer()

	deadline := time.After(2 * time.Second)
	for {
		mu.Lock()
		got := len(order)
		mu.Unlock()
		if got >= 2 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("idle teardown incomplete: %v", order)
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(order) < 2 || order[0] != "destroy" || order[1] != "shutdown" {
		t.Fatalf("expected [destroy shutdown], got %v", order)
	}
}
