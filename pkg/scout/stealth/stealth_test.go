package stealth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/inovacc/scout/pkg/scout/rod"
	"github.com/inovacc/scout/pkg/scout/rod/lib/launcher"
)

func newTestBrowser(t *testing.T) *rod.Browser {
	t.Helper()

	path, found := launcher.LookPath()
	if !found {
		t.Skipf("browser not available: no chromium found in PATH")
	}

	u, err := launcher.New().Bin(path).Headless(true).NoSandbox(true).Launch()
	if err != nil {
		t.Skipf("browser not available: %v", err)
	}

	b := rod.New().ControlURL(u)
	if err := b.Connect(); err != nil {
		t.Skipf("browser not available: %v", err)
	}

	t.Cleanup(func() { _ = b.Close() })
	return b
}

func newTestServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><head><title>Test</title></head><body><canvas id="c" width="100" height="100"></canvas></body></html>`))
	})
	return httptest.NewServer(mux)
}

func TestStealth_WebdriverHidden(t *testing.T) {
	b := newTestBrowser(t)
	ts := newTestServer()
	defer ts.Close()

	p, err := Page(b)
	if err != nil {
		t.Fatalf("stealth page: %v", err)
	}

	if err := p.Navigate(ts.URL); err != nil {
		t.Fatalf("navigate: %v", err)
	}
	_ = p.WaitLoad()

	result := p.MustEval(`() => navigator.webdriver`)
	if result.Bool() {
		t.Error("navigator.webdriver should be false/undefined in stealth mode")
	}
}

func TestStealth_ChromeObject(t *testing.T) {
	b := newTestBrowser(t)
	ts := newTestServer()
	defer ts.Close()

	p, err := Page(b)
	if err != nil {
		t.Fatalf("stealth page: %v", err)
	}

	if err := p.Navigate(ts.URL); err != nil {
		t.Fatalf("navigate: %v", err)
	}
	_ = p.WaitLoad()

	// Stealth should ensure window.chrome exists (bot detectors check for it)
	result := p.MustEval(`() => !!window.chrome`)
	if !result.Bool() {
		t.Error("window.chrome should be truthy in stealth mode")
	}
}

func TestStealth_CanvasNoise(t *testing.T) {
	b := newTestBrowser(t)
	ts := newTestServer()
	defer ts.Close()

	drawJS := `() => {
		const c = document.getElementById('c');
		const ctx = c.getContext('2d');
		ctx.fillStyle = 'red';
		ctx.fillRect(0, 0, 50, 50);
		ctx.fillStyle = 'blue';
		ctx.fillRect(50, 0, 50, 50);
		ctx.fillStyle = 'green';
		ctx.font = '14px Arial';
		ctx.fillText('test', 10, 70);
		return c.toDataURL();
	}`

	// Create two stealth pages and draw the same thing
	p1, err := Page(b)
	if err != nil {
		t.Fatalf("stealth page 1: %v", err)
	}
	if err := p1.Navigate(ts.URL); err != nil {
		t.Fatalf("navigate p1: %v", err)
	}
	_ = p1.WaitLoad()
	data1 := p1.MustEval(drawJS).String()

	p2, err := Page(b)
	if err != nil {
		t.Fatalf("stealth page 2: %v", err)
	}
	if err := p2.Navigate(ts.URL); err != nil {
		t.Fatalf("navigate p2: %v", err)
	}
	_ = p2.WaitLoad()
	data2 := p2.MustEval(drawJS).String()

	if data1 == data2 {
		t.Error("canvas toDataURL should differ between stealth pages due to noise injection")
	}
}

func TestStealth_WebGLVendor(t *testing.T) {
	b := newTestBrowser(t)
	ts := newTestServer()
	defer ts.Close()

	p, err := Page(b)
	if err != nil {
		t.Fatalf("stealth page: %v", err)
	}

	if err := p.Navigate(ts.URL); err != nil {
		t.Fatalf("navigate: %v", err)
	}
	_ = p.WaitLoad()

	result := p.MustEval(`() => {
		const c = document.createElement('canvas');
		const gl = c.getContext('webgl');
		if (!gl) return '';
		const ext = gl.getExtension('WEBGL_debug_renderer_info');
		if (!ext) return '';
		return gl.getParameter(ext.UNMASKED_VENDOR_WEBGL);
	}`)

	vendor := result.String()
	if vendor == "" {
		t.Skip("WebGL not available in this environment")
	}
	if vendor != "Intel Inc." {
		t.Errorf("expected vendor 'Intel Inc.', got %q", vendor)
	}
}

func TestStealth_ExtraJS_Applied(t *testing.T) {
	b := newTestBrowser(t)
	ts := newTestServer()
	defer ts.Close()

	p, err := Page(b)
	if err != nil {
		t.Fatalf("stealth page: %v", err)
	}

	if err := p.Navigate(ts.URL); err != nil {
		t.Fatalf("navigate: %v", err)
	}
	_ = p.WaitLoad()

	// Verify that ExtraJS was injected without errors by checking one of its effects:
	// navigator.connection should have effectiveType "4g"
	result := p.MustEval(`() => {
		if (navigator.connection && navigator.connection.effectiveType) {
			return navigator.connection.effectiveType;
		}
		return 'unknown';
	}`)

	val := result.String()
	if val != "4g" {
		t.Errorf("expected navigator.connection.effectiveType '4g', got %q", val)
	}
}
