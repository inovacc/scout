package scout

import (
	"math"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// extractMainContent finds the highest-scoring content node in the HTML tree.
// It returns the node that most likely contains the main article/content.
func extractMainContent(doc *html.Node) *html.Node {
	var best *html.Node
	bestScore := math.MinInt32

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			score := scoreNode(n)
			if score > bestScore {
				bestScore = score
				best = n
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	if best == nil {
		return doc
	}
	return best
}

func scoreNode(n *html.Node) int {
	if n.Type != html.ElementNode {
		return 0
	}

	score := 0

	// Tag-based scoring
	switch n.DataAtom {
	case atom.Article:
		score += 20
	case atom.Main:
		score += 15
	case atom.Section:
		score += 5
	case atom.Div:
		score += 2
	case atom.P:
		score += 2
	case atom.Nav:
		return -25
	case atom.Footer:
		return -25
	case atom.Aside:
		return -20
	case atom.Header:
		return -10
	case atom.Form:
		return -10
	case atom.Script, atom.Style, atom.Noscript:
		return -100
	}

	// Class/ID scoring
	classID := strings.ToLower(getAttr(n, "class") + " " + getAttr(n, "id"))
	negativePatterns := []string{"sidebar", "ad", "menu", "comment", "banner", "widget", "popup", "modal", "nav", "footer"}
	for _, p := range negativePatterns {
		if strings.Contains(classID, p) {
			score -= 15
		}
	}
	positivePatterns := []string{"article", "content", "main", "post", "entry", "body", "text"}
	for _, p := range positivePatterns {
		if strings.Contains(classID, p) {
			score += 15
		}
	}

	// Text length bonus
	text := innerText(n)
	textLen := len(strings.TrimSpace(text))
	if textLen < 25 {
		score -= 10
	} else {
		score += int(math.Log(float64(textLen))) * 2
	}

	// Link density penalty
	linkLen := linkTextLength(n)
	if textLen > 0 {
		density := float64(linkLen) / float64(textLen)
		if density > 0.5 {
			score -= 20
		}
	}

	return score
}

func innerText(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			sb.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return sb.String()
}

func linkTextLength(n *html.Node) int {
	total := 0
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.DataAtom == atom.A {
			total += len(strings.TrimSpace(innerText(n)))
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return total
}

func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
