package scout

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"golang.org/x/net/websocket"
)

func TestBridgeRecorder_StartStop(t *testing.T) {
	s := NewBridgeServer("127.0.0.1:0")
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = s.Stop() }()

	rec := NewBridgeRecorder(s)
	rec.Start()

	// Verify recording state.
	rec.mu.Lock()
	if !rec.recording {
		t.Fatal("expected recording to be true after Start")
	}
	rec.mu.Unlock()

	steps := rec.Stop()
	if len(steps) != 0 {
		t.Fatalf("expected 0 steps, got %d", len(steps))
	}

	// Double stop is safe.
	steps = rec.Stop()
	if steps != nil {
		t.Fatalf("expected nil from double Stop, got %v", steps)
	}
}

func TestBridgeRecorder_EventConversion(t *testing.T) {
	s := NewBridgeServer("127.0.0.1:0")
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = s.Stop() }()

	rec := NewBridgeRecorder(s)
	rec.Start()

	conn := connectTestClient(t, s.Addr(), "page-rec")
	defer func() { _ = conn.Close() }()

	// Send user.click event.
	clickEvt := BridgeMessage{
		Type:   "event",
		Method: "user.click",
		Params: json.RawMessage(`{"selector": "#submit-btn", "x": 100, "y": 200}`),
	}
	if err := websocket.JSON.Send(conn, clickEvt); err != nil {
		t.Fatalf("send click: %v", err)
	}

	// Send user.input event.
	inputEvt := BridgeMessage{
		Type:   "event",
		Method: "user.input",
		Params: json.RawMessage(`{"selector": "#email", "value": "test@example.com"}`),
	}
	if err := websocket.JSON.Send(conn, inputEvt); err != nil {
		t.Fatalf("send input: %v", err)
	}

	// Send navigation event.
	navEvt := BridgeMessage{
		Type:   "event",
		Method: "navigation",
		Params: json.RawMessage(`{"url": "https://example.com/page2"}`),
	}
	if err := websocket.JSON.Send(conn, navEvt); err != nil {
		t.Fatalf("send navigation: %v", err)
	}

	// Drain events channel and give subscribers time.
	for i := 0; i < 3; i++ {
		select {
		case <-s.Events():
		case <-time.After(2 * time.Second):
			t.Fatal("timeout draining events")
		}
	}
	time.Sleep(100 * time.Millisecond)

	steps := rec.Stop()
	if len(steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(steps))
	}

	// Verify click step.
	if steps[0].Action != "click" {
		t.Fatalf("step 0: expected action click, got %s", steps[0].Action)
	}
	if steps[0].Selector != "#submit-btn" {
		t.Fatalf("step 0: expected selector #submit-btn, got %s", steps[0].Selector)
	}

	// Verify type step.
	if steps[1].Action != "type" {
		t.Fatalf("step 1: expected action type, got %s", steps[1].Action)
	}
	if steps[1].Selector != "#email" {
		t.Fatalf("step 1: expected selector #email, got %s", steps[1].Selector)
	}
	if steps[1].Text != "test@example.com" {
		t.Fatalf("step 1: expected text test@example.com, got %s", steps[1].Text)
	}

	// Verify navigate step.
	if steps[2].Action != "navigate" {
		t.Fatalf("step 2: expected action navigate, got %s", steps[2].Action)
	}
	if steps[2].URL != "https://example.com/page2" {
		t.Fatalf("step 2: expected url https://example.com/page2, got %s", steps[2].URL)
	}
}

func TestBridgeRecorder_ToRecipe(t *testing.T) {
	s := NewBridgeServer("127.0.0.1:0")
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = s.Stop() }()

	rec := NewBridgeRecorder(s)
	rec.Start()

	conn := connectTestClient(t, s.Addr(), "page-recipe")
	defer func() { _ = conn.Close() }()

	evt := BridgeMessage{
		Type:   "event",
		Method: "user.click",
		Params: json.RawMessage(`{"selector": "button.login"}`),
	}
	if err := websocket.JSON.Send(conn, evt); err != nil {
		t.Fatalf("send: %v", err)
	}

	select {
	case <-s.Events():
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
	time.Sleep(100 * time.Millisecond)

	r := rec.ToRecipe("Login Flow", "https://example.com")
	if r == nil {
		t.Fatal("expected non-nil recipe")
	}
	if r.Type != "automate" {
		t.Fatalf("expected type automate, got %s", r.Type)
	}
	if r.Name != "Login Flow" {
		t.Fatalf("expected name 'Login Flow', got %s", r.Name)
	}
	if r.URL != "https://example.com" {
		t.Fatalf("expected url https://example.com, got %s", r.URL)
	}
	if len(r.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(r.Steps))
	}
	if r.Steps[0].Action != "click" {
		t.Fatalf("expected click step, got %s", r.Steps[0].Action)
	}
}

func TestBridgeRecorder_NilServer(t *testing.T) {
	rec := NewBridgeRecorder(nil)
	if rec != nil {
		t.Fatal("expected nil recorder for nil server")
	}

	// All methods on nil receiver should be safe.
	var nilRec *BridgeRecorder
	nilRec.Start()
	steps := nilRec.Stop()
	if steps != nil {
		t.Fatal("expected nil from nil Stop")
	}
	steps = nilRec.Steps()
	if steps != nil {
		t.Fatal("expected nil from nil Steps")
	}
	r := nilRec.ToRecipe("test", "http://example.com")
	if r != nil {
		t.Fatal("expected nil from nil ToRecipe")
	}
}

func TestBridgeRecorder_ConcurrentSteps(t *testing.T) {
	s := NewBridgeServer("127.0.0.1:0")
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = s.Stop() }()

	rec := NewBridgeRecorder(s)
	rec.Start()

	conn := connectTestClient(t, s.Addr(), "page-conc")
	defer func() { _ = conn.Close() }()

	const numEvents = 20
	var wg sync.WaitGroup
	wg.Add(numEvents)

	for i := 0; i < numEvents; i++ {
		go func() {
			defer wg.Done()
			evt := BridgeMessage{
				Type:   "event",
				Method: "user.click",
				Params: json.RawMessage(`{"selector": ".item"}`),
			}
			_ = websocket.JSON.Send(conn, evt)
		}()
	}

	wg.Wait()

	// Drain events.
	deadline := time.After(5 * time.Second)
	drained := 0
	for drained < numEvents {
		select {
		case <-s.Events():
			drained++
		case <-deadline:
			goto done
		}
	}
done:
	time.Sleep(200 * time.Millisecond)

	steps := rec.Steps()
	// We should have received some steps (may not be all due to non-blocking channel).
	if len(steps) == 0 {
		t.Fatal("expected at least some steps from concurrent delivery")
	}

	// Stop should also work.
	finalSteps := rec.Stop()
	if len(finalSteps) < len(steps) {
		t.Fatalf("Stop returned fewer steps (%d) than Steps (%d)", len(finalSteps), len(steps))
	}
}
