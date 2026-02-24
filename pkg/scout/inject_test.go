package scout

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func init() {
	registerTestRoutes(func(mux *http.ServeMux) {
		mux.HandleFunc("/inject-test", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Inject Test</title></head>
<body>
<div id="result"></div>
<script>
if (window.__test !== undefined) {
  document.getElementById('result').textContent = 'value:' + window.__test;
}
if (window.__fromFile !== undefined) {
  document.getElementById('result').textContent += ',file:' + window.__fromFile;
}
if (window.__dir1 !== undefined) {
  document.getElementById('result').textContent += ',dir1:' + window.__dir1;
}
if (window.__dir2 !== undefined) {
  document.getElementById('result').textContent += ',dir2:' + window.__dir2;
}
</script>
</body></html>`)
		})
	})
}

func TestWithInjectCode(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	b, err := New(
		WithHeadless(true),
		WithNoSandbox(),
		WithInjectCode("window.__test = 42"),
	)
	if err != nil {
		t.Skipf("skipping: browser unavailable: %v", err)
	}
	defer func() { _ = b.Close() }()

	page, err := b.NewPage(ts.URL + "/inject-test")
	if err != nil {
		t.Fatalf("NewPage: %v", err)
	}

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad: %v", err)
	}

	el, err := page.Element("#result")
	if err != nil {
		t.Fatalf("Element: %v", err)
	}

	text, err := el.Text()
	if err != nil {
		t.Fatalf("Text: %v", err)
	}

	if text != "value:42" {
		t.Errorf("expected 'value:42', got %q", text)
	}
}

func TestWithInjectJS(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	dir := t.TempDir()
	jsFile := filepath.Join(dir, "helper.js")
	if err := os.WriteFile(jsFile, []byte("window.__fromFile = 'yes'"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	b, err := New(
		WithHeadless(true),
		WithNoSandbox(),
		WithInjectJS(jsFile),
	)
	if err != nil {
		t.Skipf("skipping: browser unavailable: %v", err)
	}
	defer func() { _ = b.Close() }()

	page, err := b.NewPage(ts.URL + "/inject-test")
	if err != nil {
		t.Fatalf("NewPage: %v", err)
	}

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad: %v", err)
	}

	el, err := page.Element("#result")
	if err != nil {
		t.Fatalf("Element: %v", err)
	}

	text, err := el.Text()
	if err != nil {
		t.Fatalf("Text: %v", err)
	}

	if text != ",file:yes" {
		t.Errorf("expected ',file:yes', got %q", text)
	}
}

func TestWithInjectDir(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "01_first.js"), []byte("window.__dir1 = 'a'"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "02_second.js"), []byte("window.__dir2 = 'b'"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	b, err := New(
		WithHeadless(true),
		WithNoSandbox(),
		WithInjectDir(dir),
	)
	if err != nil {
		t.Skipf("skipping: browser unavailable: %v", err)
	}
	defer func() { _ = b.Close() }()

	page, err := b.NewPage(ts.URL + "/inject-test")
	if err != nil {
		t.Fatalf("NewPage: %v", err)
	}

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad: %v", err)
	}

	el, err := page.Element("#result")
	if err != nil {
		t.Fatalf("Element: %v", err)
	}

	text, err := el.Text()
	if err != nil {
		t.Fatalf("Text: %v", err)
	}

	if text != ",dir1:a,dir2:b" {
		t.Errorf("expected ',dir1:a,dir2:b', got %q", text)
	}
}

func TestWithInjectJS_NotFound(t *testing.T) {
	_, err := New(
		WithHeadless(true),
		WithNoSandbox(),
		WithInjectJS("/nonexistent/path/to/file.js"),
	)
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}

func TestWithInjectCode_Empty(t *testing.T) {
	ts := newTestServer()
	defer ts.Close()

	b, err := New(
		WithHeadless(true),
		WithNoSandbox(),
		WithInjectCode("", ""),
	)
	if err != nil {
		t.Skipf("skipping: browser unavailable: %v", err)
	}
	defer func() { _ = b.Close() }()

	// Should work normally with no scripts injected.
	page, err := b.NewPage(ts.URL + "/")
	if err != nil {
		t.Fatalf("NewPage: %v", err)
	}

	title, err := page.Title()
	if err != nil {
		t.Fatalf("Title: %v", err)
	}

	if title != "Test Page" {
		t.Errorf("expected 'Test Page', got %q", title)
	}
}
