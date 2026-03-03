package runbook

import (
	"context"
	"fmt"
	"maps"
	"strings"

	"github.com/inovacc/scout/pkg/scout"
)

// SiteAnalysis holds the result of analyzing a page's structure.
type SiteAnalysis struct {
	URL           string
	PageType      string               // "listing", "detail", "form", "article", "table", "unknown"
	Framework     *scout.FrameworkInfo `json:"framework,omitempty"`
	RenderMode    *scout.RenderInfo    `json:"render_mode,omitempty"`
	Containers    []ContainerCandidate
	Forms         []FormCandidate
	Pagination    *PaginationCandidate
	Interactables []InteractableElement
	Metadata      map[string]string
}

// ContainerCandidate represents a detected repeating element pattern.
type ContainerCandidate struct {
	Selector string
	Count    int
	Fields   []FieldCandidate
	Score    int
}

// FieldCandidate is an inferred field within a container.
type FieldCandidate struct {
	Name     string // "title", "link", "price", "image", "date", "text_N"
	Selector string // CSS selector relative to container
	Attr     string // "" for text, "href"/"src" for attrs
	Sample   string
}

// FormCandidate represents a detected form.
type FormCandidate struct {
	Selector string
	Action   string
	Method   string
	Fields   []FormFieldCandidate
}

// FormFieldCandidate is a field within a detected form.
type FormFieldCandidate struct {
	Name        string
	Type        string
	Selector    string
	Placeholder string
	Required    bool
}

// PaginationCandidate represents a detected pagination pattern.
type PaginationCandidate struct {
	Strategy     string // "click", "scroll", "url"
	NextSelector string
	Confidence   int
}

// InteractableElement is a clickable/interactive non-submit element.
type InteractableElement struct {
	Selector string
	Type     string
	Text     string
}

// AnalyzeOption configures AnalyzeSite behavior.
type AnalyzeOption func(*analyzeOptions)

type analyzeOptions struct {
	maxContainers int
}

func analyzeDefaults() *analyzeOptions {
	return &analyzeOptions{maxContainers: 5}
}

// WithMaxContainers limits the number of container candidates returned.
func WithMaxContainers(n int) AnalyzeOption {
	return func(o *analyzeOptions) { o.maxContainers = n }
}

// AnalyzeSite navigates to a URL and inspects its structure.
func AnalyzeSite(ctx context.Context, browser *scout.Browser, url string, opts ...AnalyzeOption) (*SiteAnalysis, error) {
	_ = ctx // reserved for future cancellation

	o := analyzeDefaults()
	for _, fn := range opts {
		fn(o)
	}

	page, err := browser.NewPage(url)
	if err != nil {
		return nil, fmt.Errorf("runbook: analyze: navigate: %w", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		return nil, fmt.Errorf("runbook: analyze: wait load: %w", err)
	}

	analysis := &SiteAnalysis{
		URL:      url,
		Metadata: make(map[string]string),
	}

	// Detect framework and render mode
	if fw, err := page.DetectFramework(); err == nil && fw != nil {
		analysis.Framework = fw
		if fw.SPA {
			analysis.Metadata["wait_strategy"] = "spa: use WaitStable or WaitSelector before extraction"
		}
	}

	if ri, err := page.DetectRenderMode(); err == nil {
		analysis.RenderMode = ri
		if ri.Mode == scout.RenderCSR {
			analysis.Metadata["wait_strategy"] = "csr: content is client-rendered, use WaitStable or WaitSelector"
		}
	}

	// Extract metadata
	meta, err := page.ExtractMeta()
	if err == nil && meta != nil {
		if meta.Title != "" {
			analysis.Metadata["title"] = meta.Title
		}

		if meta.Description != "" {
			analysis.Metadata["description"] = meta.Description
		}

		maps.Copy(analysis.Metadata, meta.OG)
	}

	// Detect containers
	analysis.Containers = detectContainers(page, o.maxContainers)

	// Detect forms
	analysis.Forms = detectForms(page)

	// Detect pagination
	analysis.Pagination = detectPagination(page)

	// Detect interactables
	analysis.Interactables = detectInteractables(page)

	// Determine page type
	analysis.PageType = classifyPage(analysis)

	return analysis, nil
}

// containerSelectors are CSS selectors to probe for repeating elements.
var containerSelectors = []string{
	"article",
	"[class*=\"item\"]",
	"[class*=\"card\"]",
	"[class*=\"product\"]",
	"[class*=\"result\"]",
	"ul > li",
	"ol > li",
	"tbody > tr",
	".row",
}

func detectContainers(page *scout.Page, maxResults int) []ContainerCandidate {
	var candidates []ContainerCandidate

	for _, sel := range containerSelectors {
		elements, err := page.Elements(sel)
		if err != nil || len(elements) < 3 {
			continue
		}

		c := ContainerCandidate{
			Selector: sel,
			Count:    len(elements),
			Score:    len(elements) * 10,
		}

		// Discover fields from the first element
		c.Fields = discoverFields(elements[0])
		c.Score += len(c.Fields) * 5

		candidates = append(candidates, c)
	}

	// Sort by score descending (simple insertion sort for small N)
	for i := 1; i < len(candidates); i++ {
		for j := i; j > 0 && candidates[j].Score > candidates[j-1].Score; j-- {
			candidates[j], candidates[j-1] = candidates[j-1], candidates[j]
		}
	}

	if len(candidates) > maxResults {
		candidates = candidates[:maxResults]
	}

	return candidates
}

// discoverFieldsJS is a JS function that inspects a container element's children.
const discoverFieldsJS = `() => {
	const el = this;
	const fields = [];

	// Headings → title
	const heading = el.querySelector('h1,h2,h3,h4,h5,h6');
	if (heading) {
		fields.push({name: "title", selector: heading.tagName.toLowerCase(), attr: "", sample: heading.textContent.trim()});
	}

	// Links → link
	const link = el.querySelector('a[href]');
	if (link && link.getAttribute('href')) {
		fields.push({name: "link", selector: "a", attr: "href", sample: link.getAttribute('href')});
	}

	// Images → image
	const img = el.querySelector('img[src]');
	if (img && img.getAttribute('src')) {
		fields.push({name: "image", selector: "img", attr: "src", sample: img.getAttribute('src')});
	}

	// Price patterns
	const priceEl = el.querySelector('.price, [class*="price"], span');
	if (priceEl) {
		const text = priceEl.textContent.trim();
		if (/[$€]|R\$/.test(text)) {
			const sel = el.querySelector('.price') ? '.price' : 'span';
			fields.push({name: "price", selector: sel, attr: "", sample: text});
		}
	}

	// Date/time
	const timeEl = el.querySelector('time, [datetime]');
	if (timeEl) {
		fields.push({name: "date", selector: "time", attr: "", sample: timeEl.textContent.trim()});
	}

	// Fallback: text content
	if (fields.length === 0) {
		const text = el.textContent.trim();
		if (text) {
			fields.push({name: "text_0", selector: "*", attr: "", sample: text});
		}
	}

	return fields;
}`

func discoverFields(el *scout.Element) []FieldCandidate {
	result, err := el.Eval(discoverFieldsJS)
	if err != nil {
		return nil
	}

	raw, ok := result.Value.([]any)
	if !ok {
		return nil
	}

	var fields []FieldCandidate

	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}

		fields = append(fields, FieldCandidate{
			Name:     fmt.Sprintf("%v", m["name"]),
			Selector: fmt.Sprintf("%v", m["selector"]),
			Attr:     fmt.Sprintf("%v", m["attr"]),
			Sample:   fmt.Sprintf("%v", m["sample"]),
		})
	}

	return fields
}

func detectForms(page *scout.Page) []FormCandidate {
	forms, err := page.DetectForms()
	if err != nil || len(forms) == 0 {
		return nil
	}

	var candidates []FormCandidate

	for i, f := range forms {
		fc := FormCandidate{
			Selector: fmt.Sprintf("form:nth-of-type(%d)", i+1),
			Action:   f.Action,
			Method:   f.Method,
		}

		for _, field := range f.Fields {
			ffc := FormFieldCandidate{
				Name:        field.Name,
				Type:        field.Type,
				Placeholder: field.Placeholder,
				Required:    field.Required,
			}
			if field.ID != "" {
				ffc.Selector = "#" + field.ID
			} else if field.Name != "" {
				ffc.Selector = fmt.Sprintf("[name=%q]", field.Name)
			}

			fc.Fields = append(fc.Fields, ffc)
		}

		candidates = append(candidates, fc)
	}

	return candidates
}

// paginationProbes maps selectors to strategy/confidence.
var paginationProbes = []struct {
	selector   string
	strategy   string
	confidence int
}{
	{`a[rel="next"]`, "click", 90},
	{`a.next`, "click", 70},
	{`.next`, "click", 70},
	{`[aria-label*="next"]`, "click", 75},
	{`.pagination a:last-child`, "click", 60},
}

func detectPagination(page *scout.Page) *PaginationCandidate {
	for _, probe := range paginationProbes {
		has, _ := page.Has(probe.selector)
		if has {
			return &PaginationCandidate{
				Strategy:     probe.strategy,
				NextSelector: probe.selector,
				Confidence:   probe.confidence,
			}
		}
	}

	// Check URL-based pagination pattern
	currentURL, err := page.URL()
	if err == nil && strings.Contains(currentURL, "page=") {
		return &PaginationCandidate{
			Strategy:   "url",
			Confidence: 50,
		}
	}

	return nil
}

func detectInteractables(page *scout.Page) []InteractableElement {
	selectors := []struct {
		sel string
		typ string
	}{
		{`button:not([type="submit"])`, "button"},
		{`[role="tab"]`, "tab"},
		{`[data-toggle]`, "toggle"},
	}

	var results []InteractableElement

	for _, s := range selectors {
		elements, err := page.Elements(s.sel)
		if err != nil {
			continue
		}

		for _, el := range elements {
			text, _ := el.Text()
			results = append(results, InteractableElement{
				Selector: s.sel,
				Type:     s.typ,
				Text:     text,
			})
		}
	}

	return results
}

func classifyPage(a *SiteAnalysis) string {
	// Table detection
	if len(a.Containers) > 0 {
		for _, c := range a.Containers {
			if c.Selector == "tbody > tr" {
				return "table"
			}
		}
	}

	// Form with login-like fields
	if len(a.Forms) > 0 {
		for _, f := range a.Forms {
			for _, field := range f.Fields {
				if field.Type == "password" || field.Name == "username" || field.Name == "email" {
					return "form"
				}
			}
		}
		// Generic form
		if len(a.Containers) == 0 {
			return "form"
		}
	}

	// Listing: containers with 3+ items
	if len(a.Containers) > 0 && a.Containers[0].Count >= 3 {
		return "listing"
	}

	// Article detection via metadata or article tag
	if _, ok := a.Metadata["og:type"]; ok {
		return "article"
	}

	return "unknown"
}
