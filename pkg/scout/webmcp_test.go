package scout

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func init() {
	registerTestRoutes(func(mux *http.ServeMux) {
		// Page with meta and link tags pointing to MCP endpoints.
		mux.HandleFunc("/webmcp-meta", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head>
<title>WebMCP Meta Page</title>
<meta name="mcp-server" content="/mcp-api">
<link rel="mcp" href="/mcp-tools.json">
</head><body><p>Page with MCP meta tags</p></body></html>`)
		})

		// JSON endpoint returning tool definitions (for link tag).
		mux.HandleFunc("/mcp-tools.json", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `[
				{"name":"link-tool","description":"A tool from link tag","input_schema":{"type":"object"}}
			]`)
		})

		// Mock JSON-RPC MCP API endpoint (for meta server).
		mux.HandleFunc("/mcp-api", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			if r.Method == http.MethodGet {
				// GET returns tool list.
				_, _ = fmt.Fprint(w, `[
					{"name":"greet","description":"Say hello","server_url":""},
					{"name":"add","description":"Add two numbers","server_url":""}
				]`)

				return
			}
			// POST handles JSON-RPC tool calls.
			body, _ := io.ReadAll(r.Body)

			var req struct {
				Method string `json:"method"`
				Params struct {
					Name      string         `json:"name"`
					Arguments map[string]any `json:"arguments"`
				} `json:"params"`
			}
			if err := json.Unmarshal(body, &req); err != nil {
				_, _ = fmt.Fprint(w, `{"error":{"message":"bad request"}}`)
				return
			}

			switch req.Params.Name {
			case "greet":
				name, _ := req.Params.Arguments["name"].(string)
				_, _ = fmt.Fprintf(w, `{"result":{"content":[{"text":"Hello, %s!"}],"isError":false}}`, name)
			case "add":
				a, _ := req.Params.Arguments["a"].(float64)
				b, _ := req.Params.Arguments["b"].(float64)
				_, _ = fmt.Fprintf(w, `{"result":{"content":[{"text":"%.0f"}],"isError":false}}`, a+b)
			default:
				_, _ = fmt.Fprint(w, `{"error":{"message":"unknown tool"}}`)
			}
		})

		// Well-known MCP endpoint.
		mux.HandleFunc("/.well-known/mcp", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `[
				{"name":"well-known-tool","description":"Discovered via well-known"}
			]`)
		})

		// Page with inline script type="application/mcp+json".
		mux.HandleFunc("/webmcp-script", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>WebMCP Script Page</title></head><body>
<script type="application/mcp+json">
[{"name":"script-tool","description":"From inline script"}]
</script>
<p>Page with inline MCP script</p>
</body></html>`)
		})

		// Page with JS-callable tools via window.__mcp_tools.
		mux.HandleFunc("/webmcp-js", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>WebMCP JS Page</title>
<meta name="mcp-tools" content='[{"name":"js-echo","description":"Echo via JS"}]'>
</head><body>
<script>
window.__mcp_tools = {
	"js-echo": function(params) { return "echo: " + params.message; }
};
</script>
<p>Page with JS MCP tools</p>
</body></html>`)
		})

		// Page with no MCP declarations.
		mux.HandleFunc("/webmcp-none", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Plain Page</title></head>
<body><p>No MCP here</p></body></html>`)
		})
	})
}

func TestDiscoverWebMCPTools_Meta(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(ts.URL + "/webmcp-meta")
	if err != nil {
		t.Fatalf("NewPage failed: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad failed: %v", err)
	}

	tools, err := page.DiscoverWebMCPTools()
	if err != nil {
		t.Fatalf("DiscoverWebMCPTools failed: %v", err)
	}

	if len(tools) == 0 {
		t.Fatal("expected to discover tools, got none")
	}

	// Should find tools from meta server (/mcp-api), link (/mcp-tools.json), and well-known.
	names := make(map[string]string)
	for _, tool := range tools {
		names[tool.Name] = tool.Source
	}

	if _, ok := names["greet"]; !ok {
		t.Error("expected 'greet' tool from meta server")
	}

	if _, ok := names["link-tool"]; !ok {
		t.Error("expected 'link-tool' from link tag")
	}

	if _, ok := names["well-known-tool"]; !ok {
		t.Error("expected 'well-known-tool' from .well-known/mcp")
	}
}

func TestDiscoverWebMCPTools_WellKnown(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	b := newTestBrowser(t)

	// webmcp-none has no meta/link but well-known is still served.
	page, err := b.NewPage(ts.URL + "/webmcp-none")
	if err != nil {
		t.Fatalf("NewPage failed: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad failed: %v", err)
	}

	tools, err := page.DiscoverWebMCPTools()
	if err != nil {
		t.Fatalf("DiscoverWebMCPTools failed: %v", err)
	}

	found := false

	for _, tool := range tools {
		if tool.Name == "well-known-tool" && tool.Source == "well-known" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected 'well-known-tool' from .well-known/mcp endpoint")
	}
}

func TestDiscoverWebMCPTools_Script(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(ts.URL + "/webmcp-script")
	if err != nil {
		t.Fatalf("NewPage failed: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad failed: %v", err)
	}

	tools, err := page.DiscoverWebMCPTools()
	if err != nil {
		t.Fatalf("DiscoverWebMCPTools failed: %v", err)
	}

	found := false

	for _, tool := range tools {
		if tool.Name == "script-tool" && tool.Source == "script" {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected 'script-tool' from inline script")
	}
}

func TestDiscoverWebMCPTools_None(t *testing.T) {
	// Use an isolated server that does NOT serve .well-known/mcp.
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Bare Page</title></head><body><p>Nothing</p></body></html>`)
	})
	mux.HandleFunc("/.well-known/mcp", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(ts.URL + "/")
	if err != nil {
		t.Fatalf("NewPage failed: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad failed: %v", err)
	}

	tools, err := page.DiscoverWebMCPTools()
	if err != nil {
		t.Fatalf("DiscoverWebMCPTools failed: %v", err)
	}

	if len(tools) != 0 {
		t.Errorf("expected 0 tools, got %d: %+v", len(tools), tools)
	}
}

func TestDiscoverWebMCPTools_NilPage(t *testing.T) {
	var p *Page

	tools, err := p.DiscoverWebMCPTools()
	if err != nil {
		t.Fatalf("expected nil error for nil page, got: %v", err)
	}

	if len(tools) != 0 {
		t.Errorf("expected 0 tools for nil page, got %d", len(tools))
	}
}

func TestCallWebMCPTool(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(ts.URL + "/webmcp-meta")
	if err != nil {
		t.Fatalf("NewPage failed: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad failed: %v", err)
	}

	result, err := page.CallWebMCPTool("greet", map[string]any{"name": "World"})
	if err != nil {
		t.Fatalf("CallWebMCPTool failed: %v", err)
	}

	if result.IsError {
		t.Errorf("expected no error, got: %s", result.Content)
	}

	if !strings.Contains(result.Content, "Hello, World!") {
		t.Errorf("content = %q, want to contain 'Hello, World!'", result.Content)
	}
}

func TestCallWebMCPTool_Add(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(ts.URL + "/webmcp-meta")
	if err != nil {
		t.Fatalf("NewPage failed: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad failed: %v", err)
	}

	result, err := page.CallWebMCPTool("add", map[string]any{"a": 3.0, "b": 4.0})
	if err != nil {
		t.Fatalf("CallWebMCPTool failed: %v", err)
	}

	if result.IsError {
		t.Errorf("expected no error, got: %s", result.Content)
	}

	if result.Content != "7" {
		t.Errorf("content = %q, want '7'", result.Content)
	}
}

func TestCallWebMCPTool_NotFound(t *testing.T) {
	// Use isolated server with no well-known to avoid finding any tools.
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Empty</title></head><body></body></html>`)
	})
	mux.HandleFunc("/.well-known/mcp", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(ts.URL + "/")
	if err != nil {
		t.Fatalf("NewPage failed: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad failed: %v", err)
	}

	_, err = page.CallWebMCPTool("nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for nonexistent tool")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want to contain 'not found'", err.Error())
	}
}

func TestCallWebMCPTool_ViaJS(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(ts.URL + "/webmcp-js")
	if err != nil {
		t.Fatalf("NewPage failed: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad failed: %v", err)
	}

	result, err := page.CallWebMCPTool("js-echo", map[string]any{"message": "hello"})
	if err != nil {
		t.Fatalf("CallWebMCPTool failed: %v", err)
	}

	if result.IsError {
		t.Errorf("expected no error, got: %s", result.Content)
	}

	if !strings.Contains(result.Content, "echo: hello") {
		t.Errorf("content = %q, want to contain 'echo: hello'", result.Content)
	}
}

func TestCallWebMCPTool_NilPage(t *testing.T) {
	var p *Page

	_, err := p.CallWebMCPTool("test", nil)
	if err == nil {
		t.Fatal("expected error for nil page")
	}
}

func TestWebMCPRegistry(t *testing.T) {
	r := NewWebMCPRegistry()

	// Empty registry.
	if got := r.All(); len(got) != 0 {
		t.Errorf("expected 0 tools, got %d", len(got))
	}

	if _, ok := r.Get("example.com/greet"); ok {
		t.Error("expected Get to return false for empty registry")
	}

	// Register tools.
	r.Register("example.com", []WebMCPTool{
		{Name: "greet", Description: "Say hello"},
		{Name: "add", Description: "Add numbers"},
	})

	all := r.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(all))
	}

	tool, ok := r.Get("example.com/greet")
	if !ok {
		t.Fatal("expected to find 'example.com/greet'")
	}

	if tool.Name != "greet" {
		t.Errorf("tool.Name = %q, want 'greet'", tool.Name)
	}

	// Register from different origin.
	r.Register("other.com", []WebMCPTool{
		{Name: "greet", Description: "Other greet"},
	})

	all = r.All()
	if len(all) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(all))
	}

	// Clear.
	r.Clear()

	if got := r.All(); len(got) != 0 {
		t.Errorf("expected 0 tools after Clear, got %d", len(got))
	}
}

func TestWithWebMCPAutoDiscover(t *testing.T) {
	o := defaults()
	if o.webmcpAutoDiscover {
		t.Error("expected webmcpAutoDiscover to default to false")
	}

	WithWebMCPAutoDiscover()(o)

	if !o.webmcpAutoDiscover {
		t.Error("expected webmcpAutoDiscover to be true after WithWebMCPAutoDiscover")
	}
}
