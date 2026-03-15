package guide

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Step represents a single recorded step in a guide.
type Step struct {
	Number     int       `json:"number"`
	URL        string    `json:"url"`
	PageTitle  string    `json:"page_title"`
	Screenshot []byte    `json:"-"`
	Annotation string    `json:"annotation"`
	Timestamp  time.Time `json:"timestamp"`
}

// Guide represents a completed step-by-step guide.
type Guide struct {
	Title     string    `json:"title"`
	URL       string    `json:"url"`
	StartedAt time.Time `json:"started_at"`
	Steps     []Step    `json:"steps"`
}

// Recorder tracks steps for building a guide.
type Recorder struct {
	mu    sync.Mutex
	guide *Guide
}

// NewRecorder creates a new guide recorder.
func NewRecorder() *Recorder {
	return &Recorder{}
}

// Start begins a new guide recording session.
func (r *Recorder) Start(title, url string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.guide != nil {
		return errors.New("scout: guide: already recording")
	}

	r.guide = &Guide{
		Title:     title,
		URL:       url,
		StartedAt: time.Now(),
	}

	return nil
}

// AddStep appends a step to the current guide.
func (r *Recorder) AddStep(url, pageTitle, annotation string, screenshot []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.guide == nil {
		return errors.New("scout: guide: not recording")
	}

	r.guide.Steps = append(r.guide.Steps, Step{
		Number:     len(r.guide.Steps) + 1,
		URL:        url,
		PageTitle:  pageTitle,
		Screenshot: screenshot,
		Annotation: annotation,
		Timestamp:  time.Now(),
	})

	return nil
}

// Finish finalizes the recording session and returns the guide.
func (r *Recorder) Finish() (*Guide, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.guide == nil {
		return nil, errors.New("scout: guide: not recording")
	}

	g := r.guide
	r.guide = nil

	return g, nil
}

// IsRecording reports whether the recorder is currently active.
func (r *Recorder) IsRecording() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.guide != nil
}

// RenderMarkdown produces a markdown document from the guide with base64-embedded screenshots.
func RenderMarkdown(g *Guide) ([]byte, error) {
	if g == nil {
		return nil, errors.New("scout: guide: nil guide")
	}

	var b strings.Builder

	_, _ = fmt.Fprintf(&b, "# %s\n\n", g.Title)
	_, _ = fmt.Fprintf(&b, "**URL:** %s\n\n", g.URL)
	_, _ = fmt.Fprintf(&b, "**Started:** %s\n\n", g.StartedAt.Format(time.RFC3339))
	_, _ = fmt.Fprintf(&b, "---\n\n")

	for _, step := range g.Steps {
		_, _ = fmt.Fprintf(&b, "## Step %d\n\n", step.Number)

		if step.PageTitle != "" {
			_, _ = fmt.Fprintf(&b, "**Page:** %s\n\n", step.PageTitle)
		}

		_, _ = fmt.Fprintf(&b, "**URL:** %s\n\n", step.URL)

		if step.Annotation != "" {
			_, _ = fmt.Fprintf(&b, "%s\n\n", step.Annotation)
		}

		if len(step.Screenshot) > 0 {
			encoded := base64.StdEncoding.EncodeToString(step.Screenshot)
			_, _ = fmt.Fprintf(&b, "![Step %d](data:image/png;base64,%s)\n\n", step.Number, encoded)
		}

		_, _ = fmt.Fprintf(&b, "---\n\n")
	}

	return []byte(b.String()), nil
}
