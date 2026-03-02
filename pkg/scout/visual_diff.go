package scout

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
)

// VisualDiffResult holds the result of comparing two screenshots.
type VisualDiffResult struct {
	// DiffPixels is the number of pixels that differ beyond the color threshold.
	DiffPixels int
	// TotalPixels is the total number of pixels compared.
	TotalPixels int
	// DiffPercent is the percentage of differing pixels (0-100).
	DiffPercent float64
	// Match is true when the difference is within the given threshold.
	Match bool
	// DiffImage is an optional PNG showing differences highlighted in red.
	// Only populated when generateDiff is true.
	DiffImage []byte
}

// VisualDiffOption configures a visual diff comparison.
type VisualDiffOption func(*visualDiffConfig)

type visualDiffConfig struct {
	// threshold is the maximum allowed DiffPercent (0-100). Default: 0 (exact match).
	threshold float64
	// colorThreshold is the per-channel tolerance (0-255). Default: 0.
	colorThreshold int
	// generateDiff enables generation of the diff image.
	generateDiff bool
}

// WithDiffThreshold sets the maximum allowed percentage of differing pixels (0-100).
// A threshold of 1.0 means up to 1% of pixels may differ and still match.
func WithDiffThreshold(percent float64) VisualDiffOption {
	return func(c *visualDiffConfig) { c.threshold = percent }
}

// WithColorThreshold sets the per-channel tolerance (0-255) for pixel comparison.
// A value of 5 means channels differing by 5 or less are considered equal.
func WithColorThreshold(tolerance int) VisualDiffOption {
	return func(c *visualDiffConfig) { c.colorThreshold = tolerance }
}

// WithDiffImage enables generation of a visual diff overlay PNG.
// Unchanged pixels are dimmed; differing pixels are highlighted in red.
func WithDiffImage() VisualDiffOption {
	return func(c *visualDiffConfig) { c.generateDiff = true }
}

// VisualDiff compares two PNG screenshots and returns the difference metrics.
// Both inputs must be valid PNG-encoded byte slices.
func VisualDiff(baseline, current []byte, opts ...VisualDiffOption) (*VisualDiffResult, error) {
	cfg := &visualDiffConfig{}
	for _, o := range opts {
		o(cfg)
	}

	img1, err := png.Decode(bytes.NewReader(baseline))
	if err != nil {
		return nil, fmt.Errorf("scout: visual diff: decode baseline: %w", err)
	}

	img2, err := png.Decode(bytes.NewReader(current))
	if err != nil {
		return nil, fmt.Errorf("scout: visual diff: decode current: %w", err)
	}

	b1 := img1.Bounds()
	b2 := img2.Bounds()

	// Use the intersection of both images for comparison.
	maxX := min(b2.Max.X, b1.Max.X)

	maxY := min(b2.Max.Y, b1.Max.Y)

	// Count pixels outside the intersection as diffs.
	area1 := b1.Dx() * b1.Dy()
	area2 := b2.Dx() * b2.Dy()

	totalPixels := max(area2, area1)

	intersectArea := maxX * maxY
	outsideDiffs := totalPixels - intersectArea

	var diffImg *image.RGBA
	if cfg.generateDiff {
		diffImg = image.NewRGBA(image.Rect(0, 0, maxX, maxY))
	}

	threshold := cfg.colorThreshold
	diffCount := outsideDiffs

	for y := range maxY {
		for x := range maxX {
			r1, g1, b1a, a1 := img1.At(x, y).RGBA()
			r2, g2, b2a, a2 := img2.At(x, y).RGBA()

			isDiff := channelDiff(r1, r2) > threshold ||
				channelDiff(g1, g2) > threshold ||
				channelDiff(b1a, b2a) > threshold ||
				channelDiff(a1, a2) > threshold

			if isDiff {
				diffCount++

				if diffImg != nil {
					diffImg.Set(x, y, color.RGBA{R: 255, A: 255})
				}
			} else if diffImg != nil {
				// Dim unchanged pixels.
				r, g, b, _ := img1.At(x, y).RGBA()
				diffImg.Set(x, y, color.RGBA{
					R: uint8(r >> 9), // ~50% brightness
					G: uint8(g >> 9),
					B: uint8(b >> 9),
					A: 255,
				})
			}
		}
	}

	diffPercent := 0.0
	if totalPixels > 0 {
		diffPercent = float64(diffCount) / float64(totalPixels) * 100.0
	}

	result := &VisualDiffResult{
		DiffPixels:  diffCount,
		TotalPixels: totalPixels,
		DiffPercent: math.Round(diffPercent*100) / 100, // 2 decimal places
		Match:       diffPercent <= cfg.threshold,
	}

	if diffImg != nil {
		var buf bytes.Buffer
		if err := png.Encode(&buf, diffImg); err != nil {
			return nil, fmt.Errorf("scout: visual diff: encode diff image: %w", err)
		}

		result.DiffImage = buf.Bytes()
	}

	return result, nil
}

// CompareScreenshots is a convenience method on Page that takes a baseline screenshot
// and compares it with a fresh screenshot of the current page state.
func (p *Page) CompareScreenshots(baseline []byte, opts ...VisualDiffOption) (*VisualDiffResult, error) {
	current, err := p.Screenshot()
	if err != nil {
		return nil, err
	}

	return VisualDiff(baseline, current, opts...)
}

// channelDiff returns the absolute difference between two 16-bit color channels
// scaled to 0-255.
func channelDiff(a, b uint32) int {
	a8 := int(a >> 8)
	b8 := int(b >> 8)

	d := a8 - b8
	if d < 0 {
		return -d
	}

	return d
}

// drawRect draws a rectangle outline on the diff image (unused but available for future region highlighting).
func drawRect(img *image.RGBA, r image.Rectangle, c color.RGBA) { //nolint:unused
	draw.Draw(img, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+1), &image.Uniform{c}, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(r.Min.X, r.Max.Y-1, r.Max.X, r.Max.Y), &image.Uniform{c}, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(r.Min.X, r.Min.Y, r.Min.X+1, r.Max.Y), &image.Uniform{c}, image.Point{}, draw.Src)
	draw.Draw(img, image.Rect(r.Max.X-1, r.Min.Y, r.Max.X, r.Max.Y), &image.Uniform{c}, image.Point{}, draw.Src)
}
