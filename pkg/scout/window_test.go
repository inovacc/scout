package scout

import "testing"

func TestGetWindow(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	p, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = p.Close() }()

	bounds, err := p.GetWindow()
	if err != nil {
		t.Fatalf("GetWindow() error: %v", err)
	}

	if bounds.Width <= 0 || bounds.Height <= 0 {
		t.Errorf("expected positive dimensions, got %dx%d", bounds.Width, bounds.Height)
	}

	if bounds.State != WindowStateNormal {
		t.Logf("headless returned state=%q (may differ from normal)", bounds.State)
	}
}

func TestMaximize(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	p, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = p.Close() }()

	if err := p.Maximize(); err != nil {
		t.Fatalf("Maximize() error: %v", err)
	}

	bounds, err := p.GetWindow()
	if err != nil {
		t.Fatalf("GetWindow() error: %v", err)
	}

	if bounds.State != WindowStateMaximized {
		t.Logf("headless may not honor maximize; got state=%q", bounds.State)
	}
}

func TestMinimize(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	p, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = p.Close() }()

	if err := p.Minimize(); err != nil {
		t.Fatalf("Minimize() error: %v", err)
	}

	bounds, err := p.GetWindow()
	if err != nil {
		t.Fatalf("GetWindow() error: %v", err)
	}

	if bounds.State != WindowStateMinimized {
		t.Logf("headless may not honor minimize; got state=%q", bounds.State)
	}
}

func TestFullscreen(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	p, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = p.Close() }()

	if err := p.Fullscreen(); err != nil {
		t.Fatalf("Fullscreen() error: %v", err)
	}

	bounds, err := p.GetWindow()
	if err != nil {
		t.Fatalf("GetWindow() error: %v", err)
	}

	if bounds.State != WindowStateFullscreen {
		t.Logf("headless may not honor fullscreen; got state=%q", bounds.State)
	}
}

func TestRestoreWindow(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	p, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = p.Close() }()

	if err := p.Maximize(); err != nil {
		t.Fatalf("Maximize() error: %v", err)
	}

	if err := p.RestoreWindow(); err != nil {
		t.Fatalf("RestoreWindow() error: %v", err)
	}

	bounds, err := p.GetWindow()
	if err != nil {
		t.Fatalf("GetWindow() error: %v", err)
	}

	if bounds.State != WindowStateNormal {
		t.Logf("headless may not honor restore; got state=%q", bounds.State)
	}
}

func TestSetWindowAndGetWindow(t *testing.T) {
	srv := newTestServer()
	defer srv.Close()

	b := newTestBrowser(t)

	p, err := b.NewPage(srv.URL)
	if err != nil {
		t.Fatalf("NewPage() error: %v", err)
	}

	defer func() { _ = p.Close() }()

	if err := p.SetWindow(100, 200, 800, 600); err != nil {
		t.Fatalf("SetWindow() error: %v", err)
	}

	bounds, err := p.GetWindow()
	if err != nil {
		t.Fatalf("GetWindow() error: %v", err)
	}

	// Headless Chrome may not honor exact position/size, so we just verify no error
	// and that we get reasonable values back.
	if bounds.Width <= 0 || bounds.Height <= 0 {
		t.Errorf("expected positive dimensions after SetWindow, got %dx%d", bounds.Width, bounds.Height)
	}

	t.Logf("SetWindow result: left=%d top=%d %dx%d state=%s",
		bounds.Left, bounds.Top, bounds.Width, bounds.Height, bounds.State)
}

func TestWindowNilPage(t *testing.T) {
	var p *Page

	if _, err := p.GetWindow(); err == nil {
		t.Error("GetWindow on nil page should error")
	}

	if err := p.Minimize(); err == nil {
		t.Error("Minimize on nil page should error")
	}

	if err := p.Maximize(); err == nil {
		t.Error("Maximize on nil page should error")
	}

	if err := p.Fullscreen(); err == nil {
		t.Error("Fullscreen on nil page should error")
	}

	if err := p.RestoreWindow(); err == nil {
		t.Error("RestoreWindow on nil page should error")
	}
}
