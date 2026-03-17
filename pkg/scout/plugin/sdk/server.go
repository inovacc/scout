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
	modes       map[string]ModeHandler
	extractors  map[string]ExtractorHandler
	tools       map[string]ToolHandler
	commands    map[string]CommandHandler
	completions map[string]CompletionHandler
	auth        AuthHandler
	resources   map[string]ResourceHandler
	prompts     map[string]PromptHandler
	sinks       map[string]SinkHandler
	middleware  MiddlewareHandler
	eventHandler EventHandler
	encoder     *json.Encoder
	mu          sync.Mutex
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
		modes:       make(map[string]ModeHandler),
		extractors:  make(map[string]ExtractorHandler),
		tools:       make(map[string]ToolHandler),
		commands:    make(map[string]CommandHandler),
		completions: make(map[string]CompletionHandler),
		resources:   make(map[string]ResourceHandler),
		prompts:     make(map[string]PromptHandler),
		sinks:       make(map[string]SinkHandler),
		encoder:     json.NewEncoder(os.Stdout),
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

// RegisterAuth registers an auth provider handler.
func (s *Server) RegisterAuth(handler AuthHandler) {
	s.auth = handler
}

// RegisterResource registers a resource handler by URI.
func (s *Server) RegisterResource(uri string, handler ResourceHandler) {
	s.resources[uri] = handler
}

// RegisterPrompt registers a prompt handler by name.
func (s *Server) RegisterPrompt(name string, handler PromptHandler) {
	s.prompts[name] = handler
}

// RegisterSink registers an output sink handler by name.
func (s *Server) RegisterSink(name string, handler SinkHandler) {
	s.sinks[name] = handler
}

// RegisterMiddleware registers a browser middleware handler.
func (s *Server) RegisterMiddleware(handler MiddlewareHandler) {
	s.middleware = handler
}

// OnEvent registers an event handler for browser events.
func (s *Server) OnEvent(handler EventHandler) {
	s.eventHandler = handler
}

// RegisterCommand registers a CLI command handler.
func (s *Server) RegisterCommand(name string, handler CommandHandler) {
	s.commands[name] = handler
}

// RegisterCompletion registers a completion handler for a command.
func (s *Server) RegisterCompletion(name string, handler CompletionHandler) {
	s.completions[name] = handler
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

	case "event/emit":
		if s.eventHandler != nil {
			var event EventData
			if err := json.Unmarshal(req.Params, &event); err == nil {
				s.eventHandler.OnEvent(ctx, event)
			}
		}
		// Notifications don't get responses, but since this comes as a request
		// (with an ID), acknowledge it.
		s.sendResult(req.ID, map[string]any{"ok": true})

	case "middleware/before_navigate", "middleware/after_load", "middleware/before_extract", "middleware/on_error":
		if s.middleware == nil {
			s.sendError(req.ID, -32601, "no middleware handler registered")
			return
		}

		var hookCtx MiddlewareContext
		if err := json.Unmarshal(req.Params, &hookCtx); err != nil {
			s.sendError(req.ID, -32602, "invalid params")
			return
		}

		var result *MiddlewareResult
		var mwErr error

		switch req.Method {
		case "middleware/before_navigate":
			result, mwErr = s.middleware.BeforeNavigate(ctx, hookCtx)
		case "middleware/after_load":
			result, mwErr = s.middleware.AfterLoad(ctx, hookCtx)
		case "middleware/before_extract":
			result, mwErr = s.middleware.BeforeExtract(ctx, hookCtx)
		case "middleware/on_error":
			result, mwErr = s.middleware.OnError(ctx, hookCtx)
		}

		if mwErr != nil {
			s.sendError(req.ID, -32603, mwErr.Error())
			return
		}

		s.sendResult(req.ID, result)

	case "sink/init":
		var params struct {
			Name   string         `json:"name"`
			Config map[string]any `json:"config"`
		}

		if err := json.Unmarshal(req.Params, &params); err != nil {
			s.sendError(req.ID, -32602, "invalid params")
			return
		}

		handler, ok := s.sinks[params.Name]
		if !ok {
			s.sendError(req.ID, -32601, fmt.Sprintf("unknown sink: %s", params.Name))
			return
		}

		if err := handler.Init(ctx, params.Config); err != nil {
			s.sendError(req.ID, -32603, err.Error())
			return
		}

		s.sendResult(req.ID, map[string]any{"ok": true})

	case "sink/write":
		var params struct {
			Name    string           `json:"name"`
			Results []map[string]any `json:"results"`
		}

		if err := json.Unmarshal(req.Params, &params); err != nil {
			s.sendError(req.ID, -32602, "invalid params")
			return
		}

		handler, ok := s.sinks[params.Name]
		if !ok {
			s.sendError(req.ID, -32601, fmt.Sprintf("unknown sink: %s", params.Name))
			return
		}

		if err := handler.Write(ctx, params.Results); err != nil {
			s.sendError(req.ID, -32603, err.Error())
			return
		}

		s.sendResult(req.ID, map[string]any{"ok": true})

	case "sink/flush":
		var params struct {
			Name string `json:"name"`
		}

		if err := json.Unmarshal(req.Params, &params); err != nil {
			s.sendError(req.ID, -32602, "invalid params")
			return
		}

		handler, ok := s.sinks[params.Name]
		if !ok {
			s.sendError(req.ID, -32601, fmt.Sprintf("unknown sink: %s", params.Name))
			return
		}

		if err := handler.Flush(ctx); err != nil {
			s.sendError(req.ID, -32603, err.Error())
			return
		}

		s.sendResult(req.ID, map[string]any{"ok": true})

	case "sink/close":
		var params struct {
			Name string `json:"name"`
		}

		if err := json.Unmarshal(req.Params, &params); err != nil {
			s.sendError(req.ID, -32602, "invalid params")
			return
		}

		handler, ok := s.sinks[params.Name]
		if !ok {
			s.sendError(req.ID, -32601, fmt.Sprintf("unknown sink: %s", params.Name))
			return
		}

		if err := handler.Close(ctx); err != nil {
			s.sendError(req.ID, -32603, err.Error())
			return
		}

		s.sendResult(req.ID, map[string]any{"ok": true})

	case "resource/read":
		var params struct {
			URI string `json:"uri"`
		}

		if err := json.Unmarshal(req.Params, &params); err != nil {
			s.sendError(req.ID, -32602, "invalid params")
			return
		}

		handler, ok := s.resources[params.URI]
		if !ok {
			// Try prefix matching for templates.
			for uri, h := range s.resources {
				if params.URI == uri {
					handler = h
					ok = true

					break
				}
			}
		}

		if !ok {
			s.sendError(req.ID, -32601, fmt.Sprintf("unknown resource: %s", params.URI))
			return
		}

		content, mimeType, err := handler.Read(ctx, params.URI)
		if err != nil {
			s.sendError(req.ID, -32603, err.Error())
			return
		}

		s.sendResult(req.ID, map[string]string{"content": content, "mimeType": mimeType})

	case "resource/list":
		var resources []map[string]string

		for uri := range s.resources {
			resources = append(resources, map[string]string{"uri": uri})
		}

		s.sendResult(req.ID, resources)

	case "prompt/get":
		var params struct {
			Name      string            `json:"name"`
			Arguments map[string]string `json:"arguments"`
		}

		if err := json.Unmarshal(req.Params, &params); err != nil {
			s.sendError(req.ID, -32602, "invalid params")
			return
		}

		handler, ok := s.prompts[params.Name]
		if !ok {
			s.sendError(req.ID, -32601, fmt.Sprintf("unknown prompt: %s", params.Name))
			return
		}

		messages, err := handler.Get(ctx, params.Name, params.Arguments)
		if err != nil {
			s.sendError(req.ID, -32603, err.Error())
			return
		}

		s.sendResult(req.ID, messages)

	case "prompt/list":
		var prompts []map[string]string

		for name := range s.prompts {
			prompts = append(prompts, map[string]string{"name": name})
		}

		s.sendResult(req.ID, prompts)

	case "auth/login_url":
		if s.auth == nil {
			s.sendError(req.ID, -32601, "no auth handler registered")
			return
		}

		s.sendResult(req.ID, map[string]string{"url": s.auth.LoginURL()})

	case "auth/detect":
		if s.auth == nil {
			s.sendError(req.ID, -32601, "no auth handler registered")
			return
		}

		var state PageState
		if err := json.Unmarshal(req.Params, &state); err != nil {
			s.sendError(req.ID, -32602, "invalid params")
			return
		}

		detected, err := s.auth.Detect(ctx, state)
		if err != nil {
			s.sendError(req.ID, -32603, err.Error())
			return
		}

		s.sendResult(req.ID, map[string]bool{"detected": detected})

	case "auth/capture":
		if s.auth == nil {
			s.sendError(req.ID, -32601, "no auth handler registered")
			return
		}

		var state PageState
		if err := json.Unmarshal(req.Params, &state); err != nil {
			s.sendError(req.ID, -32602, "invalid params")
			return
		}

		session, err := s.auth.Capture(ctx, state)
		if err != nil {
			s.sendError(req.ID, -32603, err.Error())
			return
		}

		s.sendResult(req.ID, session)

	case "auth/validate":
		if s.auth == nil {
			s.sendError(req.ID, -32601, "no auth handler registered")
			return
		}

		var session SessionData
		if err := json.Unmarshal(req.Params, &session); err != nil {
			s.sendError(req.ID, -32602, "invalid params")
			return
		}

		valid, reason, err := s.auth.Validate(ctx, session)
		if err != nil {
			s.sendError(req.ID, -32603, err.Error())
			return
		}

		s.sendResult(req.ID, map[string]any{"valid": valid, "reason": reason})

	case "command/execute":
		var params CommandParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			s.sendError(req.ID, -32602, "invalid params")
			return
		}

		handler, ok := s.commands[params.Command]
		if !ok {
			s.sendError(req.ID, -32601, fmt.Sprintf("unknown command: %s", params.Command))
			return
		}

		result, err := handler.Execute(ctx, params)
		if err != nil {
			s.sendError(req.ID, -32603, err.Error())
			return
		}

		s.sendResult(req.ID, result)

	case "command/complete":
		var params CompletionParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			s.sendError(req.ID, -32602, "invalid params")
			return
		}

		handler, ok := s.completions[params.Command]
		if !ok {
			s.sendResult(req.ID, []string{})
			return
		}

		suggestions, err := handler.Complete(ctx, params)
		if err != nil {
			s.sendError(req.ID, -32603, err.Error())
			return
		}

		s.sendResult(req.ID, suggestions)

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

	if s.auth != nil {
		caps = append(caps, "auth_provider")
	}

	if s.middleware != nil {
		caps = append(caps, "browser_middleware")
	}

	if s.eventHandler != nil {
		caps = append(caps, "event_hook")
	}

	if len(s.sinks) > 0 {
		caps = append(caps, "output_sink")
	}

	if len(s.resources) > 0 {
		caps = append(caps, "mcp_resource")
	}

	if len(s.prompts) > 0 {
		caps = append(caps, "mcp_prompt")
	}

	if len(s.commands) > 0 {
		caps = append(caps, "cli_command")
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
