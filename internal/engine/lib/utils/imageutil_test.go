package utils

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/inovacc/scout/internal/engine/lib/proto"
)

func TestNewImgProcessor(t *testing.T) {
	tests := []struct {
		name    string
		format  proto.PageCaptureScreenshotFormat
		wantErr bool
		isJpeg  bool
	}{
		{name: "jpeg", format: proto.PageCaptureScreenshotFormatJpeg, isJpeg: true},
		{name: "png", format: proto.PageCaptureScreenshotFormatPng},
		{name: "empty defaults to png", format: ""},
		{name: "webp maps to png", format: proto.PageCaptureScreenshotFormatWebp},
		{name: "unknown format errors", format: "gif", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewImgProcessor(tt.format)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for format %q", tt.format)
				}
				if p != nil {
					t.Fatalf("expected nil processor on error, got %T", p)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			switch tt.isJpeg {
			case true:
				if _, ok := p.(*jpegProcessor); !ok {
					t.Fatalf("expected *jpegProcessor, got %T", p)
				}
			default:
				if _, ok := p.(*pngProcessor); !ok {
					t.Fatalf("expected *pngProcessor, got %T", p)
				}
			}
		})
	}
}

func TestProcessorRoundTrip(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{R: 10, G: 20, B: 30, A: 255})
		}
	}

	t.Run("png encode then decode", func(t *testing.T) {
		p := pngProcessor{}
		bin, err := p.Encode(img, nil)
		if err != nil {
			t.Fatalf("png encode: %v", err)
		}
		out, err := p.Decode(bytes.NewReader(bin))
		if err != nil {
			t.Fatalf("png decode: %v", err)
		}
		if out.Bounds().Dx() != 4 || out.Bounds().Dy() != 4 {
			t.Fatalf("png size = %v", out.Bounds())
		}
	})

	t.Run("jpeg encode with opt then decode", func(t *testing.T) {
		p := jpegProcessor{}
		bin, err := p.Encode(img, &ImgOption{Quality: 75})
		if err != nil {
			t.Fatalf("jpeg encode: %v", err)
		}
		out, err := p.Decode(bytes.NewReader(bin))
		if err != nil {
			t.Fatalf("jpeg decode: %v", err)
		}
		if out.Bounds().Dx() != 4 {
			t.Fatalf("jpeg width = %d, want 4", out.Bounds().Dx())
		}
	})

	t.Run("jpeg encode with nil opt", func(t *testing.T) {
		p := jpegProcessor{}
		bin, err := p.Encode(img, nil)
		if err != nil {
			t.Fatalf("jpeg encode nil opt: %v", err)
		}
		if len(bin) == 0 {
			t.Fatal("expected non-empty jpeg output")
		}
	})
}

func makeColorPNG(t *testing.T, w, h int, c color.Color) []byte {
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

func TestSplicePngVertical(t *testing.T) {
	t.Run("empty returns nil nil", func(t *testing.T) {
		out, err := SplicePngVertical(nil, proto.PageCaptureScreenshotFormatPng, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out != nil {
			t.Fatalf("expected nil output, got %d bytes", len(out))
		}
	})

	t.Run("single returns image directly", func(t *testing.T) {
		src := makeColorPNG(t, 5, 5, color.RGBA{R: 1, A: 255})
		out, err := SplicePngVertical(
			[]ImgWithBox{{Img: src}},
			proto.PageCaptureScreenshotFormatPng,
			nil,
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !bytes.Equal(out, src) {
			t.Fatal("single image should be returned unchanged")
		}
	})

	t.Run("two images stacked vertically", func(t *testing.T) {
		top := makeColorPNG(t, 6, 4, color.RGBA{R: 255, A: 255})
		bottom := makeColorPNG(t, 6, 3, color.RGBA{B: 255, A: 255})
		out, err := SplicePngVertical(
			[]ImgWithBox{{Img: top}, {Img: bottom}},
			proto.PageCaptureScreenshotFormatPng,
			nil,
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		img, err := png.Decode(bytes.NewReader(out))
		if err != nil {
			t.Fatalf("decode spliced: %v", err)
		}
		if img.Bounds().Dx() != 6 || img.Bounds().Dy() != 7 {
			t.Fatalf("spliced size = %dx%d, want 6x7", img.Bounds().Dx(), img.Bounds().Dy())
		}
	})

	t.Run("respects box dimensions", func(t *testing.T) {
		a := makeColorPNG(t, 10, 10, color.RGBA{G: 255, A: 255})
		b := makeColorPNG(t, 10, 10, color.RGBA{R: 255, A: 255})
		boxA := image.Rect(0, 0, 5, 4)
		boxB := image.Rect(0, 0, 5, 6)
		out, err := SplicePngVertical(
			[]ImgWithBox{{Img: a, Box: &boxA}, {Img: b, Box: &boxB}},
			proto.PageCaptureScreenshotFormatPng,
			nil,
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		img, err := png.Decode(bytes.NewReader(out))
		if err != nil {
			t.Fatalf("decode spliced: %v", err)
		}
		if img.Bounds().Dx() != 5 || img.Bounds().Dy() != 10 {
			t.Fatalf("spliced size = %dx%d, want 5x10", img.Bounds().Dx(), img.Bounds().Dy())
		}
	})

	t.Run("invalid format errors", func(t *testing.T) {
		a := makeColorPNG(t, 2, 2, color.Black)
		b := makeColorPNG(t, 2, 2, color.White)
		_, err := SplicePngVertical(
			[]ImgWithBox{{Img: a}, {Img: b}},
			"gif",
			nil,
		)
		if err == nil {
			t.Fatal("expected error for unsupported format")
		}
	})

	t.Run("undecodable image errors", func(t *testing.T) {
		_, err := SplicePngVertical(
			[]ImgWithBox{{Img: []byte("garbage")}, {Img: []byte("garbage2")}},
			proto.PageCaptureScreenshotFormatPng,
			nil,
		)
		if err == nil {
			t.Fatal("expected error decoding garbage image")
		}
	})

	t.Run("jpeg output with quality opt", func(t *testing.T) {
		mkJPEG := func(w, h int) []byte {
			img := image.NewRGBA(image.Rect(0, 0, w, h))
			var buf bytes.Buffer
			if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
				t.Fatalf("encode jpeg: %v", err)
			}
			return buf.Bytes()
		}
		out, err := SplicePngVertical(
			[]ImgWithBox{{Img: mkJPEG(4, 4)}, {Img: mkJPEG(4, 2)}},
			proto.PageCaptureScreenshotFormatJpeg,
			&ImgOption{Quality: 60},
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		img, err := jpeg.Decode(bytes.NewReader(out))
		if err != nil {
			t.Fatalf("decode spliced jpeg: %v", err)
		}
		if img.Bounds().Dy() != 6 {
			t.Fatalf("spliced jpeg height = %d, want 6", img.Bounds().Dy())
		}
	})
}
