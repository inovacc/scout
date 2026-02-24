package scout

import (
	"fmt"
	"strings"
	"sync/atomic"
)

// SnapshotOption configures accessibility snapshot behavior.
type SnapshotOption func(*snapshotConfig)

type snapshotConfig struct {
	maxDepth         int
	filterRoles      []string
	interactableOnly bool
}

// snapshotGeneration tracks snapshot generation for ref uniqueness.
var snapshotGeneration atomic.Int64

// WithSnapshotMaxDepth limits the DOM traversal depth.
func WithSnapshotMaxDepth(n int) SnapshotOption {
	return func(c *snapshotConfig) { c.maxDepth = n }
}

// WithSnapshotFilter only includes elements with the given ARIA roles.
func WithSnapshotFilter(roles ...string) SnapshotOption {
	return func(c *snapshotConfig) { c.filterRoles = roles }
}

// WithSnapshotInteractableOnly only includes interactable elements.
func WithSnapshotInteractableOnly() SnapshotOption {
	return func(c *snapshotConfig) { c.interactableOnly = true }
}

// Snapshot returns a YAML-like accessibility tree of the current page.
func (p *Page) Snapshot() (string, error) {
	return p.SnapshotWithOptions()
}

// SnapshotWithOptions returns an accessibility tree with the given options.
func (p *Page) SnapshotWithOptions(opts ...SnapshotOption) (string, error) {
	if p == nil || p.page == nil {
		return "", fmt.Errorf("scout: snapshot: nil page")
	}

	cfg := &snapshotConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	gen := snapshotGeneration.Add(1)

	configMap := map[string]interface{}{
		"generation":      gen,
		"maxDepth":        cfg.maxDepth,
		"interactableOnly": cfg.interactableOnly,
	}
	if len(cfg.filterRoles) > 0 {
		configMap["filterRoles"] = cfg.filterRoles
	}

	result, err := p.Eval(snapshotJS, configMap)
	if err != nil {
		return "", fmt.Errorf("scout: snapshot: %w", err)
	}

	return result.String(), nil
}

// ElementByRef finds an element by its snapshot reference marker.
// The ref should be in the format "s{gen}e{id}" as produced by Snapshot().
func (p *Page) ElementByRef(ref string) (*Element, error) {
	if p == nil || p.page == nil {
		return nil, fmt.Errorf("scout: element by ref: nil page")
	}

	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("scout: element by ref: empty ref")
	}

	// Use CSS attribute selector to find the element via rod.
	selector := fmt.Sprintf(`[data-scout-ref="%s"]`, ref)
	rodEl, err := p.page.Element(selector)
	if err != nil {
		return nil, fmt.Errorf("scout: element by ref %q: %w", ref, err)
	}

	return &Element{element: rodEl}, nil
}
