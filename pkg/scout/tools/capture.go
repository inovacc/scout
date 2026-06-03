package tools

import (
	"context"
	"fmt"

	"github.com/inovacc/scout/pkg/scout"
)

// ScreenshotInput controls whether to capture the viewport or the full page.
type ScreenshotInput struct {
	FullPage bool `json:"fullPage,omitempty" jsonschema:"capture the full scrollable page instead of the viewport"`
}

// ScreenshotOutput holds the encoded PNG bytes.
type ScreenshotOutput struct {
	Data []byte `json:"data"`
}

// Screenshot captures a PNG of the current page (viewport or full page).
func Screenshot(_ context.Context, p *scout.Page, in ScreenshotInput) (*ScreenshotOutput, error) {
	if p == nil {
		return nil, fmt.Errorf("tools: screenshot: nil page")
	}

	var (
		data []byte
		err  error
	)

	if in.FullPage {
		data, err = p.FullScreenshot()
	} else {
		data, err = p.Screenshot()
	}

	if err != nil {
		return nil, fmt.Errorf("tools: screenshot: %w", err)
	}

	return &ScreenshotOutput{Data: data}, nil
}

// SnapshotInput mirrors the accessibility-tree options exposed by MCP.
type SnapshotInput struct {
	InteractableOnly bool   `json:"interactableOnly,omitempty" jsonschema:"only include interactable elements (buttons, links, inputs)"`
	MaxDepth         int    `json:"maxDepth,omitempty"         jsonschema:"maximum tree depth (0 = unlimited)"`
	Iframes          bool   `json:"iframes,omitempty"          jsonschema:"include iframe content in the tree"`
	Filter           string `json:"filter,omitempty"           jsonschema:"filter nodes by role or name substring"`
}

// SnapshotOutput holds the accessibility-tree text.
type SnapshotOutput struct {
	Snapshot string `json:"snapshot"`
}

// Snapshot returns the accessibility tree of the current page.
func Snapshot(_ context.Context, p *scout.Page, in SnapshotInput) (*SnapshotOutput, error) {
	if p == nil {
		return nil, fmt.Errorf("tools: snapshot: nil page")
	}

	var opts []scout.SnapshotOption
	if in.InteractableOnly {
		opts = append(opts, scout.WithSnapshotInteractableOnly())
	}

	if in.MaxDepth > 0 {
		opts = append(opts, scout.WithSnapshotMaxDepth(in.MaxDepth))
	}

	if in.Iframes {
		opts = append(opts, scout.WithSnapshotIframes())
	}

	if in.Filter != "" {
		opts = append(opts, scout.WithSnapshotFilter(in.Filter))
	}

	snap, err := p.SnapshotWithOptions(opts...)
	if err != nil {
		return nil, fmt.Errorf("tools: snapshot: %w", err)
	}

	return &SnapshotOutput{Snapshot: snap}, nil
}

// PDFInput mirrors the subset of PDF options exposed by MCP.
type PDFInput struct {
	Landscape       bool    `json:"landscape,omitempty"       jsonschema:"landscape orientation"`
	PrintBackground bool    `json:"printBackground,omitempty" jsonschema:"print background graphics"`
	Scale           float64 `json:"scale,omitempty"           jsonschema:"scale factor (0.1 to 2.0)"`
}

// PDFOutput holds the encoded PDF bytes.
type PDFOutput struct {
	Data []byte `json:"data"`
}

// PDF renders the current page to a PDF, applying any provided options.
func PDF(_ context.Context, p *scout.Page, in PDFInput) (*PDFOutput, error) {
	if p == nil {
		return nil, fmt.Errorf("tools: pdf: nil page")
	}

	var (
		data []byte
		err  error
	)

	if in.Landscape || in.PrintBackground || in.Scale > 0 {
		data, err = p.PDFWithOptions(scout.PDFOptions{
			Landscape:       in.Landscape,
			PrintBackground: in.PrintBackground,
			Scale:           in.Scale,
		})
	} else {
		data, err = p.PDF()
	}

	if err != nil {
		return nil, fmt.Errorf("tools: pdf: %w", err)
	}

	return &PDFOutput{Data: data}, nil
}
