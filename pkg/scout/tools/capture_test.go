package tools

import (
	"bytes"
	"context"
	"testing"
)

func TestScreenshot(t *testing.T) {
	_, p := newPageTestBrowser(t)
	srv := newTestServer(t, "<h1>shot</h1>")

	if _, err := Navigate(context.Background(), p, NavigateInput{URL: srv.URL}); err != nil {
		t.Fatal(err)
	}

	out, err := Screenshot(context.Background(), p, ScreenshotInput{})
	if err != nil {
		t.Fatalf("Screenshot: %v", err)
	}

	if len(out.Data) == 0 {
		t.Fatal("screenshot data empty")
	}

	if !bytes.HasPrefix(out.Data, []byte("\x89PNG")) {
		t.Errorf("screenshot not PNG: %x", out.Data[:4])
	}

	full, err := Screenshot(context.Background(), p, ScreenshotInput{FullPage: true})
	if err != nil {
		t.Fatalf("Screenshot full: %v", err)
	}

	if len(full.Data) == 0 {
		t.Error("full screenshot data empty")
	}

	if _, err := Screenshot(context.Background(), nil, ScreenshotInput{}); err == nil {
		t.Error("nil page should error")
	}
}

func TestSnapshot(t *testing.T) {
	_, p := newPageTestBrowser(t)
	srv := newTestServer(t, `<button>click</button>`)

	if _, err := Navigate(context.Background(), p, NavigateInput{URL: srv.URL}); err != nil {
		t.Fatal(err)
	}

	out, err := Snapshot(context.Background(), p, SnapshotInput{})
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	if out.Snapshot == "" {
		t.Error("snapshot empty")
	}

	if _, err := Snapshot(context.Background(), nil, SnapshotInput{}); err == nil {
		t.Error("nil page should error")
	}
}

func TestPDF(t *testing.T) {
	_, p := newPageTestBrowser(t)
	srv := newTestServer(t, "<h1>pdf</h1>")

	if _, err := Navigate(context.Background(), p, NavigateInput{URL: srv.URL}); err != nil {
		t.Fatal(err)
	}

	out, err := PDF(context.Background(), p, PDFInput{})
	if err != nil {
		t.Fatalf("PDF: %v", err)
	}

	if len(out.Data) == 0 {
		t.Fatal("pdf data empty")
	}

	if !bytes.HasPrefix(out.Data, []byte("%PDF")) {
		t.Errorf("not a PDF: %x", out.Data[:4])
	}

	if _, err := PDF(context.Background(), p, PDFInput{Landscape: true, Scale: 1.0}); err != nil {
		t.Errorf("PDF with options: %v", err)
	}

	if _, err := PDF(context.Background(), nil, PDFInput{}); err == nil {
		t.Error("nil page should error")
	}
}
