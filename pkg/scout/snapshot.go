package scout

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

// SnapshotOption configures accessibility snapshot behavior.
type SnapshotOption func(*snapshotConfig)

type snapshotConfig struct {
	maxDepth         int
	filterRoles      []string
	interactableOnly bool
	includeIframes   bool
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

// WithSnapshotIframes includes iframe content in the snapshot.
func WithSnapshotIframes() SnapshotOption {
	return func(c *snapshotConfig) { c.includeIframes = true }
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
		"generation":       gen,
		"maxDepth":         cfg.maxDepth,
		"interactableOnly": cfg.interactableOnly,
	}
	if len(cfg.filterRoles) > 0 {
		configMap["filterRoles"] = cfg.filterRoles
	}

	result, err := p.Eval(snapshotJS, configMap)
	if err != nil {
		return "", fmt.Errorf("scout: snapshot: %w", err)
	}

	snap := result.String()

	if cfg.includeIframes {
		iframeSnap := p.snapshotIframes(gen, cfg)
		if iframeSnap != "" {
			snap += "\n" + iframeSnap
		}
	}

	return snap, nil
}

// snapshotIframes traverses all iframes on the page and appends their snapshots.
func (p *Page) snapshotIframes(gen int64, cfg *snapshotConfig) string {
	iframes, err := p.page.Elements("iframe")
	if err != nil {
		return ""
	}

	var parts []string
	for _, iframe := range iframes {
		src, _ := iframe.Attribute("src")
		srcLabel := ""
		if src != nil {
			srcLabel = *src
		}

		framePage, err := iframe.Frame()
		if err != nil {
			parts = append(parts, fmt.Sprintf("  - [iframe src=%q] (cross-origin, access denied)", srcLabel))
			continue
		}

		iframeCfg := map[string]interface{}{
			"generation":       gen,
			"maxDepth":         cfg.maxDepth,
			"interactableOnly": cfg.interactableOnly,
		}
		if len(cfg.filterRoles) > 0 {
			iframeCfg["filterRoles"] = cfg.filterRoles
		}

		res, err := framePage.Eval(snapshotJS, iframeCfg)
		if err != nil {
			parts = append(parts, fmt.Sprintf("  - [iframe src=%q] (access denied)", srcLabel))
			continue
		}

		frameSnap := res.Value.Str()
		if frameSnap == "" {
			continue
		}

		// Indent iframe content by 2 extra spaces and add header
		var lines []string
		lines = append(lines, fmt.Sprintf("  - [iframe src=%q]", srcLabel))
		for _, line := range strings.Split(frameSnap, "\n") {
			lines = append(lines, "    "+line)
		}
		parts = append(parts, strings.Join(lines, "\n"))
	}

	return strings.Join(parts, "\n")
}

// snapshotSystemPrompt is the system prompt used when feeding snapshots to an LLM.
const snapshotSystemPrompt = `You are analyzing a web page's accessibility tree snapshot.

The snapshot uses a YAML-like format where each line represents a DOM element:
- Indentation shows nesting depth (2 spaces per level)
- Each element starts with "- " followed by its ARIA role
- Quoted strings after the role are the element's accessible name
- Properties like level=, value=, checked/unchecked, disabled appear after the name
- [ref=sNNNeNNN] markers uniquely identify each element for programmatic access
- [iframe src="..."] sections contain nested iframe content

Use the [ref=...] markers to reference specific elements when suggesting interactions.
Roles include: link, button, textbox, heading, navigation, form, list, listitem, etc.`

// SnapshotWithLLM takes an accessibility snapshot and sends it to the given LLM
// provider along with the user's prompt. Returns the LLM's response.
func SnapshotWithLLM(page *Page, provider LLMProvider, prompt string, opts ...SnapshotOption) (string, error) {
	if provider == nil {
		return "", fmt.Errorf("scout: snapshot: nil LLM provider")
	}

	snap, err := page.SnapshotWithOptions(opts...)
	if err != nil {
		return "", fmt.Errorf("scout: snapshot: %w", err)
	}

	userPrompt := prompt + "\n\n---\n\nAccessibility Tree:\n" + snap

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := provider.Complete(ctx, snapshotSystemPrompt, userPrompt)
	if err != nil {
		return "", fmt.Errorf("scout: snapshot: llm %s: %w", provider.Name(), err)
	}

	return result, nil
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
