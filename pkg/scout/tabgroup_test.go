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
