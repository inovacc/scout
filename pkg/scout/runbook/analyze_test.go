package runbook

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newAnalyzeServer(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()

	mux.HandleFunc("/analyze-listing", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Product Listing</title>
<meta name="description" content="Browse our products">
</head><body>
<div class="product"><h3>Widget A</h3><a href="/widget-a">Details</a><span class="price">$9.99</span></div>
<div class="product"><h3>Widget B</h3><a href="/widget-b">Details</a><span class="price">$19.99</span></div>
<div class="product"><h3>Widget C</h3><a href="/widget-c">Details</a><span class="price">$29.99</span></div>
<div class="product"><h3>Widget D</h3><a href="/widget-d">Details</a><span class="price">$39.99</span></div>
</body></html>`)
	})

	mux.HandleFunc("/analyze-form", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Login</title></head><body>
<form action="/login" method="POST">
<input type="text" name="username" id="username" placeholder="Username" required>
<input type="password" name="password" id="password" placeholder="Password" required>
<button type="submit">Sign In</button>
</form>
</body></html>`)
	})

	mux.HandleFunc("/analyze-article", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Blog Post</title>
<meta property="og:type" content="article">
</head><body>
<article>
<h1>Understanding Go Interfaces</h1>
<p>Go interfaces are a powerful feature that enable polymorphism without inheritance.
They define a set of method signatures that a type must implement.</p>
<p>Unlike many other languages, Go interfaces are satisfied implicitly.
There is no need to explicitly declare that a type implements an interface.</p>
<p>This design choice leads to more flexible and decoupled code.
Types can satisfy interfaces from other packages without modification.</p>
</article>
</body></html>`)
	})

	mux.HandleFunc("/analyze-paginated", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Paginated Products</title></head><body>
<div class="item"><h3>Item 1</h3></div>
<div class="item"><h3>Item 2</h3></div>
<div class="item"><h3>Item 3</h3></div>
<a class="next" href="/page2">Next Page</a>
</body></html>`)
	})

	mux.HandleFunc("/gen-listing", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Gen Test</title></head><body>
<div class="item"><h2>Item A</h2><a href="/a">Link A</a><span class="price">$10</span></div>
<div class="item"><h2>Item B</h2><a href="/b">Link B</a><span class="price">$20</span></div>
<div class="item"><h2>Item C</h2><a href="/c">Link C</a><span class="price">$30</span></div>
</body></html>`)
	})

	return httptest.NewServer(mux)
}

func TestAnalyzeSite_Listing(t *testing.T) {
	ts := newAnalyzeServer(t)
	defer ts.Close()

	b := newTestBrowser(t)

	analysis, err := AnalyzeSite(context.Background(), b, ts.URL+"/analyze-listing")
	if err != nil {
		t.Fatalf("AnalyzeSite failed: %v", err)
	}

	if analysis.PageType != "listing" {
		t.Errorf("pageType = %q, want %q", analysis.PageType, "listing")
	}

	if len(analysis.Containers) == 0 {
		t.Fatal("expected at least one container candidate")
	}

	top := analysis.Containers[0]
	if top.Count < 3 {
		t.Errorf("top container count = %d, want >= 3", top.Count)
	}

	if len(top.Fields) == 0 {
		t.Error("expected fields in top container")
	}

	// Check that field discovery found something
	fieldNames := make(map[string]bool)
	for _, f := range top.Fields {
		fieldNames[f.Name] = true
	}

	if !fieldNames["title"] {
		t.Error("expected 'title' field")
	}
}

func TestAnalyzeSite_Form(t *testing.T) {
	ts := newAnalyzeServer(t)
	defer ts.Close()

	b := newTestBrowser(t)

	analysis, err := AnalyzeSite(context.Background(), b, ts.URL+"/analyze-form")
	if err != nil {
		t.Fatalf("AnalyzeSite failed: %v", err)
	}

	if analysis.PageType != "form" {
		t.Errorf("pageType = %q, want %q", analysis.PageType, "form")
	}

	if len(analysis.Forms) == 0 {
		t.Fatal("expected at least one form")
	}

	form := analysis.Forms[0]
	if len(form.Fields) < 2 {
		t.Errorf("form fields = %d, want >= 2", len(form.Fields))
	}
}

func TestAnalyzeSite_Article(t *testing.T) {
	ts := newAnalyzeServer(t)
	defer ts.Close()

	b := newTestBrowser(t)

	analysis, err := AnalyzeSite(context.Background(), b, ts.URL+"/analyze-article")
	if err != nil {
		t.Fatalf("AnalyzeSite failed: %v", err)
	}

	if analysis.PageType != "article" {
		t.Errorf("pageType = %q, want %q", analysis.PageType, "article")
	}
}

func TestAnalyzeSite_Pagination(t *testing.T) {
	ts := newAnalyzeServer(t)
	defer ts.Close()

	b := newTestBrowser(t)

	analysis, err := AnalyzeSite(context.Background(), b, ts.URL+"/analyze-paginated")
	if err != nil {
		t.Fatalf("AnalyzeSite failed: %v", err)
	}

	if analysis.Pagination == nil {
		t.Fatal("expected pagination candidate")
	}

	if analysis.Pagination.Strategy != "click" {
		t.Errorf("pagination strategy = %q, want %q", analysis.Pagination.Strategy, "click")
	}
}

func TestAnalyzeSite_Metadata(t *testing.T) {
	ts := newAnalyzeServer(t)
	defer ts.Close()

	b := newTestBrowser(t)

	analysis, err := AnalyzeSite(context.Background(), b, ts.URL+"/analyze-listing")
	if err != nil {
		t.Fatalf("AnalyzeSite failed: %v", err)
	}

	if analysis.Metadata["title"] != "Product Listing" {
		t.Errorf("metadata title = %q, want %q", analysis.Metadata["title"], "Product Listing")
	}

	if analysis.Metadata["description"] != "Browse our products" {
		t.Errorf("metadata description = %q, want %q", analysis.Metadata["description"], "Browse our products")
	}
}

func TestGenerateRunbook_Extract(t *testing.T) {
	ts := newAnalyzeServer(t)
	defer ts.Close()

	b := newTestBrowser(t)

	analysis, err := AnalyzeSite(context.Background(), b, ts.URL+"/gen-listing")
	if err != nil {
		t.Fatalf("AnalyzeSite failed: %v", err)
	}

	r, err := GenerateRunbook(analysis)
	if err != nil {
		t.Fatalf("GenerateRunbook failed: %v", err)
	}

	if r.Type != "extract" {
		t.Errorf("type = %q, want %q", r.Type, "extract")
	}

	if r.Items == nil {
		t.Fatal("items should not be nil")
	}

	if r.Items.Container == "" {
		t.Error("container should not be empty")
	}

	if len(r.Items.Fields) == 0 {
		t.Error("fields should not be empty")
	}

	if r.Version != "1" {
		t.Errorf("version = %q, want %q", r.Version, "1")
	}
}

func TestGenerateRunbook_Automate(t *testing.T) {
	ts := newAnalyzeServer(t)
	defer ts.Close()

	b := newTestBrowser(t)

	analysis, err := AnalyzeSite(context.Background(), b, ts.URL+"/analyze-form")
	if err != nil {
		t.Fatalf("AnalyzeSite failed: %v", err)
	}

	r, err := GenerateRunbook(analysis)
	if err != nil {
		t.Fatalf("GenerateRunbook failed: %v", err)
	}

	if r.Type != "automate" {
		t.Errorf("type = %q, want %q", r.Type, "automate")
	}

	if len(r.Steps) == 0 {
		t.Error("steps should not be empty")
	}

	// First step should be navigate
	if r.Steps[0].Action != "navigate" {
		t.Errorf("first step action = %q, want %q", r.Steps[0].Action, "navigate")
	}
}

func TestGenerateRunbook_ForceType(t *testing.T) {
	ts := newAnalyzeServer(t)
	defer ts.Close()

	b := newTestBrowser(t)

	analysis, err := AnalyzeSite(context.Background(), b, ts.URL+"/analyze-form")
	if err != nil {
		t.Fatalf("AnalyzeSite failed: %v", err)
	}

	// Force extract type on a form page — should fail because no containers
	_, err = GenerateRunbook(analysis, WithGenerateType("extract"))
	if err == nil {
		t.Error("expected error when forcing extract on form-only page")
	}

	// Force automate type on listing page
	analysis2, err := AnalyzeSite(context.Background(), b, ts.URL+"/gen-listing")
	if err != nil {
		t.Fatalf("AnalyzeSite failed: %v", err)
	}

	_, err = GenerateRunbook(analysis2, WithGenerateType("automate"))
	if err == nil {
		t.Error("expected error when forcing automate on page without forms")
	}
}

func TestGenerateRunbook_Validate(t *testing.T) {
	ts := newAnalyzeServer(t)
	defer ts.Close()

	b := newTestBrowser(t)

	analysis, err := AnalyzeSite(context.Background(), b, ts.URL+"/gen-listing")
	if err != nil {
		t.Fatalf("AnalyzeSite failed: %v", err)
	}

	r, err := GenerateRunbook(analysis)
	if err != nil {
		t.Fatalf("GenerateRunbook failed: %v", err)
	}

	if err := r.Validate(); err != nil {
		t.Errorf("generated runbook failed validation: %v", err)
	}
}

func TestGenerateRunbook_RunEndToEnd(t *testing.T) {
	ts := newAnalyzeServer(t)
	defer ts.Close()

	b := newTestBrowser(t)

	analysis, err := AnalyzeSite(context.Background(), b, ts.URL+"/gen-listing")
	if err != nil {
		t.Fatalf("AnalyzeSite failed: %v", err)
	}

	r, err := GenerateRunbook(analysis)
	if err != nil {
		t.Fatalf("GenerateRunbook failed: %v", err)
	}

	result, err := Apply(context.Background(), b, r)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(result.Items) < 3 {
		t.Errorf("items count = %d, want >= 3", len(result.Items))
	}
}

func TestGenerateRunbook_NilAnalysis(t *testing.T) {
	_, err := GenerateRunbook(nil)
	if err == nil {
		t.Error("expected error for nil analysis")
	}
}
