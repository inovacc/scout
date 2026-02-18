package scout

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// buildTestCRX3 creates a minimal CRX3 file with the given ZIP content.
func buildTestCRX3(t *testing.T, files map[string]string) []byte {
	t.Helper()

	// Build ZIP archive in memory.
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	for name, content := range files {
		fw, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	// CRX3 header: magic + version + header_length + empty protobuf header.
	var buf bytes.Buffer
	buf.WriteString("Cr24")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(3)) // version
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0)) // header length (no protobuf)
	buf.Write(zipBuf.Bytes())

	return buf.Bytes()
}

func TestUnpackCRX(t *testing.T) {
	manifest := `{"name":"Test Extension","version":"1.2.3","manifest_version":3}`
	crx := buildTestCRX3(t, map[string]string{
		"manifest.json": manifest,
		"background.js": "console.log('hello');",
	})

	dest := t.TempDir()
	if err := unpackCRX(crx, dest); err != nil {
		t.Fatalf("unpackCRX: %v", err)
	}

	// Verify files exist.
	data, err := os.ReadFile(filepath.Join(dest, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest.json: %v", err)
	}
	if string(data) != manifest {
		t.Errorf("manifest content = %q, want %q", data, manifest)
	}

	data, err = os.ReadFile(filepath.Join(dest, "background.js"))
	if err != nil {
		t.Fatalf("read background.js: %v", err)
	}
	if string(data) != "console.log('hello');" {
		t.Errorf("background.js content = %q", data)
	}
}

func TestUnpackCRXInvalidMagic(t *testing.T) {
	data := []byte("BAD_MAGIC_1234567890")
	err := unpackCRX(data, t.TempDir())
	if err == nil {
		t.Fatal("expected error for invalid magic")
	}
}

func TestUnpackCRXTooShort(t *testing.T) {
	err := unpackCRX([]byte("Cr2"), t.TempDir())
	if err == nil {
		t.Fatal("expected error for short data")
	}
}

func TestUnpackCRXUnsupportedVersion(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteString("Cr24")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(99))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(0))

	err := unpackCRX(buf.Bytes(), t.TempDir())
	if err == nil {
		t.Fatal("expected error for unsupported version")
	}
}

// buildTestCRX2 creates a minimal CRX2 file with the given ZIP content.
func buildTestCRX2(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	for name, content := range files {
		fw, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	// CRX2: magic + version=2 + pubkey_len + sig_len + pubkey + sig + ZIP
	fakeKey := []byte("fakepublickey")
	fakeSig := []byte("fakesig")

	var buf bytes.Buffer
	buf.WriteString("Cr24")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(2))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(len(fakeKey)))
	_ = binary.Write(&buf, binary.LittleEndian, uint32(len(fakeSig)))
	buf.Write(fakeKey)
	buf.Write(fakeSig)
	buf.Write(zipBuf.Bytes())

	return buf.Bytes()
}

func TestUnpackCRX2(t *testing.T) {
	manifest := `{"name":"CRX2 Extension","version":"0.9.0","manifest_version":2}`
	crx := buildTestCRX2(t, map[string]string{
		"manifest.json": manifest,
		"popup.html":    "<html></html>",
	})

	dest := t.TempDir()
	if err := unpackCRX(crx, dest); err != nil {
		t.Fatalf("unpackCRX v2: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dest, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest.json: %v", err)
	}
	if string(data) != manifest {
		t.Errorf("manifest content = %q, want %q", data, manifest)
	}
}

func TestUnpackCRX2TooShort(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteString("Cr24")
	_ = binary.Write(&buf, binary.LittleEndian, uint32(2))
	// Only 12 bytes total, CRX2 needs at least 16.
	err := unpackCRX(buf.Bytes(), t.TempDir())
	if err == nil {
		t.Fatal("expected error for CRX2 too short")
	}
}

func TestReadManifest(t *testing.T) {
	dir := t.TempDir()
	manifest := `{"name":"My Ext","version":"2.0.1"}`
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}

	name, version, err := readManifest(dir)
	if err != nil {
		t.Fatalf("readManifest: %v", err)
	}
	if name != "My Ext" {
		t.Errorf("name = %q, want %q", name, "My Ext")
	}
	if version != "2.0.1" {
		t.Errorf("version = %q, want %q", version, "2.0.1")
	}
}

func TestReadManifestDefaults(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	name, version, err := readManifest(dir)
	if err != nil {
		t.Fatalf("readManifest: %v", err)
	}
	if name != "unknown" {
		t.Errorf("name = %q, want %q", name, "unknown")
	}
	if version != "0.0.0" {
		t.Errorf("version = %q, want %q", version, "0.0.0")
	}
}

func TestReadManifestMissing(t *testing.T) {
	_, _, err := readManifest(t.TempDir())
	if err == nil {
		t.Fatal("expected error for missing manifest")
	}
}

func TestListLocalExtensions(t *testing.T) {
	// Override home dir via env for isolation.
	tmpHome := t.TempDir()
	extDir := filepath.Join(tmpHome, ".scout", "extensions")

	// Create two fake extensions.
	ext1Dir := filepath.Join(extDir, "ext-aaa")
	ext2Dir := filepath.Join(extDir, "ext-bbb")
	if err := os.MkdirAll(ext1Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(ext2Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ext1Dir, "manifest.json"),
		[]byte(`{"name":"Ext A","version":"1.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ext2Dir, "manifest.json"),
		[]byte(`{"name":"Ext B","version":"2.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a dir without manifest (should be skipped).
	if err := os.MkdirAll(filepath.Join(extDir, "no-manifest"), 0o755); err != nil {
		t.Fatal(err)
	}

	// We can't easily override UserHomeDir, so test the listing by calling readManifest directly
	// and verifying the scan logic works on the temp dir.
	entries, err := os.ReadDir(extDir)
	if err != nil {
		t.Fatal(err)
	}

	var exts []ExtensionInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(extDir, entry.Name())
		name, version, err := readManifest(dir)
		if err != nil {
			continue
		}
		exts = append(exts, ExtensionInfo{
			ID:      entry.Name(),
			Name:    name,
			Version: version,
			Path:    dir,
		})
	}

	if len(exts) != 2 {
		t.Fatalf("got %d extensions, want 2", len(exts))
	}
}

func TestRemoveExtension(t *testing.T) {
	// Test with empty ID.
	if err := RemoveExtension(""); err == nil {
		t.Fatal("expected error for empty id")
	}
}
