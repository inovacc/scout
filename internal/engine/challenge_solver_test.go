package engine

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

func init() {
	registerTestRoutes(func(mux *http.ServeMux) {
		// Cloudflare-like page that clears after JS runs.
		mux.HandleFunc("/challenge-cf-wait", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Just a moment</title></head>
<body>
<div id="cf-browser-verification">Checking your browser...</div>
<div id="challenge-running">Running challenge</div>
<script>
setTimeout(function() {
	document.getElementById('cf-browser-verification').remove();
	document.getElementById('challenge-running').remove();
	document.body.innerHTML = '<h1 id="content">Welcome</h1>';
}, 500);
</script>
</body></html>`)
		})

		// Destination after bypass.
		mux.HandleFunc("/challenge-solved", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Success</title></head>
<body><h1>Access Granted</h1></body></html>`)
		})

		// Page that always reports challenges (never clears).
		mux.HandleFunc("/challenge-persistent", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Just a moment</title></head>
<body>
<div id="cf-browser-verification">Checking your browser...</div>
<div id="challenge-running">Running</div>
<script>window.__cf_chl_opt = {};</script>
</body></html>`)
		})
	})
}

func TestChallengeSolver_NilBrowser(t *testing.T) {
	cs := NewChallengeSolver(nil)

	err := cs.Solve(nil)
	if err == nil {
		t.Fatal("expected error for nil browser/page")
	}
}

func TestChallengeSolver_Register(t *testing.T) {
	b := newTestBrowser(t)

	ts := newTestServer()
	defer ts.Close()

	var called atomic.Int32

	cs := NewChallengeSolver(b)
	cs.Register(ChallengeCloudflare, func(_ *Page, _ ChallengeInfo) error {
		called.Add(1)
		return nil
	})

	page, err := b.NewPage(ts.URL + "/challenge-persistent")
	if err != nil {
		t.Fatal(err)
	}

	if err := page.WaitLoad(); err != nil {
		t.Fatal(err)
	}

	if err := cs.Solve(page); err != nil {
		t.Fatalf("solve error: %v", err)
	}

	if called.Load() != 1 {
		t.Fatalf("expected custom solver to be called once, got %d", called.Load())
	}
}

func TestChallengeSolver_NoChallenge(t *testing.T) {
	b := newTestBrowser(t)

	ts := newTestServer()
	defer ts.Close()

	cs := NewChallengeSolver(b)

	page, err := b.NewPage(ts.URL + "/challenge-solved")
	if err != nil {
		t.Fatal(err)
	}

	if err := page.WaitLoad(); err != nil {
		t.Fatal(err)
	}

	if err := cs.Solve(page); err != nil {
		t.Fatalf("expected nil error for no challenge, got: %v", err)
	}
}

func TestChallengeSolver_CloudflareWait(t *testing.T) {
	b := newTestBrowser(t)

	ts := newTestServer()
	defer ts.Close()

	cs := NewChallengeSolver(b)

	page, err := b.NewPage(ts.URL + "/challenge-cf-wait")
	if err != nil {
		t.Fatal(err)
	}

	if err := page.WaitLoad(); err != nil {
		t.Fatal(err)
	}

	// The page removes challenge elements after 500ms JS timeout.
	if err := cs.Solve(page); err != nil {
		t.Fatalf("expected cloudflare wait to succeed, got: %v", err)
	}

	// Verify the content appeared.
	has, _ := page.Has("#content")
	if !has {
		t.Fatal("expected #content element after challenge cleared")
	}
}

func TestChallengeSolver_SolveAll_MaxRetries(t *testing.T) {
	b := newTestBrowser(t)

	ts := newTestServer()
	defer ts.Close()

	// Register a solver that always fails, so retries exhaust.
	cs := NewChallengeSolver(b)
	cs.Register(ChallengeCloudflare, func(_ *Page, _ ChallengeInfo) error {
		return fmt.Errorf("always fails")
	})

	page, err := b.NewPage(ts.URL + "/challenge-persistent")
	if err != nil {
		t.Fatal(err)
	}

	if err := page.WaitLoad(); err != nil {
		t.Fatal(err)
	}

	err = cs.SolveAll(page)
	if err == nil {
		t.Fatal("expected error after max retries")
	}

	if testing.Verbose() {
		t.Logf("got expected error: %v", err)
	}
}

func TestNavigateWithBypass_NilSolver(t *testing.T) {
	b := newTestBrowser(t)

	ts := newTestServer()
	defer ts.Close()

	page, err := b.NewPage("")
	if err != nil {
		t.Fatal(err)
	}

	// nil solver should fall back to normal navigation.
	if err := NavigateWithBypass(page, ts.URL+"/challenge-solved", nil); err != nil {
		t.Fatalf("expected nil solver to navigate normally, got: %v", err)
	}

	title, _ := page.Title()
	if title != "Success" {
		t.Fatalf("expected title 'Success', got %q", title)
	}
}

func TestSolverOptions(t *testing.T) {
	o := defaultSolverOptions()
	if o.timeout != 30*time.Second {
		t.Fatalf("expected default timeout 30s, got %v", o.timeout)
	}

	WithSolverTimeout(10 * time.Second)(o)

	if o.timeout != 10*time.Second {
		t.Fatalf("expected timeout 10s, got %v", o.timeout)
	}

	svc := NewTwoCaptchaService("test-key")
	WithSolverService(svc)(o)

	if len(o.services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(o.services))
	}
}

func TestCaptchaSolverService_Interface(t *testing.T) {
	// Verify both services implement the interface at compile time.
	var (
		_ CaptchaSolverService = (*TwoCaptchaService)(nil)
		_ CaptchaSolverService = (*CapSolverService)(nil)
	)

	tc := NewTwoCaptchaService("key")
	if tc.Name() != "2captcha" {
		t.Fatalf("expected name '2captcha', got %q", tc.Name())
	}

	cs := NewCapSolverService("key")
	if cs.Name() != "capsolver" {
		t.Fatalf("expected name 'capsolver', got %q", cs.Name())
	}
}
