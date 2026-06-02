package server

import (
	"sync/atomic"
	"testing"
)

func TestReconcileCallsReapHook(t *testing.T) {
	var called atomic.Int32

	orig := reapHook
	reapHook = func() int {
		called.Add(1)
		return 3
	}
	t.Cleanup(func() { reapHook = orig })

	s := New()
	killed := s.Reconcile()

	if called.Load() != 1 {
		t.Fatalf("expected reapHook called once, got %d", called.Load())
	}
	if killed != 3 {
		t.Fatalf("expected Reconcile to return reap count 3, got %d", killed)
	}
}

func TestReconcileEmptyMapNoAdoption(t *testing.T) {
	// Map is empty at boot: Reconcile must not panic and must not
	// touch s.sessions (no adoption — just on-disk reap).
	orig := reapHook
	reapHook = func() int { return 0 }
	t.Cleanup(func() { reapHook = orig })

	s := New()
	_ = s.Reconcile()

	n := 0
	s.sessions.Range(func(_, _ any) bool { n++; return true })
	if n != 0 {
		t.Fatalf("expected empty session map after Reconcile, got %d entries", n)
	}
}
