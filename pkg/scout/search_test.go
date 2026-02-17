package scout

import (
	"fmt"
	"net/http"
	"testing"
)

func init() {
	registerTestRoutes(searchTestRoutes)
}

func searchTestRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/serp-google", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>test - Google Search</title></head>
<body>
<div class="g">
  <h3>First Result</h3>
  <a href="https://example.com/first">example.com</a>
  <div class="VwiC3b">This is the first snippet</div>
</div>
<div class="g">
  <h3>Second Result</h3>
  <a href="https://example.com/second">example.com</a>
  <div class="VwiC3b">This is the second snippet</div>
</div>
<div class="g">
  <h3>Third Result</h3>
  <a href="https://example.com/third">example.com</a>
  <div class="VwiC3b">Third snippet here</div>
</div>
<a id="pnnext" href="/serp-google-page2">Next</a>
</body></html>`)
	})

	mux.HandleFunc("/serp-google-page2", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>test - Google Search Page 2</title></head>
<body>
<div class="g">
  <h3>Fourth Result</h3>
  <a href="https://example.com/fourth">example.com</a>
  <div class="VwiC3b">Fourth snippet</div>
</div>
</body></html>`)
	})

	mux.HandleFunc("/serp-bing", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>test - Bing</title></head>
<body>
<li class="b_algo">
  <h2><a href="https://example.com/bing1">Bing Result 1</a></h2>
  <p>Bing snippet 1</p>
</li>
<li class="b_algo">
  <h2><a href="https://example.com/bing2">Bing Result 2</a></h2>
  <p>Bing snippet 2</p>
</li>
</body></html>`)
	})

	mux.HandleFunc("/serp-ddg", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>test at DuckDuckGo</title></head>
<body>
<div data-result="1" class="result">
  <a class="result__a" href="https://example.com/ddg1">DDG Result 1</a>
  <a class="result__snippet">DDG snippet 1</a>
</div>
<div data-result="2" class="result">
  <a class="result__a" href="https://example.com/ddg2">DDG Result 2</a>
  <a class="result__snippet">DDG snippet 2</a>
</div>
</body></html>`)
	})
}

func TestSearchGoogleParser(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/serp-google")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	results, err := googleParser.parse(page, "test", Google)
	if err != nil {
		t.Fatalf("parse() error: %v", err)
	}

	if len(results.Results) != 3 {
		t.Fatalf("Results count = %d, want 3", len(results.Results))
	}

	r := results.Results[0]
	if r.Title != "First Result" {
		t.Errorf("Title = %q, want First Result", r.Title)
	}

	if r.URL != "https://example.com/first" {
		t.Errorf("URL = %q", r.URL)
	}

	if r.Snippet != "This is the first snippet" {
		t.Errorf("Snippet = %q", r.Snippet)
	}

	if r.Position != 1 {
		t.Errorf("Position = %d, want 1", r.Position)
	}
}

func TestSearchBingParser(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/serp-bing")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	results, err := bingParser.parse(page, "test", Bing)
	if err != nil {
		t.Fatalf("parse() error: %v", err)
	}

	if len(results.Results) != 2 {
		t.Fatalf("Results count = %d, want 2", len(results.Results))
	}

	r := results.Results[0]
	if r.Title != "Bing Result 1" {
		t.Errorf("Title = %q", r.Title)
	}

	if r.URL != "https://example.com/bing1" {
		t.Errorf("URL = %q", r.URL)
	}
}

func TestSearchDDGParser(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/serp-ddg")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = page.Close() }()

	results, err := ddgParser.parse(page, "test", DuckDuckGo)
	if err != nil {
		t.Fatalf("parse() error: %v", err)
	}

	if len(results.Results) != 2 {
		t.Fatalf("Results count = %d, want 2", len(results.Results))
	}

	r := results.Results[0]
	if r.Title != "DDG Result 1" {
		t.Errorf("Title = %q", r.Title)
	}
}

func TestSearchOptions(t *testing.T) {
	o := searchDefaults()

	if o.engine != Google {
		t.Errorf("default engine = %d, want Google", o.engine)
	}

	if o.maxPages != 1 {
		t.Errorf("default maxPages = %d, want 1", o.maxPages)
	}

	WithSearchEngine(Bing)(o)

	if o.engine != Bing {
		t.Errorf("engine = %d, want Bing", o.engine)
	}

	WithSearchMaxPages(5)(o)

	if o.maxPages != 5 {
		t.Errorf("maxPages = %d, want 5", o.maxPages)
	}

	WithSearchLanguage("pt-BR")(o)

	if o.language != "pt-BR" {
		t.Errorf("language = %q", o.language)
	}

	WithSearchRegion("br")(o)

	if o.region != "br" {
		t.Errorf("region = %q", o.region)
	}
}

func TestCleanSearchURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://example.com", "https://example.com"},
		{"/url?q=https://example.com/page&sa=U", "https://example.com/page"},
		{"/url?url=https://example.com/other&sa=U", "https://example.com/other"},
		{"/url?noparam=1", "/url?noparam=1"},
		{"", ""},
	}
	for _, tt := range tests {
		got := cleanSearchURL(tt.input)
		if got != tt.want {
			t.Errorf("cleanSearchURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGetParser(t *testing.T) {
	if p := getParser(Google); p != &googleParser {
		t.Error("getParser(Google) should return googleParser")
	}

	if p := getParser(Bing); p != &bingParser {
		t.Error("getParser(Bing) should return bingParser")
	}

	if p := getParser(DuckDuckGo); p != &ddgParser {
		t.Error("getParser(DuckDuckGo) should return ddgParser")
	}
}
