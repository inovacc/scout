package scout

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-rod/rod"
)

// ExtractOption configures extraction behavior.
type ExtractOption func(*extractOptions)

type extractOptions struct {
	// placeholder for future options like custom parsers
}

// TableData holds the parsed content of an HTML table.
type TableData struct {
	Headers []string
	Rows    [][]string
}

// MetaData holds common page metadata (title, description, OG tags, etc.).
type MetaData struct {
	Title       string
	Description string
	Canonical   string
	OG          map[string]string
	Twitter     map[string]string
	JSONLD      []json.RawMessage
}

// Extract populates a struct from the page DOM using `scout:"selector"` tags.
//
// Tag format:
//
//	`scout:"css-selector"`       — extracts text content
//	`scout:"css-selector@attr"`  — extracts attribute value
//
// Supported field types: string, int, int64, float64, bool, []string,
// nested struct (selector scopes a container), []struct (one per match).
func (p *Page) Extract(target any, _ ...ExtractOption) error {
	rv := reflect.ValueOf(target)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("scout: extract: target must be a non-nil pointer to struct")
	}
	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return fmt.Errorf("scout: extract: target must be a pointer to struct")
	}
	return extractStruct(p.page, nil, rv)
}

// Extract populates a struct from the element's subtree using `scout:"selector"` tags.
func (e *Element) Extract(target any, _ ...ExtractOption) error {
	rv := reflect.ValueOf(target)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("scout: extract: target must be a non-nil pointer to struct")
	}
	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return fmt.Errorf("scout: extract: target must be a pointer to struct")
	}
	return extractStruct(e.element.Page(), e.element, rv)
}

// ExtractTable extracts an HTML table into structured data.
func (p *Page) ExtractTable(selector string) (*TableData, error) {
	el, err := p.page.Element(selector)
	if err != nil {
		return nil, fmt.Errorf("scout: extract table %q: %w", selector, err)
	}
	return extractTableFromElement(el)
}

// ExtractTableMap extracts an HTML table as a slice of maps keyed by header text.
func (p *Page) ExtractTableMap(selector string) ([]map[string]string, error) {
	td, err := p.ExtractTable(selector)
	if err != nil {
		return nil, err
	}
	result := make([]map[string]string, len(td.Rows))
	for i, row := range td.Rows {
		m := make(map[string]string, len(td.Headers))
		for j, header := range td.Headers {
			if j < len(row) {
				m[header] = row[j]
			}
		}
		result[i] = m
	}
	return result, nil
}

// ExtractMeta extracts common page metadata.
func (p *Page) ExtractMeta() (*MetaData, error) {
	meta := &MetaData{
		OG:      make(map[string]string),
		Twitter: make(map[string]string),
	}

	// Title
	title, err := p.page.Eval(`() => document.title || ""`)
	if err != nil {
		return nil, fmt.Errorf("scout: extract meta title: %w", err)
	}
	meta.Title = title.Value.Str()

	// Meta tags via JS for reliability
	result, err := p.page.Eval(`() => {
		const get = (sel, attr) => {
			const el = document.querySelector(sel);
			return el ? (el.getAttribute(attr) || "") : "";
		};
		const getAll = (prefix) => {
			const result = {};
			document.querySelectorAll('meta[property^="' + prefix + '"]').forEach(el => {
				const key = el.getAttribute("property") || "";
				const val = el.getAttribute("content") || "";
				if (key) result[key] = val;
			});
			return result;
		};
		const getAllName = (prefix) => {
			const result = {};
			document.querySelectorAll('meta[name^="' + prefix + '"]').forEach(el => {
				const key = el.getAttribute("name") || "";
				const val = el.getAttribute("content") || "";
				if (key) result[key] = val;
			});
			return result;
		};
		const jsonld = [];
		document.querySelectorAll('script[type="application/ld+json"]').forEach(el => {
			try { jsonld.push(JSON.parse(el.textContent)); } catch(e) {}
		});
		return {
			description: get('meta[name="description"]', 'content'),
			canonical: get('link[rel="canonical"]', 'href'),
			og: getAll('og:'),
			twitter: getAllName('twitter:'),
			jsonld: jsonld
		};
	}`)
	if err != nil {
		return nil, fmt.Errorf("scout: extract meta: %w", err)
	}

	raw := result.Value
	meta.Description = raw.Get("description").Str()
	meta.Canonical = raw.Get("canonical").Str()

	ogObj := raw.Get("og")
	for key, val := range ogObj.Map() {
		meta.OG[key] = val.Str()
	}

	twObj := raw.Get("twitter")
	for key, val := range twObj.Map() {
		meta.Twitter[key] = val.Str()
	}

	jsonldArr := raw.Get("jsonld")
	for _, item := range jsonldArr.Arr() {
		b, err := json.Marshal(item.Val())
		if err == nil {
			meta.JSONLD = append(meta.JSONLD, json.RawMessage(b))
		}
	}

	return meta, nil
}

// ExtractText extracts the text content of the first element matching the selector.
func (p *Page) ExtractText(selector string) (string, error) {
	el, err := p.page.Element(selector)
	if err != nil {
		return "", fmt.Errorf("scout: extract text %q: %w", selector, err)
	}
	text, err := el.Text()
	if err != nil {
		return "", fmt.Errorf("scout: extract text %q: %w", selector, err)
	}
	return text, nil
}

// ExtractTexts extracts the text content of all elements matching the selector.
func (p *Page) ExtractTexts(selector string) ([]string, error) {
	els, err := p.page.Elements(selector)
	if err != nil {
		return nil, fmt.Errorf("scout: extract texts %q: %w", selector, err)
	}
	texts := make([]string, 0, len(els))
	for _, el := range els {
		text, err := el.Text()
		if err != nil {
			return nil, fmt.Errorf("scout: extract texts %q: %w", selector, err)
		}
		texts = append(texts, text)
	}
	return texts, nil
}

// ExtractAttribute extracts an attribute value from the first matching element.
func (p *Page) ExtractAttribute(selector, attr string) (string, error) {
	el, err := p.page.Element(selector)
	if err != nil {
		return "", fmt.Errorf("scout: extract attribute %q %q: %w", selector, attr, err)
	}
	val, err := el.Attribute(attr)
	if err != nil {
		return "", fmt.Errorf("scout: extract attribute %q %q: %w", selector, attr, err)
	}
	if val == nil {
		return "", nil
	}
	return *val, nil
}

// ExtractAttributes extracts an attribute value from all matching elements.
func (p *Page) ExtractAttributes(selector, attr string) ([]string, error) {
	els, err := p.page.Elements(selector)
	if err != nil {
		return nil, fmt.Errorf("scout: extract attributes %q %q: %w", selector, attr, err)
	}
	result := make([]string, 0, len(els))
	for _, el := range els {
		val, err := el.Attribute(attr)
		if err != nil {
			return nil, fmt.Errorf("scout: extract attributes %q %q: %w", selector, attr, err)
		}
		if val != nil {
			result = append(result, *val)
		}
	}
	return result, nil
}

// ExtractLinks extracts all href values from <a> elements on the page.
func (p *Page) ExtractLinks() ([]string, error) {
	return p.ExtractAttributes("a[href]", "href")
}

// --- internal helpers ---

func extractTableFromElement(el *rod.Element) (*TableData, error) {
	td := &TableData{}

	// Extract headers
	headerEls, err := el.Elements("thead th")
	if err == nil && len(headerEls) > 0 {
		for _, h := range headerEls {
			text, err := h.Text()
			if err != nil {
				return nil, fmt.Errorf("scout: extract table header: %w", err)
			}
			td.Headers = append(td.Headers, text)
		}
	} else {
		// Fallback: first row as headers
		firstRowCells, err := el.Elements("tr:first-child th, tr:first-child td")
		if err == nil {
			for _, c := range firstRowCells {
				text, err := c.Text()
				if err != nil {
					return nil, fmt.Errorf("scout: extract table header: %w", err)
				}
				td.Headers = append(td.Headers, text)
			}
		}
	}

	// Extract rows from tbody, or all tr if no tbody
	rowEls, err := el.Elements("tbody tr")
	if err != nil || len(rowEls) == 0 {
		rowEls, err = el.Elements("tr")
		if err != nil {
			return nil, fmt.Errorf("scout: extract table rows: %w", err)
		}
		// Skip header row if we pulled headers from first row
		if len(td.Headers) > 0 && len(rowEls) > 0 {
			rowEls = rowEls[1:]
		}
	}

	for _, row := range rowEls {
		cells, err := row.Elements("td")
		if err != nil {
			return nil, fmt.Errorf("scout: extract table cells: %w", err)
		}
		var rowData []string
		for _, cell := range cells {
			text, err := cell.Text()
			if err != nil {
				return nil, fmt.Errorf("scout: extract table cell: %w", err)
			}
			rowData = append(rowData, text)
		}
		if len(rowData) > 0 {
			td.Rows = append(td.Rows, rowData)
		}
	}

	return td, nil
}

// parseTag parses a scout struct tag into selector and optional attribute.
// Format: "selector" or "selector@attr"
func parseTag(tag string) (selector, attr string) {
	if i := strings.LastIndex(tag, "@"); i > 0 {
		return tag[:i], tag[i+1:]
	}
	return tag, ""
}

// findElements finds elements relative to a scope element, or from page root.
func findElements(page *rod.Page, scope *rod.Element, selector string) (rod.Elements, error) {
	if scope != nil {
		return page.ElementsByJS(rod.Eval(`(sel) => this.querySelectorAll(sel)`, selector).This(scope.Object))
	}
	return page.Elements(selector)
}

// findElement finds the first element relative to a scope, or from page root.
func findElement(page *rod.Page, scope *rod.Element, selector string) (*rod.Element, error) {
	if scope != nil {
		return page.ElementByJS(rod.Eval(`(sel) => this.querySelector(sel)`, selector).This(scope.Object))
	}
	return page.Element(selector)
}

func extractStruct(page *rod.Page, scope *rod.Element, rv reflect.Value) error {
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		tag := field.Tag.Get("scout")
		if tag == "" || tag == "-" {
			continue
		}

		selector, attr := parseTag(tag)
		fv := rv.Field(i)

		if err := setField(page, scope, fv, field.Type, selector, attr); err != nil {
			return fmt.Errorf("scout: extract field %q: %w", field.Name, err)
		}
	}
	return nil
}

func setField(page *rod.Page, scope *rod.Element, fv reflect.Value, ft reflect.Type, selector, attr string) error {
	switch ft.Kind() {
	case reflect.String:
		return setStringField(page, scope, fv, selector, attr)
	case reflect.Int, reflect.Int64:
		return setIntField(page, scope, fv, selector, attr)
	case reflect.Float64:
		return setFloatField(page, scope, fv, selector, attr)
	case reflect.Bool:
		return setBoolField(page, scope, fv, selector, attr)
	case reflect.Slice:
		return setSliceField(page, scope, fv, ft, selector, attr)
	case reflect.Struct:
		// Nested struct: selector scopes a container element
		el, err := findElement(page, scope, selector)
		if err != nil {
			return nil // element not found, leave struct zero-valued
		}
		return extractStruct(page, el, fv)
	default:
		return nil
	}
}

func getTextOrAttr(el *rod.Element, attr string) (string, error) {
	if attr != "" {
		val, err := el.Attribute(attr)
		if err != nil {
			return "", err
		}
		if val == nil {
			return "", nil
		}
		return *val, nil
	}
	return el.Text()
}

func setStringField(page *rod.Page, scope *rod.Element, fv reflect.Value, selector, attr string) error {
	el, err := findElement(page, scope, selector)
	if err != nil {
		return nil // not found, leave as zero value
	}
	val, err := getTextOrAttr(el, attr)
	if err != nil {
		return err
	}
	fv.SetString(val)
	return nil
}

func setIntField(page *rod.Page, scope *rod.Element, fv reflect.Value, selector, attr string) error {
	el, err := findElement(page, scope, selector)
	if err != nil {
		return nil
	}
	val, err := getTextOrAttr(el, attr)
	if err != nil {
		return err
	}
	val = strings.TrimSpace(val)
	if n, err := strconv.ParseInt(val, 10, 64); err == nil {
		fv.SetInt(n)
	}
	return nil
}

func setFloatField(page *rod.Page, scope *rod.Element, fv reflect.Value, selector, attr string) error {
	el, err := findElement(page, scope, selector)
	if err != nil {
		return nil
	}
	val, err := getTextOrAttr(el, attr)
	if err != nil {
		return err
	}
	val = strings.TrimSpace(val)
	if f, err := strconv.ParseFloat(val, 64); err == nil {
		fv.SetFloat(f)
	}
	return nil
}

func setBoolField(page *rod.Page, scope *rod.Element, fv reflect.Value, selector, attr string) error {
	el, err := findElement(page, scope, selector)
	if err != nil {
		return nil
	}
	val, err := getTextOrAttr(el, attr)
	if err != nil {
		return err
	}
	val = strings.TrimSpace(strings.ToLower(val))
	fv.SetBool(val == "true" || val == "1" || val == "yes")
	return nil
}

func setSliceField(page *rod.Page, scope *rod.Element, fv reflect.Value, ft reflect.Type, selector, attr string) error {
	elemType := ft.Elem()

	if elemType.Kind() == reflect.String {
		// []string: collect text/attr from all matches
		els, err := findElements(page, scope, selector)
		if err != nil {
			return nil
		}
		result := reflect.MakeSlice(ft, 0, len(els))
		for _, el := range els {
			val, err := getTextOrAttr(el, attr)
			if err != nil {
				continue
			}
			result = reflect.Append(result, reflect.ValueOf(val))
		}
		fv.Set(result)
		return nil
	}

	if elemType.Kind() == reflect.Struct {
		// []struct: one struct per matching element
		els, err := findElements(page, scope, selector)
		if err != nil {
			return nil
		}
		result := reflect.MakeSlice(ft, len(els), len(els))
		for i, el := range els {
			if err := extractStruct(page, el, result.Index(i)); err != nil {
				return err
			}
		}
		fv.Set(result)
		return nil
	}

	return nil
}
