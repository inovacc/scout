package recipe

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/inovacc/scout/pkg/scout"
)

// runExtract executes an extraction recipe.
func runExtract(ctx context.Context, browser *scout.Browser, r *Recipe) (*Result, error) {
	page, err := browser.NewPage(r.URL)
	if err != nil {
		return nil, fmt.Errorf("recipe: navigate: %w", err)
	}

	if err := page.WaitLoad(); err != nil {
		return nil, fmt.Errorf("recipe: wait load: %w", err)
	}

	if r.WaitFor != "" {
		if _, err := page.Element(r.WaitFor); err != nil {
			return nil, fmt.Errorf("recipe: wait for %q: %w", r.WaitFor, err)
		}
	}

	result := &Result{}
	maxPages := 1
	if r.Pagination != nil && r.Pagination.MaxPages > 0 {
		maxPages = r.Pagination.MaxPages
	}

	seen := make(map[string]bool)

	for pageNum := 0; pageNum < maxPages; pageNum++ {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}

		items, err := extractPage(page, r.Items)
		if err != nil {
			return result, fmt.Errorf("recipe: extract page %d: %w", pageNum+1, err)
		}

		for _, item := range items {
			if r.Pagination != nil && r.Pagination.DedupField != "" {
				key := item[r.Pagination.DedupField]
				if seen[key] {
					continue
				}
				seen[key] = true
			}
			result.Items = append(result.Items, item)
		}

		// Navigate to next page if pagination is configured and not on last page
		if r.Pagination == nil || pageNum >= maxPages-1 {
			break
		}

		if r.Pagination.DelayMs > 0 {
			time.Sleep(time.Duration(r.Pagination.DelayMs) * time.Millisecond)
		}

		if !advancePage(page, r.Pagination) {
			break
		}

		// Wait for content to settle after pagination
		_ = page.WaitLoad()
		if r.WaitFor != "" {
			_, _ = page.Element(r.WaitFor)
		}
	}

	return result, nil
}

// extractPage extracts items from the current page.
func extractPage(page *scout.Page, spec *ItemSpec) ([]map[string]string, error) {
	elements, err := page.Elements(spec.Container)
	if err != nil {
		return nil, fmt.Errorf("container %q: %w", spec.Container, err)
	}

	var items []map[string]string
	for _, el := range elements {
		item := make(map[string]string)
		for name, sel := range spec.Fields {
			val, err := extractField(el, page, sel)
			if err != nil {
				// Non-fatal: field may not exist for every item
				continue
			}
			item[name] = val
		}
		if len(item) > 0 {
			items = append(items, item)
		}
	}

	return items, nil
}

// extractField resolves a field selector.
// "+" prefix means sibling row (look in page context, not element).
// "@attr" suffix means extract attribute instead of text.
func extractField(el *scout.Element, page *scout.Page, sel string) (string, error) {
	sibling := strings.HasPrefix(sel, "+")
	if sibling {
		sel = sel[1:]
	}

	var attrName string
	if idx := strings.LastIndex(sel, "@"); idx > 0 {
		attrName = sel[idx+1:]
		sel = sel[:idx]
	}

	var target *scout.Element
	var err error

	if sibling {
		// Search from page level for sibling content
		target, err = page.Element(sel)
	} else {
		// Search within the container element
		target, err = el.Element(sel)
	}

	if err != nil {
		return "", err
	}

	if attrName != "" {
		val, found, err := target.Attribute(attrName)
		if err != nil {
			return "", err
		}
		if !found {
			return "", fmt.Errorf("attribute %q not found", attrName)
		}
		return val, nil
	}

	return target.Text()
}

// advancePage moves to the next page using the configured pagination strategy.
func advancePage(page *scout.Page, p *Pagination) bool {
	switch p.Strategy {
	case "click":
		if p.NextSelector == "" {
			return false
		}
		el, err := page.Element(p.NextSelector)
		if err != nil {
			return false
		}
		if err := el.Click(); err != nil {
			return false
		}
		return true

	case "scroll":
		_, err := page.Eval("window.scrollTo(0, document.body.scrollHeight)")
		return err == nil

	default:
		return false
	}
}
