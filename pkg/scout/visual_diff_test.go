package scout

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

// makePNG creates a solid-color PNG of the given size.
func makePNG(w, h int, c color.Color) []byte {
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

// makePNGWithBlock creates a PNG with a colored block at the given position.
func makePNGWithBlock(w, h int, bg, block color.Color, bx, by, bw, bh int) []byte { //nolint:unparam
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, bg)
		}
	}

	for y := by; y < by+bh && y < h; y++ {
		for x := bx; x < bx+bw && x < w; x++ {
			img.Set(x, y, block)
		}
	}

	var buf bytes.Buffer

	_ = png.Encode(&buf, img)

	return buf.Bytes()
}

func TestVisualDiff_IdenticalImages(t *testing.T) {
	img := makePNG(100, 100, color.White)

	result, err := VisualDiff(img, img)
	if err != nil {
		t.Fatalf("VisualDiff: %v", err)
	}

	if result.DiffPixels != 0 {
		t.Errorf("DiffPixels = %d, want 0", result.DiffPixels)
	}

	if result.DiffPercent != 0 {
		t.Errorf("DiffPercent = %f, want 0", result.DiffPercent)
	}

	if !result.Match {
		t.Error("expected Match=true for identical images")
	}
}

func TestVisualDiff_CompletelyDifferent(t *testing.T) {
	white := makePNG(100, 100, color.White)
	black := makePNG(100, 100, color.Black)

	result, err := VisualDiff(white, black)
	if err != nil {
		t.Fatalf("VisualDiff: %v", err)
	}

	if result.DiffPixels != 10000 {
		t.Errorf("DiffPixels = %d, want 10000", result.DiffPixels)
	}

	if result.DiffPercent != 100.0 {
		t.Errorf("DiffPercent = %f, want 100", result.DiffPercent)
	}

	if result.Match {
		t.Error("expected Match=false")
	}
}

func TestVisualDiff_PartialDifference(t *testing.T) {
	// 100x100 white image with a 10x10 red block = 1% different.
	baseline := makePNG(100, 100, color.White)
	current := makePNGWithBlock(100, 100, color.White, color.RGBA{R: 255, A: 255}, 0, 0, 10, 10)

	result, err := VisualDiff(baseline, current)
	if err != nil {
		t.Fatalf("VisualDiff: %v", err)
	}

	if result.DiffPixels != 100 {
		t.Errorf("DiffPixels = %d, want 100", result.DiffPixels)
	}

	if result.DiffPercent != 1.0 {
		t.Errorf("DiffPercent = %f, want 1.0", result.DiffPercent)
	}
}

func TestVisualDiff_ThresholdMatch(t *testing.T) {
	baseline := makePNG(100, 100, color.White)
	current := makePNGWithBlock(100, 100, color.White, color.RGBA{R: 255, A: 255}, 0, 0, 10, 10)

	// 1% diff, threshold 2% → should match.
	result, err := VisualDiff(baseline, current, WithDiffThreshold(2.0))
	if err != nil {
		t.Fatalf("VisualDiff: %v", err)
	}

	if !result.Match {
		t.Error("expected Match=true with 2% threshold and 1% diff")
	}
}

func TestVisualDiff_ThresholdNoMatch(t *testing.T) {
	baseline := makePNG(100, 100, color.White)
	current := makePNGWithBlock(100, 100, color.White, color.RGBA{R: 255, A: 255}, 0, 0, 10, 10)

	// 1% diff, threshold 0.5% → should not match.
	result, err := VisualDiff(baseline, current, WithDiffThreshold(0.5))
	if err != nil {
		t.Fatalf("VisualDiff: %v", err)
	}

	if result.Match {
		t.Error("expected Match=false with 0.5% threshold and 1% diff")
	}
}

func TestVisualDiff_ColorThreshold(t *testing.T) {
	// White (255,255,255) vs near-white (250,250,250) — diff of 5 per channel.
	baseline := makePNG(10, 10, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	current := makePNG(10, 10, color.RGBA{R: 250, G: 250, B: 250, A: 255})

	// Without color threshold: all pixels differ.
	r1, _ := VisualDiff(baseline, current)
	if r1.DiffPixels != 100 {
		t.Errorf("without color threshold: DiffPixels = %d, want 100", r1.DiffPixels)
	}

	// With color threshold of 5: all pixels match.
	r2, _ := VisualDiff(baseline, current, WithColorThreshold(5))
	if r2.DiffPixels != 0 {
		t.Errorf("with color threshold 5: DiffPixels = %d, want 0", r2.DiffPixels)
	}
}

func TestVisualDiff_DifferentSizes(t *testing.T) {
	small := makePNG(50, 50, color.White)
	large := makePNG(100, 100, color.White)

	result, err := VisualDiff(small, large)
	if err != nil {
		t.Fatalf("VisualDiff: %v", err)
	}

	// 50x50=2500 intersection, 100x100=10000 total → 7500 outside diffs.
	if result.TotalPixels != 10000 {
		t.Errorf("TotalPixels = %d, want 10000", result.TotalPixels)
	}

	if result.DiffPixels != 7500 {
		t.Errorf("DiffPixels = %d, want 7500", result.DiffPixels)
	}
}

func TestVisualDiff_GenerateDiffImage(t *testing.T) {
	baseline := makePNG(50, 50, color.White)
	current := makePNGWithBlock(50, 50, color.White, color.Black, 10, 10, 10, 10)

	result, err := VisualDiff(baseline, current, WithDiffImage())
	if err != nil {
		t.Fatalf("VisualDiff: %v", err)
	}

	if len(result.DiffImage) == 0 {
		t.Fatal("expected non-empty DiffImage")
	}

	// Verify the diff image is valid PNG.
	diffImg, err := png.Decode(bytes.NewReader(result.DiffImage))
	if err != nil {
		t.Fatalf("decode diff image: %v", err)
	}

	// Check that a pixel in the changed region is red.
	r, g, b, a := diffImg.At(15, 15).RGBA()
	if r>>8 != 255 || g>>8 != 0 || b>>8 != 0 || a>>8 != 255 {
		t.Errorf("diff pixel at (15,15) = (%d,%d,%d,%d), want red", r>>8, g>>8, b>>8, a>>8)
	}

	// Unchanged pixel should be dimmed (not bright white).
	r2, _, _, _ := diffImg.At(0, 0).RGBA() //nolint:dogsled
	if r2>>8 > 200 {
		t.Errorf("unchanged pixel brightness %d, expected dimmed", r2>>8)
	}
}

func TestVisualDiff_InvalidPNG(t *testing.T) {
	valid := makePNG(10, 10, color.White)

	_, err := VisualDiff([]byte("not a png"), valid)
	if err == nil {
		t.Error("expected error for invalid baseline")
	}

	_, err = VisualDiff(valid, []byte("not a png"))
	if err == nil {
		t.Error("expected error for invalid current")
	}
}

func TestChannelDiff(t *testing.T) {
	tests := []struct {
		a, b uint32
		want int
	}{
		{0xFFFF, 0xFFFF, 0},
		{0xFFFF, 0x0000, 255},
		{0x8080, 0x8080, 0},
		{0x0000, 0x0A0A, 10},
	}

	for _, tt := range tests {
		got := channelDiff(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("channelDiff(%x, %x) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}
