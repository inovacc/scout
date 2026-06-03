// Package flow records a browser flow's network traffic, turns it (via an LLM
// analyzer) into a reviewed declarative FlowSpec, and replays the underlying
// REST/GraphQL calls deterministically with no browser.
package flow

import (
	"encoding/json"
	"fmt"
	"os"
)

// Capture is the flow pipeline's normalized record of a captured flow: one
// CaptureEntry per HTTP request/response pair (bodies preserved). It is the
// input to Analyze and the golden for Verify.
type Capture struct {
	Version string         `json:"version"`
	Name    string         `json:"name,omitempty"`
	Entries []CaptureEntry `json:"entries"`
}

// CaptureEntry is a single request/response pair.
type CaptureEntry struct {
	Method      string            `json:"method"`
	URL         string            `json:"url"`
	ReqHeaders  map[string]string `json:"req_headers,omitempty"`
	ReqBody     string            `json:"req_body,omitempty"`
	Status      int               `json:"status"`
	RespHeaders map[string]string `json:"resp_headers,omitempty"`
	RespBody    string            `json:"resp_body,omitempty"`
	MimeType    string            `json:"mime_type,omitempty"`
}

// SaveCapture writes c as pretty JSON with owner-only permissions (0o600). A
// capture is the RAW recording of a browser flow: its request/response headers
// and bodies contain live secrets (auth tokens, cookies, CSRF, session IDs), so
// it must be protected at rest. (Only the derived FlowSpec is secret-free, via
// ${secret.*} refs.) os.WriteFile does not tighten a pre-existing file, so we
// also Chmod to defend against a previously world-readable path.
func SaveCapture(path string, c *Capture) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("scout: flow: marshal capture: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("scout: flow: write capture: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("scout: flow: chmod capture: %w", err)
	}
	return nil
}

// LoadCapture reads a capture.json from disk.
func LoadCapture(path string) (*Capture, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("scout: flow: read capture: %w", err)
	}
	var c Capture
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("scout: flow: parse capture: %w", err)
	}
	return &c, nil
}
