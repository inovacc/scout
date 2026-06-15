package engine

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

const dxTestHTML = `<!DOCTYPE html><html><head><title>DX Test</title></head><body>
<h1>Welcome</h1>
<button id="submit">Submit</button>
<label for="user">Username</label><input id="user" type="text">
<input type="search" placeholder="Search here">
<div data-testid="greeting">Hello Scout</div>
<input type="checkbox" id="agree">
<ul><li>one</li><li>two</li><li>three</li></ul>
<a href="/next">Next page</a>
</body></html>`

func dxServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(dxTestHTML))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestLocators(t *testing.T) {
	b := newTestBrowser(t)
	srv := dxServer(t)

	p, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("new page: %v", err)
	}
	if err := p.WaitLoad(); err != nil {
		t.Fatalf("wait load: %v", err)
	}

	t.Run("GetByTestID text", func(t *testing.T) {
		got, err := p.GetByTestID("greeting").TextContent()
		if err != nil || got != "Hello Scout" {
			t.Fatalf("got %q err %v, want %q", got, err, "Hello Scout")
		}
	})

	t.Run("GetByRole button name", func(t *testing.T) {
		loc := p.GetByRole("button", WithName("Submit"))
		if vis, err := loc.IsVisible(); err != nil || !vis {
			t.Fatalf("button not visible: vis=%v err=%v", vis, err)
		}
		if err := loc.Click(); err != nil {
			t.Fatalf("click: %v", err)
		}
	})

	t.Run("GetByLabel fill", func(t *testing.T) {
		loc := p.GetByLabel("Username")
		if err := loc.Fill("alice"); err != nil {
			t.Fatalf("fill: %v", err)
		}
		if v, err := loc.InputValue(); err != nil || v != "alice" {
			t.Fatalf("value=%q err=%v want alice", v, err)
		}
	})

	t.Run("GetByPlaceholder fill", func(t *testing.T) {
		loc := p.GetByPlaceholder("Search")
		if err := loc.Fill("golang"); err != nil {
			t.Fatalf("fill: %v", err)
		}
		if v, _ := loc.InputValue(); v != "golang" {
			t.Fatalf("value=%q want golang", v)
		}
	})

	t.Run("GetByRole listitem count", func(t *testing.T) {
		n, err := p.GetByRole("listitem").Count()
		if err != nil || n != 3 {
			t.Fatalf("count=%d err=%v want 3", n, err)
		}
	})

	t.Run("Check checkbox", func(t *testing.T) {
		loc := p.GetByRole("checkbox")
		if err := loc.Check(); err != nil {
			t.Fatalf("check: %v", err)
		}
		if c, _ := loc.IsChecked(); !c {
			t.Fatalf("checkbox not checked")
		}
	})
}

func TestExpectAssertions(t *testing.T) {
	b := newTestBrowser(t)
	srv := dxServer(t)
	p, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("new page: %v", err)
	}
	_ = p.WaitLoad()

	if err := Expect(p.GetByTestID("greeting")).ToBeVisible(); err != nil {
		t.Fatalf("ToBeVisible: %v", err)
	}
	if err := Expect(p.GetByTestID("greeting")).ToHaveText("Hello Scout"); err != nil {
		t.Fatalf("ToHaveText: %v", err)
	}
	if err := Expect(p.GetByRole("heading")).ToContainText("Welcome"); err != nil {
		t.Fatalf("ToContainText: %v", err)
	}
	if err := Expect(p.GetByRole("listitem")).ToHaveCount(3); err != nil {
		t.Fatalf("ToHaveCount: %v", err)
	}
	if err := Expect(p.GetByText("does-not-exist")).Not().ToBeVisible(); err != nil {
		t.Fatalf("Not().ToBeVisible: %v", err)
	}
	if err := ExpectPage(p).ToHaveTitle("DX Test"); err != nil {
		t.Fatalf("ToHaveTitle: %v", err)
	}
}

func TestEmulation(t *testing.T) {
	b := newTestBrowser(t)
	srv := dxServer(t)
	p, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("new page: %v", err)
	}
	_ = p.WaitLoad()

	t.Run("color scheme", func(t *testing.T) {
		if err := p.SetColorScheme("dark"); err != nil {
			t.Fatalf("set color scheme: %v", err)
		}
		r, err := p.Eval("() => matchMedia('(prefers-color-scheme: dark)').matches")
		if err != nil || !r.Bool() {
			t.Fatalf("dark not active: %v err=%v", r, err)
		}
	})

	t.Run("emulate device viewport", func(t *testing.T) {
		if err := p.EmulateDevice("iPhone 13"); err != nil {
			t.Fatalf("emulate device: %v", err)
		}
		// A page with no <meta viewport> lays out at the 980px mobile default,
		// so assert on devicePixelRatio (set by setDeviceMetricsOverride) — the
		// robust signal that the device metrics applied. iPhone 13 DPR = 3.
		r, err := p.Eval("() => window.devicePixelRatio")
		if err != nil {
			t.Fatalf("eval devicePixelRatio: %v", err)
		}
		if dpr := r.Float(); dpr != 3 {
			t.Fatalf("devicePixelRatio=%v, want 3 (iPhone 13 DPR) after emulation", dpr)
		}
	})

	t.Run("timezone override", func(t *testing.T) {
		if err := p.SetTimezone("Asia/Tokyo"); err != nil {
			t.Fatalf("set timezone: %v", err)
		}
		r, err := p.Eval("() => Intl.DateTimeFormat().resolvedOptions().timeZone")
		if err != nil || r.String() != "Asia/Tokyo" {
			t.Fatalf("timezone=%q err=%v want Asia/Tokyo", r.String(), err)
		}
	})
}
