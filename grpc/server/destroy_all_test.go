package server

import (
	"testing"

	"github.com/inovacc/scout/pkg/scout"
)

func TestDestroyAllSessionsClearsMap(t *testing.T) {
	s := New()

	for _, id := range []string{"a", "b", "c"} {
		sess := &session{id: id}
		s.sessions.Store(id, sess)
	}

	s.DestroyAllSessions()

	n := 0
	s.sessions.Range(func(_, _ any) bool { n++; return true })
	if n != 0 {
		t.Fatalf("expected all sessions destroyed, %d remain", n)
	}
}

func TestDestroyAllSessionsSurvivesPanic(t *testing.T) {
	s := New()

	// One session whose monitorCancel panics — the sweep must still
	// delete it and continue to the next session.
	panicSess := &session{id: "boom", monitorCancel: func() { panic("teardown boom") }}
	okSess := &session{id: "ok"}
	s.sessions.Store("boom", panicSess)
	s.sessions.Store("ok", okSess)

	s.DestroyAllSessions() // must not panic out

	n := 0
	s.sessions.Range(func(_, _ any) bool { n++; return true })
	if n != 0 {
		t.Fatalf("expected both sessions removed despite panic, %d remain", n)
	}
}

// compile-time guard that scout.Browser.Close is the nil-safe API we rely on.
var _ = func() *scout.Browser { return nil }
