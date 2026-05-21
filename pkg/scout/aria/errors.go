package aria

import (
	"errors"
	"fmt"
)

var (
	ErrStaleRef     = errors.New("scout: aria: stale ref")
	ErrNoSnapshot   = errors.New("scout: aria: no snapshot")
	ErrAmbiguousRef = errors.New("scout: aria: ambiguous ref")
	ErrCapture      = errors.New("scout: aria: capture failed")
	ErrTruncated    = errors.New("scout: aria: snapshot truncated")
)

// StaleRefError is returned when a caller requests a snapshot version older
// than what the cache holds.
type StaleRefError struct {
	Ref              string
	HaveVersion      uint64
	RequestedVersion uint64
}

func (e *StaleRefError) Error() string {
	return fmt.Sprintf("scout: aria: stale ref %s (have v%d, requested v%d)",
		e.Ref, e.HaveVersion, e.RequestedVersion)
}

func (e *StaleRefError) Unwrap() error { return ErrStaleRef }

// NoSnapshotError is returned when a page has no cached accessibility snapshot.
type NoSnapshotError struct {
	PageID string
}

func (e *NoSnapshotError) Error() string {
	return fmt.Sprintf("scout: aria: no snapshot for page %s", e.PageID)
}

func (e *NoSnapshotError) Unwrap() error { return ErrNoSnapshot }

// AmbiguousRefError is returned when a ref maps to a detached backend node.
type AmbiguousRefError struct {
	Ref           string
	BackendNodeID int64
}

func (e *AmbiguousRefError) Error() string {
	return fmt.Sprintf("scout: aria: ambiguous ref %s (backend node %d detached)",
		e.Ref, e.BackendNodeID)
}

func (e *AmbiguousRefError) Unwrap() error { return ErrAmbiguousRef }

// TruncatedError is returned when a snapshot was cut short due to node/byte cap.
type TruncatedError struct {
	Captured int
	Total    int
	Reason   string
}

func (e *TruncatedError) Error() string {
	return fmt.Sprintf("scout: aria: snapshot truncated: %s (%d/%d nodes)",
		e.Reason, e.Captured, e.Total)
}

func (e *TruncatedError) Unwrap() error { return ErrTruncated }
