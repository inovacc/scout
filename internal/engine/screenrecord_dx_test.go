package engine

import (
	"image/gif"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestRecordVideoSaveGIF exercises the Playwright-style one-call recording path:
// Page.RecordVideo -> Stop -> SaveGIF, then verifies the output is a valid GIF.
func TestRecordVideoSaveGIF(t *testing.T) {
	b := newTestBrowser(t)
	srv := dxServer(t)
	p, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("new page: %v", err)
	}
	_ = p.WaitLoad()

	rec, err := p.RecordVideo(WithRecordQuality(60), WithRecordSize(320, 240))
	if err != nil {
		t.Fatalf("record video: %v", err)
	}

	// Navigate (dxServer serves the page for any path) to force repaints so the
	// screencast emits frames, then let them settle.
	_ = p.Navigate(srv.URL + "/again")
	_ = p.WaitLoad()
	time.Sleep(500 * time.Millisecond)

	if err := rec.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}

	if rec.FrameCount() == 0 {
		t.Skip("no screencast frames captured in this environment")
	}

	out := filepath.Join(t.TempDir(), "rec.gif")
	if err := rec.SaveGIF(out); err != nil {
		t.Fatalf("save gif: %v", err)
	}

	f, err := os.Open(out)
	if err != nil {
		t.Fatalf("open gif: %v", err)
	}
	defer f.Close()

	g, err := gif.DecodeAll(f)
	if err != nil {
		t.Fatalf("output is not a valid animated GIF: %v", err)
	}
	if len(g.Image) == 0 {
		t.Fatalf("gif has no frames")
	}
}
