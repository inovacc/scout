package aria_test

import (
	"errors"
	"testing"

	"github.com/inovacc/scout/pkg/scout/aria"
)

func TestStaleRefError_IsAndAs(t *testing.T) {
	t.Parallel()
	src := &aria.StaleRefError{Ref: "e15", HaveVersion: 14, RequestedVersion: 11}
	if !errors.Is(src, aria.ErrStaleRef) {
		t.Fatalf("errors.Is(src, ErrStaleRef) = false, want true")
	}
	var dst *aria.StaleRefError
	if !errors.As(src, &dst) {
		t.Fatalf("errors.As(src, &dst) = false")
	}
	if dst.Ref != "e15" || dst.HaveVersion != 14 || dst.RequestedVersion != 11 {
		t.Fatalf("typed fields lost: %+v", dst)
	}
}

func TestNoSnapshotError_IsAndAs(t *testing.T) {
	t.Parallel()
	src := &aria.NoSnapshotError{PageID: "p1"}
	if !errors.Is(src, aria.ErrNoSnapshot) {
		t.Fatalf("errors.Is failed")
	}
	var dst *aria.NoSnapshotError
	if !errors.As(src, &dst) || dst.PageID != "p1" {
		t.Fatalf("typed lost: %+v", dst)
	}
}

func TestAmbiguousRefError_IsAndAs(t *testing.T) {
	t.Parallel()
	src := &aria.AmbiguousRefError{Ref: "e3", BackendNodeID: 4242}
	if !errors.Is(src, aria.ErrAmbiguousRef) {
		t.Fatalf("errors.Is failed")
	}
	var dst *aria.AmbiguousRefError
	if !errors.As(src, &dst) || dst.Ref != "e3" || dst.BackendNodeID != 4242 {
		t.Fatalf("typed lost: %+v", dst)
	}
}

func TestTruncatedError_IsAndAs(t *testing.T) {
	t.Parallel()
	src := &aria.TruncatedError{Captured: 9999, Total: 50000, Reason: "node cap"}
	if !errors.Is(src, aria.ErrTruncated) {
		t.Fatalf("errors.Is failed")
	}
	var dst *aria.TruncatedError
	if !errors.As(src, &dst) || dst.Captured != 9999 {
		t.Fatalf("typed lost: %+v", dst)
	}
}

func TestErrorMessages_ScoutPrefix(t *testing.T) {
	t.Parallel()
	tests := []struct {
		err  error
		want string
	}{
		{(&aria.StaleRefError{Ref: "e1", HaveVersion: 2, RequestedVersion: 1}),
			"scout: aria: stale ref e1 (have v2, requested v1)"},
		{(&aria.NoSnapshotError{PageID: "p1"}),
			"scout: aria: no snapshot for page p1"},
		{(&aria.AmbiguousRefError{Ref: "e3", BackendNodeID: 4242}),
			"scout: aria: ambiguous ref e3 (backend node 4242 detached)"},
		{(&aria.TruncatedError{Captured: 5, Total: 10, Reason: "byte cap"}),
			"scout: aria: snapshot truncated: byte cap (5/10 nodes)"},
	}
	for _, tc := range tests {
		if got := tc.err.Error(); got != tc.want {
			t.Errorf("Error()=%q want %q", got, tc.want)
		}
	}
}
