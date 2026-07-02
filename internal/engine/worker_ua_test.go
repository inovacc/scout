package engine

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// TestWorkerUserAgentMatchesMainThread verifies plan 009 Step 1: a Web Worker
// reports the same navigator.userAgent as the main thread, and neither contains
// "HeadlessChrome". The main-thread UA is a per-page CDP override; workers run in
// a separate realm it never reaches, so before this fix a worker leaked the real
// HeadlessChrome/<v> UA — the inconsistency deviceandbrowserinfo and CreepJS key
// on (hasInconsistentWorkerValues). Applying the resolved UA as the process-global
// --user-agent switch makes the main thread and every worker agree.
//
// Real browser required; skips under -short or when Chromium is unavailable.
func TestWorkerUserAgentMatchesMainThread(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires a real browser")
	}

	const probe = `() => new Promise((resolve, reject) => {
		const blob = new Blob(["postMessage(navigator.userAgent)"], {type: "text/javascript"});
		const w = new Worker(URL.createObjectURL(blob));
		w.onmessage = (e) => resolve({main: navigator.userAgent, worker: e.data});
		w.onerror = (err) => reject(new Error("worker: " + (err && err.message ? err.message : "error")));
	})`

	for _, stealth := range []bool{false, true} {
		name := "plain"
		opts := []Option{WithHeadless(true), WithNoSandbox(), WithTimeout(30 * time.Second), WithoutBridge()}
		if stealth {
			name = "stealth"
			opts = append(opts, WithStealth())
		}

		t.Run(name, func(t *testing.T) {
			b, err := New(opts...)
			if err != nil {
				t.Skipf("browser unavailable: %v", err)
			}
			defer func() { _ = b.Close() }()

			page, err := b.NewPage("about:blank")
			if err != nil {
				t.Skipf("new page: %v", err)
			}
			defer func() { _ = page.Close() }()

			if err := page.WaitLoad(); err != nil {
				t.Skipf("wait load: %v", err)
			}

			res, err := page.Eval(probe)
			if err != nil {
				t.Fatalf("eval worker probe: %v", err)
			}

			var out struct {
				Main   string `json:"main"`
				Worker string `json:"worker"`
			}

			raw, err := json.Marshal(res.Value)
			if err != nil {
				t.Fatalf("marshal probe result: %v", err)
			}
			if err := json.Unmarshal(raw, &out); err != nil {
				t.Fatalf("unmarshal probe result %q: %v", string(raw), err)
			}

			if out.Main == "" || out.Worker == "" {
				t.Fatalf("empty UA: main=%q worker=%q", out.Main, out.Worker)
			}
			if out.Main != out.Worker {
				t.Errorf("worker UA differs from main thread:\n  main=  %q\n  worker=%q", out.Main, out.Worker)
			}
			if strings.Contains(out.Worker, "HeadlessChrome") {
				t.Errorf("worker UA leaks HeadlessChrome: %q", out.Worker)
			}
			if strings.Contains(out.Main, "HeadlessChrome") {
				t.Errorf("main UA leaks HeadlessChrome: %q", out.Main)
			}
		})
	}
}
