package scout

import (
	"testing"
)

func TestCallExposed_NilServer(t *testing.T) {
	var s *BridgeServer
	_, err := s.CallExposed("page1", "myFunc")
	if err == nil {
		t.Fatal("expected error for nil server")
	}
}

func TestEmitEvent_NilServer(t *testing.T) {
	var s *BridgeServer
	err := s.EmitEvent("page1", "myEvent", map[string]any{"key": "value"})
	if err == nil {
		t.Fatal("expected error for nil server")
	}
}

func TestQueryShadowDOM_NilServer(t *testing.T) {
	var s *BridgeServer
	_, err := s.QueryShadowDOM("page1", "div")
	if err == nil {
		t.Fatal("expected error for nil server")
	}
}

func TestListFrames_NilServer(t *testing.T) {
	var s *BridgeServer
	_, err := s.ListFrames("page1")
	if err == nil {
		t.Fatal("expected error for nil server")
	}
}

func TestSendToFrame_NilServer(t *testing.T) {
	var s *BridgeServer
	_, err := s.SendToFrame("page1", 0, "test", nil)
	if err == nil {
		t.Fatal("expected error for nil server")
	}
}

func TestQueryShadowDOM_NoClient(t *testing.T) {
	s := NewBridgeServer("127.0.0.1:0")
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = s.Stop() }()

	_, err := s.QueryShadowDOM("nonexistent", "div")
	if err == nil {
		t.Fatal("expected error for nonexistent client")
	}
}

func TestListFrames_NoClient(t *testing.T) {
	s := NewBridgeServer("127.0.0.1:0")
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = s.Stop() }()

	_, err := s.ListFrames("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent client")
	}
}

func TestBridgeFallback_NilPage(t *testing.T) {
	f := NewBridgeFallback(nil)

	_, err := f.Query("div")
	if err == nil {
		t.Fatal("expected error for nil page Query")
	}

	err = f.Click("div")
	if err == nil {
		t.Fatal("expected error for nil page Click")
	}

	err = f.Type("input", "text")
	if err == nil {
		t.Fatal("expected error for nil page Type")
	}

	_, err = f.Eval("1+1")
	if err == nil {
		t.Fatal("expected error for nil page Eval")
	}
}

func TestBridgeFallback_Query(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping browser test in short mode")
	}

	ts := newTestServer()
	defer ts.Close()

	b := newTestBrowser(t)
	defer func() { _ = b.Close() }()

	page, err := b.NewPage(ts.URL + "/")
	if err != nil {
		t.Fatalf("NewPage: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad: %v", err)
	}

	f := NewBridgeFallback(page)

	results, err := f.Query("h1")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected at least one h1 element")
	}

	if results[0]["tag"] != "h1" {
		t.Errorf("expected tag h1, got %v", results[0]["tag"])
	}
}

func TestBridgeFallback_Click(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping browser test in short mode")
	}

	ts := newTestServer()
	defer ts.Close()

	b := newTestBrowser(t)
	defer func() { _ = b.Close() }()

	page, err := b.NewPage(ts.URL + "/form")
	if err != nil {
		t.Fatalf("NewPage: %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.WaitLoad(); err != nil {
		t.Fatalf("WaitLoad: %v", err)
	}

	f := NewBridgeFallback(page)

	// Click should not error on existing element.
	err = f.Click("form")
	if err != nil {
		t.Fatalf("Click: %v", err)
	}
}

func TestBridgeFallback_BridgeConnected(t *testing.T) {
	f := &BridgeFallback{}
	if f.bridgeConnected() {
		t.Error("expected not connected for empty fallback")
	}

	var nilFb *BridgeFallback
	if nilFb.bridgeConnected() {
		t.Error("expected not connected for nil fallback")
	}
}

func TestBridgeFallback_SetPageID(t *testing.T) {
	f := NewBridgeFallback(nil)
	f.SetPageID("test-page")
	if f.pageID != "test-page" {
		t.Errorf("expected pageID test-page, got %s", f.pageID)
	}

	// Nil safety.
	var nilFb *BridgeFallback
	nilFb.SetPageID("test") // should not panic
}
