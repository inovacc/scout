package scout

import (
	"encoding/json"
	"fmt"
	"slices"
)

// BridgeFallback provides CDP-based equivalents for bridge operations when the
// bridge extension WebSocket is not connected. It wraps a Page and falls back
// to direct CDP evaluation.
type BridgeFallback struct {
	page   *Page
	bridge *BridgeServer
	pageID string
}

// NewBridgeFallback creates a new BridgeFallback that wraps the given page.
// If a bridge server and pageID are provided, it will attempt bridge operations
// first and fall back to CDP if the bridge is not connected.
func NewBridgeFallback(page *Page) *BridgeFallback {
	if page == nil {
		return &BridgeFallback{}
	}

	var bs *BridgeServer
	if page.browser != nil {
		bs = page.browser.BridgeServer()
	}

	return &BridgeFallback{
		page:   page,
		bridge: bs,
	}
}

// SetPageID sets the bridge page ID for bridge-first operations.
func (f *BridgeFallback) SetPageID(id string) {
	if f == nil {
		return
	}

	f.pageID = id
}

// bridgeConnected returns true if the bridge server has the target client connected.
func (f *BridgeFallback) bridgeConnected() bool {
	if f == nil || f.bridge == nil || f.pageID == "" {
		return false
	}

	return slices.Contains(f.bridge.Clients(), f.pageID)
}

// Query queries DOM elements, falling back to CDP page.Eval if bridge is unavailable.
func (f *BridgeFallback) Query(selector string) ([]map[string]any, error) {
	if f == nil || f.page == nil {
		return nil, fmt.Errorf("scout: bridge: fallback: nil page")
	}

	// Try bridge first.
	if f.bridgeConnected() {
		result, err := f.bridge.QueryShadowDOM(f.pageID, selector)
		if err == nil {
			return result, nil
		}
	}

	// CDP fallback.
	res, err := f.page.Eval(`(selector) => {
		var els = document.querySelectorAll(selector);
		var results = [];
		for (var i = 0; i < els.length; i++) {
			results.push({
				tag: els[i].tagName.toLowerCase(),
				text: els[i].textContent.trim().substring(0, 200),
				html: els[i].outerHTML.substring(0, 500),
			});
		}
		return results;
	}`, selector)
	if err != nil {
		return nil, fmt.Errorf("scout: bridge: fallback query: %w", err)
	}

	raw, err := json.Marshal(res.Value)
	if err != nil {
		return nil, fmt.Errorf("scout: bridge: fallback query: marshal: %w", err)
	}

	var out []map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("scout: bridge: fallback query: unmarshal: %w", err)
	}

	return out, nil
}

// Click clicks an element by selector, falling back to CDP if bridge is unavailable.
func (f *BridgeFallback) Click(selector string) error {
	if f == nil || f.page == nil {
		return fmt.Errorf("scout: bridge: fallback: nil page")
	}

	// Try bridge first.
	if f.bridgeConnected() {
		err := f.bridge.ClickElement(f.pageID, selector)
		if err == nil {
			return nil
		}
	}

	// CDP fallback.
	el, err := f.page.Element(selector)
	if err != nil {
		return fmt.Errorf("scout: bridge: fallback click: %w", err)
	}

	if err := el.Click(); err != nil {
		return fmt.Errorf("scout: bridge: fallback click: %w", err)
	}

	return nil
}

// Type types text into an element by selector, falling back to CDP if bridge is unavailable.
func (f *BridgeFallback) Type(selector, text string) error {
	if f == nil || f.page == nil {
		return fmt.Errorf("scout: bridge: fallback: nil page")
	}

	// Try bridge first.
	if f.bridgeConnected() {
		err := f.bridge.TypeText(f.pageID, selector, text)
		if err == nil {
			return nil
		}
	}

	// CDP fallback.
	el, err := f.page.Element(selector)
	if err != nil {
		return fmt.Errorf("scout: bridge: fallback type: %w", err)
	}

	if err := el.Input(text); err != nil {
		return fmt.Errorf("scout: bridge: fallback type: %w", err)
	}

	return nil
}

// Eval evaluates JavaScript on the page, falling back to CDP. This method
// always uses CDP since bridge eval is not meaningfully different.
func (f *BridgeFallback) Eval(js string) (*EvalResult, error) {
	if f == nil || f.page == nil {
		return nil, fmt.Errorf("scout: bridge: fallback: nil page")
	}

	return f.page.Eval(js)
}
