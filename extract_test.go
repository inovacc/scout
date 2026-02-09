package scout

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func init() {
	registerTestRoutes(extractTestRoutes)
}

func extractTestRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/extract", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Extract Test</title></head>
<body>
<div class="product">
  <h2 class="title">Widget Pro</h2>
  <span class="price">29.99</span>
  <img class="hero" src="/images/widget.png"/>
  <a class="detail" href="/products/widget">Details</a>
  <span class="tag">electronics</span>
  <span class="tag">gadgets</span>
  <span class="count">42</span>
  <span class="available">true</span>
</div>
</body></html>`)
	})

	mux.HandleFunc("/table", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Table Test</title></head>
<body>
<table id="data">
  <thead><tr><th>Name</th><th>Age</th><th>City</th></tr></thead>
  <tbody>
    <tr><td>Alice</td><td>30</td><td>NYC</td></tr>
    <tr><td>Bob</td><td>25</td><td>LA</td></tr>
    <tr><td>Charlie</td><td>35</td><td>Chicago</td></tr>
  </tbody>
</table>
<table id="no-thead">
  <tr><th>X</th><th>Y</th></tr>
  <tr><td>1</td><td>2</td></tr>
  <tr><td>3</td><td>4</td></tr>
</table>
</body></html>`)
	})

	mux.HandleFunc("/meta", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head>
<title>Meta Test Page</title>
<meta name="description" content="A test page for metadata extraction"/>
<link rel="canonical" href="https://example.com/meta"/>
<meta property="og:title" content="OG Title"/>
<meta property="og:description" content="OG Description"/>
<meta property="og:image" content="https://example.com/image.png"/>
<meta name="twitter:card" content="summary"/>
<meta name="twitter:title" content="Twitter Title"/>
<script type="application/ld+json">{"@type":"WebPage","name":"Test"}</script>
</head>
<body><h1>Meta Test</h1></body></html>`)
	})

	mux.HandleFunc("/links", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Links</title></head>
<body>
<a href="/page1">Page 1</a>
<a href="/page2">Page 2</a>
<a href="https://example.com">External</a>
</body></html>`)
	})

	mux.HandleFunc("/nested", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Nested</title></head>
<body>
<div class="card">
  <div class="header"><span class="name">Card Title</span></div>
  <div class="body"><p class="desc">Card description</p></div>
</div>
</body></html>`)
	})

	mux.HandleFunc("/products-list", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Products</title></head>
<body>
<div class="item"><h3 class="name">Alpha</h3><span class="price">10</span></div>
<div class="item"><h3 class="name">Beta</h3><span class="price">20</span></div>
<div class="item"><h3 class="name">Gamma</h3><span class="price">30</span></div>
</body></html>`)
	})
}

func TestExtractStruct(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)
	page, err := b.NewPage(srv.URL + "/extract")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	type Product struct {
		Name      string   `scout:"h2.title"`
		Price     float64  `scout:"span.price"`
		Image     string   `scout:"img.hero@src"`
		Link      string   `scout:"a.detail@href"`
		Tags      []string `scout:"span.tag"`
		Count     int      `scout:"span.count"`
		Available bool     `scout:"span.available"`
	}

	var p Product
	if err := page.Extract(&p); err != nil {
		t.Fatalf("Extract() error: %v", err)
	}

	if p.Name != "Widget Pro" {
		t.Errorf("Name = %q, want %q", p.Name, "Widget Pro")
	}
	if p.Price != 29.99 {
		t.Errorf("Price = %v, want 29.99", p.Price)
	}
	if p.Image != "/images/widget.png" {
		t.Errorf("Image = %q, want %q", p.Image, "/images/widget.png")
	}
	if p.Link != "/products/widget" {
		t.Errorf("Link = %q, want %q", p.Link, "/products/widget")
	}
	if len(p.Tags) != 2 || p.Tags[0] != "electronics" || p.Tags[1] != "gadgets" {
		t.Errorf("Tags = %v, want [electronics gadgets]", p.Tags)
	}
	if p.Count != 42 {
		t.Errorf("Count = %d, want 42", p.Count)
	}
	if !p.Available {
		t.Error("Available should be true")
	}
}

func TestExtractStructFromElement(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)
	page, err := b.NewPage(srv.URL + "/extract")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	el, err := page.Element(".product")
	if err != nil {
		t.Fatalf("Element() error: %v", err)
	}

	type Product struct {
		Name string `scout:"h2.title"`
	}

	var p Product
	if err := el.Extract(&p); err != nil {
		t.Fatalf("Extract() error: %v", err)
	}
	if p.Name != "Widget Pro" {
		t.Errorf("Name = %q, want %q", p.Name, "Widget Pro")
	}
}

func TestExtractInvalidTarget(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)
	page, err := b.NewPage(srv.URL + "/extract")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	var s string
	if err := page.Extract(&s); err == nil {
		t.Error("Extract(&string) should return error")
	}

	if err := page.Extract(nil); err == nil {
		t.Error("Extract(nil) should return error")
	}
}

func TestExtractTable(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)
	page, err := b.NewPage(srv.URL + "/table")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	td, err := page.ExtractTable("#data")
	if err != nil {
		t.Fatalf("ExtractTable() error: %v", err)
	}

	if len(td.Headers) != 3 {
		t.Fatalf("Headers count = %d, want 3", len(td.Headers))
	}
	if td.Headers[0] != "Name" || td.Headers[1] != "Age" || td.Headers[2] != "City" {
		t.Errorf("Headers = %v", td.Headers)
	}
	if len(td.Rows) != 3 {
		t.Fatalf("Rows count = %d, want 3", len(td.Rows))
	}
	if td.Rows[0][0] != "Alice" || td.Rows[0][1] != "30" || td.Rows[0][2] != "NYC" {
		t.Errorf("Row 0 = %v", td.Rows[0])
	}
}

func TestExtractTableMap(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)
	page, err := b.NewPage(srv.URL + "/table")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	maps, err := page.ExtractTableMap("#data")
	if err != nil {
		t.Fatalf("ExtractTableMap() error: %v", err)
	}

	if len(maps) != 3 {
		t.Fatalf("rows = %d, want 3", len(maps))
	}
	if maps[0]["Name"] != "Alice" {
		t.Errorf("maps[0][Name] = %q, want Alice", maps[0]["Name"])
	}
	if maps[1]["Age"] != "25" {
		t.Errorf("maps[1][Age] = %q, want 25", maps[1]["Age"])
	}
}

func TestExtractTableNoThead(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)
	page, err := b.NewPage(srv.URL + "/table")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	td, err := page.ExtractTable("#no-thead")
	if err != nil {
		t.Fatalf("ExtractTable() error: %v", err)
	}

	if len(td.Headers) != 2 || td.Headers[0] != "X" || td.Headers[1] != "Y" {
		t.Errorf("Headers = %v, want [X Y]", td.Headers)
	}
	if len(td.Rows) != 2 {
		t.Errorf("Rows = %d, want 2", len(td.Rows))
	}
}

func TestExtractMeta(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)
	page, err := b.NewPage(srv.URL + "/meta")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	meta, err := page.ExtractMeta()
	if err != nil {
		t.Fatalf("ExtractMeta() error: %v", err)
	}

	if meta.Title != "Meta Test Page" {
		t.Errorf("Title = %q, want %q", meta.Title, "Meta Test Page")
	}
	if meta.Description != "A test page for metadata extraction" {
		t.Errorf("Description = %q", meta.Description)
	}
	if meta.Canonical != "https://example.com/meta" {
		t.Errorf("Canonical = %q", meta.Canonical)
	}
	if meta.OG["og:title"] != "OG Title" {
		t.Errorf("OG title = %q", meta.OG["og:title"])
	}
	if meta.OG["og:image"] != "https://example.com/image.png" {
		t.Errorf("OG image = %q", meta.OG["og:image"])
	}
	if meta.Twitter["twitter:card"] != "summary" {
		t.Errorf("Twitter card = %q", meta.Twitter["twitter:card"])
	}
	if len(meta.JSONLD) != 1 {
		t.Fatalf("JSONLD count = %d, want 1", len(meta.JSONLD))
	}
	var ld map[string]any
	if err := json.Unmarshal(meta.JSONLD[0], &ld); err != nil {
		t.Fatalf("JSONLD unmarshal error: %v", err)
	}
	if ld["@type"] != "WebPage" {
		t.Errorf("JSONLD @type = %v", ld["@type"])
	}
}

func TestExtractText(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)
	page, err := b.NewPage(srv.URL + "/extract")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	text, err := page.ExtractText("h2.title")
	if err != nil {
		t.Fatalf("ExtractText() error: %v", err)
	}
	if text != "Widget Pro" {
		t.Errorf("ExtractText() = %q, want %q", text, "Widget Pro")
	}
}

func TestExtractTexts(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)
	page, err := b.NewPage(srv.URL + "/extract")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	texts, err := page.ExtractTexts("span.tag")
	if err != nil {
		t.Fatalf("ExtractTexts() error: %v", err)
	}
	if len(texts) != 2 || texts[0] != "electronics" || texts[1] != "gadgets" {
		t.Errorf("ExtractTexts() = %v", texts)
	}
}

func TestExtractAttribute(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)
	page, err := b.NewPage(srv.URL + "/extract")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	val, err := page.ExtractAttribute("img.hero", "src")
	if err != nil {
		t.Fatalf("ExtractAttribute() error: %v", err)
	}
	if val != "/images/widget.png" {
		t.Errorf("ExtractAttribute() = %q", val)
	}
}

func TestExtractAttributes(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)
	page, err := b.NewPage(srv.URL + "/links")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	vals, err := page.ExtractAttributes("a", "href")
	if err != nil {
		t.Fatalf("ExtractAttributes() error: %v", err)
	}
	if len(vals) != 3 {
		t.Errorf("ExtractAttributes() returned %d values, want 3", len(vals))
	}
}

func TestExtractLinks(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)
	page, err := b.NewPage(srv.URL + "/links")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	links, err := page.ExtractLinks()
	if err != nil {
		t.Fatalf("ExtractLinks() error: %v", err)
	}
	if len(links) != 3 {
		t.Errorf("ExtractLinks() returned %d links, want 3", len(links))
	}
}

func TestExtractNestedStruct(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)
	page, err := b.NewPage(srv.URL + "/nested")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	type Header struct {
		Name string `scout:"span.name"`
	}
	type Card struct {
		Header Header `scout:"div.header"`
		Desc   string `scout:"p.desc"`
	}

	var c Card
	if err := page.Extract(&c); err != nil {
		t.Fatalf("Extract() error: %v", err)
	}
	if c.Header.Name != "Card Title" {
		t.Errorf("Header.Name = %q, want %q", c.Header.Name, "Card Title")
	}
	if c.Desc != "Card description" {
		t.Errorf("Desc = %q, want %q", c.Desc, "Card description")
	}
}

func TestExtractSliceOfStructs(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)
	page, err := b.NewPage(srv.URL + "/products-list")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	type Item struct {
		Name  string `scout:"h3.name"`
		Price int    `scout:"span.price"`
	}
	type PageData struct {
		Items []Item `scout:"div.item"`
	}

	var pd PageData
	if err := page.Extract(&pd); err != nil {
		t.Fatalf("Extract() error: %v", err)
	}
	if len(pd.Items) != 3 {
		t.Fatalf("Items count = %d, want 3", len(pd.Items))
	}
	if pd.Items[0].Name != "Alpha" || pd.Items[0].Price != 10 {
		t.Errorf("Items[0] = %+v", pd.Items[0])
	}
	if pd.Items[2].Name != "Gamma" || pd.Items[2].Price != 30 {
		t.Errorf("Items[2] = %+v", pd.Items[2])
	}
}
