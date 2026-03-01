package scout

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func parseHTML(t *testing.T, s string) *html.Node {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(s))
	if err != nil {
		t.Fatalf("parse HTML: %v", err)
	}
	return doc
}

func TestScoreNode_Tags(t *testing.T) {
	tests := []struct {
		name     string
		atom     atom.Atom
		wantSign int // 1=positive, -1=negative, 0=zero
	}{
		{"article", atom.Article, 1},
		{"main", atom.Main, 1},
		{"section", atom.Section, 1},
		{"div", atom.Div, 1},
		{"nav", atom.Nav, -1},
		{"footer", atom.Footer, -1},
		{"aside", atom.Aside, -1},
		{"header", atom.Header, -1},
		{"form", atom.Form, -1},
		{"script", atom.Script, -1},
		{"style", atom.Style, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &html.Node{
				Type:     html.ElementNode,
				DataAtom: tt.atom,
				Data:     tt.atom.String(),
			}
			// Add text child so text length doesn't dominate
			n.AppendChild(&html.Node{Type: html.TextNode, Data: strings.Repeat("word ", 20)})
			score := scoreNode(n)
			switch tt.wantSign {
			case 1:
				if score <= 0 {
					t.Errorf("expected positive score for %s, got %d", tt.name, score)
				}
			case -1:
				if score >= 0 {
					t.Errorf("expected negative score for %s, got %d", tt.name, score)
				}
			}
		})
	}
}

func TestScoreNode_NonElement(t *testing.T) {
	n := &html.Node{Type: html.TextNode, Data: "hello"}
	if score := scoreNode(n); score != 0 {
		t.Errorf("text node should score 0, got %d", score)
	}
}

func TestScoreNode_PositiveClassID(t *testing.T) {
	patterns := []string{"article", "content", "main", "post", "entry", "body", "text"}
	for _, p := range patterns {
		t.Run("class_"+p, func(t *testing.T) {
			n := &html.Node{
				Type:     html.ElementNode,
				DataAtom: atom.Div,
				Data:     "div",
				Attr:     []html.Attribute{{Key: "class", Val: p}},
			}
			n.AppendChild(&html.Node{Type: html.TextNode, Data: strings.Repeat("word ", 20)})
			score := scoreNode(n)
			if score <= 0 {
				t.Errorf("class %q should boost score, got %d", p, score)
			}
		})
	}
}

func TestScoreNode_NegativeClassID(t *testing.T) {
	patterns := []string{"sidebar", "ad", "menu", "comment", "banner", "widget", "popup", "modal", "nav", "footer"}
	for _, p := range patterns {
		t.Run("class_"+p, func(t *testing.T) {
			n := &html.Node{
				Type:     html.ElementNode,
				DataAtom: atom.Div,
				Data:     "div",
				Attr:     []html.Attribute{{Key: "class", Val: p}},
			}
			n.AppendChild(&html.Node{Type: html.TextNode, Data: "short"})
			score := scoreNode(n)
			if score >= 0 {
				t.Errorf("class %q should penalize score, got %d", p, score)
			}
		})
	}
}

func TestScoreNode_ShortText(t *testing.T) {
	n := &html.Node{
		Type:     html.ElementNode,
		DataAtom: atom.Div,
		Data:     "div",
	}
	n.AppendChild(&html.Node{Type: html.TextNode, Data: "hi"})
	score := scoreNode(n)
	if score > 0 {
		t.Errorf("short text should penalize, got %d", score)
	}
}

func TestScoreNode_LinkDensityPenalty(t *testing.T) {
	// Div where all text is inside links
	n := &html.Node{Type: html.ElementNode, DataAtom: atom.Div, Data: "div"}
	a := &html.Node{Type: html.ElementNode, DataAtom: atom.A, Data: "a"}
	a.AppendChild(&html.Node{Type: html.TextNode, Data: strings.Repeat("link text ", 10)})
	n.AppendChild(a)

	score := scoreNode(n)
	// High link density should result in penalty
	if score > 5 {
		t.Errorf("high link density should penalize, got %d", score)
	}
}

func TestInnerText(t *testing.T) {
	n := &html.Node{Type: html.ElementNode, DataAtom: atom.Div, Data: "div"}
	n.AppendChild(&html.Node{Type: html.TextNode, Data: "hello "})
	span := &html.Node{Type: html.ElementNode, DataAtom: atom.Span, Data: "span"}
	span.AppendChild(&html.Node{Type: html.TextNode, Data: "world"})
	n.AppendChild(span)

	got := innerText(n)
	if got != "hello world" {
		t.Errorf("got %q, want %q", got, "hello world")
	}
}

func TestLinkTextLength(t *testing.T) {
	n := &html.Node{Type: html.ElementNode, DataAtom: atom.Div, Data: "div"}
	n.AppendChild(&html.Node{Type: html.TextNode, Data: "not a link"})
	a := &html.Node{Type: html.ElementNode, DataAtom: atom.A, Data: "a"}
	a.AppendChild(&html.Node{Type: html.TextNode, Data: "click here"})
	n.AppendChild(a)

	got := linkTextLength(n)
	if got != len("click here") {
		t.Errorf("got %d, want %d", got, len("click here"))
	}
}

func TestGetAttr(t *testing.T) {
	n := &html.Node{
		Type: html.ElementNode,
		Attr: []html.Attribute{
			{Key: "class", Val: "main-content"},
			{Key: "id", Val: "article"},
		},
	}

	if got := getAttr(n, "class"); got != "main-content" {
		t.Errorf("got %q", got)
	}
	if got := getAttr(n, "id"); got != "article" {
		t.Errorf("got %q", got)
	}
	if got := getAttr(n, "missing"); got != "" {
		t.Errorf("got %q for missing attr", got)
	}
}

func TestExtractMainContent(t *testing.T) {
	htmlStr := `<html><body>
		<nav>Navigation links here</nav>
		<article class="content">
			<p>This is the main article content with enough text to score well in the readability analysis and should be selected as main content.</p>
		</article>
		<footer>Footer stuff</footer>
	</body></html>`

	doc := parseHTML(t, htmlStr)
	best := extractMainContent(doc)

	if best == nil {
		t.Fatal("should find a best node")
	}

	text := innerText(best)
	if !strings.Contains(text, "main article content") {
		t.Errorf("expected article content, got %q", text)
	}
}

func TestExtractMainContent_EmptyDoc(t *testing.T) {
	doc := &html.Node{Type: html.DocumentNode}
	result := extractMainContent(doc)
	if result != doc {
		t.Error("empty doc should return doc itself")
	}
}

func TestExtractMainContent_DivVsNav(t *testing.T) {
	htmlStr := `<html><body>
		<nav><a href="/">Home</a><a href="/about">About</a><a href="/contact">Contact</a></nav>
		<div class="main-content">
			<p>A substantial paragraph of real content that should be identified as the main content area of this page rather than the navigation section above.</p>
		</div>
	</body></html>`

	doc := parseHTML(t, htmlStr)
	best := extractMainContent(doc)
	text := innerText(best)

	if strings.Contains(text, "Home") && !strings.Contains(text, "substantial") {
		t.Error("should prefer content div over nav")
	}
}
