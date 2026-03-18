package monitor

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBaselineManager_CaptureAndLoad(t *testing.T) {
	dir := t.TempDir()
	mgr := NewBaselineManager(dir)

	// Create a simple test PNG.
	screenshot := createTestPNG(100, 100, color.RGBA{R: 255, A: 255})

	// Capture.
	b, err := mgr.Capture("https://example.com", screenshot)
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}

	if b.URL != "https://example.com" {
		t.Errorf("URL = %q", b.URL)
	}

	if b.Width != 100 || b.Height != 100 {
		t.Errorf("size = %dx%d", b.Width, b.Height)
	}

	// Load.
	loaded, err := mgr.Load("https://example.com")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.Checksum != b.Checksum {
		t.Errorf("checksum mismatch")
	}
}

func TestBaselineManager_LoadMissing(t *testing.T) {
	dir := t.TempDir()
	mgr := NewBaselineManager(dir)

	_, err := mgr.Load("https://nonexistent.com")
	if err == nil {
		t.Error("expected error for missing baseline")
	}
}

func TestBaselineManager_List(t *testing.T) {
	dir := t.TempDir()
	mgr := NewBaselineManager(dir)

	_, _ = mgr.Capture("https://a.com", createTestPNG(10, 10, color.RGBA{A: 255}))
	_, _ = mgr.Capture("https://b.com", createTestPNG(10, 10, color.RGBA{G: 255, A: 255}))

	list, err := mgr.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(list) != 2 {
		t.Errorf("list count = %d, want 2", len(list))
	}
}

func TestCompare_Identical(t *testing.T) {
	img := createTestPNG(50, 50, color.RGBA{R: 100, G: 100, B: 100, A: 255})

	score, changed, err := Compare(img, img, 0.01)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	if score != 0 {
		t.Errorf("score = %f, want 0", score)
	}

	if changed {
		t.Error("identical images should not be marked as changed")
	}
}

func TestCompare_Different(t *testing.T) {
	a := createTestPNG(50, 50, color.RGBA{R: 255, A: 255})
	b := createTestPNG(50, 50, color.RGBA{B: 255, A: 255})

	score, changed, err := Compare(a, b, 0.01)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	if score == 0 {
		t.Error("different images should have score > 0")
	}

	if !changed {
		t.Error("different images should be marked as changed")
	}
}

func TestCompare_DifferentSizes(t *testing.T) {
	a := createTestPNG(50, 50, color.RGBA{A: 255})
	b := createTestPNG(100, 100, color.RGBA{A: 255})

	score, changed, err := Compare(a, b, 0.01)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}

	if score != 1.0 {
		t.Errorf("different sizes: score = %f, want 1.0", score)
	}

	if !changed {
		t.Error("different sizes should be changed")
	}
}

func TestMonitor_Check(t *testing.T) {
	dir := t.TempDir()
	baselines := NewBaselineManager(filepath.Join(dir, "baselines"))

	cfg := Config{
		URL:       "https://example.com",
		Interval:  time.Second,
		OutputDir: filepath.Join(dir, "output"),
		Threshold: 0.01,
	}

	var changes []Result

	m := New(cfg, baselines, func(r Result) {
		changes = append(changes, r)
	})

	// First check — captures baseline.
	img := createTestPNG(100, 100, color.RGBA{R: 200, G: 200, B: 200, A: 255})
	result := m.check(func(_ string) ([]byte, error) { return img, nil })

	if result.Error != "" {
		t.Errorf("first check error: %s", result.Error)
	}

	// Second check — same image, no change.
	result = m.check(func(_ string) ([]byte, error) { return img, nil })

	if result.Changed {
		t.Error("same image should not be changed")
	}

	// Third check — different image, should detect change.
	img2 := createTestPNG(100, 100, color.RGBA{R: 0, G: 0, B: 255, A: 255})
	result = m.check(func(_ string) ([]byte, error) { return img2, nil })

	if !result.Changed {
		t.Error("different image should be changed")
	}

	// Output file should exist.
	if result.Screenshot == "" {
		t.Error("screenshot path empty")
	} else if _, err := os.Stat(result.Screenshot); err != nil {
		t.Errorf("screenshot file not found: %v", err)
	}
}

func createTestPNG(w, h int, c color.Color) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))

	for y := range h {
		for x := range w {
			img.Set(x, y, c)
		}
	}

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)

	return buf.Bytes()
}
