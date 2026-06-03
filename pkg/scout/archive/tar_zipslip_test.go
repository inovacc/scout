package archive

import (
	"archive/tar"
	"bytes"
	"path/filepath"
	"testing"
)

// TestSymlinkTargetWithinDest exercises the pure containment check (no FS ops,
// so it is cross-platform).
func TestSymlinkTargetWithinDest(t *testing.T) {
	dest := t.TempDir()
	link := filepath.Join(dest, "sub", "evil")

	if err := symlinkTargetWithinDest(dest, link, "../../../escape"); err == nil {
		t.Error("escaping relative symlink target should be rejected")
	}
	if err := symlinkTargetWithinDest(dest, link, "sibling"); err != nil {
		t.Errorf("in-tree symlink target should be allowed, got %v", err)
	}
}

// TestExtractTarRejectsEscapingSymlink proves a tar with a symlink whose target
// escapes destDir is refused before the link is ever created.
func TestExtractTarRejectsEscapingSymlink(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{
		Name:     "evil",
		Typeflag: tar.TypeSymlink,
		Linkname: "../../../../tmp/evil-escape",
		Mode:     0o777,
	}); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	if err := extractTar(&buf, t.TempDir()); err == nil {
		t.Fatal("expected error for escaping symlink target, got nil")
	}
}
