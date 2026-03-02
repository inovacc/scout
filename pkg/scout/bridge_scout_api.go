package scout

import (
	"encoding/base64"
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

// AutoFillFormViaScout fills a form using the window.__scout API as a fallback path.
// It evaluates JavaScript directly via CDP to fill form fields by name/id.
func (f *BridgeFallback) AutoFillForm(selector string, data map[string]string) error {
	if f == nil || f.page == nil {
		return fmt.Errorf("scout: bridge: fallback: nil page")
	}

	// Try bridge first.
	if f.bridgeConnected() {
		err := f.bridge.AutoFillForm(f.pageID, selector, data)
		if err == nil {
			return nil
		}
	}

	// CDP fallback via page eval.
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("scout: bridge: fallback autofill: marshal data: %w", err)
	}

	_, err = f.page.Eval(`(selector, dataJSON) => {
		var form = document.querySelector(selector);
		if (!form) throw new Error("form not found: " + selector);
		var data = JSON.parse(dataJSON);
		var keys = Object.keys(data);
		for (var i = 0; i < keys.length; i++) {
			var key = keys[i];
			var value = data[key];
			var field = form.querySelector('[name="' + key + '"]') ||
			            form.querySelector('#' + key);
			if (!field) continue;
			if (field.tagName === "SELECT") {
				field.value = value;
			} else if (field.type === "checkbox" || field.type === "radio") {
				field.checked = (value === "true" || value === "1" || value === "on");
			} else {
				field.value = value;
			}
			field.dispatchEvent(new Event("input", { bubbles: true }));
			field.dispatchEvent(new Event("change", { bubbles: true }));
		}
	}`, selector, string(dataJSON))
	if err != nil {
		return fmt.Errorf("scout: bridge: fallback autofill: %w", err)
	}

	return nil
}

// FetchFileViaScout downloads a file using the window.__scout API as a fallback path.
// It evaluates JavaScript directly via CDP using XMLHttpRequest to fetch the URL.
func (f *BridgeFallback) FetchFile(url string) ([]byte, error) {
	if f == nil || f.page == nil {
		return nil, fmt.Errorf("scout: bridge: fallback: nil page")
	}

	// Try bridge first.
	if f.bridgeConnected() {
		data, err := f.bridge.DownloadFile(f.pageID, url)
		if err == nil {
			return data, nil
		}
	}

	// CDP fallback via page eval.
	res, err := f.page.Eval(`(url) => {
		var xhr = new XMLHttpRequest();
		xhr.open("GET", url, false);
		xhr.overrideMimeType("text/plain; charset=x-user-defined");
		xhr.send(null);
		if (xhr.status < 200 || xhr.status >= 300) {
			throw new Error("fetch failed: HTTP " + xhr.status);
		}
		var raw = xhr.responseText;
		var bytes = new Uint8Array(raw.length);
		for (var i = 0; i < raw.length; i++) {
			bytes[i] = raw.charCodeAt(i) & 0xff;
		}
		var binary = "";
		for (var j = 0; j < bytes.length; j++) {
			binary += String.fromCharCode(bytes[j]);
		}
		return btoa(binary);
	}`, url)
	if err != nil {
		return nil, fmt.Errorf("scout: bridge: fallback fetch file: %w", err)
	}

	b64, ok := res.Value.(string)
	if !ok {
		return nil, fmt.Errorf("scout: bridge: fallback fetch file: unexpected result type")
	}

	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("scout: bridge: fallback fetch file: decode base64: %w", err)
	}

	return decoded, nil
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
