package scout

import "strings"

// ExtractionRequest configures what to extract from a page.
type ExtractionRequest struct {
	Selectors     []string // CSS selectors for text extraction
	Attrs         []string // "selector@attr" specs
	TableSelector string   // CSS selector for table
	Links         bool     // Extract all links
	Meta          bool     // Extract metadata
}

// ExtractionResult holds all extracted data.
type ExtractionResult struct {
	URL       string              `json:"url"`
	Selectors map[string][]string `json:"selectors,omitempty"`
	Attrs     map[string][]string `json:"attrs,omitempty"`
	Table     *TableData          `json:"table,omitempty"`
	Links     []string            `json:"links,omitempty"`
	Meta      *MetaData           `json:"meta,omitempty"`
	Errors    []string            `json:"errors,omitempty"`
}

// ExtractAll runs all requested extractions on the page.
// Errors are collected in result.Errors rather than returned,
// so partial results are always available.
func (p *Page) ExtractAll(req *ExtractionRequest) *ExtractionResult {
	result := &ExtractionResult{}
	if u, err := p.URL(); err == nil {
		result.URL = u
	}

	if len(req.Selectors) > 0 {
		result.Selectors = make(map[string][]string)
		for _, sel := range req.Selectors {
			texts, err := p.ExtractTexts(sel)
			if err != nil {
				result.Errors = append(result.Errors, "selector "+sel+": "+err.Error())
				continue
			}

			result.Selectors[sel] = texts
		}
	}

	if len(req.Attrs) > 0 {
		result.Attrs = make(map[string][]string)
		for _, spec := range req.Attrs {
			sel, attr, ok := ParseAttrSpec(spec)
			if !ok {
				result.Errors = append(result.Errors, "invalid attr spec "+spec+" (use selector@attr)")
				continue
			}

			values, err := p.ExtractAttributes(sel, attr)
			if err != nil {
				result.Errors = append(result.Errors, "attr "+spec+": "+err.Error())
				continue
			}

			result.Attrs[spec] = values
		}
	}

	if req.TableSelector != "" {
		td, err := p.ExtractTable(req.TableSelector)
		if err != nil {
			result.Errors = append(result.Errors, "table "+req.TableSelector+": "+err.Error())
		} else {
			result.Table = td
		}
	}

	if req.Links {
		links, err := p.ExtractLinks()
		if err != nil {
			result.Errors = append(result.Errors, "links: "+err.Error())
		} else {
			result.Links = links
		}
	}

	if req.Meta {
		meta, err := p.ExtractMeta()
		if err != nil {
			result.Errors = append(result.Errors, "meta: "+err.Error())
		} else {
			result.Meta = meta
		}
	}

	return result
}

// ParseAttrSpec parses "selector@attr" into selector and attribute name.
func ParseAttrSpec(spec string) (selector, attr string, ok bool) {
	idx := strings.LastIndex(spec, "@")
	if idx <= 0 || idx >= len(spec)-1 {
		return "", "", false
	}

	return spec[:idx], spec[idx+1:], true
}
