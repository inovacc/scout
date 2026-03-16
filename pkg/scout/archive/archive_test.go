package archive

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// DetectFormat
// ---------------------------------------------------------------------------

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     Format
	}{
		{"zip", "browser.zip", FormatZip},
		{"tar.gz", "browser.tar.gz", FormatTarGz},
		{"tgz", "browser.tgz", FormatTarGz},
		{"tar", "browser.tar", FormatTar},
		{"deb", "browser.deb", FormatDeb},
		{"rpm", "browser.rpm", FormatRPM},
		{"uppercase zip", "BROWSER.ZIP", FormatZip},
		{"mixed case tar.gz", "Browser.Tar.Gz", FormatTarGz},
		{"uppercase TGZ", "FILE.TGZ", FormatTarGz},
		{"unknown", "browser.exe", Format("")},
		{"empty", "", Format("")},
		{"no extension", "browser", Format("")},
		{"dot only", ".", Format("")},
		{"path with zip", "path/to/browser.zip", FormatZip},
		{"path with tar.gz", "/opt/downloads/chrome.tar.gz", FormatTarGz},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectFormat(tt.filename)
			if got != tt.want {
				t.Errorf("DetectFormat(%q) = %q, want %q", tt.filename, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ForExtractor
// ---------------------------------------------------------------------------

func TestForExtractor(t *testing.T) {
	tests := []struct {
		name       string
		format     Format
		wantNil    bool
		wantFormat Format
	}{
		{"zip", FormatZip, false, FormatZip},
		{"tar.gz", FormatTarGz, false, FormatTarGz},
		{"tar", FormatTar, false, FormatTar},
		{"deb", FormatDeb, false, FormatDeb},
		{"rpm", FormatRPM, false, FormatRPM},
		{"unknown", Format("unknown"), true, ""},
		{"empty", Format(""), true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := ForExtractor(tt.format)
			if tt.wantNil {
				if e != nil {
					t.Errorf("ForExtractor(%q) = %T, want nil", tt.format, e)
				}
				return
			}
			if e == nil {
				t.Fatalf("ForExtractor(%q) = nil, want non-nil", tt.format)
			}
			if e.Format() != tt.wantFormat {
				t.Errorf("ForExtractor(%q).Format() = %q, want %q", tt.format, e.Format(), tt.wantFormat)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Extract (top-level dispatcher)
// ---------------------------------------------------------------------------

func TestExtract(t *testing.T) {
	t.Run("unsupported format", func(t *testing.T) {
		err := Extract([]byte("data"), "file.exe", t.TempDir())
		if err == nil {
			t.Fatal("expected error for unsupported format")
		}
	})

	t.Run("zip round-trip", func(t *testing.T) {
		data := createTestZip(t, map[string]string{
			"hello.txt": "world",
		})
		dest := t.TempDir()
		if err := Extract(data, "test.zip", dest); err != nil {
			t.Fatalf("Extract: %v", err)
		}
		assertFileContent(t, filepath.Join(dest, "hello.txt"), "world")
	})

	t.Run("tar.gz round-trip", func(t *testing.T) {
		data := createTestTarGz(t, map[string]string{
			"greeting.txt": "hello",
		})
		dest := t.TempDir()
		if err := Extract(data, "test.tar.gz", dest); err != nil {
			t.Fatalf("Extract: %v", err)
		}
		assertFileContent(t, filepath.Join(dest, "greeting.txt"), "hello")
	})

	t.Run("tgz round-trip", func(t *testing.T) {
		data := createTestTarGz(t, map[string]string{
			"file.txt": "content",
		})
		dest := t.TempDir()
		if err := Extract(data, "test.tgz", dest); err != nil {
			t.Fatalf("Extract: %v", err)
		}
		assertFileContent(t, filepath.Join(dest, "file.txt"), "content")
	})
}

// ---------------------------------------------------------------------------
// ZipExtractor
// ---------------------------------------------------------------------------

func TestZipExtractor(t *testing.T) {
	t.Run("single file", func(t *testing.T) {
		data := createTestZip(t, map[string]string{
			"test.txt": "hello zip",
		})
		dest := t.TempDir()
		e := &ZipExtractor{}
		if err := e.Extract(data, dest); err != nil {
			t.Fatalf("Extract: %v", err)
		}
		assertFileContent(t, filepath.Join(dest, "test.txt"), "hello zip")
	})

	t.Run("nested directories", func(t *testing.T) {
		data := createTestZip(t, map[string]string{
			"a/b/c.txt": "nested",
		})
		dest := t.TempDir()
		e := &ZipExtractor{}
		if err := e.Extract(data, dest); err != nil {
			t.Fatalf("Extract: %v", err)
		}
		assertFileContent(t, filepath.Join(dest, "a", "b", "c.txt"), "nested")
	})

	t.Run("multiple files", func(t *testing.T) {
		data := createTestZip(t, map[string]string{
			"one.txt":     "1",
			"two.txt":     "2",
			"sub/three.txt": "3",
		})
		dest := t.TempDir()
		e := &ZipExtractor{}
		if err := e.Extract(data, dest); err != nil {
			t.Fatalf("Extract: %v", err)
		}
		assertFileContent(t, filepath.Join(dest, "one.txt"), "1")
		assertFileContent(t, filepath.Join(dest, "two.txt"), "2")
		assertFileContent(t, filepath.Join(dest, "sub", "three.txt"), "3")
	})

	t.Run("directory entry", func(t *testing.T) {
		data := createTestZipWithDirs(t, []string{"mydir/"}, nil)
		dest := t.TempDir()
		e := &ZipExtractor{}
		if err := e.Extract(data, dest); err != nil {
			t.Fatalf("Extract: %v", err)
		}
		info, err := os.Stat(filepath.Join(dest, "mydir"))
		if err != nil {
			t.Fatalf("stat dir: %v", err)
		}
		if !info.IsDir() {
			t.Error("expected directory")
		}
	})

	t.Run("invalid zip data", func(t *testing.T) {
		e := &ZipExtractor{}
		if err := e.Extract([]byte("not a zip"), t.TempDir()); err == nil {
			t.Fatal("expected error for invalid zip")
		}
	})

	t.Run("format", func(t *testing.T) {
		e := &ZipExtractor{}
		if e.Format() != FormatZip {
			t.Errorf("Format() = %q, want %q", e.Format(), FormatZip)
		}
	})
}

// ---------------------------------------------------------------------------
// TarExtractor
// ---------------------------------------------------------------------------

func TestTarExtractor(t *testing.T) {
	t.Run("single file", func(t *testing.T) {
		data := createTestTar(t, map[string]string{
			"test.txt": "hello tar",
		})
		dest := t.TempDir()
		e := &TarExtractor{}
		if err := e.Extract(data, dest); err != nil {
			t.Fatalf("Extract: %v", err)
		}
		assertFileContent(t, filepath.Join(dest, "test.txt"), "hello tar")
	})

	t.Run("nested directories", func(t *testing.T) {
		data := createTestTar(t, map[string]string{
			"a/b/deep.txt": "deep",
		})
		dest := t.TempDir()
		e := &TarExtractor{}
		if err := e.Extract(data, dest); err != nil {
			t.Fatalf("Extract: %v", err)
		}
		assertFileContent(t, filepath.Join(dest, "a", "b", "deep.txt"), "deep")
	})

	t.Run("with directory entry", func(t *testing.T) {
		data := createTestTarWithDirEntries(t, []string{"mydir/"}, map[string]string{
			"mydir/file.txt": "inside",
		})
		dest := t.TempDir()
		e := &TarExtractor{}
		if err := e.Extract(data, dest); err != nil {
			t.Fatalf("Extract: %v", err)
		}
		info, err := os.Stat(filepath.Join(dest, "mydir"))
		if err != nil {
			t.Fatalf("stat dir: %v", err)
		}
		if !info.IsDir() {
			t.Error("expected directory")
		}
		assertFileContent(t, filepath.Join(dest, "mydir", "file.txt"), "inside")
	})

	t.Run("format", func(t *testing.T) {
		e := &TarExtractor{}
		if e.Format() != FormatTar {
			t.Errorf("Format() = %q, want %q", e.Format(), FormatTar)
		}
	})
}

// ---------------------------------------------------------------------------
// TarGzExtractor
// ---------------------------------------------------------------------------

func TestTarGzExtractor(t *testing.T) {
	t.Run("single file", func(t *testing.T) {
		data := createTestTarGz(t, map[string]string{
			"test.txt": "hello tar.gz",
		})
		dest := t.TempDir()
		e := &TarGzExtractor{}
		if err := e.Extract(data, dest); err != nil {
			t.Fatalf("Extract: %v", err)
		}
		assertFileContent(t, filepath.Join(dest, "test.txt"), "hello tar.gz")
	})

	t.Run("multiple files", func(t *testing.T) {
		data := createTestTarGz(t, map[string]string{
			"a.txt":     "aaa",
			"dir/b.txt": "bbb",
		})
		dest := t.TempDir()
		e := &TarGzExtractor{}
		if err := e.Extract(data, dest); err != nil {
			t.Fatalf("Extract: %v", err)
		}
		assertFileContent(t, filepath.Join(dest, "a.txt"), "aaa")
		assertFileContent(t, filepath.Join(dest, "dir", "b.txt"), "bbb")
	})

	t.Run("invalid gzip", func(t *testing.T) {
		e := &TarGzExtractor{}
		if err := e.Extract([]byte("not gzip"), t.TempDir()); err == nil {
			t.Fatal("expected error for invalid gzip")
		}
	})

	t.Run("format", func(t *testing.T) {
		e := &TarGzExtractor{}
		if e.Format() != FormatTarGz {
			t.Errorf("Format() = %q, want %q", e.Format(), FormatTarGz)
		}
	})
}

// ---------------------------------------------------------------------------
// DebExtractor
// ---------------------------------------------------------------------------

func TestDebExtractor(t *testing.T) {
	t.Run("valid deb", func(t *testing.T) {
		data := createTestDeb(t, map[string]string{
			"usr/bin/hello.txt": "deb content",
		})
		dest := t.TempDir()
		e := &DebExtractor{}
		if err := e.Extract(data, dest); err != nil {
			t.Fatalf("Extract: %v", err)
		}
		assertFileContent(t, filepath.Join(dest, "usr", "bin", "hello.txt"), "deb content")
	})

	t.Run("not an ar archive", func(t *testing.T) {
		e := &DebExtractor{}
		if err := e.Extract([]byte("not a deb"), t.TempDir()); err == nil {
			t.Fatal("expected error for invalid deb")
		}
	})

	t.Run("ar with no data.tar", func(t *testing.T) {
		// Build a valid ar archive with only a control.tar.gz entry.
		var buf bytes.Buffer
		buf.WriteString("!<arch>\n")
		writeArEntry(&buf, "control.tar.gz", []byte("fake"))
		e := &DebExtractor{}
		if err := e.Extract(buf.Bytes(), t.TempDir()); err == nil {
			t.Fatal("expected error when data.tar.* not found")
		}
	})

	t.Run("truncated ar header", func(t *testing.T) {
		e := &DebExtractor{}
		// Valid ar magic but truncated header.
		if err := e.Extract([]byte("!<arch>\n" + "short"), t.TempDir()); err == nil {
			t.Fatal("expected error for truncated header")
		}
	})

	t.Run("format", func(t *testing.T) {
		e := &DebExtractor{}
		if e.Format() != FormatDeb {
			t.Errorf("Format() = %q, want %q", e.Format(), FormatDeb)
		}
	})
}

// ---------------------------------------------------------------------------
// RPMExtractor
// ---------------------------------------------------------------------------

func TestRPMExtractor(t *testing.T) {
	t.Run("valid rpm", func(t *testing.T) {
		data := createTestRPM(t, map[string]string{
			"usr/bin/app.txt": "rpm content",
		})
		dest := t.TempDir()
		e := &RPMExtractor{}
		if err := e.Extract(data, dest); err != nil {
			t.Fatalf("Extract: %v", err)
		}
		assertFileContent(t, filepath.Join(dest, "usr", "bin", "app.txt"), "rpm content")
	})

	t.Run("not an rpm file", func(t *testing.T) {
		e := &RPMExtractor{}
		if err := e.Extract([]byte("not an rpm"), t.TempDir()); err == nil {
			t.Fatal("expected error for invalid rpm")
		}
	})

	t.Run("truncated rpm", func(t *testing.T) {
		e := &RPMExtractor{}
		if err := e.Extract([]byte{0xED, 0xAB, 0xEE, 0xDB}, t.TempDir()); err == nil {
			t.Fatal("expected error for truncated rpm")
		}
	})

	t.Run("format", func(t *testing.T) {
		e := &RPMExtractor{}
		if e.Format() != FormatRPM {
			t.Errorf("Format() = %q, want %q", e.Format(), FormatRPM)
		}
	})
}

// ---------------------------------------------------------------------------
// pathSlipCheck
// ---------------------------------------------------------------------------

func TestPathSlipCheck(t *testing.T) {
	tests := []struct {
		name      string
		entry     string
		wantErr   bool
	}{
		{"normal file", "file.txt", false},
		{"nested file", "a/b/c.txt", false},
		{"traversal", "../../../etc/passwd", true},
		{"sneaky traversal", "a/../../etc/passwd", true},
		{"current dir", ".", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dest := t.TempDir()
			_, err := pathSlipCheck(dest, tt.entry)
			if tt.wantErr && err == nil {
				t.Error("expected error for path traversal")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseHex
// ---------------------------------------------------------------------------

func TestParseHex(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"00000000", 0},
		{"00000001", 1},
		{"0000000A", 10},
		{"0000000a", 10},
		{"000000FF", 255},
		{"000000ff", 255},
		{"00000100", 256},
		{"DEADBEEF", 0xDEADBEEF},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseHex([]byte(tt.input))
			if got != tt.want {
				t.Errorf("parseHex(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// isDataTar
// ---------------------------------------------------------------------------

func TestIsDataTar(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"data.tar.gz", true},
		{"data.tar.xz", true},
		{"data.tar.zst", true},
		{"data.tar.bz2", true},
		{"data.tar", true},
		{"control.tar.gz", false},
		{"other.tar.gz", false},
		{"data.tar.lz4", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDataTar(tt.name)
			if got != tt.want {
				t.Errorf("isDataTar(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// trimRight
// ---------------------------------------------------------------------------

func TestTrimRight(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"hello   ", "hello"},
		{"hello/", "hello"},
		{"hello/  ", "hello"},
		{"   ", ""},
		{"///", ""},
		{"", ""},
		{"a / ", "a"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q", tt.input), func(t *testing.T) {
			got := trimRight(tt.input)
			if got != tt.want {
				t.Errorf("trimRight(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// writeFile
// ---------------------------------------------------------------------------

func TestWriteFile(t *testing.T) {
	t.Run("creates parent dirs", func(t *testing.T) {
		dest := t.TempDir()
		path := filepath.Join(dest, "a", "b", "file.txt")
		if err := writeFile(path, []byte("test"), 0o644); err != nil {
			t.Fatalf("writeFile: %v", err)
		}
		assertFileContent(t, path, "test")
	})

	t.Run("overwrites existing file", func(t *testing.T) {
		dest := t.TempDir()
		path := filepath.Join(dest, "file.txt")
		if err := writeFile(path, []byte("first"), 0o644); err != nil {
			t.Fatalf("writeFile first: %v", err)
		}
		if err := writeFile(path, []byte("second"), 0o644); err != nil {
			t.Fatalf("writeFile second: %v", err)
		}
		assertFileContent(t, path, "second")
	})
}

// ---------------------------------------------------------------------------
// newByteReader
// ---------------------------------------------------------------------------

func TestNewByteReader(t *testing.T) {
	data := []byte("hello world")
	r := newByteReader(data)
	if r.Len() != len(data) {
		t.Errorf("reader length = %d, want %d", r.Len(), len(data))
	}
}

// ===========================================================================
// Test helpers
// ===========================================================================

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != want {
		t.Errorf("file %s content = %q, want %q", path, string(got), want)
	}
}

func createTestZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		fw, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

func createTestZipWithDirs(t *testing.T, dirs []string, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, dir := range dirs {
		hdr := &zip.FileHeader{Name: dir}
		hdr.SetMode(os.ModeDir | 0o755)
		if _, err := zw.CreateHeader(hdr); err != nil {
			t.Fatalf("zip create dir %s: %v", dir, err)
		}
	}
	for name, content := range files {
		fw, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

func createTestTar(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("tar write header %s: %v", name, err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("tar write %s: %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	return buf.Bytes()
}

func createTestTarWithDirEntries(t *testing.T, dirs []string, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, dir := range dirs {
		hdr := &tar.Header{
			Name:     dir,
			Typeflag: tar.TypeDir,
			Mode:     0o755,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("tar write dir header %s: %v", dir, err)
		}
	}
	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(content)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("tar write header %s: %v", name, err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("tar write %s: %v", name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	return buf.Bytes()
}

func createTestTarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	tarData := createTestTar(t, files)
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(tarData); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

// createTestDeb builds a minimal .deb (ar archive) containing a data.tar.gz
// member with the specified files.
func createTestDeb(t *testing.T, files map[string]string) []byte {
	t.Helper()
	dataTarGz := createTestTarGz(t, files)

	var buf bytes.Buffer
	// ar magic
	buf.WriteString("!<arch>\n")
	// control.tar.gz (dummy, skipped by extractor)
	writeArEntry(&buf, "control.tar.gz", []byte("fake-control"))
	// data.tar.gz (the real payload)
	writeArEntry(&buf, "data.tar.gz", dataTarGz)

	return buf.Bytes()
}

// writeArEntry appends a single ar entry to buf.
func writeArEntry(buf *bytes.Buffer, name string, data []byte) {
	// ar header: 16 name + 12 mtime + 6 uid + 6 gid + 8 mode + 10 size + 2 magic = 60 bytes.
	hdr := fmt.Sprintf("%-16s%-12d%-6d%-6d%-8s%-10d`\n", name, 0, 0, 0, "100644", len(data))
	buf.WriteString(hdr)
	buf.Write(data)
	// Pad to even boundary.
	if len(data)%2 != 0 {
		buf.WriteByte('\n')
	}
}

// createTestRPM builds a minimal RPM containing a gzip-compressed cpio (newc)
// archive with the specified files.
func createTestRPM(t *testing.T, files map[string]string) []byte {
	t.Helper()
	cpioData := createTestCPIO(t, files)

	// gzip the cpio
	var gzBuf bytes.Buffer
	gw := gzip.NewWriter(&gzBuf)
	if _, err := gw.Write(cpioData); err != nil {
		t.Fatalf("gzip cpio: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}

	var buf bytes.Buffer

	// RPM lead: 96 bytes, magic 0xEDABEEDB at offset 0.
	lead := make([]byte, 96)
	binary.BigEndian.PutUint32(lead[0:4], 0xEDABEEDB)
	buf.Write(lead)

	// Signature header (minimal: magic + 0 entries + 0 data store).
	writeRPMHeader(&buf, 0, 0)

	// Align to 8-byte boundary.
	for buf.Len()%8 != 0 {
		buf.WriteByte(0)
	}

	// Main header (minimal).
	writeRPMHeader(&buf, 0, 0)

	// Payload (gzip-compressed cpio).
	buf.Write(gzBuf.Bytes())

	return buf.Bytes()
}

func writeRPMHeader(buf *bytes.Buffer, nindex, hsize uint32) {
	// Magic: 8E AD E8 + version 01 + 4 reserved.
	buf.Write([]byte{0x8E, 0xAD, 0xE8, 0x01, 0x00, 0x00, 0x00, 0x00})
	// nindex (4 bytes) + hsize (4 bytes).
	b := make([]byte, 8)
	binary.BigEndian.PutUint32(b[0:4], nindex)
	binary.BigEndian.PutUint32(b[4:8], hsize)
	buf.Write(b)
}

// createTestCPIO creates a cpio newc format archive.
func createTestCPIO(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer

	for name, content := range files {
		writeCPIOEntry(&buf, "./"+name, []byte(content), 0o100644)
	}

	// TRAILER!!!
	writeCPIOEntry(&buf, "TRAILER!!!", nil, 0)

	return buf.Bytes()
}

func writeCPIOEntry(buf *bytes.Buffer, name string, data []byte, mode int) {
	nameBytes := append([]byte(name), 0) // null-terminated
	namesize := len(nameBytes)
	filesize := len(data)

	// newc header: 110 bytes of hex ASCII fields.
	hdr := fmt.Sprintf("070701"+
		"%08X"+ // ino
		"%08X"+ // mode
		"%08X"+ // uid
		"%08X"+ // gid
		"%08X"+ // nlink
		"%08X"+ // mtime
		"%08X"+ // filesize
		"%08X"+ // devmajor
		"%08X"+ // devminor
		"%08X"+ // rdevmajor
		"%08X"+ // rdevminor
		"%08X"+ // namesize
		"%08X", // check
		0, mode, 0, 0, 1, 0, filesize, 0, 0, 0, 0, namesize, 0)

	buf.WriteString(hdr)
	buf.Write(nameBytes)

	// Pad name to 4-byte boundary (header + name).
	total := 110 + namesize
	if total%4 != 0 {
		pad := 4 - (total % 4)
		buf.Write(make([]byte, pad))
	}

	// File data.
	if filesize > 0 {
		buf.Write(data)
		// Pad data to 4-byte boundary.
		if filesize%4 != 0 {
			pad := 4 - (filesize % 4)
			buf.Write(make([]byte, pad))
		}
	}
}
