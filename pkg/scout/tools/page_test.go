package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/inovacc/scout/pkg/scout"
)

// newPageTestBrowser returns a headless browser + a blank page, skipping when
// Chromium is unavailable (project norm — real browser, no mocks).
func newPageTestBrowser(t *testing.T) (*scout.Browser, *scout.Page) {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping browser test in short mode")
	}

	b, err := scout.New(scout.WithHeadless(true), scout.WithNoSandbox())
	if err != nil {
		t.Skipf("browser unavailable: %v", err)
	}

	t.Cleanup(func() { _ = b.Close() })

	p, err := b.NewPage("")
	if err != nil {
		t.Skipf("page unavailable: %v", err)
	}

	return b, p
}

// newTestServer serves a minimal HTML document with the given body fragment.
func newTestServer(t *testing.T, body string) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><head><title>T</title></head><body>" + body + "</body></html>"))
	}))
	t.Cleanup(srv.Close)

	return srv
}

func TestNavigate(t *testing.T) {
	_, p := newPageTestBrowser(t)
	srv := newTestServer(t, "<h1>hi</h1>")

	out, err := Navigate(context.Background(), p, NavigateInput{URL: srv.URL})
	if err != nil {
		t.Fatalf("Navigate: %v", err)
	}

	if out.Title != "T" {
		t.Errorf("Title = %q, want T", out.Title)
	}

	if out.URL == "" {
		t.Errorf("URL empty")
	}

	if _, err := Navigate(context.Background(), nil, NavigateInput{URL: srv.URL}); err == nil {
		t.Error("nil page should error")
	}

	if _, err := Navigate(context.Background(), p, NavigateInput{URL: ""}); err == nil {
		t.Error("empty url should error")
	}
}

func TestBackForward(t *testing.T) {
	b, p := newPageTestBrowser(t)
	if b == nil {
		t.Fatal("nil browser")
	}

	srv1 := newTestServer(t, "<p>one</p>")
	srv2 := newTestServer(t, "<p>two</p>")

	if _, err := Navigate(context.Background(), p, NavigateInput{URL: srv1.URL}); err != nil {
		t.Fatal(err)
	}

	if _, err := Navigate(context.Background(), p, NavigateInput{URL: srv2.URL}); err != nil {
		t.Fatal(err)
	}

	if _, err := Back(context.Background(), p, BackInput{}); err != nil {
		t.Errorf("Back: %v", err)
	}

	if _, err := Forward(context.Background(), p, ForwardInput{}); err != nil {
		t.Errorf("Forward: %v", err)
	}

	if _, err := Back(context.Background(), nil, BackInput{}); err == nil {
		t.Error("nil page Back should error")
	}

	if _, err := Forward(context.Background(), nil, ForwardInput{}); err == nil {
		t.Error("nil page Forward should error")
	}
}

func TestReload(t *testing.T) {
	_, p := newPageTestBrowser(t)
	srv := newTestServer(t, "<p>x</p>")

	if _, err := Navigate(context.Background(), p, NavigateInput{URL: srv.URL}); err != nil {
		t.Fatal(err)
	}

	if _, err := Reload(context.Background(), p, ReloadInput{}); err != nil {
		t.Errorf("Reload: %v", err)
	}

	if _, err := Reload(context.Background(), nil, ReloadInput{}); err == nil {
		t.Error("nil page should error")
	}
}

func TestWaitForElement(t *testing.T) {
	_, p := newPageTestBrowser(t)
	srv := newTestServer(t, `<div id="ready">ok</div>`)

	if _, err := Navigate(context.Background(), p, NavigateInput{URL: srv.URL}); err != nil {
		t.Fatal(err)
	}

	if _, err := Wait(context.Background(), p, WaitInput{Selector: "#ready"}); err != nil {
		t.Errorf("Wait selector: %v", err)
	}

	if _, err := Wait(context.Background(), p, WaitInput{}); err != nil {
		t.Errorf("Wait load: %v", err)
	}

	if _, err := Wait(context.Background(), nil, WaitInput{}); err == nil {
		t.Error("nil page should error")
	}
}

func TestClickTypeExtract(t *testing.T) {
	_, p := newPageTestBrowser(t)
	srv := newTestServer(t, `<input id="f"><button id="b">go</button>`)

	if _, err := Navigate(context.Background(), p, NavigateInput{URL: srv.URL}); err != nil {
		t.Fatal(err)
	}

	if _, err := Type(context.Background(), p, TypeInput{Selector: "#f", Text: "hello"}); err != nil {
		t.Errorf("Type: %v", err)
	}

	if _, err := Click(context.Background(), p, ClickInput{Selector: "#b"}); err != nil {
		t.Errorf("Click: %v", err)
	}

	out, err := Extract(context.Background(), p, ExtractInput{Selector: "#b"})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	if out.Text != "go" {
		t.Errorf("Extract Text = %q, want go", out.Text)
	}

	if _, err := Click(context.Background(), nil, ClickInput{Selector: "#b"}); err == nil {
		t.Error("nil page Click should error")
	}

	if _, err := Click(context.Background(), p, ClickInput{}); err == nil {
		t.Error("empty selector Click should error")
	}

	if _, err := Type(context.Background(), p, TypeInput{}); err == nil {
		t.Error("empty selector Type should error")
	}

	if _, err := Extract(context.Background(), p, ExtractInput{}); err == nil {
		t.Error("empty selector Extract should error")
	}
}

func TestEval(t *testing.T) {
	_, p := newPageTestBrowser(t)
	srv := newTestServer(t, "<p>x</p>")

	if _, err := Navigate(context.Background(), p, NavigateInput{URL: srv.URL}); err != nil {
		t.Fatal(err)
	}

	out, err := Eval(context.Background(), p, EvalInput{Expression: "() => 1+2"})
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}

	if out.Result != "3" {
		t.Errorf("Eval Result = %q, want 3", out.Result)
	}

	if _, err := Eval(context.Background(), nil, EvalInput{Expression: "1"}); err == nil {
		t.Error("nil page should error")
	}

	if _, err := Eval(context.Background(), p, EvalInput{}); err == nil {
		t.Error("empty expression should error")
	}
}

func TestContentReads(t *testing.T) {
	_, p := newPageTestBrowser(t)
	srv := newTestServer(t, "<h1>hello</h1>")

	if _, err := Navigate(context.Background(), p, NavigateInput{URL: srv.URL}); err != nil {
		t.Fatal(err)
	}

	titleOut, err := Title(context.Background(), p, TitleInput{})
	if err != nil {
		t.Fatalf("Title: %v", err)
	}

	if titleOut.Title != "T" {
		t.Errorf("Title = %q, want T", titleOut.Title)
	}

	urlOut, err := URL(context.Background(), p, URLInput{})
	if err != nil {
		t.Fatalf("URL: %v", err)
	}

	if urlOut.URL == "" {
		t.Errorf("URL empty")
	}

	htmlOut, err := HTML(context.Background(), p, HTMLInput{})
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}

	if !strings.Contains(htmlOut.HTML, "<body>") {
		t.Errorf("HTML missing <body>: %q", htmlOut.HTML)
	}

	mdOut, err := Markdown(context.Background(), p, MarkdownInput{})
	if err != nil {
		t.Fatalf("Markdown: %v", err)
	}

	if mdOut.Markdown == "" {
		t.Errorf("Markdown empty")
	}

	if _, err := Cookies(context.Background(), p, CookiesInput{}); err != nil {
		t.Errorf("Cookies: %v", err)
	}

	if _, err := HTML(context.Background(), nil, HTMLInput{}); err == nil {
		t.Error("nil page HTML should error")
	}

	if _, err := Markdown(context.Background(), nil, MarkdownInput{}); err == nil {
		t.Error("nil page Markdown should error")
	}

	if _, err := Cookies(context.Background(), nil, CookiesInput{}); err == nil {
		t.Error("nil page Cookies should error")
	}

	if _, err := URL(context.Background(), nil, URLInput{}); err == nil {
		t.Error("nil page URL should error")
	}

	if _, err := Title(context.Background(), nil, TitleInput{}); err == nil {
		t.Error("nil page Title should error")
	}
}
