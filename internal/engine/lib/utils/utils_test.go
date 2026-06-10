package utils

import (
	"bytes"
	"encoding/hex"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNoop(t *testing.T) {
	// Should not panic and should return nothing.
	Noop()
}

func TestE(t *testing.T) {
	t.Run("no error passes through", func(t *testing.T) {
		out := E("a", "b", 42)
		if len(out) != 3 {
			t.Fatalf("expected 3 args returned, got %d", len(out))
		}
		if out[2].(int) != 42 {
			t.Fatalf("expected last arg 42, got %v", out[2])
		}
	})

	t.Run("error triggers Panic", func(t *testing.T) {
		orig := Panic
		var captured any
		Panic = func(v any) { captured = v }
		defer func() { Panic = orig }()

		E("value", errors.New("boom"))
		err, ok := captured.(error)
		if !ok {
			t.Fatalf("expected captured error, got %T", captured)
		}
		if err.Error() != "boom" {
			t.Fatalf("expected boom error, got %v", err)
		}
	})

	t.Run("nil error does not panic", func(t *testing.T) {
		orig := Panic
		called := false
		Panic = func(_ any) { called = true }
		defer func() { Panic = orig }()

		E("only", error(nil))
		if called {
			t.Fatal("Panic should not be called for nil error")
		}
	})
}

func TestS(t *testing.T) {
	tests := []struct {
		name   string
		tpl    string
		params []any
		want   string
	}{
		{name: "no params", tpl: "hello", want: "hello"},
		{name: "single value", tpl: "hi {{.name}}", params: []any{"name", "bob"}, want: "hi bob"},
		{
			name:   "func param",
			tpl:    "{{up .name}}",
			params: []any{"name", "go", "up", strings.ToUpper},
			want:   "GO",
		},
		{
			name:   "two values",
			tpl:    "{{.a}}-{{.b}}",
			params: []any{"a", "x", "b", "y"},
			want:   "x-y",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := S(tt.tpl, tt.params...)
			if got != tt.want {
				t.Fatalf("S() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRandString(t *testing.T) {
	for _, l := range []int{0, 1, 8, 16} {
		s := RandString(l)
		// hex encoding doubles the byte count.
		if len(s) != l*2 {
			t.Fatalf("RandString(%d) length = %d, want %d", l, len(s), l*2)
		}
		if _, err := hex.DecodeString(s); err != nil {
			t.Fatalf("RandString(%d) is not valid hex: %v", l, err)
		}
	}

	// Two calls must differ (probabilistically) for non-trivial length.
	first, second := RandString(16), RandString(16)
	if first == second {
		t.Fatal("two RandString(16) calls returned identical output")
	}
}

func TestMkdir(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "a", "b", "c")
	if err := Mkdir(target); err != nil {
		t.Fatalf("Mkdir returned error: %v", err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("stat after Mkdir: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("Mkdir target is not a directory")
	}
}

func TestAbsolutePaths(t *testing.T) {
	got := AbsolutePaths([]string{"foo.txt", "bar/baz.go"})
	if len(got) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(got))
	}
	for i, p := range got {
		if !filepath.IsAbs(p) {
			t.Fatalf("path %d = %q is not absolute", i, p)
		}
	}

	t.Run("empty slice returns empty", func(t *testing.T) {
		out := AbsolutePaths(nil)
		if len(out) != 0 {
			t.Fatalf("expected empty result, got %v", out)
		}
	})
}

func TestOutputFileAndReadString(t *testing.T) {
	dir := t.TempDir()

	t.Run("string data", func(t *testing.T) {
		p := filepath.Join(dir, "nested", "str.txt")
		if err := OutputFile(p, "hello world"); err != nil {
			t.Fatalf("OutputFile string: %v", err)
		}
		got, err := ReadString(p)
		if err != nil {
			t.Fatalf("ReadString: %v", err)
		}
		if got != "hello world" {
			t.Fatalf("got %q, want %q", got, "hello world")
		}
	})

	t.Run("byte data", func(t *testing.T) {
		p := filepath.Join(dir, "bytes.bin")
		want := []byte{0x01, 0x02, 0xff}
		if err := OutputFile(p, want); err != nil {
			t.Fatalf("OutputFile bytes: %v", err)
		}
		got, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read back: %v", err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("reader data", func(t *testing.T) {
		// OutputFile's io.Reader branch leaves the file handle open (it does not
		// close the file), which on Windows blocks t.TempDir() auto-cleanup. Use a
		// manually-managed dir so this known leak does not fail the suite.
		rdir, err := os.MkdirTemp("", "scout-outputfile-reader-*")
		if err != nil {
			t.Fatalf("mkdir temp: %v", err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(rdir) })

		p := filepath.Join(rdir, "reader.txt")
		if err := OutputFile(p, strings.NewReader("from reader")); err != nil {
			t.Fatalf("OutputFile reader: %v", err)
		}
		got, err := ReadString(p)
		if err != nil {
			t.Fatalf("ReadString: %v", err)
		}
		if got != "from reader" {
			t.Fatalf("got %q, want %q", got, "from reader")
		}
	})

	t.Run("json default", func(t *testing.T) {
		p := filepath.Join(dir, "data.json")
		if err := OutputFile(p, map[string]int{"n": 7}); err != nil {
			t.Fatalf("OutputFile json: %v", err)
		}
		got, err := ReadString(p)
		if err != nil {
			t.Fatalf("ReadString: %v", err)
		}
		if got != `{"n":7}` {
			t.Fatalf("got %q, want %q", got, `{"n":7}`)
		}
	})

	t.Run("ReadString missing file errors", func(t *testing.T) {
		_, err := ReadString(filepath.Join(dir, "does-not-exist"))
		if err == nil {
			t.Fatal("expected error reading missing file")
		}
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected ErrNotExist, got %v", err)
		}
	})
}

func TestMustToJSON(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want string
	}{
		{name: "map", in: map[string]int{"a": 1}, want: `{"a":1}`},
		{name: "slice", in: []int{1, 2, 3}, want: `[1,2,3]`},
		{name: "string with html chars", in: "<b>&", want: `"<b>&"`},
		{name: "number", in: 3.5, want: `3.5`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MustToJSON(tt.in); got != tt.want {
				t.Fatalf("MustToJSON = %q, want %q", got, tt.want)
			}
			gotBytes := MustToJSONBytes(tt.in)
			if string(gotBytes) != tt.want {
				t.Fatalf("MustToJSONBytes = %q, want %q", string(gotBytes), tt.want)
			}
		})
	}
}

func TestFileExists(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "exists.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "existing file", path: file, want: true},
		{name: "missing file", path: filepath.Join(dir, "nope.txt"), want: false},
		{name: "directory is not a file", path: dir, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FileExists(tt.path); got != tt.want {
				t.Fatalf("FileExists(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestFormatCLIArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "no spaces", args: []string{"go", "build", "./..."}, want: "go build ./..."},
		{name: "arg with space quoted", args: []string{"echo", "hello world"}, want: `echo "hello world"`},
		{name: "empty", args: nil, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatCLIArgs(tt.args); got != tt.want {
				t.Fatalf("FormatCLIArgs = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEscapeGoString(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "plain", in: "hello", want: "`hello`"},
		{name: "with backtick", in: "a`b", want: "`a` + \"`\" + `b`"},
		{name: "empty", in: "", want: "``"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EscapeGoString(tt.in); got != tt.want {
				t.Fatalf("EscapeGoString = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDump(t *testing.T) {
	got := Dump(map[string]int{"a": 1})
	if !strings.Contains(got, "\"a\"") {
		t.Fatalf("Dump output missing key: %q", got)
	}
	if !strings.Contains(got, "1") {
		t.Fatalf("Dump output missing value: %q", got)
	}

	t.Run("multiple values joined with space", func(t *testing.T) {
		out := Dump(1, 2)
		if !strings.Contains(out, " ") {
			t.Fatalf("expected space-joined output, got %q", out)
		}
	})
}

func TestMultiLogger(t *testing.T) {
	var a, b []any
	logA := Log(func(msg ...any) { a = msg })
	logB := Log(func(msg ...any) { b = msg })

	ml := MultiLogger(logA, logB)
	ml.Println("x", 1)

	if len(a) != 2 || a[0] != "x" || a[1] != 1 {
		t.Fatalf("logger A did not receive message: %v", a)
	}
	if len(b) != 2 || b[0] != "x" || b[1] != 1 {
		t.Fatalf("logger B did not receive message: %v", b)
	}
}

func TestLoggerQuiet(t *testing.T) {
	// Must not panic when invoked.
	LoggerQuiet.Println("ignored", 1, 2)
}

func decodePNG(t *testing.T, bin []byte) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(bin))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	return img
}

func makePNG(t *testing.T, w, h int, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func makeJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}

func TestCropImage(t *testing.T) {
	t.Run("png crop", func(t *testing.T) {
		src := makePNG(t, 20, 20, color.RGBA{R: 255, A: 255})
		out, err := CropImage(src, 0, 5, 5, 10, 8)
		if err != nil {
			t.Fatalf("CropImage png: %v", err)
		}
		img := decodePNG(t, out)
		if img.Bounds().Dx() != 10 || img.Bounds().Dy() != 8 {
			t.Fatalf("cropped png size = %dx%d, want 10x8", img.Bounds().Dx(), img.Bounds().Dy())
		}
	})

	t.Run("jpeg crop default quality", func(t *testing.T) {
		src := makeJPEG(t, 30, 30)
		out, err := CropImage(src, 0, 0, 0, 12, 6)
		if err != nil {
			t.Fatalf("CropImage jpeg: %v", err)
		}
		img, err := jpeg.Decode(bytes.NewReader(out))
		if err != nil {
			t.Fatalf("decode cropped jpeg: %v", err)
		}
		if img.Bounds().Dx() != 12 || img.Bounds().Dy() != 6 {
			t.Fatalf("cropped jpeg size = %dx%d, want 12x6", img.Bounds().Dx(), img.Bounds().Dy())
		}
	})

	t.Run("invalid image data errors", func(t *testing.T) {
		_, err := CropImage([]byte("not an image"), 0, 0, 0, 1, 1)
		if err == nil {
			t.Fatal("expected error for invalid image bytes")
		}
	})
}
