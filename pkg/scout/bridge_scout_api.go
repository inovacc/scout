package scout

import (
	"encoding/json"
	"fmt"
)

// CallExposed calls a JavaScript function registered via window.__scout.expose()
// in the page identified by pageID. The function is invoked with the given arguments
// and the result is returned as raw JSON.
func (s *BridgeServer) CallExposed(pageID, funcName string, args ...any) (json.RawMessage, error) {
	if s == nil {
		return nil, fmt.Errorf("scout: bridge: server is nil")
	}

	resp, err := s.Send(pageID, "__scout_call_exposed", map[string]any{
		"name": funcName,
		"args": args,
	})
	if err != nil {
		return nil, fmt.Errorf("scout: bridge: call exposed %q: %w", funcName, err)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("scout: bridge: call exposed %q: unmarshal: %w", funcName, err)
	}

	if errStr, ok := result["error"]; ok {
		return nil, fmt.Errorf("scout: bridge: call exposed %q: %v", funcName, errStr)
	}

	raw, err := json.Marshal(result["result"])
	if err != nil {
		return nil, fmt.Errorf("scout: bridge: call exposed %q: marshal result: %w", funcName, err)
	}

	return raw, nil
}

// EmitEvent emits an event to the page's window.__scout.on() listeners.
func (s *BridgeServer) EmitEvent(pageID, eventName string, data any) error {
	if s == nil {
		return fmt.Errorf("scout: bridge: server is nil")
	}

	resp, err := s.Send(pageID, "__scout_emit_event", map[string]any{
		"event": eventName,
		"data":  data,
	})
	if err != nil {
		return fmt.Errorf("scout: bridge: emit event %q: %w", eventName, err)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("scout: bridge: emit event %q: unmarshal: %w", eventName, err)
	}

	if errStr, ok := result["error"]; ok {
		return fmt.Errorf("scout: bridge: emit event %q: %v", eventName, errStr)
	}

	return nil
}

// QueryShadowDOM queries the page's DOM with shadow DOM piercing. It walks into
// open shadow roots recursively to find elements matching the CSS selector.
func (s *BridgeServer) QueryShadowDOM(pageID, selector string) ([]map[string]any, error) {
	if s == nil {
		return nil, fmt.Errorf("scout: bridge: server is nil")
	}

	resp, err := s.Send(pageID, "__scout_shadow_query", map[string]any{
		"selector": selector,
		"all":      true,
	})
	if err != nil {
		return nil, fmt.Errorf("scout: bridge: shadow query: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("scout: bridge: shadow query: unmarshal: %w", err)
	}

	if errStr, ok := result["error"]; ok {
		return nil, fmt.Errorf("scout: bridge: shadow query: %v", errStr)
	}

	elems, _ := result["elements"].([]any)
	out := make([]map[string]any, 0, len(elems))
	for _, e := range elems {
		if m, ok := e.(map[string]any); ok {
			out = append(out, m)
		}
	}

	return out, nil
}

// ListFrames returns information about all frames in the page.
func (s *BridgeServer) ListFrames(pageID string) ([]map[string]any, error) {
	if s == nil {
		return nil, fmt.Errorf("scout: bridge: server is nil")
	}

	resp, err := s.Send(pageID, "__scout_list_frames", map[string]any{})
	if err != nil {
		return nil, fmt.Errorf("scout: bridge: list frames: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("scout: bridge: list frames: unmarshal: %w", err)
	}

	if errStr, ok := result["error"]; ok {
		return nil, fmt.Errorf("scout: bridge: list frames: %v", errStr)
	}

	frames, _ := result["frames"].([]any)
	out := make([]map[string]any, 0, len(frames))
	for _, f := range frames {
		if m, ok := f.(map[string]any); ok {
			out = append(out, m)
		}
	}

	return out, nil
}

// SendToFrame sends a command to a specific frame in the page. The frameIndex
// identifies which frame to target (0-based, matching the order from ListFrames).
func (s *BridgeServer) SendToFrame(pageID string, frameIndex int, method string, params any) (*BridgeMessage, error) {
	if s == nil {
		return nil, fmt.Errorf("scout: bridge: server is nil")
	}

	return s.Send(pageID, "__scout_send_to_frame", map[string]any{
		"frameIndex": frameIndex,
		"method":     method,
		"params":     params,
	})
}
