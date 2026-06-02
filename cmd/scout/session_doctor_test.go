package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/inovacc/scout/internal/engine/session"
)

// withTempSessions points the session layer at a fresh temp dir for the
// duration of the test and restores the original resolver afterwards.
func withTempSessions(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	orig := session.SessionsDir
	session.SessionsDir = func() string { return dir }
	t.Cleanup(func() { session.SessionsDir = orig })

	return dir
}

// TestSessionDoctorCleanExitsZero verifies doctor returns nil (exit 0) when
// there are no session folders at all (the at-rest invariant).
func TestSessionDoctorCleanExitsZero(t *testing.T) {
	_ = withTempSessions(t)

	cmd := newSessionDoctorCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(nil)

	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatalf("doctor on clean machine: want nil error, got %v", err)
	}

	if !strings.Contains(out.String(), "invariant holds") {
		t.Fatalf("expected clean verdict in output, got:\n%s", out.String())
	}
}

// TestSessionDoctorZombieExitsNonZero fabricates a CORRUPT folder (a dir with
// no scout.pid) and asserts doctor reports a violation and returns an error.
func TestSessionDoctorZombieExitsNonZero(t *testing.T) {
	dir := withTempSessions(t)

	// A bare directory with no scout.pid classifies as statusCorrupt.
	if err := mkdirAll(filepath.Join(dir, "1CHPNBN00000ABTMCOGNDUHRXOOPVGAQGIGA")); err != nil {
		t.Fatalf("mkdir fake session: %v", err)
	}

	cmd := newSessionDoctorCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(nil)

	if err := cmd.RunE(cmd, nil); err == nil {
		t.Fatalf("doctor with corrupt folder: want non-nil error, got nil\noutput:\n%s", out.String())
	}
}

// mkdirAll is a thin test helper so the test file needs no os import churn.
func mkdirAll(p string) error { return osMkdirAll(p) }

func osMkdirAll(p string) error { return os.MkdirAll(p, 0o700) }
