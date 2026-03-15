package sdk

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// Server is the plugin-side framework for handling JSON-RPC 2.0 requests from Scout.
type Server struct {
	modes      map[string]ModeHandler
	extractors map[string]ExtractorHandler
	tools      map[string]ToolHandler
	encoder    *json.Encoder
	mu         sync.Mutex
}

// ModeHandler handles scrape requests.
type ModeHandler interface {
	Scrape(ctx context.Context, params ScrapeParams) ([]Result, error)
}

// ExtractorHandler handles extract requests.
type ExtractorHandler interface {
	Extract(ctx context.Context, params ExtractParams) (any, error)
}

// ToolHandler handles MCP tool calls.
type ToolHandler interface {
	Call(ctx context.Context, arguments map[string]any) (*ToolResult, error)
}

// ToolHandlerFunc adapts a function to ToolHandler.
type ToolHandlerFunc func(ctx context.Context, arguments map[string]any) (*ToolResult, error)

func (f ToolHandlerFunc) Call(ctx context.Context, arguments map[string]any) (*ToolResult, error) {
	return f(ctx, arguments)
}

// ScrapeParams are the parameters for a scrape request.
type ScrapeParams struct {
	Mode    string         `json:"mode"`
	Options map[string]any `json:"options,omitempty"`
}

// ExtractParams are the parameters for an extract request.
type ExtractParams struct {
	Name   string         `json:"name"`
	HTML   string         `json:"html"`
	URL    string         `json:"url"`
	Params map[string]any `json:"params,omitempty"`
}

// Result is a scraper result emitted by a plugin.
type Result struct {
	Type      string         `json:"type"`
	Source    string         `json:"source"`
	ID        string         `json:"id"`
	Timestamp string         `json:"timestamp,omitempty"`
	Author    string         `json:"author,omitempty"`
	Content   string         `json:"content,omitempty"`
	URL       string         `json:"url,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// ToolResult is the result of an MCP tool call.
type ToolResult struct {
	Content []ContentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// ContentItem is a single content item in a tool result.
type ContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// TextResult creates a ToolResult with a single text content item.
func TextResult(text string) *ToolResult {
	return &ToolResult{
		Content: []ContentItem{{Type: "text", Text: text}},
	}
}

// ErrorResult creates an error ToolResult.
func ErrorResult(msg string) *ToolResult {
	return &ToolResult{
		Content: []ContentItem{{Type: "text", Text: msg}},
		IsError: true,
	}
}

// NewServer creates a new plugin server.
func NewServer() *Server {
	return &Server{
		modes:      make(map[string]ModeHandler),
		extractors: make(map[string]ExtractorHandler),
		tools:      make(map[string]ToolHandler),
		encoder:    json.NewEncoder(os.Stdout),
	}
}

// RegisterMode registers a scraper mode handler.
func (s *Server) RegisterMode(name string, handler ModeHandler) {
	s.modes[name] = handler
}

// RegisterExtractor registers an extractor handler.
func (s *Server) RegisterExtractor(name string, handler ExtractorHandler) {
	s.extractors[name] = handler
}

// RegisterTool registers an MCP tool handler.
func (s *Server) RegisterTool(name string, handler ToolHandler) {
	s.tools[name] = handler
}

// Run starts the JSON-RPC 2.0 message loop, reading from stdin and writing to stdout.
func (s *Server) Run() error {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	ctx := context.Background()

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			s.sendError(0, -32700, "parse error")
			continue
		}

		s.handleRequest(ctx, &req)
	}

	return scanner.Err()
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int64           `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

func (s *Server) handleRequest(ctx context.Context, req *request) {
	switch req.Method {
	case "initialize":
		s.sendResult(req.ID, map[string]any{
			"name":         "plugin",
			"version":      "1.0.0",
			"capabilities": s.capabilities(),
		})

	case "scrape":
		var params ScrapeParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			s.sendError(req.ID, -32602, "invalid params")
			return
		}

		handler, ok := s.modes[params.Mode]
		if !ok {
			s.sendError(req.ID, -32601, fmt.Sprintf("unknown mode: %s", params.Mode))
			return
		}

		results, err := handler.Scrape(ctx, params)
		if err != nil {
			s.sendError(req.ID, -32603, err.Error())
			return
		}

		s.sendResult(req.ID, results)

	case "extract":
		var params ExtractParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			s.sendError(req.ID, -32602, "invalid params")
			return
		}

		handler, ok := s.extractors[params.Name]
		if !ok {
			s.sendError(req.ID, -32601, fmt.Sprintf("unknown extractor: %s", params.Name))
			return
		}

		data, err := handler.Extract(ctx, params)
		if err != nil {
			s.sendError(req.ID, -32603, err.Error())
			return
		}

		s.sendResult(req.ID, data)

	case "tool/call":
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments,omitempty"`
		}

		if err := json.Unmarshal(req.Params, &params); err != nil {
			s.sendError(req.ID, -32602, "invalid params")
			return
		}

		handler, ok := s.tools[params.Name]
		if !ok {
			s.sendError(req.ID, -32601, fmt.Sprintf("unknown tool: %s", params.Name))
			return
		}

		result, err := handler.Call(ctx, params.Arguments)
		if err != nil {
			s.sendError(req.ID, -32603, err.Error())
			return
		}

		s.sendResult(req.ID, result)

	case "shutdown":
		s.sendResult(req.ID, map[string]any{})

		os.Exit(0)

	default:
		s.sendError(req.ID, -32601, fmt.Sprintf("method not found: %s", req.Method))
	}
}

func (s *Server) capabilities() []string {
	var caps []string
	if len(s.modes) > 0 {
		caps = append(caps, "scraper_mode")
	}

	if len(s.extractors) > 0 {
		caps = append(caps, "extractor")
	}

	if len(s.tools) > 0 {
		caps = append(caps, "mcp_tool")
	}

	return caps
}

// Emit sends a notification to Scout (for streaming results, progress, or logs).
func (s *Server) Emit(method string, params any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.encoder.Encode(map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
		"params":  params,
	})
}

// EmitResult sends a streamed scraper result notification.
func (s *Server) EmitResult(result Result) error {
	return s.Emit("result", result)
}

// EmitLog sends a log notification.
func (s *Server) EmitLog(level, message string) error {
	return s.Emit("log", map[string]string{"level": level, "message": message})
}

func (s *Server) sendResult(id int64, result any) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_ = s.encoder.Encode(map[string]any{ //nolint:errchkjson // fire-and-forget JSON-RPC response
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	})
}

func (s *Server) sendError(id int64, code int, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	_ = s.encoder.Encode(map[string]any{ //nolint:errchkjson // fire-and-forget JSON-RPC error
		"jsonrpc": "2.0",
		"id":      id,
		"error":   map[string]any{"code": code, "message": message},
	})
}
