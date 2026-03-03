package engine

import (
	"testing"
	"time"
)

func TestWaitSafe_NilPage(t *testing.T) {
	var p *Page

	err := p.WaitSafe(100 * time.Millisecond)
	if err == nil {
		t.Error("expected error for nil page")
	}
}

func TestWaitSafe_Normal(t *testing.T) {
	b := newTestBrowser(t)

	srv := newTestServer()
	defer srv.Close()

	page, err := b.NewPage(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}

	if err := page.WaitLoad(); err != nil {
		t.Fatal(err)
	}

	err = page.WaitSafe(500 * time.Millisecond)
	if err != nil {
		t.Errorf("WaitSafe should succeed on stable page: %v", err)
	}
}

func TestBrowserClose_NoZombies(t *testing.T) {
	b := newOwnedTestBrowser(t)

	// Verify Close returns no error and is idempotent.
	if err := b.Close(); err != nil {
		t.Fatalf("first Close failed: %v", err)
	}

	if err := b.Close(); err != nil {
		t.Fatalf("second Close (idempotent) failed: %v", err)
	}
}

func TestBrowserClose_Nil(t *testing.T) {
	var b *Browser
	if err := b.Close(); err != nil {
		t.Fatalf("nil browser Close should not error: %v", err)
	}
}

func TestHijack_InvalidRegexp(t *testing.T) {
	b := newTestBrowser(t)

	srv := newTestServer()
	defer srv.Close()

	page, err := b.NewPage(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}

	if err := page.WaitLoad(); err != nil {
		t.Fatal(err)
	}

	_, err = page.Hijack("[invalid", func(ctx *HijackContext) {})
	if err == nil {
		t.Error("expected error for invalid regexp pattern")
	}
}
