package scout

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/inovacc/scout/pkg/rod/lib/proto"
)

// ════════════════════════ Screen Record Types ════════════════════════

// screenFrame holds a single captured screencast frame.
type screenFrame struct {
	Data      []byte    // JPEG image data
	Timestamp time.Time // when the frame was received
	Index     int       // sequential frame number
}

// screenRecordOpts holds configuration for the screen recorder.
type screenRecordOpts struct {
	quality   int    // JPEG quality 1-100
	maxWidth  int    // maximum frame width (0 = no limit)
	maxHeight int    // maximum frame height (0 = no limit)
	format    string // image format ("jpeg")
}

func screenRecordDefaults() screenRecordOpts {
	return screenRecordOpts{
		quality: 80,
		format:  "jpeg",
	}
}

// ════════════════════════ Screen Record Options ════════════════════════

// ScreenRecordOption configures a ScreenRecorder.
type ScreenRecordOption func(*screenRecordOpts)

// WithRecordQuality sets the JPEG compression quality (1-100). Default: 80.
func WithRecordQuality(q int) ScreenRecordOption {
	return func(o *screenRecordOpts) {
		if q < 1 {
			q = 1
		}
		if q > 100 {
			q = 100
		}
		o.quality = q
	}
}

// WithRecordSize sets the maximum frame dimensions. Zero means no limit.
func WithRecordSize(w, h int) ScreenRecordOption {
	return func(o *screenRecordOpts) {
		o.maxWidth = w
		o.maxHeight = h
	}
}

// ════════════════════════ ScreenRecorder ════════════════════════

// ScreenRecorder captures browser screen frames via CDP Page.startScreencast.
type ScreenRecorder struct {
	mu        sync.Mutex
	page      *Page
	frames    []screenFrame
	recording bool
	done      chan struct{}
	opts      screenRecordOpts
	startTime time.Time
}

// NewScreenRecorder creates a new screen recorder for the given page.
// Call Start() to begin capturing frames.
func NewScreenRecorder(page *Page, opts ...ScreenRecordOption) *ScreenRecorder {
	if page == nil {
		return nil
	}

	o := screenRecordDefaults()
	for _, fn := range opts {
		fn(&o)
	}

	return &ScreenRecorder{
		page: page,
		opts: o,
		done: make(chan struct{}),
	}
}

// Start begins capturing screencast frames from the page.
func (r *ScreenRecorder) Start() error {
	if r == nil {
		return fmt.Errorf("scout: screenrecord: nil recorder")
	}

	r.mu.Lock()
	if r.recording {
		r.mu.Unlock()
		return fmt.Errorf("scout: screenrecord: already recording")
	}
	r.recording = true
	r.startTime = time.Now()
	r.mu.Unlock()

	rodPage := r.page.RodPage()

	// Start listening for screencast frames before enabling screencast.
	go rodPage.EachEvent(func(e *proto.PageScreencastFrame) {
		r.mu.Lock()
		if !r.recording {
			r.mu.Unlock()
			return
		}
		r.mu.Unlock()

		// Decode the base64 image data.
		// The Data field from CDP is already base64-decoded by rod's proto package
		// into []byte, but some versions send it as a base64 string.
		frameData := e.Data
		if len(frameData) == 0 {
			// Acknowledge even empty frames.
			_ = proto.PageScreencastFrameAck{SessionID: e.SessionID}.Call(rodPage)
			return
		}

		// If the data looks like base64 (no JPEG magic bytes), decode it.
		if len(frameData) > 2 && frameData[0] != 0xFF {
			decoded, err := base64.StdEncoding.DecodeString(string(frameData))
			if err == nil {
				frameData = decoded
			}
		}

		r.mu.Lock()
		frame := screenFrame{
			Data:      frameData,
			Timestamp: time.Now(),
			Index:     len(r.frames),
		}
		r.frames = append(r.frames, frame)
		r.mu.Unlock()

		// Acknowledge the frame to receive the next one.
		_ = proto.PageScreencastFrameAck{SessionID: e.SessionID}.Call(rodPage)
	})()

	// Enable the screencast.
	quality := r.opts.quality
	req := proto.PageStartScreencast{
		Format:  proto.PageStartScreencastFormatJpeg,
		Quality: &quality,
	}
	if r.opts.maxWidth > 0 {
		req.MaxWidth = &r.opts.maxWidth
	}
	if r.opts.maxHeight > 0 {
		req.MaxHeight = &r.opts.maxHeight
	}

	if err := req.Call(rodPage); err != nil {
		r.mu.Lock()
		r.recording = false
		r.mu.Unlock()
		return fmt.Errorf("scout: screenrecord: start: %w", err)
	}

	return nil
}

// Stop ends the screen recording. It is safe to call multiple times.
func (r *ScreenRecorder) Stop() error {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	if !r.recording {
		r.mu.Unlock()
		return nil
	}
	r.recording = false
	r.mu.Unlock()

	if err := (proto.PageStopScreencast{}).Call(r.page.RodPage()); err != nil {
		return fmt.Errorf("scout: screenrecord: stop: %w", err)
	}

	return nil
}

// Frames returns a copy of the captured frames.
func (r *ScreenRecorder) Frames() []screenFrame {
	if r == nil {
		return nil
	}

	r.mu.Lock()
	frames := make([]screenFrame, len(r.frames))
	copy(frames, r.frames)
	r.mu.Unlock()

	return frames
}

// FrameCount returns the number of captured frames.
func (r *ScreenRecorder) FrameCount() int {
	if r == nil {
		return 0
	}

	r.mu.Lock()
	n := len(r.frames)
	r.mu.Unlock()

	return n
}

// Duration returns the time span from the first to the last captured frame.
func (r *ScreenRecorder) Duration() time.Duration {
	if r == nil {
		return 0
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.frames) < 2 {
		return 0
	}

	return r.frames[len(r.frames)-1].Timestamp.Sub(r.frames[0].Timestamp)
}

// ExportGIF encodes all captured frames as an animated GIF.
func (r *ScreenRecorder) ExportGIF(w io.Writer) error {
	if r == nil {
		return fmt.Errorf("scout: screenrecord: nil recorder")
	}

	r.mu.Lock()
	frames := make([]screenFrame, len(r.frames))
	copy(frames, r.frames)
	r.mu.Unlock()

	if len(frames) == 0 {
		return fmt.Errorf("scout: screenrecord: no frames to export")
	}

	g := &gif.GIF{}

	for i, frame := range frames {
		img, err := jpeg.Decode(bytes.NewReader(frame.Data))
		if err != nil {
			return fmt.Errorf("scout: screenrecord: decode frame %d: %w", i, err)
		}

		bounds := img.Bounds()
		palettedImg := image.NewPaletted(bounds, palette.Plan9)
		draw.Draw(palettedImg, bounds, img, bounds.Min, draw.Src)

		// Calculate delay in centiseconds between frames.
		delay := 10 // default 100ms
		if i < len(frames)-1 {
			dt := frames[i+1].Timestamp.Sub(frame.Timestamp)
			delay = int(dt.Milliseconds() / 10)
			if delay < 1 {
				delay = 1
			}
		}

		g.Image = append(g.Image, palettedImg)
		g.Delay = append(g.Delay, delay)
	}

	if err := gif.EncodeAll(w, g); err != nil {
		return fmt.Errorf("scout: screenrecord: encode gif: %w", err)
	}

	return nil
}

// ExportFrames saves each captured frame as an individual JPEG file in the given directory.
func (r *ScreenRecorder) ExportFrames(dir string) error {
	if r == nil {
		return fmt.Errorf("scout: screenrecord: nil recorder")
	}

	r.mu.Lock()
	frames := make([]screenFrame, len(r.frames))
	copy(frames, r.frames)
	r.mu.Unlock()

	if len(frames) == 0 {
		return fmt.Errorf("scout: screenrecord: no frames to export")
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("scout: screenrecord: create dir: %w", err)
	}

	for i, frame := range frames {
		name := filepath.Join(dir, fmt.Sprintf("frame_%04d.jpg", i))
		if err := os.WriteFile(name, frame.Data, 0o644); err != nil {
			return fmt.Errorf("scout: screenrecord: write frame %d: %w", i, err)
		}
	}

	return nil
}
