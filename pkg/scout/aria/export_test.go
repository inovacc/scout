package aria

import proto2 "github.com/inovacc/scout/internal/engine/lib/proto"

// ConvertForTest is exposed for the aria_test package only. It exercises the
// truncation path of convertAXNodes without needing a real browser.
func ConvertForTest(in []*proto2.AccessibilityAXNode, maxNodes int) ([]Node, bool) {
	cfg := &captureCfg{maxNodes: maxNodes}
	counter := 0
	out, truncated := convertAXNodes(in, cfg, "", &counter)
	return out, truncated
}
