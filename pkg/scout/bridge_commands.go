package scout

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// QueryDOM queries DOM elements via the bridge WebSocket.
// If all is true, returns all matching elements; otherwise returns the first match.
func (s *BridgeServer) QueryDOM(pageID, selector string, all bool) ([]map[string]any, error) {
	if s == nil {
		return nil, fmt.Errorf("scout: bridge: server is nil")
	}

	resp, err := s.Send(pageID, "dom.query", map[string]any{
		"selector": selector,
		"all":      all,
	})
	if err != nil {
		return nil, fmt.Errorf("scout: bridge: query DOM: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("scout: bridge: query DOM: unmarshal: %w", err)
	}

	if errStr, ok := result["error"]; ok {
		return nil, fmt.Errorf("scout: bridge: query DOM: %v", errStr)
	}

	if all {
		elems, _ := result["elements"].([]any)
		out := make([]map[string]any, 0, len(elems))
		for _, e := range elems {
			if m, ok := e.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out, nil
	}

	// Single element result.
	return []map[string]any{result}, nil
}

// ClickElement clicks an element matching selector via the bridge.
func (s *BridgeServer) ClickElement(pageID, selector string) error {
	if s == nil {
		return fmt.Errorf("scout: bridge: server is nil")
	}

	resp, err := s.Send(pageID, "dom.click", map[string]any{
		"selector": selector,
	})
	if err != nil {
		return fmt.Errorf("scout: bridge: click: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("scout: bridge: click: unmarshal: %w", err)
	}

	if errStr, ok := result["error"]; ok {
		return fmt.Errorf("scout: bridge: click: %v", errStr)
	}

	return nil
}

// TypeText types text into an element matching selector via the bridge.
func (s *BridgeServer) TypeText(pageID, selector, text string) error {
	if s == nil {
		return fmt.Errorf("scout: bridge: server is nil")
	}

	resp, err := s.Send(pageID, "dom.type", map[string]any{
		"selector": selector,
		"text":     text,
	})
	if err != nil {
		return fmt.Errorf("scout: bridge: type: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("scout: bridge: type: unmarshal: %w", err)
	}

	if errStr, ok := result["error"]; ok {
		return fmt.Errorf("scout: bridge: type: %v", errStr)
	}

	return nil
}

// InsertHTML inserts HTML adjacent to an element. Position must be one of:
// "beforebegin", "afterbegin", "beforeend", "afterend".
func (s *BridgeServer) InsertHTML(pageID, selector, position, html string) error {
	if s == nil {
		return fmt.Errorf("scout: bridge: server is nil")
	}

	resp, err := s.Send(pageID, "dom.insert", map[string]any{
		"selector": selector,
		"position": position,
		"html":     html,
	})
	if err != nil {
		return fmt.Errorf("scout: bridge: insert HTML: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("scout: bridge: insert HTML: unmarshal: %w", err)
	}

	if errStr, ok := result["error"]; ok {
		return fmt.Errorf("scout: bridge: insert HTML: %v", errStr)
	}

	return nil
}

// RemoveElement removes an element matching selector from the DOM.
func (s *BridgeServer) RemoveElement(pageID, selector string) error {
	if s == nil {
		return fmt.Errorf("scout: bridge: server is nil")
	}

	resp, err := s.Send(pageID, "dom.remove", map[string]any{
		"selector": selector,
	})
	if err != nil {
		return fmt.Errorf("scout: bridge: remove: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("scout: bridge: remove: unmarshal: %w", err)
	}

	if errStr, ok := result["error"]; ok {
		return fmt.Errorf("scout: bridge: remove: %v", errStr)
	}

	return nil
}

// ModifyAttribute sets an attribute on an element matching selector.
func (s *BridgeServer) ModifyAttribute(pageID, selector, attr, value string) error {
	if s == nil {
		return fmt.Errorf("scout: bridge: server is nil")
	}

	resp, err := s.Send(pageID, "dom.modifyAttr", map[string]any{
		"selector":  selector,
		"attribute": attr,
		"value":     value,
	})
	if err != nil {
		return fmt.Errorf("scout: bridge: modify attr: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("scout: bridge: modify attr: unmarshal: %w", err)
	}

	if errStr, ok := result["error"]; ok {
		return fmt.Errorf("scout: bridge: modify attr: %v", errStr)
	}

	return nil
}

// GetClipboard reads clipboard text from the page via the bridge.
func (s *BridgeServer) GetClipboard(pageID string) (string, error) {
	if s == nil {
		return "", fmt.Errorf("scout: bridge: server is nil")
	}

	resp, err := s.Send(pageID, "clipboard.read", map[string]any{})
	if err != nil {
		return "", fmt.Errorf("scout: bridge: clipboard read: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return "", fmt.Errorf("scout: bridge: clipboard read: unmarshal: %w", err)
	}

	if errStr, ok := result["error"]; ok {
		return "", fmt.Errorf("scout: bridge: clipboard read: %v", errStr)
	}

	text, _ := result["text"].(string)
	return text, nil
}

// SetClipboard writes text to the clipboard via the bridge.
func (s *BridgeServer) SetClipboard(pageID, text string) error {
	if s == nil {
		return fmt.Errorf("scout: bridge: server is nil")
	}

	resp, err := s.Send(pageID, "clipboard.write", map[string]any{
		"text": text,
	})
	if err != nil {
		return fmt.Errorf("scout: bridge: clipboard write: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("scout: bridge: clipboard write: unmarshal: %w", err)
	}

	if errStr, ok := result["error"]; ok {
		return fmt.Errorf("scout: bridge: clipboard write: %v", errStr)
	}

	return nil
}

// ListTabs lists all open browser tabs via the bridge extension background script.
// It sends to the first connected client (background script).
func (s *BridgeServer) ListTabs() ([]map[string]any, error) {
	if s == nil {
		return nil, fmt.Errorf("scout: bridge: server is nil")
	}

	clients := s.Clients()
	if len(clients) == 0 {
		return nil, fmt.Errorf("scout: bridge: no clients connected")
	}

	resp, err := s.Send(clients[0], "tab.list", map[string]any{})
	if err != nil {
		return nil, fmt.Errorf("scout: bridge: list tabs: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("scout: bridge: list tabs: unmarshal: %w", err)
	}

	tabs, _ := result["tabs"].([]any)
	out := make([]map[string]any, 0, len(tabs))
	for _, t := range tabs {
		if m, ok := t.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out, nil
}

// CloseTab closes a browser tab by its tab ID.
func (s *BridgeServer) CloseTab(tabID int) error {
	if s == nil {
		return fmt.Errorf("scout: bridge: server is nil")
	}

	clients := s.Clients()
	if len(clients) == 0 {
		return fmt.Errorf("scout: bridge: no clients connected")
	}

	resp, err := s.Send(clients[0], "tab.close", map[string]any{
		"tabId": tabID,
	})
	if err != nil {
		return fmt.Errorf("scout: bridge: close tab: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("scout: bridge: close tab: unmarshal: %w", err)
	}

	if errStr, ok := result["error"]; ok {
		return fmt.Errorf("scout: bridge: close tab: %v", errStr)
	}

	return nil
}

// ConsoleMessages retrieves captured console messages from the page.
// The content script must have console capture enabled via console.capture first.
func (s *BridgeServer) ConsoleMessages(pageID string) ([]map[string]any, error) {
	if s == nil {
		return nil, fmt.Errorf("scout: bridge: server is nil")
	}

	resp, err := s.Send(pageID, "console.get", map[string]any{})
	if err != nil {
		return nil, fmt.Errorf("scout: bridge: console messages: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("scout: bridge: console messages: unmarshal: %w", err)
	}

	if errStr, ok := result["error"]; ok {
		return nil, fmt.Errorf("scout: bridge: console messages: %v", errStr)
	}

	msgs, _ := result["messages"].([]any)
	out := make([]map[string]any, 0, len(msgs))
	for _, m := range msgs {
		if entry, ok := m.(map[string]any); ok {
			out = append(out, entry)
		}
	}
	return out, nil
}

// ObserveDOM starts a MutationObserver on elements matching selector.
// Mutations are emitted as bridge events of type "dom.mutation".
func (s *BridgeServer) ObserveDOM(pageID, selector string) error {
	if s == nil {
		return fmt.Errorf("scout: bridge: server is nil")
	}

	resp, err := s.Send(pageID, "dom.observe", map[string]any{
		"selector": selector,
	})
	if err != nil {
		return fmt.Errorf("scout: bridge: observe DOM: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("scout: bridge: observe DOM: unmarshal: %w", err)
	}

	if errStr, ok := result["error"]; ok {
		return fmt.Errorf("scout: bridge: observe DOM: %v", errStr)
	}

	return nil
}

// AutoFillForm finds a form by selector and fills each field by name/id matching
// the map keys with the map values. Input and change events are dispatched for
// framework compatibility.
func (s *BridgeServer) AutoFillForm(pageID, selector string, data map[string]string) error {
	if s == nil {
		return fmt.Errorf("scout: bridge: server is nil")
	}

	resp, err := s.Send(pageID, "form.autofill", map[string]any{
		"selector": selector,
		"data":     data,
	})
	if err != nil {
		return fmt.Errorf("scout: bridge: autofill form: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("scout: bridge: autofill form: unmarshal: %w", err)
	}

	if errStr, ok := result["error"]; ok {
		return fmt.Errorf("scout: bridge: autofill form: %v", errStr)
	}

	return nil
}

// DownloadFile fetches a URL via the page's fetch API (inheriting cookies/auth)
// and returns the response body as bytes. The data is base64-encoded over the bridge.
func (s *BridgeServer) DownloadFile(pageID, url string) ([]byte, error) {
	if s == nil {
		return nil, fmt.Errorf("scout: bridge: server is nil")
	}

	resp, err := s.Send(pageID, "fetch.download", map[string]any{
		"url": url,
	})
	if err != nil {
		return nil, fmt.Errorf("scout: bridge: download file: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("scout: bridge: download file: unmarshal: %w", err)
	}

	if errStr, ok := result["error"]; ok {
		return nil, fmt.Errorf("scout: bridge: download file: %v", errStr)
	}

	b64, _ := result["data"].(string)
	if b64 == "" {
		return []byte{}, nil
	}

	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("scout: bridge: download file: decode base64: %w", err)
	}

	return decoded, nil
}

// StartConsoleCapture enables console message interception on the page.
func (s *BridgeServer) StartConsoleCapture(pageID string) error {
	if s == nil {
		return fmt.Errorf("scout: bridge: server is nil")
	}

	resp, err := s.Send(pageID, "console.capture", map[string]any{})
	if err != nil {
		return fmt.Errorf("scout: bridge: console capture: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return fmt.Errorf("scout: bridge: console capture: unmarshal: %w", err)
	}

	if errStr, ok := result["error"]; ok {
		return fmt.Errorf("scout: bridge: console capture: %v", errStr)
	}

	return nil
}
