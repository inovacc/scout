package aria_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/inovacc/scout/pkg/scout/aria"
)

func TestStore_PutGet(t *testing.T) {
	t.Parallel()
	s := aria.NewStore()
	snap := &aria.Snapshot{PageID: "p1", Version: 1}
	s.Put("p1", snap)

	got, ok := s.Get("p1")
	if !ok || got.Version != 1 {
		t.Fatalf("Get returned (%v, %v), want (v1, true)", got, ok)
	}

	_, ok = s.Get("p2")
	if ok {
		t.Fatalf("Get(p2) ok=true, want false")
	}
}

func TestStore_Clear(t *testing.T) {
	t.Parallel()
	s := aria.NewStore()
	s.Put("p1", &aria.Snapshot{PageID: "p1", Version: 1})
	s.Clear("p1")
	if _, ok := s.Get("p1"); ok {
		t.Fatalf("Get after Clear returned ok=true")
	}
}

func TestStore_Resolve_NoSnapshot(t *testing.T) {
	t.Parallel()
	s := aria.NewStore()
	_, err := s.Resolve("p1", "e15")
	var nse *aria.NoSnapshotError
	if !errors.As(err, &nse) || nse.PageID != "p1" {
		t.Fatalf("Resolve err=%v want NoSnapshotError{p1}", err)
	}
}

func TestStore_Resolve_StaleRef(t *testing.T) {
	t.Parallel()
	s := aria.NewStore()
	s.Put("p1", &aria.Snapshot{
		PageID:  "p1",
		Version: 7,
		Nodes: []aria.Node{
			{Ref: "e1", BackendNodeID: 100},
			{Ref: "e2", BackendNodeID: 200},
		},
	})
	_, err := s.Resolve("p1", "e99")
	var sre *aria.StaleRefError
	if !errors.As(err, &sre) || sre.HaveVersion != 7 {
		t.Fatalf("Resolve unknown ref err=%v want StaleRefError v=7", err)
	}
}

func TestStore_Resolve_Hit(t *testing.T) {
	t.Parallel()
	s := aria.NewStore()
	s.Put("p1", &aria.Snapshot{
		PageID:  "p1",
		Version: 3,
		Nodes:   []aria.Node{{Ref: "e1", BackendNodeID: 4242}},
	})
	bn, err := s.Resolve("p1", "e1")
	if err != nil || bn != 4242 {
		t.Fatalf("Resolve(e1)=(%d,%v), want (4242,nil)", bn, err)
	}
}

func TestStore_Concurrent(t *testing.T) {
	t.Parallel()
	s := aria.NewStore()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(i int) { defer wg.Done(); s.Put("p", &aria.Snapshot{PageID: "p", Version: uint64(i)}) }(i)
		go func() { defer wg.Done(); _, _ = s.Get("p") }()
	}
	wg.Wait()
}
