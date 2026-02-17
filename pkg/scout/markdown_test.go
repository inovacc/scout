package scout

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func init() {
	registerTestRoutes(func(mux *http.ServeMux) {
		mux.HandleFunc("/markdown", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Markdown Test</title></head>
<body>
<nav><a href="/home">Home</a> | <a href="/about">About</a></nav>
<article class="post-content">
<h1>Main Article</h1>
<p>This is the <strong>main</strong> content with a <a href="https://example.com">link</a>.</p>
<p>Second paragraph with <em>emphasis</em> and <code>inline code</code>.</p>
<ul>
  <li>Item 1</li>
  <li>Item 2</li>
</ul>
<table>
  <tr><th>Name</th><th>Value</th></tr>
  <tr><td>Alpha</td><td>100</td></tr>
  <tr><td>Beta</td><td>200</td></tr>
</table>
<blockquote><p>A wise quote.</p></blockquote>
<pre><code class="language-go">fmt.Println("hello")</code></pre>
<img src="/logo.png" alt="Logo"/>
<hr/>
<p>End of article.</p>
</article>
<footer>Copyright 2024</footer>
</body></html>`)
		})
	})
}

// --- Pure-function tests (no browser) ---

func TestConvertHeadings(t *testing.T) {
	html := `<h1>One</h1><h2>Two</h2><h3>Three</h3><h4>Four</h4><h5>Five</h5><h6>Six</h6>`

	md, err := convertHTMLToMarkdown(html)
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"# One", "## Two", "### Three", "#### Four", "##### Five", "###### Six"} {
		if !strings.Contains(md, want) {
			t.Errorf("missing %q in:\n%s", want, md)
		}
	}
}

func TestConvertParagraph(t *testing.T) {
	md, err := convertHTMLToMarkdown(`<p>Hello world.</p><p>Second.</p>`)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(md, "Hello world.") || !strings.Contains(md, "Second.") {
		t.Errorf("unexpected: %s", md)
	}
}

func TestConvertBoldItalic(t *testing.T) {
	md, err := convertHTMLToMarkdown(`<p><strong>bold</strong> and <em>italic</em></p>`)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(md, "**bold**") {
		t.Errorf("missing bold: %s", md)
	}

	if !strings.Contains(md, "_italic_") {
		t.Errorf("missing italic: %s", md)
	}
}

func TestConvertLinks(t *testing.T) {
	md, err := convertHTMLToMarkdown(`<a href="https://go.dev">Go</a>`)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(md, "[Go](https://go.dev)") {
		t.Errorf("unexpected: %s", md)
	}

	// Without links
	md2, err := convertHTMLToMarkdown(`<a href="https://go.dev">Go</a>`, WithIncludeLinks(false))
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(md2, "[Go]") {
		t.Errorf("should not have link syntax: %s", md2)
	}
}

func TestConvertImages(t *testing.T) {
	md, err := convertHTMLToMarkdown(`<img src="/logo.png" alt="Logo"/>`)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(md, "![Logo](/logo.png)") {
		t.Errorf("unexpected: %s", md)
	}

	// Without images
	md2, err := convertHTMLToMarkdown(`<img src="/logo.png" alt="Logo"/>`, WithIncludeImages(false))
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(md2, "![Logo]") {
		t.Errorf("should not have image: %s", md2)
	}
}

func TestConvertInlineCode(t *testing.T) {
	md, err := convertHTMLToMarkdown(`<p>Use <code>fmt.Println</code> here.</p>`)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(md, "`fmt.Println`") {
		t.Errorf("unexpected: %s", md)
	}
}

func TestConvertCodeBlock(t *testing.T) {
	md, err := convertHTMLToMarkdown(`<pre><code class="language-go">func main() {}</code></pre>`)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(md, "```go\n") {
		t.Errorf("missing language fence: %s", md)
	}

	if !strings.Contains(md, "func main() {}") {
		t.Errorf("missing code: %s", md)
	}
}

func TestConvertBlockquote(t *testing.T) {
	md, err := convertHTMLToMarkdown(`<blockquote><p>Quoted text.</p></blockquote>`)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(md, "> ") {
		t.Errorf("missing blockquote prefix: %s", md)
	}

	if !strings.Contains(md, "Quoted text.") {
		t.Errorf("missing text: %s", md)
	}
}

func TestConvertUnorderedList(t *testing.T) {
	md, err := convertHTMLToMarkdown(`<ul><li>Apple</li><li>Banana</li></ul>`)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(md, "- Apple") || !strings.Contains(md, "- Banana") {
		t.Errorf("unexpected: %s", md)
	}
}

func TestConvertOrderedList(t *testing.T) {
	md, err := convertHTMLToMarkdown(`<ol><li>First</li><li>Second</li></ol>`)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(md, "1. First") || !strings.Contains(md, "2. Second") {
		t.Errorf("unexpected: %s", md)
	}
}

func TestConvertNestedList(t *testing.T) {
	md, err := convertHTMLToMarkdown(`<ul><li>Parent<ul><li>Child</li></ul></li></ul>`)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(md, "- Parent") {
		t.Errorf("missing parent: %s", md)
	}

	if !strings.Contains(md, "  - Child") {
		t.Errorf("missing indented child: %s", md)
	}
}

func TestConvertTable(t *testing.T) {
	md, err := convertHTMLToMarkdown(`<table>
<tr><th>A</th><th>B</th></tr>
<tr><td>1</td><td>2</td></tr>
</table>`)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(md, "| A") {
		t.Errorf("missing header: %s", md)
	}

	if !strings.Contains(md, "---") {
		t.Errorf("missing separator: %s", md)
	}

	if !strings.Contains(md, "| 1") {
		t.Errorf("missing data: %s", md)
	}
}

func TestConvertHR(t *testing.T) {
	md, err := convertHTMLToMarkdown(`<p>Above</p><hr/><p>Below</p>`)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(md, "---") {
		t.Errorf("missing hr: %s", md)
	}
}

func TestConvertScriptStripped(t *testing.T) {
	md, err := convertHTMLToMarkdown(`<p>Hello</p><script>alert("x")</script><style>.a{}</style><p>World</p>`)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(md, "alert") || strings.Contains(md, ".a{}") {
		t.Errorf("script/style not stripped: %s", md)
	}
}

func TestConvertBaseURL(t *testing.T) {
	md, err := convertHTMLToMarkdown(`<a href="/about">About</a>`, WithBaseURL("https://example.com"))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(md, "https://example.com/about") {
		t.Errorf("URL not resolved: %s", md)
	}
}

func TestReadabilityScoring(t *testing.T) {
	htmlStr := `<html><body>
<nav><a href="/">Home</a><a href="/about">About</a><a href="/contact">Contact</a></nav>
<article class="post-content"><p>This is a long article with plenty of text content that should score highly in readability analysis because it has substantial text.</p></article>
<aside class="sidebar"><p>Ad</p></aside>
</body></html>`

	md, err := convertHTMLToMarkdown(htmlStr, WithMainContentOnly())
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(md, "long article") {
		t.Errorf("main content not found: %s", md)
	}
}

func TestConvertMainOnly(t *testing.T) {
	htmlStr := `<html><body>
<nav>Navigation here with many links <a href="/">a</a><a href="/b">b</a><a href="/c">c</a></nav>
<article><h1>Title</h1><p>Main body text that is interesting and relevant to the reader with enough content to score well.</p></article>
<footer>Footer stuff</footer>
</body></html>`

	full, _ := convertHTMLToMarkdown(htmlStr)
	main, _ := convertHTMLToMarkdown(htmlStr, WithMainContentOnly())

	if len(main) >= len(full) {
		t.Errorf("main-only should be shorter: main=%d full=%d", len(main), len(full))
	}
}

// --- Browser integration tests ---

func TestPageMarkdown(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/markdown")
	if err != nil {
		t.Fatal(err)
	}

	if err := page.WaitLoad(); err != nil {
		t.Fatal(err)
	}

	md, err := page.Markdown()
	if err != nil {
		t.Fatal(err)
	}

	checks := []string{"# Main Article", "**main**", "[link](https://example.com)", "- Item 1", "| Name", "```go"}
	for _, want := range checks {
		if !strings.Contains(md, want) {
			t.Errorf("missing %q in:\n%s", want, md)
		}
	}
}

func TestPageMarkdownContent(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/markdown")
	if err != nil {
		t.Fatal(err)
	}

	if err := page.WaitLoad(); err != nil {
		t.Fatal(err)
	}

	md, err := page.MarkdownContent()
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(md, "Main Article") {
		t.Errorf("missing article content: %s", md)
	}
}
