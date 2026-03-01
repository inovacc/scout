package scout

import (
	"testing"
	"time"
)

func TestNewTabGroup(t *testing.T) {
	b := newTestBrowser(t)

	tg, err := b.NewTabGroup(3)
	if err != nil {
		t.Fatalf("NewTabGroup(3): %v", err)
	}
	defer func() { _ = tg.Close() }()

	if tg.Len() != 3 {
		t.Fatalf("Len() = %d, want 3", tg.Len())
	}
	for i := 0; i < 3; i++ {
		if tg.Tab(i) == nil {
			t.Fatalf("Tab(%d) returned nil", i)
		}
	}
}

func TestNewTabGroupZero(t *testing.T) {
	b := newTestBrowser(t)

	_, err := b.NewTabGroup(0)
	if err == nil {
		t.Fatal("expected error for n=0, got nil")
	}
}

func TestTabGroupNilBrowser(t *testing.T) {
	var b *Browser
	_, err := b.NewTabGroup(1)
	if err == nil {
		t.Fatal("expected error for nil browser, got nil")
	}
}

func TestTabGroupOptions(t *testing.T) {
	b := newTestBrowser(t)

	tg, err := b.NewTabGroup(1,
		WithTabGroupRateLimit(5.0),
		WithTabGroupTimeout(10*time.Second),
	)
	if err != nil {
		t.Fatalf("NewTabGroup: %v", err)
	}
	defer func() { _ = tg.Close() }()

	if tg.limiter == nil {
		t.Fatal("expected limiter to be set")
	}
	if tg.timeout != 10*time.Second {
		t.Fatalf("timeout = %v, want 10s", tg.timeout)
	}
}

func TestTabGroupDo(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	b := newTestBrowser(t)

	tg, err := b.NewTabGroup(2)
	if err != nil {
		t.Fatalf("NewTabGroup: %v", err)
	}
	defer func() { _ = tg.Close() }()

	err = tg.Do(0, func(p *Page) error {
		return p.Navigate(srv.URL)
	})
	if err != nil {
		t.Fatalf("Do(0): %v", err)
	}
	title, err := tg.Tab(0).Title()
	if err != nil {
		t.Fatalf("Title: %v", err)
	}
	if title == "" {
		t.Fatal("expected non-empty title")
	}
}

func TestTabGroupDoAll(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	b := newTestBrowser(t)

	tg, err := b.NewTabGroup(3)
	if err != nil {
		t.Fatalf("NewTabGroup: %v", err)
	}
	defer func() { _ = tg.Close() }()

	err = tg.DoAll(func(i int, p *Page) error {
		return p.Navigate(srv.URL)
	})
	if err != nil {
		t.Fatalf("DoAll: %v", err)
	}
	for i := 0; i < 3; i++ {
		title, err := tg.Tab(i).Title()
		if err != nil {
			t.Fatalf("Tab(%d) Title: %v", i, err)
		}
		if title == "" {
			t.Fatalf("Tab(%d) expected non-empty title", i)
		}
	}
}

func TestTabGroupDoParallel(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	b := newTestBrowser(t)

	tg, err := b.NewTabGroup(3)
	if err != nil {
		t.Fatalf("NewTabGroup: %v", err)
	}
	defer func() { _ = tg.Close() }()

	errs := tg.DoParallel(func(i int, p *Page) error {
		return p.Navigate(srv.URL)
	})
	for i, e := range errs {
		if e != nil {
			t.Fatalf("DoParallel tab %d: %v", i, e)
		}
	}
}

func TestTabGroupBroadcast(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()
	b := newTestBrowser(t)

	tg, err := b.NewTabGroup(3)
	if err != nil {
		t.Fatalf("NewTabGroup: %v", err)
	}
	defer func() { _ = tg.Close() }()

	errs := tg.Broadcast(func(p *Page) error {
		return p.Navigate(srv.URL)
	})
	for i, e := range errs {
		if e != nil {
			t.Fatalf("Broadcast tab %d: %v", i, e)
		}
	}
}

func TestTabGroupDoOutOfRange(t *testing.T) {
	b := newTestBrowser(t)

	tg, err := b.NewTabGroup(1)
	if err != nil {
		t.Fatalf("NewTabGroup: %v", err)
	}
	defer func() { _ = tg.Close() }()

	err = tg.Do(5, func(p *Page) error { return nil })
	if err == nil {
		t.Fatal("expected error for out-of-range index, got nil")
	}
}

func TestTabGroupCloseIdempotent(t *testing.T) {
	b := newTestBrowser(t)

	tg, err := b.NewTabGroup(1)
	if err != nil {
		t.Fatalf("NewTabGroup: %v", err)
	}

	if err := tg.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := tg.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}

	// nil TabGroup close
	var nilTG *TabGroup
	if err := nilTG.Close(); err != nil {
		t.Fatalf("nil Close: %v", err)
	}
}
