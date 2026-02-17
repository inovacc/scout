package scout

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// MarkdownOption configures HTML-to-Markdown conversion.
type MarkdownOption func(*markdownOptions)

type markdownOptions struct {
	mainOnly      bool
	includeImages bool
	includeLinks  bool
	baseURL       string
}

func defaultMarkdownOptions() *markdownOptions {
	return &markdownOptions{
		includeImages: true,
		includeLinks:  true,
	}
}

// WithMainContentOnly enables readability scoring to extract only the main content.
func WithMainContentOnly() MarkdownOption {
	return func(o *markdownOptions) { o.mainOnly = true }
}

// WithIncludeImages controls whether images are included in the output.
func WithIncludeImages(v bool) MarkdownOption {
	return func(o *markdownOptions) { o.includeImages = v }
}

// WithIncludeLinks controls whether links are rendered as markdown links or plain text.
func WithIncludeLinks(v bool) MarkdownOption {
	return func(o *markdownOptions) { o.includeLinks = v }
}

// WithBaseURL sets a base URL for resolving relative URLs in links and images.
func WithBaseURL(u string) MarkdownOption {
	return func(o *markdownOptions) { o.baseURL = u }
}

// Markdown converts the page HTML to Markdown.
func (p *Page) Markdown(opts ...MarkdownOption) (string, error) {
	rawHTML, err := p.HTML()
	if err != nil {
		return "", fmt.Errorf("scout: markdown: %w", err)
	}

	pageURL := p.page.MustInfo().URL
	allOpts := append([]MarkdownOption{WithBaseURL(pageURL)}, opts...)

	return convertHTMLToMarkdown(rawHTML, allOpts...)
}

// MarkdownContent converts only the main content of the page to Markdown.
func (p *Page) MarkdownContent(opts ...MarkdownOption) (string, error) {
	return p.Markdown(append([]MarkdownOption{WithMainContentOnly()}, opts...)...)
}

// convertHTMLToMarkdown is the pure-function core: parses HTML and produces Markdown.
func convertHTMLToMarkdown(rawHTML string, opts ...MarkdownOption) (string, error) {
	o := defaultMarkdownOptions()
	for _, fn := range opts {
		fn(o)
	}

	doc, err := html.Parse(strings.NewReader(rawHTML))
	if err != nil {
		return "", fmt.Errorf("scout: parse html: %w", err)
	}

	root := doc
	if o.mainOnly {
		root = extractMainContent(doc)
	}

	ctx := &renderCtx{
		opts: o,
	}
	ctx.walk(root)

	return normalizeWhitespace(ctx.sb.String()), nil
}

// renderCtx carries state while walking the HTML tree.
type renderCtx struct {
	sb        strings.Builder
	opts      *markdownOptions
	listStack []listInfo
	inPre     bool
}

type listInfo struct {
	ordered bool
	index   int
}

func (c *renderCtx) walk(n *html.Node) {
	switch n.Type {
	case html.TextNode:
		text := n.Data
		if !c.inPre {
			text = collapseSpaces(text)
		}
		c.sb.WriteString(text)
		return
	case html.ElementNode:
		// skip
	default:
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			c.walk(ch)
		}
		return
	}

	switch n.DataAtom {
	case atom.Script, atom.Style, atom.Noscript, atom.Iframe, atom.Svg:
		return
	case atom.H1:
		c.heading(n, 1)
	case atom.H2:
		c.heading(n, 2)
	case atom.H3:
		c.heading(n, 3)
	case atom.H4:
		c.heading(n, 4)
	case atom.H5:
		c.heading(n, 5)
	case atom.H6:
		c.heading(n, 6)
	case atom.P:
		c.block(n, "", "")
	case atom.Br:
		c.sb.WriteString("\n")
	case atom.Hr:
		c.ensureNewline()
		c.sb.WriteString("---\n\n")
	case atom.Strong, atom.B:
		c.inline(n, "**")
	case atom.Em, atom.I:
		c.inline(n, "_")
	case atom.Code:
		if c.inPre {
			c.walkChildren(n)
		} else {
			c.inline(n, "`")
		}
	case atom.Pre:
		c.codeBlock(n)
	case atom.Blockquote:
		c.blockquote(n)
	case atom.A:
		c.link(n)
	case atom.Img:
		c.image(n)
	case atom.Ul:
		c.list(n, false)
	case atom.Ol:
		c.list(n, true)
	case atom.Li:
		c.walkChildren(n)
	case atom.Table:
		c.table(n)
	case atom.Div, atom.Section, atom.Article, atom.Main, atom.Header, atom.Footer, atom.Nav, atom.Aside:
		c.block(n, "", "")
	default:
		c.walkChildren(n)
	}
}

func (c *renderCtx) walkChildren(n *html.Node) {
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		c.walk(ch)
	}
}

func (c *renderCtx) heading(n *html.Node, level int) {
	c.ensureNewline()
	c.sb.WriteString(strings.Repeat("#", level))
	c.sb.WriteString(" ")
	c.walkChildren(n)
	c.sb.WriteString("\n\n")
}

func (c *renderCtx) block(n *html.Node, prefix, suffix string) {
	c.ensureNewline()
	c.sb.WriteString(prefix)
	c.walkChildren(n)
	c.sb.WriteString(suffix)
	c.sb.WriteString("\n\n")
}

func (c *renderCtx) inline(n *html.Node, marker string) {
	text := strings.TrimSpace(innerText(n))
	if text == "" {
		return
	}
	c.sb.WriteString(marker)
	c.walkChildren(n)
	c.sb.WriteString(marker)
}

func (c *renderCtx) codeBlock(n *html.Node) {
	c.ensureNewline()
	// Detect language from <code class="language-xxx">
	lang := ""
	if code := findChild(n, atom.Code); code != nil {
		cls := getAttr(code, "class")
		if strings.HasPrefix(cls, "language-") {
			lang = strings.TrimPrefix(cls, "language-")
		}
	}
	c.sb.WriteString("```")
	c.sb.WriteString(lang)
	c.sb.WriteString("\n")
	c.inPre = true
	c.walkChildren(n)
	c.inPre = false
	// Ensure newline before closing fence
	s := c.sb.String()
	if len(s) > 0 && s[len(s)-1] != '\n' {
		c.sb.WriteString("\n")
	}
	c.sb.WriteString("```\n\n")
}

func (c *renderCtx) blockquote(n *html.Node) {
	// Render children into a sub-context, then prefix each line.
	sub := &renderCtx{opts: c.opts, listStack: c.listStack, inPre: c.inPre}
	sub.walkChildren(n)
	lines := strings.Split(strings.TrimRight(sub.sb.String(), "\n"), "\n")
	c.ensureNewline()
	for _, line := range lines {
		c.sb.WriteString("> ")
		c.sb.WriteString(line)
		c.sb.WriteString("\n")
	}
	c.sb.WriteString("\n")
}

func (c *renderCtx) link(n *html.Node) {
	href := getAttr(n, "href")
	text := strings.TrimSpace(innerText(n))
	if text == "" {
		text = href
	}
	if !c.opts.includeLinks || href == "" {
		c.sb.WriteString(text)
		return
	}
	href = c.resolveURL(href)
	c.sb.WriteString("[")
	c.sb.WriteString(text)
	c.sb.WriteString("](")
	c.sb.WriteString(href)
	c.sb.WriteString(")")
}

func (c *renderCtx) image(n *html.Node) {
	if !c.opts.includeImages {
		return
	}
	src := getAttr(n, "src")
	alt := getAttr(n, "alt")
	if src == "" {
		return
	}
	src = c.resolveURL(src)
	c.sb.WriteString("![")
	c.sb.WriteString(alt)
	c.sb.WriteString("](")
	c.sb.WriteString(src)
	c.sb.WriteString(")")
}

func (c *renderCtx) list(n *html.Node, ordered bool) {
	c.ensureNewline()
	c.listStack = append(c.listStack, listInfo{ordered: ordered, index: 0})
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if ch.Type == html.ElementNode && ch.DataAtom == atom.Li {
			c.listItem(ch)
		}
	}
	c.listStack = c.listStack[:len(c.listStack)-1]
	if len(c.listStack) == 0 {
		c.sb.WriteString("\n")
	}
}

func (c *renderCtx) listItem(n *html.Node) {
	depth := len(c.listStack) - 1
	indent := strings.Repeat("  ", depth)
	info := &c.listStack[len(c.listStack)-1]

	c.sb.WriteString(indent)
	if info.ordered {
		info.index++
		c.sb.WriteString(fmt.Sprintf("%d. ", info.index))
	} else {
		c.sb.WriteString("- ")
	}

	// Render children, but handle nested lists separately
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if ch.Type == html.ElementNode && (ch.DataAtom == atom.Ul || ch.DataAtom == atom.Ol) {
			c.sb.WriteString("\n")
			c.walk(ch)
		} else {
			c.walk(ch)
		}
	}
	// End item with newline if not already
	s := c.sb.String()
	if len(s) > 0 && s[len(s)-1] != '\n' {
		c.sb.WriteString("\n")
	}
}

func (c *renderCtx) table(n *html.Node) {
	rows := collectTableRows(n)
	if len(rows) == 0 {
		return
	}

	// Determine column count
	cols := 0
	for _, row := range rows {
		if len(row) > cols {
			cols = len(row)
		}
	}
	if cols == 0 {
		return
	}

	// Pad rows
	for i := range rows {
		for len(rows[i]) < cols {
			rows[i] = append(rows[i], "")
		}
	}

	// Column widths
	widths := make([]int, cols)
	for _, row := range rows {
		for j, cell := range row {
			if len(cell) > widths[j] {
				widths[j] = len(cell)
			}
		}
	}
	for i := range widths {
		if widths[i] < 3 {
			widths[i] = 3
		}
	}

	c.ensureNewline()

	// Header row
	c.sb.WriteString("|")
	for j, cell := range rows[0] {
		c.sb.WriteString(" ")
		c.sb.WriteString(cell)
		c.sb.WriteString(strings.Repeat(" ", widths[j]-len(cell)))
		c.sb.WriteString(" |")
	}
	c.sb.WriteString("\n")

	// Separator
	c.sb.WriteString("|")
	for _, w := range widths {
		c.sb.WriteString(" ")
		c.sb.WriteString(strings.Repeat("-", w))
		c.sb.WriteString(" |")
	}
	c.sb.WriteString("\n")

	// Data rows
	for _, row := range rows[1:] {
		c.sb.WriteString("|")
		for j, cell := range row {
			c.sb.WriteString(" ")
			c.sb.WriteString(cell)
			c.sb.WriteString(strings.Repeat(" ", widths[j]-len(cell)))
			c.sb.WriteString(" |")
		}
		c.sb.WriteString("\n")
	}
	c.sb.WriteString("\n")
}

func collectTableRows(table *html.Node) [][]string {
	var rows [][]string
	var walkTable func(*html.Node)
	walkTable = func(n *html.Node) {
		if n.Type == html.ElementNode && n.DataAtom == atom.Tr {
			var cells []string
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				if c.Type == html.ElementNode && (c.DataAtom == atom.Td || c.DataAtom == atom.Th) {
					cells = append(cells, strings.TrimSpace(innerText(c)))
				}
			}
			rows = append(rows, cells)
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walkTable(c)
		}
	}
	walkTable(table)
	return rows
}

func (c *renderCtx) resolveURL(href string) string {
	if c.opts.baseURL == "" {
		return href
	}
	base, err := url.Parse(c.opts.baseURL)
	if err != nil {
		return href
	}
	ref, err := url.Parse(href)
	if err != nil {
		return href
	}
	return base.ResolveReference(ref).String()
}

func (c *renderCtx) ensureNewline() {
	s := c.sb.String()
	if len(s) > 0 && s[len(s)-1] != '\n' {
		c.sb.WriteString("\n")
	}
}

func findChild(n *html.Node, a atom.Atom) *html.Node {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.DataAtom == a {
			return c
		}
	}
	return nil
}

func collapseSpaces(s string) string {
	return reSpaces.ReplaceAllString(s, " ")
}

var reSpaces = regexp.MustCompile(`[\s]+`)

// normalizeWhitespace cleans up the final markdown output.
func normalizeWhitespace(s string) string {
	// Collapse 3+ consecutive newlines into 2
	s = reMultiNewline.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s) + "\n"
}

var reMultiNewline = regexp.MustCompile(`\n{3,}`)
