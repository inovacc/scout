package aria

import (
	"sync"
)

// Store maps page IDs to their current Snapshot. Concurrent-safe.
type Store struct {
	mu sync.RWMutex
	m  map[string]*Snapshot
}

// NewStore returns an empty Store ready for use.
func NewStore() *Store {
	return &Store{m: make(map[string]*Snapshot)}
}

// Put stores snap under pageID, replacing any previous entry.
func (s *Store) Put(pageID string, snap *Snapshot) {
	s.mu.Lock()
	s.m[pageID] = snap
	s.mu.Unlock()
}

// Get returns the current Snapshot for pageID and true, or nil and false.
func (s *Store) Get(pageID string) (*Snapshot, bool) {
	s.mu.RLock()
	snap, ok := s.m[pageID]
	s.mu.RUnlock()
	return snap, ok
}

// Clear removes the snapshot for pageID, if any.
func (s *Store) Clear(pageID string) {
	s.mu.Lock()
	delete(s.m, pageID)
	s.mu.Unlock()
}

// Resolve returns the CDP backend node ID for a ref, or a typed error.
// NoSnapshotError: never snapshotted this page. StaleRefError: snapshotted
// but the ref is unknown in the current version.
func (s *Store) Resolve(pageID, ref string) (int64, error) {
	snap, ok := s.Get(pageID)
	if !ok {
		return 0, &NoSnapshotError{PageID: pageID}
	}
	for i := range snap.Nodes {
		if snap.Nodes[i].Ref == ref {
			return snap.Nodes[i].BackendNodeID, nil
		}
	}
	return 0, &StaleRefError{
		Ref:              ref,
		HaveVersion:      snap.Version,
		RequestedVersion: 0,
	}
}
