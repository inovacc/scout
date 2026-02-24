package scout

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// WebMCPTool represents an MCP tool discovered on a web page.
type WebMCPTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	ServerURL   string          `json:"server_url,omitempty"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
	Source      string          `json:"source"` // "meta", "well-known", "link", "script"
}

// WebMCPToolResult holds the result of calling a web-exposed MCP tool.
type WebMCPToolResult struct {
	Content string `json:"content"`
	IsError bool   `json:"is_error"`
}

// DiscoverWebMCPTools scans the current page for MCP tool declarations.
// Checks: <meta name="mcp-server">, <meta name="mcp-tools">, <link rel="mcp">,
// <script type="application/mcp+json">, and /.well-known/mcp endpoint.
func (p *Page) DiscoverWebMCPTools() ([]WebMCPTool, error) {
	if p == nil || p.page == nil {
		return nil, nil
	}

	var tools []WebMCPTool

	// 1. Discover from meta and link tags and inline scripts via JS evaluation.
	jsResult, err := p.Eval(`() => {
		const result = { metaServer: '', metaTools: '', links: [], scripts: [] };

		// <meta name="mcp-server" content="url">
		const serverMeta = document.querySelector('meta[name="mcp-server"]');
		if (serverMeta) {
			result.metaServer = serverMeta.getAttribute('content') || '';
		}

		// <meta name="mcp-tools" content="json">
		const toolsMeta = document.querySelector('meta[name="mcp-tools"]');
		if (toolsMeta) {
			result.metaTools = toolsMeta.getAttribute('content') || '';
		}

		// <link rel="mcp" href="url">
		const mcpLinks = document.querySelectorAll('link[rel="mcp"]');
		mcpLinks.forEach(link => {
			result.links.push(link.getAttribute('href') || '');
		});

		// <script type="application/mcp+json">
		const mcpScripts = document.querySelectorAll('script[type="application/mcp+json"]');
		mcpScripts.forEach(script => {
			result.scripts.push(script.textContent || '');
		});

		return result;
	}`)
	if err != nil {
		return nil, fmt.Errorf("scout: webmcp: eval discovery: %w", err)
	}

	pageURL, _ := p.URL()
	origin := extractOrigin(pageURL)

	// Parse JS result.
	var disc struct {
		MetaServer string   `json:"metaServer"`
		MetaTools  string   `json:"metaTools"`
		Links      []string `json:"links"`
		Scripts    []string `json:"scripts"`
	}
	if err := jsResult.Decode(&disc); err != nil {
		return nil, fmt.Errorf("scout: webmcp: parse discovery result: %w", err)
	}

	// Process meta server tag — fetch tool list from server URL.
	if disc.MetaServer != "" {
		serverURL := resolveMCPURL(origin, disc.MetaServer)
		fetched, fetchErr := fetchMCPToolsJSON(serverURL)
		if fetchErr == nil {
			for i := range fetched {
				fetched[i].Source = "meta"
				if fetched[i].ServerURL == "" {
					fetched[i].ServerURL = serverURL
				}
			}
			tools = append(tools, fetched...)
		}
	}

	// Process meta tools tag — inline JSON tool definitions.
	if disc.MetaTools != "" {
		var metaTools []WebMCPTool
		if err := json.Unmarshal([]byte(disc.MetaTools), &metaTools); err == nil {
			for i := range metaTools {
				metaTools[i].Source = "meta"
			}
			tools = append(tools, metaTools...)
		}
	}

	// Process link tags.
	for _, href := range disc.Links {
		if href == "" {
			continue
		}
		linkURL := resolveMCPURL(origin, href)
		fetched, fetchErr := fetchMCPToolsJSON(linkURL)
		if fetchErr == nil {
			for i := range fetched {
				fetched[i].Source = "link"
			}
			tools = append(tools, fetched...)
		}
	}

	// Process inline scripts.
	for _, scriptContent := range disc.Scripts {
		if scriptContent == "" {
			continue
		}
		var scriptTools []WebMCPTool
		if err := json.Unmarshal([]byte(strings.TrimSpace(scriptContent)), &scriptTools); err == nil {
			for i := range scriptTools {
				scriptTools[i].Source = "script"
			}
			tools = append(tools, scriptTools...)
		}
	}

	// 2. Try /.well-known/mcp endpoint.
	if origin != "" {
		wellKnownURL := origin + "/.well-known/mcp"
		fetched, fetchErr := fetchMCPToolsJSON(wellKnownURL)
		if fetchErr == nil {
			for i := range fetched {
				fetched[i].Source = "well-known"
			}
			tools = append(tools, fetched...)
		}
	}

	// Deduplicate by name (first wins).
	seen := make(map[string]struct{})
	deduped := make([]WebMCPTool, 0, len(tools))
	for _, t := range tools {
		if _, ok := seen[t.Name]; ok {
			continue
		}
		seen[t.Name] = struct{}{}
		deduped = append(deduped, t)
	}

	return deduped, nil
}

// CallWebMCPTool invokes a discovered WebMCP tool by name with the given parameters.
// It first discovers tools on the page, finds the named tool, and calls it.
// If the tool has a server_url, it sends a JSON-RPC 2.0 POST request.
// Otherwise, it tries calling window.__mcp_tools[name](params) in the page context.
func (p *Page) CallWebMCPTool(name string, params map[string]any) (*WebMCPToolResult, error) {
	if p == nil || p.page == nil {
		return nil, fmt.Errorf("scout: webmcp: nil page")
	}

	tools, err := p.DiscoverWebMCPTools()
	if err != nil {
		return nil, fmt.Errorf("scout: webmcp: discover: %w", err)
	}

	var tool *WebMCPTool
	for i := range tools {
		if tools[i].Name == name {
			tool = &tools[i]
			break
		}
	}
	if tool == nil {
		return nil, fmt.Errorf("scout: webmcp: tool %q not found", name)
	}

	// If tool has a server URL, use JSON-RPC 2.0.
	if tool.ServerURL != "" {
		return callToolViaHTTP(tool.ServerURL, name, params)
	}

	// Otherwise try calling via page JS.
	return p.callToolViaJS(name, params)
}

// callToolViaHTTP sends a JSON-RPC 2.0 request to the tool's server URL.
func callToolViaHTTP(serverURL, toolName string, params map[string]any) (*WebMCPToolResult, error) {
	reqBody := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      toolName,
			"arguments": params,
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("scout: webmcp: marshal request: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(serverURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("scout: webmcp: http call: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("scout: webmcp: read response: %w", err)
	}

	var rpcResp struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("scout: webmcp: parse response: %w", err)
	}

	if rpcResp.Error != nil {
		return &WebMCPToolResult{
			Content: rpcResp.Error.Message,
			IsError: true,
		}, nil
	}

	var content string
	for _, c := range rpcResp.Result.Content {
		if content != "" {
			content += "\n"
		}
		content += c.Text
	}

	return &WebMCPToolResult{
		Content: content,
		IsError: rpcResp.Result.IsError,
	}, nil
}

// callToolViaJS tries to invoke window.__mcp_tools[name](params) in the page.
func (p *Page) callToolViaJS(name string, params map[string]any) (*WebMCPToolResult, error) {
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("scout: webmcp: marshal params: %w", err)
	}

	js := fmt.Sprintf(`() => {
		if (typeof window.__mcp_tools === 'undefined' || typeof window.__mcp_tools[%q] !== 'function') {
			return JSON.stringify({ content: 'tool not callable', is_error: true });
		}
		try {
			const result = window.__mcp_tools[%q](%s);
			if (typeof result === 'string') {
				return JSON.stringify({ content: result, is_error: false });
			}
			return JSON.stringify({ content: JSON.stringify(result), is_error: false });
		} catch (e) {
			return JSON.stringify({ content: e.message, is_error: true });
		}
	}`, name, name, string(paramsJSON))

	evalResult, err := p.Eval(js)
	if err != nil {
		return nil, fmt.Errorf("scout: webmcp: eval call: %w", err)
	}

	var result WebMCPToolResult
	if err := json.Unmarshal([]byte(evalResult.String()), &result); err != nil {
		return nil, fmt.Errorf("scout: webmcp: parse js result: %w", err)
	}

	return &result, nil
}

// fetchMCPToolsJSON fetches a JSON endpoint and parses it as a list of WebMCPTool.
func fetchMCPToolsJSON(rawURL string) ([]WebMCPTool, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(rawURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var tools []WebMCPTool
	if err := json.Unmarshal(body, &tools); err != nil {
		return nil, err
	}

	return tools, nil
}

// extractOrigin returns the scheme + host from a URL string.
func extractOrigin(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

// resolveMCPURL resolves a potentially relative URL against an origin.
func resolveMCPURL(origin, ref string) string {
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return ref
	}
	return strings.TrimRight(origin, "/") + "/" + strings.TrimLeft(ref, "/")
}
