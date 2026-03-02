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

	for i := range 3 {
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

	for i := range 3 {
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

func TestTabGroupNavigate(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	tg, err := b.NewTabGroup(2)
	if err != nil {
		t.Fatalf("NewTabGroup: %v", err)
	}

	defer func() { _ = tg.Close() }()

	errs := tg.Navigate(srv.URL, srv.URL+"/page2")
	for i, e := range errs {
		if e != nil {
			t.Fatalf("Navigate tab %d: %v", i, e)
		}
	}
}

func TestTabGroupNavigateMismatch(t *testing.T) {
	b := newTestBrowser(t)

	tg, err := b.NewTabGroup(2)
	if err != nil {
		t.Fatalf("NewTabGroup: %v", err)
	}

	defer func() { _ = tg.Close() }()

	errs := tg.Navigate("http://localhost:1")
	for i, e := range errs {
		if e == nil {
			t.Fatalf("Navigate tab %d: expected error, got nil", i)
		}
	}
}

func TestTabGroupWait(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	tg, err := b.NewTabGroup(1)
	if err != nil {
		t.Fatalf("NewTabGroup: %v", err)
	}

	defer func() { _ = tg.Close() }()

	errs := tg.Navigate(srv.URL)
	if errs[0] != nil {
		t.Fatalf("Navigate: %v", errs[0])
	}

	err = tg.Wait(0, func(p *Page) bool {
		title, _ := p.Title()
		return title != ""
	}, 5*time.Second)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
}

func TestTabGroupWaitTimeout(t *testing.T) {
	b := newTestBrowser(t)

	tg, err := b.NewTabGroup(1)
	if err != nil {
		t.Fatalf("NewTabGroup: %v", err)
	}

	defer func() { _ = tg.Close() }()

	err = tg.Wait(0, func(p *Page) bool {
		return false
	}, 100*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestTabGroupCollect(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	tg, err := b.NewTabGroup(2)
	if err != nil {
		t.Fatalf("NewTabGroup: %v", err)
	}

	defer func() { _ = tg.Close() }()

	errs := tg.Navigate(srv.URL, srv.URL+"/page2")
	for i, e := range errs {
		if e != nil {
			t.Fatalf("Navigate tab %d: %v", i, e)
		}
	}

	titles, cerrs := TabGroupCollect(tg, func(p *Page) (string, error) {
		return p.Title()
	})
	for i, e := range cerrs {
		if e != nil {
			t.Fatalf("Collect tab %d: %v", i, e)
		}
	}

	if len(titles) != 2 {
		t.Fatalf("expected 2 titles, got %d", len(titles))
	}

	for i, title := range titles {
		if title == "" {
			t.Fatalf("tab %d: expected non-empty title", i)
		}
	}
}

func TestTabGroupStore(t *testing.T) {
	b := newTestBrowser(t)

	tg, err := b.NewTabGroup(1)
	if err != nil {
		t.Fatalf("NewTabGroup: %v", err)
	}

	defer func() { _ = tg.Close() }()

	tg.Store.Store("token", "abc123")

	val, ok := tg.Store.Load("token")
	if !ok {
		t.Fatal("expected Store to contain 'token'")
	}

	if val.(string) != "abc123" {
		t.Fatalf("Store value = %v, want abc123", val)
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
