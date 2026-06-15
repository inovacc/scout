package engine

import (
	"testing"
	"time"
)

func TestClock(t *testing.T) {
	b := newTestBrowser(t)
	srv := dxServer(t)
	p, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("new page: %v", err)
	}
	_ = p.WaitLoad()

	clk := p.Clock()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := clk.Install(base); err != nil {
		t.Fatalf("install: %v", err)
	}

	t.Run("date frozen at install time", func(t *testing.T) {
		r, err := p.Eval("() => Date.now()")
		if err != nil || int64(r.Float()) != base.UnixMilli() {
			t.Fatalf("Date.now=%v err=%v want %d", r.Float(), err, base.UnixMilli())
		}
	})

	t.Run("timer fires only after fast-forward", func(t *testing.T) {
		if _, err := p.Eval("() => { window.__fired = false; setTimeout(function(){ window.__fired = true; }, 5000); return null; }"); err != nil {
			t.Fatalf("schedule: %v", err)
		}
		if r, _ := p.Eval("() => window.__fired"); r.Bool() {
			t.Fatalf("timer fired prematurely")
		}
		if err := clk.FastForward(6 * time.Second); err != nil {
			t.Fatalf("fast-forward: %v", err)
		}
		if r, _ := p.Eval("() => window.__fired"); !r.Bool() {
			t.Fatalf("timer did not fire after fast-forward")
		}
	})

	t.Run("now advances with fast-forward", func(t *testing.T) {
		got, err := clk.Now()
		if err != nil {
			t.Fatalf("now: %v", err)
		}
		want := base.Add(6 * time.Second).UnixMilli()
		if got.UnixMilli() != want {
			t.Fatalf("now=%d want %d", got.UnixMilli(), want)
		}
	})
}
