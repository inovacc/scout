package guide

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestRecorderLifecycle(t *testing.T) {
	r := NewRecorder()

	if r.IsRecording() {
		t.Fatal("expected not recording initially")
	}

	// Start recording.
	if err := r.Start("Test Guide", "https://example.com"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if !r.IsRecording() {
		t.Fatal("expected recording after Start")
	}

	// Add steps.
	fakeScreenshot := []byte("fake-png-data")

	if err := r.AddStep("https://example.com", "Example", "First step", fakeScreenshot); err != nil {
		t.Fatalf("AddStep: %v", err)
	}

	if err := r.AddStep("https://example.com/page2", "Page 2", "Second step", nil); err != nil {
		t.Fatalf("AddStep: %v", err)
	}

	// Finish.
	g, err := r.Finish()
	if err != nil {
		t.Fatalf("Finish: %v", err)
	}

	if r.IsRecording() {
		t.Fatal("expected not recording after Finish")
	}

	if g.Title != "Test Guide" {
		t.Errorf("title = %q, want %q", g.Title, "Test Guide")
	}

	if g.URL != "https://example.com" {
		t.Errorf("url = %q, want %q", g.URL, "https://example.com")
	}

	if len(g.Steps) != 2 {
		t.Fatalf("steps = %d, want 2", len(g.Steps))
	}

	if g.Steps[0].Number != 1 {
		t.Errorf("step 0 number = %d, want 1", g.Steps[0].Number)
	}

	if g.Steps[1].Number != 2 {
		t.Errorf("step 1 number = %d, want 2", g.Steps[1].Number)
	}

	if g.Steps[0].Annotation != "First step" {
		t.Errorf("step 0 annotation = %q, want %q", g.Steps[0].Annotation, "First step")
	}
}

func TestRecorderDoubleStart(t *testing.T) {
	r := NewRecorder()

	if err := r.Start("Guide 1", "https://example.com"); err != nil {
		t.Fatalf("Start: %v", err)
	}

	err := r.Start("Guide 2", "https://example.com")
	if err == nil {
		t.Fatal("expected error on double Start")
	}

	if !strings.Contains(err.Error(), "already recording") {
		t.Errorf("error = %q, want 'already recording'", err.Error())
	}
}

func TestRecorderFinishWithoutStart(t *testing.T) {
	r := NewRecorder()

	_, err := r.Finish()
	if err == nil {
		t.Fatal("expected error on Finish without Start")
	}

	if !strings.Contains(err.Error(), "not recording") {
		t.Errorf("error = %q, want 'not recording'", err.Error())
	}
}

func TestRecorderAddStepWithoutStart(t *testing.T) {
	r := NewRecorder()

	err := r.AddStep("https://example.com", "Test", "annotation", nil)
	if err == nil {
		t.Fatal("expected error on AddStep without Start")
	}

	if !strings.Contains(err.Error(), "not recording") {
		t.Errorf("error = %q, want 'not recording'", err.Error())
	}
}

func TestRenderMarkdown(t *testing.T) {
	fakeScreenshot := []byte("fake-png-data")
	g := &Guide{
		Title: "My Guide",
		URL:   "https://example.com",
		Steps: []Step{
			{
				Number:     1,
				URL:        "https://example.com",
				PageTitle:  "Example",
				Annotation: "Navigate to the home page",
				Screenshot: fakeScreenshot,
			},
			{
				Number:    2,
				URL:       "https://example.com/about",
				PageTitle: "About",
			},
		},
	}

	md, err := RenderMarkdown(g)
	if err != nil {
		t.Fatalf("RenderMarkdown: %v", err)
	}

	content := string(md)

	// Check title.
	if !strings.Contains(content, "# My Guide") {
		t.Error("missing title in markdown")
	}

	// Check URL.
	if !strings.Contains(content, "**URL:** https://example.com") {
		t.Error("missing URL in markdown")
	}

	// Check step headers.
	if !strings.Contains(content, "## Step 1") {
		t.Error("missing Step 1 header")
	}

	if !strings.Contains(content, "## Step 2") {
		t.Error("missing Step 2 header")
	}

	// Check annotation.
	if !strings.Contains(content, "Navigate to the home page") {
		t.Error("missing annotation in markdown")
	}

	// Check base64 screenshot.
	encoded := base64.StdEncoding.EncodeToString(fakeScreenshot)
	if !strings.Contains(content, encoded) {
		t.Error("missing base64 screenshot in markdown")
	}

	// Step 2 should not have screenshot image tag.
	if strings.Contains(content, "![Step 2]") {
		t.Error("step 2 should not have screenshot")
	}
}

func TestRenderMarkdownNilGuide(t *testing.T) {
	_, err := RenderMarkdown(nil)
	if err == nil {
		t.Fatal("expected error for nil guide")
	}
}
