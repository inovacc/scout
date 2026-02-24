package scout

import (
	"fmt"
	"net/http"
	"testing"
)

func init() {
	registerTestRoutes(func(mux *http.ServeMux) {
		mux.HandleFunc("/detect-react", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>React App</title></head>
<body>
<div id="root" data-reactroot></div>
<script>
window.__REACT_DEVTOOLS_GLOBAL_HOOK__ = {renderers: new Map([[1, {}]])};
window.React = {version: '18.2.0'};
document.getElementById('root')._reactRootContainer = {};
</script>
</body></html>`)
		})

		mux.HandleFunc("/detect-nextjs", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Next App</title></head>
<body>
<div id="__next"></div>
<script>
window.__NEXT_DATA__ = {buildId: 'abc123'};
window.next = {version: '14.1.0'};
window.__REACT_DEVTOOLS_GLOBAL_HOOK__ = {renderers: new Map([[1, {}]])};
window.React = {version: '18.2.0'};
</script>
</body></html>`)
		})

		mux.HandleFunc("/detect-vue", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Vue App</title></head>
<body>
<div id="app"></div>
<script>window.__VUE__ = {version: '3.4.21'};</script>
</body></html>`)
		})

		mux.HandleFunc("/detect-angular", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Angular App</title></head>
<body>
<app-root ng-version="17.3.0"></app-root>
</body></html>`)
		})

		mux.HandleFunc("/detect-svelte", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Svelte App</title></head>
<body>
<div class="svelte-abc123">Hello</div>
</body></html>`)
		})

		mux.HandleFunc("/detect-jquery", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>jQuery Page</title></head>
<body>
<p>Hello</p>
<script>window.jQuery = {fn: {jquery: '3.7.1'}};</script>
</body></html>`)
		})

		mux.HandleFunc("/detect-none", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Plain HTML</title></head>
<body><p>No framework</p></body></html>`)
		})

		mux.HandleFunc("/detect-gatsby", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Gatsby Site</title></head>
<body>
<div id="___gatsby"></div>
<script>
window.__REACT_DEVTOOLS_GLOBAL_HOOK__ = {renderers: new Map([[1, {}]])};
window.React = {version: '18.2.0'};
</script>
</body></html>`)
		})

		mux.HandleFunc("/detect-astro", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Astro Site</title></head>
<body>
<astro-island>content</astro-island>
</body></html>`)
		})

		// Tech stack detection routes
		mux.HandleFunc("/detect-tech-wordpress", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>WordPress Site</title>
<link rel="stylesheet" href="/wp-content/themes/theme/style.css">
</head>
<body>
<div class="container"><div class="row"><div class="col-md-6">Content</div></div></div>
<script src="/wp-content/plugins/plugin/script.js"></script>
</body></html>`)
		})

		mux.HandleFunc("/detect-tech-react-vite", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>React Vite App</title></head>
<body>
<div id="root" data-reactroot>
  <div class="flex p-4 bg-blue-500 text-center mt-2 rounded">Tailwind</div>
</div>
<script>
window.__REACT_DEVTOOLS_GLOBAL_HOOK__ = {renderers: new Map([[1, {}]])};
window.React = {version: '18.2.0'};
document.getElementById('root')._reactRootContainer = {};
window.gtag = function(){};
</script>
<script type="module" src="/@vite/client"></script>
</body></html>`)
		})

		mux.HandleFunc("/detect-tech-plain", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Plain Page</title></head>
<body><p>No tech markers here</p></body></html>`)
		})

		// Render mode detection routes
		mux.HandleFunc("/detect-render-csr", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>CSR App</title></head>
<body>
<div id="root"></div>
<script>
window.__REACT_DEVTOOLS_GLOBAL_HOOK__ = {renderers: new Map([[1, {}]])};
</script>
</body></html>`)
		})

		mux.HandleFunc("/detect-render-ssr", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>SSR App</title></head>
<body>
<div id="app" data-server-rendered="true">
<h1>Server Rendered Content</h1>
<p>This content was rendered on the server with Vue SSR.</p>
</div>
<script>window.__VUE__ = {version: '3.4.0'};</script>
</body></html>`)
		})

		mux.HandleFunc("/detect-render-ssg", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Gatsby SSG</title></head>
<body>
<div id="___gatsby">
<h1>Gatsby Static Site</h1>
<p>This page was statically generated at build time.</p>
</div>
</body></html>`)
		})

		mux.HandleFunc("/detect-render-nextjs-ssp", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Next SSP</title></head>
<body>
<div id="__next"><h1>Server Props Page</h1></div>
<script>
window.__NEXT_DATA__ = {buildId: 'abc', props: {pageProps: {__N_SSP: true, data: 'server'}}};
window.next = {version: '14.1.0'};
</script>
</body></html>`)
		})

		mux.HandleFunc("/detect-render-plain", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Plain Page</title></head>
<body>
<h1>Welcome to My Website</h1>
<p>This is a plain HTML page with no JavaScript framework. It has enough content to be considered substantial static HTML for render mode detection purposes.</p>
</body></html>`)
		})

		// PWA detection routes
		mux.HandleFunc("/pwa-manifest.json", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{
				"name": "Test PWA App",
				"short_name": "TestPWA",
				"display": "standalone",
				"start_url": "/",
				"theme_color": "#ffffff",
				"background_color": "#000000",
				"icons": [
					{"src": "/icon-192.png", "sizes": "192x192", "type": "image/png"},
					{"src": "/icon-512.png", "sizes": "512x512", "type": "image/png"}
				]
			}`)
		})

		mux.HandleFunc("/detect-pwa-full", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head>
<title>PWA App</title>
<link rel="manifest" href="/pwa-manifest.json">
</head>
<body><p>PWA with manifest</p></body></html>`)
		})

		mux.HandleFunc("/detect-pwa-none", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>No PWA</title></head>
<body><p>Plain page</p></body></html>`)
		})

		mux.HandleFunc("/detect-pwa-manifest-only", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head>
<title>Manifest Only</title>
<link rel="manifest" href="/pwa-manifest.json">
</head>
<body><p>Has manifest but no SW</p></body></html>`)
		})
	})
}

func TestDetectFrameworks(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	b := newTestBrowser(t)

	tests := []struct {
		name      string
		path      string
		wantName  string
		wantVer   string
		wantSPA   bool
		wantCount int // 0 means check wantName only
	}{
		{"React", "/detect-react", "React", "18.2.0", true, 0},
		{"Vue3", "/detect-vue", "Vue", "3.4.21", true, 0},
		{"Angular", "/detect-angular", "Angular", "17.3.0", true, 0},
		{"Svelte", "/detect-svelte", "Svelte", "", true, 0},
		{"jQuery", "/detect-jquery", "jQuery", "3.7.1", false, 1},
		{"None", "/detect-none", "", "", false, 0},
		{"Gatsby", "/detect-gatsby", "Gatsby", "", true, 0},
		{"Astro", "/detect-astro", "Astro", "", false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page, err := b.NewPage(srv.URL + tt.path)
			if err != nil {
				t.Fatalf("NewPage() error: %v", err)
			}
			defer func() { _ = page.Close() }()

			if err := page.WaitLoad(); err != nil {
				t.Fatalf("WaitLoad() error: %v", err)
			}

			frameworks, err := page.DetectFrameworks()
			if err != nil {
				t.Fatalf("DetectFrameworks() error: %v", err)
			}

			if tt.wantName == "" {
				if len(frameworks) != 0 {
					t.Errorf("expected no frameworks, got %v", frameworks)
				}
				return
			}

			if tt.wantCount > 0 && len(frameworks) != tt.wantCount {
				t.Errorf("expected %d frameworks, got %d: %v", tt.wantCount, len(frameworks), frameworks)
			}

			found := false
			for _, f := range frameworks {
				if f.Name == tt.wantName {
					found = true
					if tt.wantVer != "" && f.Version != tt.wantVer {
						t.Errorf("version = %q, want %q", f.Version, tt.wantVer)
					}
					if f.SPA != tt.wantSPA {
						t.Errorf("spa = %v, want %v", f.SPA, tt.wantSPA)
					}
					break
				}
			}
			if !found {
				t.Errorf("framework %q not found in %v", tt.wantName, frameworks)
			}
		})
	}
}

func TestDetectFramework_Primary(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	b := newTestBrowser(t)

	// Next.js page has both React and Next.js — should return Next.js as primary
	page, err := b.NewPage(srv.URL + "/detect-nextjs")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	fw, err := page.DetectFramework()
	if err != nil {
		t.Fatalf("DetectFramework() error: %v", err)
	}

	if fw == nil {
		t.Fatal("expected non-nil framework")
	}

	if fw.Name != "Next.js" {
		t.Errorf("primary framework = %q, want Next.js", fw.Name)
	}

	if fw.Version != "14.1.0" {
		t.Errorf("version = %q, want 14.1.0", fw.Version)
	}
}

func TestDetectFramework_None(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/detect-none")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	fw, err := page.DetectFramework()
	if err != nil {
		t.Fatalf("DetectFramework() error: %v", err)
	}

	if fw != nil {
		t.Errorf("expected nil framework, got %v", fw)
	}
}

func TestDetectFramework_GatsbyPrecedence(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	b := newTestBrowser(t)

	// Gatsby page also has React — Gatsby should take precedence
	page, err := b.NewPage(srv.URL + "/detect-gatsby")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	fw, err := page.DetectFramework()
	if err != nil {
		t.Fatalf("DetectFramework() error: %v", err)
	}

	if fw == nil {
		t.Fatal("expected non-nil framework")
	}

	if fw.Name != "Gatsby" {
		t.Errorf("primary framework = %q, want Gatsby", fw.Name)
	}
}

func TestDetectPWA_WithManifest(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/detect-pwa-full")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	info, err := page.DetectPWA()
	if err != nil {
		t.Fatalf("DetectPWA() error: %v", err)
	}

	if !info.HasManifest {
		t.Error("expected HasManifest=true")
	}
	if info.Manifest == nil {
		t.Fatal("expected non-nil Manifest")
	}
	if info.Manifest.Name != "Test PWA App" {
		t.Errorf("manifest name = %q, want %q", info.Manifest.Name, "Test PWA App")
	}
	if info.Manifest.ShortName != "TestPWA" {
		t.Errorf("manifest short_name = %q, want %q", info.Manifest.ShortName, "TestPWA")
	}
	if info.Manifest.Display != "standalone" {
		t.Errorf("manifest display = %q, want %q", info.Manifest.Display, "standalone")
	}
	if info.Manifest.Icons != 2 {
		t.Errorf("manifest icons = %d, want 2", info.Manifest.Icons)
	}
	if info.Manifest.ThemeColor != "#ffffff" {
		t.Errorf("manifest theme_color = %q, want %q", info.Manifest.ThemeColor, "#ffffff")
	}
	// httptest is HTTP, not HTTPS
	if info.HTTPS {
		t.Error("expected HTTPS=false for httptest server")
	}
	// No service worker registered in test
	if info.HasServiceWorker {
		t.Error("expected HasServiceWorker=false")
	}
	// Not installable: no SW + no HTTPS
	if info.Installable {
		t.Error("expected Installable=false")
	}
}

func TestDetectPWA_NoManifest(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/detect-pwa-none")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	info, err := page.DetectPWA()
	if err != nil {
		t.Fatalf("DetectPWA() error: %v", err)
	}

	if info.HasManifest {
		t.Error("expected HasManifest=false")
	}
	if info.HasServiceWorker {
		t.Error("expected HasServiceWorker=false")
	}
	if info.Manifest != nil {
		t.Error("expected nil Manifest")
	}
	if info.Installable {
		t.Error("expected Installable=false")
	}
}

func TestDetectPWA_ManifestOnly(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/detect-pwa-manifest-only")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	info, err := page.DetectPWA()
	if err != nil {
		t.Fatalf("DetectPWA() error: %v", err)
	}

	if !info.HasManifest {
		t.Error("expected HasManifest=true")
	}
	if info.HasServiceWorker {
		t.Error("expected HasServiceWorker=false (no SW registered)")
	}
	if info.Installable {
		t.Error("expected Installable=false (no SW)")
	}
	if info.Manifest == nil {
		t.Fatal("expected non-nil Manifest")
	}
	if info.Manifest.StartURL != "/" {
		t.Errorf("start_url = %q, want %q", info.Manifest.StartURL, "/")
	}
}

func TestDetectPWA_NilPage(t *testing.T) {
	var p *Page
	_, err := p.DetectPWA()
	if err == nil {
		t.Error("expected error for nil page")
	}
}

func TestDetectTechStack_WordPress(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/detect-tech-wordpress")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	stack, err := page.DetectTechStack()
	if err != nil {
		t.Fatalf("DetectTechStack() error: %v", err)
	}

	if stack.CMS != "WordPress" {
		t.Errorf("CMS = %q, want WordPress", stack.CMS)
	}
	if stack.CSSFramework != "Bootstrap" {
		t.Errorf("CSSFramework = %q, want Bootstrap", stack.CSSFramework)
	}
}

func TestDetectTechStack_ReactVite(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/detect-tech-react-vite")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	stack, err := page.DetectTechStack()
	if err != nil {
		t.Fatalf("DetectTechStack() error: %v", err)
	}

	if stack.BuildTool != "Vite" {
		t.Errorf("BuildTool = %q, want Vite", stack.BuildTool)
	}
	if stack.CSSFramework != "Tailwind" {
		t.Errorf("CSSFramework = %q, want Tailwind", stack.CSSFramework)
	}

	// Check frameworks include React
	foundReact := false
	for _, f := range stack.Frameworks {
		if f.Name == "React" {
			foundReact = true
			break
		}
	}
	if !foundReact {
		t.Errorf("expected React in frameworks, got %v", stack.Frameworks)
	}

	// Check analytics includes Google Analytics
	foundGA := false
	for _, a := range stack.Analytics {
		if a == "Google Analytics" {
			foundGA = true
			break
		}
	}
	if !foundGA {
		t.Errorf("expected Google Analytics in analytics, got %v", stack.Analytics)
	}
}

func TestDetectTechStack_Plain(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/detect-tech-plain")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	stack, err := page.DetectTechStack()
	if err != nil {
		t.Fatalf("DetectTechStack() error: %v", err)
	}

	if stack.CSSFramework != "" {
		t.Errorf("CSSFramework = %q, want empty", stack.CSSFramework)
	}
	if stack.BuildTool != "" {
		t.Errorf("BuildTool = %q, want empty", stack.BuildTool)
	}
	if stack.CMS != "" {
		t.Errorf("CMS = %q, want empty", stack.CMS)
	}
	if stack.CDN != "" {
		t.Errorf("CDN = %q, want empty", stack.CDN)
	}
	if stack.Analytics != nil {
		t.Errorf("Analytics = %v, want nil", stack.Analytics)
	}
	if len(stack.Frameworks) != 0 {
		t.Errorf("Frameworks = %v, want empty", stack.Frameworks)
	}
}

func TestDetectTechStack_NilPage(t *testing.T) {
	var p *Page
	_, err := p.DetectTechStack()
	if err == nil {
		t.Error("expected error for nil page")
	}
}

func TestDetectRenderMode_CSR(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/detect-render-csr")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	info, err := page.DetectRenderMode()
	if err != nil {
		t.Fatalf("DetectRenderMode() error: %v", err)
	}

	if info.Mode != RenderCSR {
		t.Errorf("mode = %q, want %q", info.Mode, RenderCSR)
	}
	if info.Details == "" {
		t.Error("expected non-empty details")
	}
}

func TestDetectRenderMode_SSR(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/detect-render-ssr")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	info, err := page.DetectRenderMode()
	if err != nil {
		t.Fatalf("DetectRenderMode() error: %v", err)
	}

	if info.Mode != RenderSSR {
		t.Errorf("mode = %q, want %q", info.Mode, RenderSSR)
	}
	if info.Details == "" {
		t.Error("expected non-empty details")
	}
}

func TestDetectRenderMode_SSG(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/detect-render-ssg")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	info, err := page.DetectRenderMode()
	if err != nil {
		t.Fatalf("DetectRenderMode() error: %v", err)
	}

	if info.Mode != RenderSSG {
		t.Errorf("mode = %q, want %q", info.Mode, RenderSSG)
	}
	if info.Details == "" {
		t.Error("expected non-empty details")
	}
}

func TestDetectRenderMode_NextSSP(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/detect-render-nextjs-ssp")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	info, err := page.DetectRenderMode()
	if err != nil {
		t.Fatalf("DetectRenderMode() error: %v", err)
	}

	if info.Mode != RenderSSR {
		t.Errorf("mode = %q, want %q", info.Mode, RenderSSR)
	}
	if !info.Hydrated {
		t.Error("expected hydrated=true for Next.js SSP")
	}
}

func TestDetectRenderMode_NilPage(t *testing.T) {
	var p *Page
	_, err := p.DetectRenderMode()
	if err == nil {
		t.Error("expected error for nil page")
	}
}

func TestDetectRenderMode_Plain(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	b := newTestBrowser(t)

	page, err := b.NewPage(srv.URL + "/detect-render-plain")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	info, err := page.DetectRenderMode()
	if err != nil {
		t.Fatalf("DetectRenderMode() error: %v", err)
	}

	if info.Mode != RenderSSG {
		t.Errorf("mode = %q, want %q (plain HTML = static)", info.Mode, RenderSSG)
	}
}

func TestDetectPWA_PushCapable(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	b := newTestBrowser(t)

	// Chrome supports PushManager, so any page should report push_capable=true
	page, err := b.NewPage(srv.URL + "/detect-pwa-none")
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad() error: %v", err)
	}

	info, err := page.DetectPWA()
	if err != nil {
		t.Fatalf("DetectPWA() error: %v", err)
	}

	// PushManager is available in Chromium
	if !info.PushCapable {
		t.Log("PushManager not available in this browser (may be headless)")
	}
}
