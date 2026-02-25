package scout

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScreenRecorder_NilPage(t *testing.T) {
	r := NewScreenRecorder(nil)
	if r != nil {
		t.Fatal("expected nil recorder for nil page")
	}

	// All methods should be nil-safe.
	if err := r.Start(); err == nil {
		t.Error("expected error from nil Start")
	}
	if err := r.Stop(); err != nil {
		t.Errorf("expected nil error from nil Stop, got %v", err)
	}
	if r.Frames() != nil {
		t.Error("expected nil frames")
	}
	if r.FrameCount() != 0 {
		t.Error("expected 0 frame count")
	}
	if r.Duration() != 0 {
		t.Error("expected 0 duration")
	}
	if err := r.ExportGIF(&bytes.Buffer{}); err == nil {
		t.Error("expected error from nil ExportGIF")
	}
	if err := r.ExportFrames(t.TempDir()); err == nil {
		t.Error("expected error from nil ExportFrames")
	}
}

func TestScreenRecorder_Options(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	b := newTestBrowser(t)
	page, err := b.NewPage(ts.URL)
	if err != nil {
		t.Fatalf("new page: %v", err)
	}

	r := NewScreenRecorder(page,
		WithRecordQuality(50),
		WithRecordSize(640, 480),
	)
	if r == nil {
		t.Fatal("expected non-nil recorder")
	}
	if r.opts.quality != 50 {
		t.Errorf("quality = %d, want 50", r.opts.quality)
	}
	if r.opts.maxWidth != 640 {
		t.Errorf("maxWidth = %d, want 640", r.opts.maxWidth)
	}
	if r.opts.maxHeight != 480 {
		t.Errorf("maxHeight = %d, want 480", r.opts.maxHeight)
	}
}

func TestScreenRecorder_OptionsClamp(t *testing.T) {
	var o screenRecordOpts
	WithRecordQuality(0)(&o)
	if o.quality != 1 {
		t.Errorf("quality clamped to %d, want 1", o.quality)
	}
	WithRecordQuality(200)(&o)
	if o.quality != 100 {
		t.Errorf("quality clamped to %d, want 100", o.quality)
	}
}

func TestScreenRecorder_StartStop(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	b := newTestBrowser(t)
	page, err := b.NewPage(ts.URL)
	if err != nil {
		t.Fatalf("new page: %v", err)
	}

	r := NewScreenRecorder(page)
	if err := r.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Let it record a few frames.
	time.Sleep(500 * time.Millisecond)

	if err := r.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}

	t.Logf("captured %d frames", r.FrameCount())
}

func TestScreenRecorder_DoubleStop(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	b := newTestBrowser(t)
	page, err := b.NewPage(ts.URL)
	if err != nil {
		t.Fatalf("new page: %v", err)
	}

	r := NewScreenRecorder(page)
	if err := r.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	if err := r.Stop(); err != nil {
		t.Fatalf("first stop: %v", err)
	}
	if err := r.Stop(); err != nil {
		t.Fatalf("second stop should be idempotent: %v", err)
	}
}

func TestScreenRecorder_ExportGIF(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	b := newTestBrowser(t)
	page, err := b.NewPage(ts.URL)
	if err != nil {
		t.Fatalf("new page: %v", err)
	}

	r := NewScreenRecorder(page, WithRecordQuality(30), WithRecordSize(320, 240))
	if err := r.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Navigate to generate different frames.
	_ = page.WaitLoad()
	_ = page.Navigate(ts.URL + "/page2")
	_ = page.WaitLoad()
	time.Sleep(500 * time.Millisecond)

	if err := r.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}

	if r.FrameCount() == 0 {
		t.Skip("no frames captured (headless screencast may not produce frames on this platform)")
	}

	var buf bytes.Buffer
	if err := r.ExportGIF(&buf); err != nil {
		t.Fatalf("export gif: %v", err)
	}

	if buf.Len() == 0 {
		t.Error("expected non-empty GIF output")
	}

	// GIF magic bytes.
	if !bytes.HasPrefix(buf.Bytes(), []byte("GIF")) {
		t.Error("output does not start with GIF magic bytes")
	}

	t.Logf("GIF size: %d bytes, frames: %d", buf.Len(), r.FrameCount())
}

func TestScreenRecorder_ExportFrames(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	b := newTestBrowser(t)
	page, err := b.NewPage(ts.URL)
	if err != nil {
		t.Fatalf("new page: %v", err)
	}

	r := NewScreenRecorder(page, WithRecordSize(320, 240))
	if err := r.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	_ = page.WaitLoad()
	time.Sleep(500 * time.Millisecond)

	if err := r.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}

	if r.FrameCount() == 0 {
		t.Skip("no frames captured (headless screencast may not produce frames on this platform)")
	}

	dir := filepath.Join(t.TempDir(), "frames")
	if err := r.ExportFrames(dir); err != nil {
		t.Fatalf("export frames: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}

	if len(entries) != r.FrameCount() {
		t.Errorf("file count = %d, want %d", len(entries), r.FrameCount())
	}

	t.Logf("exported %d frame files", len(entries))
}

func TestScreenRecorder_Duration(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	b := newTestBrowser(t)
	page, err := b.NewPage(ts.URL)
	if err != nil {
		t.Fatalf("new page: %v", err)
	}

	r := NewScreenRecorder(page)
	if err := r.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	_ = page.WaitLoad()
	time.Sleep(600 * time.Millisecond)

	if err := r.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}

	if r.FrameCount() < 2 {
		t.Skip("not enough frames for duration test")
	}

	dur := r.Duration()
	if dur <= 0 {
		t.Errorf("duration = %v, want > 0", dur)
	}

	t.Logf("duration: %v, frames: %d", dur, r.FrameCount())
}
