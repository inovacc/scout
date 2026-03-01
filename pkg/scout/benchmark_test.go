package scout

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// newBenchServer returns an httptest server with rich HTML for benchmarking.
func newBenchServer() *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Bench Page</title></head>
<body>
<h1>Benchmark Test Page</h1>
<p id="info">Some descriptive text for benchmarking purposes.</p>
<table id="data">
  <thead><tr><th>Name</th><th>Value</th></tr></thead>
  <tbody>
    <tr><td>Alpha</td><td>100</td></tr>
    <tr><td>Beta</td><td>200</td></tr>
    <tr><td>Gamma</td><td>300</td></tr>
  </tbody>
</table>
<form id="myform">
  <input name="username" type="text" value="bench"/>
  <input name="email" type="email" value="bench@test.com"/>
  <select name="role"><option value="admin">Admin</option><option value="user">User</option></select>
</form>
<ul id="items">
  <li class="item">Item 1</li>
  <li class="item">Item 2</li>
  <li class="item">Item 3</li>
</ul>
<article class="post-content">
  <h2 class="title">Article Title</h2>
  <p>This is a long article paragraph with enough text content to score well in readability analysis. It contains multiple sentences and meaningful content that represents a typical web page article body.</p>
  <p>A second paragraph adds more depth to the content, making the readability scorer identify this as the main content area of the page.</p>
  <span class="price">$42.99</span>
  <img class="hero" src="/img/hero.png" alt="Hero"/>
  <a class="detail" href="/detail/1">Details</a>
  <span class="tag">go</span>
  <span class="tag">benchmark</span>
  <span class="count">7</span>
</article>
<a href="/page2">Link 1</a>
<a href="/page3">Link 2</a>
</body></html>`)
	})

	return httptest.NewServer(mux)
}

func newBenchBrowser(b *testing.B) *Browser {
	b.Helper()

	br, err := New(
		WithHeadless(true),
		WithNoSandbox(),
		WithTimeout(30e9),
	)
	if err != nil {
		b.Skipf("skipping: browser unavailable: %v", err)
	}

	b.Cleanup(func() { _ = br.Close() })

	return br
}

func BenchmarkNewBrowser(b *testing.B) {
	// Verify browser is available first.
	probe, err := New(WithHeadless(true), WithNoSandbox())
	if err != nil {
		b.Skipf("skipping: browser unavailable: %v", err)
	}
	_ = probe.Close()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		br, err := New(WithHeadless(true), WithNoSandbox())
		if err != nil {
			b.Fatalf("New() error: %v", err)
		}

		_ = br.Close()
	}
}

func BenchmarkNewPage(b *testing.B) {
	srv := newBenchServer()
	defer srv.Close()

	br := newBenchBrowser(b)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		page, err := br.NewPage(srv.URL)
		if err != nil {
			b.Fatalf("NewPage() error: %v", err)
		}

		_ = page.Close()
	}
}

func BenchmarkExtract(b *testing.B) {
	srv := newBenchServer()
	defer srv.Close()

	br := newBenchBrowser(b)

	page, err := br.NewPage(srv.URL)
	if err != nil {
		b.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		b.Fatalf("WaitLoad() error: %v", err)
	}

	type Article struct {
		Title string   `scout:"h2.title"`
		Price string   `scout:"span.price"`
		Image string   `scout:"img.hero@src"`
		Link  string   `scout:"a.detail@href"`
		Tags  []string `scout:"span.tag"`
		Count int      `scout:"span.count"`
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var a Article
		if err := page.Extract(&a); err != nil {
			b.Fatalf("Extract() error: %v", err)
		}
	}
}

func BenchmarkEval(b *testing.B) {
	srv := newBenchServer()
	defer srv.Close()

	br := newBenchBrowser(b)

	page, err := br.NewPage(srv.URL)
	if err != nil {
		b.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		b.Fatalf("WaitLoad() error: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := page.Eval("() => 1 + 1")
		if err != nil {
			b.Fatalf("Eval() error: %v", err)
		}
	}
}

func BenchmarkScreenshot(b *testing.B) {
	srv := newBenchServer()
	defer srv.Close()

	br := newBenchBrowser(b)

	page, err := br.NewPage(srv.URL)
	if err != nil {
		b.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		b.Fatalf("WaitLoad() error: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		data, err := page.Screenshot()
		if err != nil {
			b.Fatalf("Screenshot() error: %v", err)
		}

		if len(data) == 0 {
			b.Fatal("Screenshot() returned empty data")
		}
	}
}

func BenchmarkMarkdown(b *testing.B) {
	srv := newBenchServer()
	defer srv.Close()

	br := newBenchBrowser(b)

	page, err := br.NewPage(srv.URL)
	if err != nil {
		b.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		b.Fatalf("WaitLoad() error: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		md, err := page.Markdown()
		if err != nil {
			b.Fatalf("Markdown() error: %v", err)
		}

		if md == "" {
			b.Fatal("Markdown() returned empty string")
		}
	}
}

func BenchmarkReadability(b *testing.B) {
	// This benchmark tests the readability scoring without a browser.
	const testHTML = `<html><body>
<nav><a href="/">Home</a><a href="/about">About</a></nav>
<article class="post-content">
  <h1>Main Article</h1>
  <p>This is a long article paragraph with enough text content to score well in readability analysis. It contains multiple sentences and meaningful content that represents a typical web page article body.</p>
  <p>A second paragraph adds more depth to the content, making the readability scorer identify this as the main content area of the page rather than the navigation or sidebar.</p>
  <p>Third paragraph continues with even more substantial text to ensure the scoring algorithm has enough signal to work with properly.</p>
</article>
<aside><p>Sidebar ad</p></aside>
</body></html>`

	doc, err := html.Parse(strings.NewReader(testHTML))
	if err != nil {
		b.Fatalf("html.Parse() error: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		node := extractMainContent(doc)
		if node == nil {
			b.Fatal("extractMainContent() returned nil")
		}
	}
}

func BenchmarkSnapshot(b *testing.B) {
	srv := newBenchServer()
	defer srv.Close()

	br := newBenchBrowser(b)

	page, err := br.NewPage(srv.URL)
	if err != nil {
		b.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		b.Fatalf("WaitLoad() error: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		snap, err := page.Snapshot()
		if err != nil {
			b.Fatalf("Snapshot() error: %v", err)
		}

		if snap == "" {
			b.Fatal("Snapshot() returned empty string")
		}
	}
}
