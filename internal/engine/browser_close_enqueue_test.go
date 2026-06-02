package engine

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/inovacc/scout/internal/engine/session"
)

func TestCloseEnqueuesLockedDir(t *testing.T) {
	if runtime.GOOS != "windows" {
		// On Unix, RemoveAll succeeds even with an open handle; the locked-dir
		// scenario this guards is Windows-specific (AV / Search Indexer).
		t.Skip("locked-dir enqueue is a Windows file-lock scenario")
	}

	// Redirect the sessions root to a temp dir and restore after.
	orig := session.SessionsDir
	tmp := t.TempDir()
	session.SessionsDir = func() string { return tmp }
	t.Cleanup(func() { session.SessionsDir = orig })

	// Fabricate a non-reusable session dir with a data/ subdir.
	const sid = "1CHPNBN00000ABTMCOGNDUHRXOOPVGAQGIGA"
	dataDir := SessionDataDir(sid)
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatalf("mkdir session data dir: %v", err)
	}

	// Hold an open handle on a file inside data/ so os.RemoveAll fails on
	// Windows (open handles block deletion).
	lockPath := filepath.Join(dataDir, "LOCK")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("open lock file: %v", err)
	}
	defer func() { _ = f.Close() }()

	before := session.PendingCleanupCount()

	// Build a non-reusable browser bound to this session with no real launcher
	// or CDP connection, then Close it. The non-reusable branch attempts
	// os.RemoveAll(SessionDir(sid)); the open handle forces failure, which must
	// enqueue the dir via session.RecordCleanupFailure.
	b := &Browser{
		opts:      &options{sessionID: sid, reusableSession: false},
		sessionID: sid,
		done:      make(chan struct{}),
	}

	if err := b.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	after := session.PendingCleanupCount()
	if after != before+1 {
		t.Fatalf("PendingCleanupCount = %d, want %d (dir should be enqueued)", after, before+1)
	}
}
