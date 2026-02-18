package scout

import (
	"encoding/json"
	"fmt"
	"strings"
)

// SwaggerSpec holds a parsed OpenAPI/Swagger specification.
type SwaggerSpec struct {
	SpecURL  string            `json:"spec_url"`
	Version  string            `json:"version"`
	Format   string            `json:"format"`
	Info     SwaggerInfo       `json:"info"`
	Servers  []SwaggerServer   `json:"servers,omitempty"`
	Paths    []SwaggerPath     `json:"paths,omitempty"`
	Schemas  map[string]any    `json:"schemas,omitempty"`
	Security []SwaggerSecurity `json:"security,omitempty"`
	Raw      json.RawMessage   `json:"raw,omitempty"`
}

// SwaggerInfo holds API metadata.
type SwaggerInfo struct {
	Title       string            `json:"title"`
	Description string            `json:"description,omitempty"`
	Version     string            `json:"version"`
	Contact     map[string]string `json:"contact,omitempty"`
	License     map[string]string `json:"license,omitempty"`
}

// SwaggerServer represents a server/base URL for the API.
type SwaggerServer struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

// SwaggerPath represents a single API operation.
type SwaggerPath struct {
	Path        string                `json:"path"`
	Method      string                `json:"method"`
	OperationID string                `json:"operation_id,omitempty"`
	Summary     string                `json:"summary,omitempty"`
	Tags        []string              `json:"tags,omitempty"`
	Parameters  []SwaggerParam        `json:"parameters,omitempty"`
	Responses   map[string]string     `json:"responses,omitempty"`
	Security    []map[string][]string `json:"security,omitempty"`
}

// SwaggerParam represents an API parameter.
type SwaggerParam struct {
	Name     string `json:"name"`
	In       string `json:"in"`
	Required bool   `json:"required,omitempty"`
	Type     string `json:"type,omitempty"`
}

// SwaggerSecurity describes a security scheme.
type SwaggerSecurity struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	In     string `json:"in,omitempty"`
	Scheme string `json:"scheme,omitempty"`
}

// SwaggerOption configures swagger extraction behavior.
type SwaggerOption func(*swaggerOptions)

type swaggerOptions struct {
	endpointsOnly bool
	includeRaw    bool
}

func swaggerDefaults() *swaggerOptions {
	return &swaggerOptions{}
}

// WithSwaggerEndpointsOnly extracts only paths/endpoints, skipping schemas.
func WithSwaggerEndpointsOnly(v bool) SwaggerOption {
	return func(o *swaggerOptions) { o.endpointsOnly = v }
}

// WithSwaggerRaw includes the raw spec JSON in the result.
func WithSwaggerRaw(v bool) SwaggerOption {
	return func(o *swaggerOptions) { o.includeRaw = v }
}

// DetectSwagger checks if the current page is a Swagger/OpenAPI UI and returns
// the spec URL if found.
func (p *Page) DetectSwagger() (string, error) {
	// Try Swagger UI 3+ config
	res, err := p.Eval(`() => {
		// Swagger UI 3+
		if (window.ui && window.ui.getConfigs) {
			const cfg = window.ui.getConfigs();
			if (cfg && cfg.url) return cfg.url;
			if (cfg && cfg.urls && cfg.urls.length > 0) return cfg.urls[0].url;
		}
		// Check for spec URL in swagger-ui init script
		const scripts = document.querySelectorAll('script');
		for (const s of scripts) {
			const text = s.textContent || '';
			const m = text.match(/url\s*[:=]\s*["']([^"']*(?:swagger|openapi|api-docs)[^"']*)["']/i);
			if (m) return m[1];
		}
		// Check for link to spec in page
		const links = document.querySelectorAll('a[href*="swagger.json"], a[href*="openapi.json"], a[href*="api-docs"]');
		if (links.length > 0) return links[0].href;
		return '';
	}`)
	if err != nil {
		return "", fmt.Errorf("scout: detect swagger: %w", err)
	}

	specURL := res.String()
	if specURL != "" {
		return specURL, nil
	}

	// Check for Swagger UI element presence
	res, err = p.Eval(`() => {
		const swaggerEl = document.querySelector('#swagger-ui, .swagger-ui');
		const redocEl = document.querySelector('redoc, [id*="redoc"]');
		if (swaggerEl || redocEl) return 'detected';
		const title = (document.title || '').toLowerCase();
		if (title.includes('swagger') || title.includes('openapi') || title.includes('api doc')) return 'detected';
		return '';
	}`)
	if err != nil {
		return "", fmt.Errorf("scout: detect swagger: %w", err)
	}

	if res.String() == "detected" {
		// UI detected but no spec URL found — try common paths
		return "", nil
	}

	return "", nil
}

// ExtractSwagger detects a Swagger/OpenAPI UI on the current page, fetches the spec,
// and returns a parsed SwaggerSpec.
func (p *Page) ExtractSwagger(opts ...SwaggerOption) (*SwaggerSpec, error) {
	cfg := swaggerDefaults()
	for _, fn := range opts {
		fn(cfg)
	}

	specURL, err := p.DetectSwagger()
	if err != nil {
		return nil, err
	}

	if specURL == "" {
		// Try to fetch spec from the page's JavaScript context
		res, err := p.Eval(`() => {
			// Try to get spec from Swagger UI store
			if (window.ui && window.ui.specSelectors && window.ui.specSelectors.specJson) {
				const spec = window.ui.specSelectors.specJson().toJS ? window.ui.specSelectors.specJson().toJS() : window.ui.specSelectors.specJson();
				if (spec && (spec.swagger || spec.openapi)) return JSON.stringify(spec);
			}
			return '';
		}`)
		if err != nil {
			return nil, fmt.Errorf("scout: extract swagger: %w", err)
		}

		raw := res.String()
		if raw != "" {
			return parseSwaggerSpec("", []byte(raw), cfg)
		}

		return nil, fmt.Errorf("scout: extract swagger: no spec URL or inline spec found")
	}

	// Resolve relative URL
	if !strings.HasPrefix(specURL, "http://") && !strings.HasPrefix(specURL, "https://") {
		pageURL, err := p.URL()
		if err != nil {
			return nil, fmt.Errorf("scout: extract swagger: get page url: %w", err)
		}
		if strings.HasPrefix(specURL, "/") {
			// Absolute path — combine with origin
			idx := strings.Index(pageURL[8:], "/") // skip https://
			if idx >= 0 {
				specURL = pageURL[:idx+8] + specURL
			} else {
				specURL = pageURL + specURL
			}
		} else {
			// Relative path
			lastSlash := strings.LastIndex(pageURL, "/")
			if lastSlash >= 0 {
				specURL = pageURL[:lastSlash+1] + specURL
			}
		}
	}

	// Fetch spec via browser fetch
	res, err := p.Eval(`(url) => fetch(url).then(r => r.text())`, specURL)
	if err != nil {
		return nil, fmt.Errorf("scout: extract swagger: fetch spec: %w", err)
	}

	raw := res.String()
	if raw == "" {
		return nil, fmt.Errorf("scout: extract swagger: empty spec response from %s", specURL)
	}

	return parseSwaggerSpec(specURL, []byte(raw), cfg)
}

// ExtractSwagger navigates to the given URL and extracts the Swagger/OpenAPI spec.
func (b *Browser) ExtractSwagger(url string, opts ...SwaggerOption) (*SwaggerSpec, error) {
	page, err := b.NewPage(url)
	if err != nil {
		return nil, fmt.Errorf("scout: extract swagger: navigate: %w", err)
	}
	if err := page.WaitLoad(); err != nil {
		return nil, fmt.Errorf("scout: extract swagger: wait load: %w", err)
	}

	return page.ExtractSwagger(opts...)
}

func parseSwaggerSpec(specURL string, raw []byte, cfg *swaggerOptions) (*SwaggerSpec, error) {
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("scout: extract swagger: parse spec: %w", err)
	}

	spec := &SwaggerSpec{
		SpecURL: specURL,
		Format:  "json",
	}

	if cfg.includeRaw {
		spec.Raw = json.RawMessage(raw)
	}

	// Determine version
	if v, ok := doc["swagger"].(string); ok {
		spec.Version = v
	} else if v, ok := doc["openapi"].(string); ok {
		spec.Version = v
	}

	// Parse info
	if info, ok := doc["info"].(map[string]any); ok {
		spec.Info = parseSwaggerInfo(info)
	}

	// Parse servers (OpenAPI 3.x)
	if servers, ok := doc["servers"].([]any); ok {
		for _, s := range servers {
			if sm, ok := s.(map[string]any); ok {
				srv := SwaggerServer{
					URL:         strVal(sm, "url"),
					Description: strVal(sm, "description"),
				}
				spec.Servers = append(spec.Servers, srv)
			}
		}
	}

	// Parse host/basePath (Swagger 2.0)
	if host := strVal(doc, "host"); host != "" {
		scheme := "https"
		if schemes, ok := doc["schemes"].([]any); ok && len(schemes) > 0 {
			if s, ok := schemes[0].(string); ok {
				scheme = s
			}
		}
		basePath := strVal(doc, "basePath")
		spec.Servers = append(spec.Servers, SwaggerServer{
			URL: scheme + "://" + host + basePath,
		})
	}

	// Parse paths
	if paths, ok := doc["paths"].(map[string]any); ok {
		spec.Paths = parseSwaggerPaths(paths)
	}

	// Parse schemas (unless endpoints only)
	if !cfg.endpointsOnly {
		spec.Schemas = parseSwaggerSchemas(doc)
	}

	// Parse security definitions
	spec.Security = parseSwaggerSecurityDefs(doc)

	return spec, nil
}

func parseSwaggerInfo(info map[string]any) SwaggerInfo {
	si := SwaggerInfo{
		Title:       strVal(info, "title"),
		Description: strVal(info, "description"),
		Version:     strVal(info, "version"),
	}

	if contact, ok := info["contact"].(map[string]any); ok {
		si.Contact = mapStrStr(contact)
	}
	if license, ok := info["license"].(map[string]any); ok {
		si.License = mapStrStr(license)
	}

	return si
}

func parseSwaggerPaths(paths map[string]any) []SwaggerPath {
	var result []SwaggerPath

	for path, methods := range paths {
		methodMap, ok := methods.(map[string]any)
		if !ok {
			continue
		}

		for method, opData := range methodMap {
			method = strings.ToUpper(method)
			if !isHTTPMethod(method) {
				continue
			}

			op, ok := opData.(map[string]any)
			if !ok {
				continue
			}

			sp := SwaggerPath{
				Path:        path,
				Method:      method,
				OperationID: strVal(op, "operationId"),
				Summary:     strVal(op, "summary"),
			}

			if tags, ok := op["tags"].([]any); ok {
				for _, t := range tags {
					if s, ok := t.(string); ok {
						sp.Tags = append(sp.Tags, s)
					}
				}
			}

			if params, ok := op["parameters"].([]any); ok {
				for _, param := range params {
					if pm, ok := param.(map[string]any); ok {
						sp.Parameters = append(sp.Parameters, SwaggerParam{
							Name:     strVal(pm, "name"),
							In:       strVal(pm, "in"),
							Required: boolVal(pm, "required"),
							Type:     strVal(pm, "type"),
						})
					}
				}
			}

			if responses, ok := op["responses"].(map[string]any); ok {
				sp.Responses = make(map[string]string)
				for code, resp := range responses {
					if rm, ok := resp.(map[string]any); ok {
						sp.Responses[code] = strVal(rm, "description")
					}
				}
			}

			if security, ok := op["security"].([]any); ok {
				for _, sec := range security {
					if sm, ok := sec.(map[string]any); ok {
						entry := make(map[string][]string)
						for k, v := range sm {
							if arr, ok := v.([]any); ok {
								for _, item := range arr {
									if s, ok := item.(string); ok {
										entry[k] = append(entry[k], s)
									}
								}
								if entry[k] == nil {
									entry[k] = []string{}
								}
							}
						}
						sp.Security = append(sp.Security, entry)
					}
				}
			}

			result = append(result, sp)
		}
	}

	return result
}

func parseSwaggerSchemas(doc map[string]any) map[string]any {
	// OpenAPI 3.x
	if components, ok := doc["components"].(map[string]any); ok {
		if schemas, ok := components["schemas"].(map[string]any); ok {
			return schemas
		}
	}

	// Swagger 2.0
	if definitions, ok := doc["definitions"].(map[string]any); ok {
		return definitions
	}

	return nil
}

func parseSwaggerSecurityDefs(doc map[string]any) []SwaggerSecurity {
	var defs map[string]any

	// OpenAPI 3.x
	if components, ok := doc["components"].(map[string]any); ok {
		if sd, ok := components["securitySchemes"].(map[string]any); ok {
			defs = sd
		}
	}

	// Swagger 2.0
	if defs == nil {
		if sd, ok := doc["securityDefinitions"].(map[string]any); ok {
			defs = sd
		}
	}

	if defs == nil {
		return nil
	}

	var result []SwaggerSecurity
	for name, def := range defs {
		dm, ok := def.(map[string]any)
		if !ok {
			continue
		}

		result = append(result, SwaggerSecurity{
			Name:   name,
			Type:   strVal(dm, "type"),
			In:     strVal(dm, "in"),
			Scheme: strVal(dm, "scheme"),
		})
	}

	return result
}

func strVal(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func boolVal(m map[string]any, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

func mapStrStr(m map[string]any) map[string]string {
	result := make(map[string]string)
	for k, v := range m {
		if s, ok := v.(string); ok {
			result[k] = s
		}
	}
	return result
}

func isHTTPMethod(m string) bool {
	switch m {
	case "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS", "TRACE":
		return true
	}
	return false
}
