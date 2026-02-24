package scout

import (
	"net/http"
	"strings"
	"testing"
)

func init() {
	registerTestRoutes(func(mux *http.ServeMux) {
		mux.HandleFunc("/snapshot-basic", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<!DOCTYPE html>
<html><body>
<nav aria-label="Main Menu">
  <a href="/home">Home</a>
  <a href="/about">About</a>
</nav>
<main>
  <h1>Welcome</h1>
  <p>Some text content</p>
  <button>Submit</button>
</main>
</body></html>`))
		})

		mux.HandleFunc("/snapshot-form", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<!DOCTYPE html>
<html><body>
<form aria-label="Login">
  <label for="email">Email</label>
  <input type="email" id="email" placeholder="you@example.com">
  <label for="pass">Password</label>
  <input type="password" id="pass">
  <input type="checkbox" id="remember"> <label for="remember">Remember me</label>
  <button type="submit">Log In</button>
</form>
</body></html>`))
		})

		mux.HandleFunc("/snapshot-nested", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<!DOCTYPE html>
<html><body>
<main>
  <article>
    <h2>Article Title</h2>
    <section>
      <h3>Section 1</h3>
      <ul>
        <li>Item 1</li>
        <li>Item 2</li>
      </ul>
    </section>
  </article>
</main>
</body></html>`))
		})

		mux.HandleFunc("/snapshot-hidden", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<!DOCTYPE html>
<html><body>
<nav aria-label="Visible Nav">
  <a href="/visible">Visible Link</a>
</nav>
<div aria-hidden="true">
  <a href="/hidden">Hidden Link</a>
</div>
<div hidden>
  <button>Hidden Button</button>
</div>
</body></html>`))
		})
	})
}

func TestSnapshot_Basic(t *testing.T) {
	b := newTestBrowser(t)
	srv := newTestServer()
	defer srv.Close()

	page, err := b.NewPage(srv.URL + "/snapshot-basic")
	if err != nil {
		t.Fatal(err)
	}
	if err := page.WaitLoad(); err != nil {
		t.Fatal(err)
	}

	snap, err := page.Snapshot()
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(snap, "- document") {
		t.Error("snapshot should start with document")
	}
	if !strings.Contains(snap, `navigation "Main Menu"`) {
		t.Error("snapshot should contain navigation with label")
	}
	if !strings.Contains(snap, `link "Home"`) {
		t.Error("snapshot should contain Home link")
	}
	if !strings.Contains(snap, `link "About"`) {
		t.Error("snapshot should contain About link")
	}
	if !strings.Contains(snap, `heading "Welcome" level=1`) {
		t.Error("snapshot should contain h1 heading with level")
	}
	if !strings.Contains(snap, `button "Submit"`) {
		t.Error("snapshot should contain Submit button")
	}
	if !strings.Contains(snap, "[ref=") {
		t.Error("snapshot should contain ref markers")
	}
}

func TestSnapshot_Form(t *testing.T) {
	b := newTestBrowser(t)
	srv := newTestServer()
	defer srv.Close()

	page, err := b.NewPage(srv.URL + "/snapshot-form")
	if err != nil {
		t.Fatal(err)
	}
	if err := page.WaitLoad(); err != nil {
		t.Fatal(err)
	}

	snap, err := page.Snapshot()
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(snap, `form "Login"`) {
		t.Error("snapshot should contain form with label")
	}
	if !strings.Contains(snap, "textbox") {
		t.Error("snapshot should contain textbox for email input")
	}
	if !strings.Contains(snap, `button "Log In"`) {
		t.Error("snapshot should contain Log In button")
	}
	if !strings.Contains(snap, "checkbox") {
		t.Error("snapshot should contain checkbox")
	}
}

func TestSnapshot_ElementByRef(t *testing.T) {
	b := newTestBrowser(t)
	srv := newTestServer()
	defer srv.Close()

	page, err := b.NewPage(srv.URL + "/snapshot-basic")
	if err != nil {
		t.Fatal(err)
	}
	if err := page.WaitLoad(); err != nil {
		t.Fatal(err)
	}

	snap, err := page.Snapshot()
	if err != nil {
		t.Fatal(err)
	}

	for _, line := range strings.Split(snap, "\n") {
		if strings.Contains(line, `button "Submit"`) {
			start := strings.Index(line, "[ref=")
			end := strings.Index(line[start:], "]")
			if start < 0 || end < 0 {
				t.Fatal("could not find ref in button line")
			}
			ref := line[start+5 : start+end]

			el, err := page.ElementByRef(ref)
			if err != nil {
				t.Fatalf("ElementByRef(%q) failed: %v", ref, err)
			}

			text, err := el.Text()
			if err != nil {
				t.Fatal(err)
			}
			if text != "Submit" {
				t.Errorf("expected 'Submit', got %q", text)
			}
			return
		}
	}
	t.Fatal("button not found in snapshot")
}

func TestSnapshot_MaxDepth(t *testing.T) {
	b := newTestBrowser(t)
	srv := newTestServer()
	defer srv.Close()

	page, err := b.NewPage(srv.URL + "/snapshot-nested")
	if err != nil {
		t.Fatal(err)
	}
	if err := page.WaitLoad(); err != nil {
		t.Fatal(err)
	}

	full, err := page.Snapshot()
	if err != nil {
		t.Fatal(err)
	}

	shallow, err := page.SnapshotWithOptions(WithSnapshotMaxDepth(2))
	if err != nil {
		t.Fatal(err)
	}

	fullLines := strings.Count(full, "\n")
	shallowLines := strings.Count(shallow, "\n")
	if shallowLines >= fullLines {
		t.Errorf("shallow (%d lines) should have fewer lines than full (%d lines)", shallowLines, fullLines)
	}
}

func TestSnapshot_InteractableOnly(t *testing.T) {
	b := newTestBrowser(t)
	srv := newTestServer()
	defer srv.Close()

	page, err := b.NewPage(srv.URL + "/snapshot-basic")
	if err != nil {
		t.Fatal(err)
	}
	if err := page.WaitLoad(); err != nil {
		t.Fatal(err)
	}

	snap, err := page.SnapshotWithOptions(WithSnapshotInteractableOnly())
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(snap, "link") {
		t.Error("interactable snapshot should contain links")
	}
	if !strings.Contains(snap, "button") {
		t.Error("interactable snapshot should contain buttons")
	}
	if strings.Contains(snap, "heading") {
		t.Error("interactable snapshot should NOT contain headings")
	}
}

func TestSnapshot_Hidden(t *testing.T) {
	b := newTestBrowser(t)
	srv := newTestServer()
	defer srv.Close()

	page, err := b.NewPage(srv.URL + "/snapshot-hidden")
	if err != nil {
		t.Fatal(err)
	}
	if err := page.WaitLoad(); err != nil {
		t.Fatal(err)
	}

	snap, err := page.Snapshot()
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(snap, `link "Visible Link"`) {
		t.Error("snapshot should contain visible link")
	}
	if strings.Contains(snap, "Hidden Link") {
		t.Error("snapshot should NOT contain aria-hidden link")
	}
	if strings.Contains(snap, "Hidden Button") {
		t.Error("snapshot should NOT contain hidden button")
	}
}

func TestSnapshot_NilPage(t *testing.T) {
	var p *Page
	_, err := p.Snapshot()
	if err == nil {
		t.Error("expected error for nil page")
	}
}

func TestElementByRef_NotFound(t *testing.T) {
	b := newTestBrowser(t)
	srv := newTestServer()
	defer srv.Close()

	page, err := b.NewPage(srv.URL + "/snapshot-basic")
	if err != nil {
		t.Fatal(err)
	}
	if err := page.WaitLoad(); err != nil {
		t.Fatal(err)
	}

	_, err = page.ElementByRef("s999e999")
	if err == nil {
		t.Error("expected error for nonexistent ref")
	}
}

func TestElementByRef_EmptyRef(t *testing.T) {
	b := newTestBrowser(t)
	srv := newTestServer()
	defer srv.Close()

	page, err := b.NewPage(srv.URL + "/snapshot-basic")
	if err != nil {
		t.Fatal(err)
	}

	_, err = page.ElementByRef("")
	if err == nil {
		t.Error("expected error for empty ref")
	}
}
