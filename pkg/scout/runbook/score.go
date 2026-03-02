package runbook

import (
	"fmt"
	"strings"
)

// SelectorScore holds the resilience assessment for a single CSS selector.
type SelectorScore struct {
	Selector string  `json:"selector"`
	Score    float64 `json:"score"`
	Tier     string  `json:"tier"` // "excellent", "good", "fair", "fragile"
	Reason   string  `json:"reason"`
}

// ScoreSelector evaluates a CSS selector's resilience and returns a score.
// Scores range from 0 (most fragile) to 1 (most resilient).
func ScoreSelector(sel string) SelectorScore {
	// Strip runbook-specific prefixes/suffixes for scoring.
	css := selectorToCSS(sel)
	if css == "" {
		return SelectorScore{Selector: sel, Score: 0, Tier: "fragile", Reason: "empty selector"}
	}

	score := 0.5 // baseline

	var reasons []string

	// Excellent: data-* attributes
	if strings.Contains(css, "[data-") {
		score = 0.95

		reasons = append(reasons, "uses data-* attribute")
	}

	// Excellent: ID selector
	if containsIDSelector(css) && score < 0.9 {
		score = 0.9

		reasons = append(reasons, "uses ID selector")
	}

	// Good: ARIA role
	if strings.Contains(css, "[role=") || strings.Contains(css, "[role=\"") {
		if score < 0.8 {
			score = 0.8

			reasons = append(reasons, "uses ARIA role")
		}
	}

	// Good: semantic HTML tags
	if containsSemanticTag(css) && score < 0.75 {
		score = 0.75

		reasons = append(reasons, "uses semantic HTML tag")
	}

	// Fair: class-based (only if nothing better matched)
	if strings.Contains(css, ".") && score <= 0.5 {
		score = 0.6

		reasons = append(reasons, "class-based selector")
	}

	// Penalties for fragile patterns
	depth := combinatorDepth(css)
	if depth >= 3 {
		penalty := float64(depth-2) * 0.15
		score -= penalty

		reasons = append(reasons, fmt.Sprintf("deeply nested (%d levels)", depth))
	}

	if containsPositional(css) {
		score -= 0.2

		reasons = append(reasons, "uses positional pseudo-class")
	}

	if isTagOnly(css) && !containsSemanticTag(css) {
		score = 0.3
		reasons = []string{"tag-only selector (no class, ID, or attribute)"}
	}

	// Clamp
	if score < 0 {
		score = 0
	}

	if score > 1 {
		score = 1
	}

	tier := scoreTier(score)

	reason := strings.Join(reasons, "; ")
	if reason == "" {
		reason = "generic selector"
	}

	return SelectorScore{
		Selector: sel,
		Score:    score,
		Tier:     tier,
		Reason:   reason,
	}
}

// ScoreRunbookSelectors scores every selector referenced in a runbook.
func ScoreRunbookSelectors(r *Runbook) map[string]SelectorScore {
	sels := collectSelectors(r)

	scores := make(map[string]SelectorScore, len(sels))
	for name, sel := range sels {
		scores[name] = ScoreSelector(sel)
	}

	return scores
}

// scoreTier maps a numeric score to a tier label.
func scoreTier(score float64) string {
	switch {
	case score >= 0.9:
		return "excellent"
	case score >= 0.7:
		return "good"
	case score >= 0.5:
		return "fair"
	default:
		return "fragile"
	}
}

// containsIDSelector checks for #id patterns (not inside attribute brackets).
func containsIDSelector(sel string) bool {
	for i, ch := range sel {
		if ch == '#' {
			// Make sure it's not inside [...].
			if i == 0 || sel[i-1] != '\\' {
				return true
			}
		}
	}

	return false
}

// semanticTags are HTML5 semantic elements that convey meaning.
var semanticTags = []string{
	"article", "nav", "header", "main", "section", "aside", "footer",
	"figure", "figcaption", "details", "summary", "dialog", "time",
}

// containsSemanticTag checks if the selector starts with or contains a semantic tag.
func containsSemanticTag(sel string) bool {
	lower := strings.ToLower(sel)
	for _, tag := range semanticTags {
		// Check if tag appears as a word boundary (start of selector, after space, or after >).
		idx := strings.Index(lower, tag)
		if idx < 0 {
			continue
		}
		// Verify it's a tag name, not part of a class/attr.
		if idx > 0 {
			prev := lower[idx-1]
			if prev != ' ' && prev != '>' && prev != '+' && prev != '~' {
				continue
			}
		}
		// Check end boundary.
		end := idx + len(tag)
		if end < len(lower) {
			next := lower[end]
			if next != ' ' && next != '>' && next != '+' && next != '~' && next != '.' && next != '#' && next != '[' && next != ':' && next != 0 {
				continue
			}
		}

		return true
	}

	return false
}

// combinatorDepth counts the number of CSS combinator levels (space, >, +, ~).
func combinatorDepth(sel string) int {
	depth := 0
	inBracket := false
	inQuote := byte(0)

	for i := 0; i < len(sel); i++ {
		ch := sel[i]
		if inQuote != 0 {
			if ch == inQuote {
				inQuote = 0
			}

			continue
		}

		switch ch {
		case '"', '\'':
			inQuote = ch
		case '[':
			inBracket = true
		case ']':
			inBracket = false
		case ' ', '>', '+', '~':
			if !inBracket {
				// Skip consecutive whitespace/combinators.
				for i+1 < len(sel) && (sel[i+1] == ' ' || sel[i+1] == '>' || sel[i+1] == '+' || sel[i+1] == '~') {
					i++
				}

				depth++
			}
		}
	}

	return depth
}

// containsPositional checks for nth-child, nth-of-type, etc.
func containsPositional(sel string) bool {
	lower := strings.ToLower(sel)

	return strings.Contains(lower, ":nth-child") ||
		strings.Contains(lower, ":nth-of-type") ||
		strings.Contains(lower, ":first-child") ||
		strings.Contains(lower, ":last-child") ||
		strings.Contains(lower, ":nth-last-child")
}

// isTagOnly checks if a selector is just a bare HTML tag name with no qualifiers.
func isTagOnly(sel string) bool {
	s := strings.TrimSpace(sel)
	if s == "" {
		return false
	}
	// Must be a simple word with no special CSS characters.
	for _, ch := range s {
		if ch == '.' || ch == '#' || ch == '[' || ch == ':' || ch == ' ' || ch == '>' || ch == '+' || ch == '~' || ch == '*' {
			return false
		}
	}

	return true
}
