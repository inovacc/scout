package scout

import "github.com/go-rod/rod"

// SelectorType specifies how a <select> option selector matches content.
// These re-export rod's SelectorType constants for convenience.
type SelectorType = rod.SelectorType

const (
	// SelectorText matches options by their visible text content (regex).
	SelectorText SelectorType = rod.SelectorTypeText
	// SelectorCSS matches options using a CSS selector.
	SelectorCSS SelectorType = rod.SelectorTypeCSSSector
	// SelectorRegex matches options by regex against their text.
	SelectorRegex SelectorType = rod.SelectorTypeRegex
)
