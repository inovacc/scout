package aria

import (
	"time"

	proto2 "github.com/inovacc/scout/internal/engine/lib/proto"
)

// ConvertForTest is exposed for the aria_test package only. It exercises the
// truncation path of convertAXNodes without needing a real browser.
func ConvertForTest(in []*proto2.AccessibilityAXNode, maxNodes int) ([]Node, bool) {
	cfg := &captureCfg{maxNodes: maxNodes}
	counter := 0
	out, truncated := convertAXNodes(in, cfg, "", &counter)
	return out, truncated
}

// ConvertWithPrefixForTest exposes convertAXNodes with a frame prefix and a
// shared counter so tests can verify ref numbering and frame-prefix behavior
// without a real browser.
func ConvertWithPrefixForTest(in []*proto2.AccessibilityAXNode, maxNodes int, framePrefix string, counter *int) ([]Node, bool) {
	cfg := &captureCfg{maxNodes: maxNodes}
	return convertAXNodes(in, cfg, framePrefix, counter)
}

// AXStringsForTest exposes axStrings for the aria_test package only.
func AXStringsForTest(ax *proto2.AccessibilityAXNode) (role, name, val string) {
	return axStrings(ax)
}

// CollectChildFrameIDsForTest exposes collectChildFrameIDs as string slices for
// the aria_test package only (PageFrameID is a string).
func CollectChildFrameIDsForTest(tree *proto2.PageFrameTree) []string {
	ids := collectChildFrameIDs(tree)
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = string(id)
	}
	return out
}

// DefaultCfgForTest exposes the default captureCfg values for the aria_test
// package only.
func DefaultCfgForTest() (maxNodes int, timeout time.Duration) {
	c := defaultCfg()
	return c.maxNodes, c.timeout
}

// ApplyOptionsForTest applies capture options to a default config and returns
// the resulting values, so tests can verify WithMaxNodes / WithCaptureTimeout.
func ApplyOptionsForTest(opts ...Option) (maxNodes int, timeout time.Duration) {
	c := defaultCfg()
	for _, opt := range opts {
		opt(c)
	}
	return c.maxNodes, c.timeout
}
