package vault

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCaptureFromPageGrabsCookiesAndStorage(t *testing.T) {
	b, cleanup := newInjectTestBrowser(t)
	defer cleanup()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "sid", Value: "cap-cookie-7", Path: "/"})
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><script>
localStorage.setItem('lk','lv');
sessionStorage.setItem('sk','sv');
</script>ok</body></html>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	page, err := b.NewPage(srv.URL + "/")
	if err != nil {
		t.Fatalf("NewPage: %v", err)
	}
	defer func() { _ = page.Close() }()
	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad: %v", err)
	}

	in, err := CaptureFromPage(page, "cap")
	if err != nil {
		t.Fatalf("CaptureFromPage: %v", err)
	}
	if in.Name != "cap" {
		t.Errorf("Name = %q, want cap", in.Name)
	}

	var found bool
	for _, c := range in.Cookies {
		if c.Name == "sid" && c.Value == "cap-cookie-7" {
			found = true
		}
	}
	if !found {
		t.Errorf("captured cookies %v missing sid=cap-cookie-7", in.Cookies)
	}

	origin := originFrom(srv.URL)
	st, ok := in.Storage[origin]
	if !ok {
		t.Fatalf("no storage captured for origin %q (have %v)", origin, in.Storage)
	}
	if st.LocalStorage["lk"] != "lv" {
		t.Errorf("localStorage lk = %q, want lv", st.LocalStorage["lk"])
	}
	if st.SessionStorage["sk"] != "sv" {
		t.Errorf("sessionStorage sk = %q, want sv", st.SessionStorage["sk"])
	}
}
