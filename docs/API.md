# Scout API Reference

## MCP Tools Reference (18 Built-in)

### Browser Tools

| Tool | Description | Parameters | Required |
|------|-------------|------------|----------|
| `navigate` | Navigate the browser to a URL | `url` (string) | url |
| `click` | Click an element by CSS selector | `selector` (string) | selector |
| `type` | Type text into an element by CSS selector | `selector` (string), `text` (string) | selector, text |
| `extract` | Extract text from an element by CSS selector | `selector` (string) | selector |
| `eval` | Evaluate JavaScript in the page | `expression` (string) | expression |
| `back` | Navigate back in browser history | _none_ | |
| `forward` | Navigate forward in browser history | _none_ | |
| `wait` | Wait for a page condition (load, selector) | `selector` (string) | |

### Capture Tools

| Tool | Description | Parameters | Required |
|------|-------------|------------|----------|
| `screenshot` | Take a screenshot of the current page | `fullPage` (boolean) | |
| `snapshot` | Get the accessibility tree of the current page | `interactableOnly` (boolean), `maxDepth` (integer), `iframes` (boolean), `filter` (string) | |
| `pdf` | Generate a PDF of the current page | `landscape` (boolean), `printBackground` (boolean), `scale` (number, 0.1-2.0) | |

### Session Tools

| Tool | Description | Parameters | Required |
|------|-------------|------------|----------|
| `session_list` | List current session info (URL, title of current page) | _none_ | |
| `session_reset` | Close the current browser and page, forcing re-initialization on next use | _none_ | |
| `open` | Open a URL in a visible (headed) browser for manual inspection | `url` (string), `devtools` (boolean) | url |

### Swarm Tools

| Tool | Description | Parameters | Required |
|------|-------------|------------|----------|
| `swarm_crawl` | Crawl a website using multiple browser workers in parallel | `url` (string), `workers` (integer, default 2, max 8), `depth` (integer, default 2), `maxPages` (integer, default 50) | url |

### WebSocket Tools

| Tool | Description | Parameters | Required |
|------|-------------|------------|----------|
| `ws_listen` | Monitor WebSocket traffic on the current page | `urlFilter` (string), `duration` (integer, default 10, max 60 seconds) | |
| `ws_send` | Send a message to an active WebSocket connection via JavaScript evaluation | `script` (string) | script |
| `ws_connections` | List active WebSocket connections on the current page | _none_ | |

---

## Agent HTTP API Reference

The Agent HTTP server provides a REST API for AI agent frameworks to interact with Scout browser automation. Default listen address: `localhost:9000`.

### GET /health

Returns server health status.

**Response:**

```json
{
  "status": "ok",
  "tools": 9,
  "version": "1.0.0"
}
```

### GET /tools

Returns tool schemas in OpenAI function calling format. Alias: `GET /tools/openai`.

**Response:**

```json
[
  {
    "type": "function",
    "function": {
      "name": "navigate",
      "description": "Navigate the browser to a URL and wait for the page to load",
      "parameters": {
        "type": "object",
        "properties": {
          "url": { "type": "string", "description": "The URL to navigate to" }
        },
        "required": ["url"]
      }
    }
  }
]
```

### GET /tools/anthropic

Returns tool schemas in Anthropic `tool_use` format.

**Response:**

```json
[
  {
    "name": "navigate",
    "description": "Navigate the browser to a URL and wait for the page to load",
    "input_schema": {
      "type": "object",
      "properties": {
        "url": { "type": "string", "description": "The URL to navigate to" }
      },
      "required": ["url"]
    }
  }
]
```

### GET /tools/schema

Returns all tool schemas as formatted JSON (OpenAI format, indented).

**Response:** Same structure as `GET /tools` but pretty-printed.

### POST /call

Execute a tool by name.

**Request body:**

```json
{
  "name": "navigate",
  "arguments": {
    "url": "https://example.com"
  }
}
```

**Success response (200):**

```json
{
  "content": "Navigated to https://example.com (title: Example Domain)"
}
```

**Tool error response (200):**

```json
{
  "content": "element not found",
  "is_error": true
}
```

**Unknown tool response (404):**

```json
{
  "content": "agent: unknown tool \"foo\"",
  "is_error": true
}
```

**Invalid request (400):**

```json
{
  "error": "missing 'name' field"
}
```

### GET /metrics

Returns metrics in Prometheus text exposition format.

### GET /metrics/json

Returns metrics as JSON.

---

## Agent Provider Tools (9)

These tools are exposed via the `Provider` for integration with OpenAI and Anthropic agent frameworks. Use `OpenAITools()` for OpenAI function calling format or `AnthropicTools()` for Anthropic `tool_use` format.

| Tool | Description | Parameters | Required |
|------|-------------|------------|----------|
| `navigate` | Navigate the browser to a URL and wait for the page to load | `url` (string) | url |
| `screenshot` | Take a screenshot of the current page | `fullPage` (boolean) | |
| `extract_text` | Extract text content from an element using a CSS selector | `selector` (string) | selector |
| `click` | Click an element on the page | `selector` (string) | selector |
| `type_text` | Type text into an input element | `selector` (string), `text` (string) | selector, text |
| `markdown` | Extract the current page content as Markdown | `mainOnly` (boolean) | |
| `eval` | Evaluate JavaScript in the page context and return the result | `script` (string) | script |
| `page_url` | Get the current page URL | _none_ | |
| `page_title` | Get the current page title | _none_ | |

### Go Usage

```go
import "github.com/inovacc/scout/pkg/scout/agent"

browser, _ := scout.New()
provider := agent.NewProvider(browser)

// OpenAI integration
tools := provider.OpenAITools()

// Anthropic integration
tools := provider.AnthropicTools()

// Execute a tool
result, err := provider.Call(ctx, "navigate", map[string]any{"url": "https://example.com"})
```

### HTTP Server Usage

```go
srv, _ := agent.NewServer(agent.ServerConfig{
    Addr:     "localhost:9000",
    Headless: true,
    Stealth:  true,
})
defer srv.Close()
srv.ListenAndServe(ctx)
```

---

## MCP Resources (3)

Resources provide read-only access to page state via the MCP resource protocol.

| URI | Name | Description |
|-----|------|-------------|
| `scout://page/markdown` | Page Markdown | Returns the current page content converted to Markdown |
| `scout://page/url` | Page URL | Returns the current page URL as plain text |
| `scout://page/title` | Page Title | Returns the current page title as plain text |

### Usage

Resources are accessed via the MCP `resources/read` method:

```json
{
  "method": "resources/read",
  "params": {
    "uri": "scout://page/markdown"
  }
}
```

**Response:**

```json
{
  "contents": [
    {
      "uri": "scout://page/markdown",
      "text": "# Example Domain\n\nThis domain is for use in illustrative examples..."
    }
  ]
}
```
