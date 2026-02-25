package scout

import (
	"encoding/json"
	"testing"
	"time"

	"golang.org/x/net/websocket"
)

// mockBridgeClient connects to a BridgeServer and auto-responds to requests.
func mockBridgeClient(t *testing.T, addr, pageID string, handler func(BridgeMessage) any) *websocket.Conn {
	t.Helper()
	conn := connectTestClient(t, addr, pageID)

	go func() {
		for {
			var msg BridgeMessage
			if err := websocket.JSON.Receive(conn, &msg); err != nil {
				return
			}
			if msg.Type != "request" {
				continue
			}
			result := handler(msg)
			resultRaw, _ := json.Marshal(result)
			resp := BridgeMessage{
				ID:     msg.ID,
				Type:   "response",
				Result: resultRaw,
			}
			if err := websocket.JSON.Send(conn, resp); err != nil {
				return
			}
		}
	}()

	return conn
}

func TestQueryDOM(t *testing.T) {
	s := NewBridgeServer("127.0.0.1:0")
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = s.Stop() }()

	conn := mockBridgeClient(t, s.Addr(), "page-q", func(msg BridgeMessage) any {
		if msg.Method != "dom.query" {
			return map[string]any{"error": "unexpected method"}
		}
		var p map[string]any
		_ = json.Unmarshal(msg.Params, &p)

		if p["all"] == true {
			return map[string]any{
				"count": 2,
				"elements": []map[string]any{
					{"tag": "h1", "text": "Hello"},
					{"tag": "h1", "text": "World"},
				},
			}
		}
		return map[string]any{"found": true, "tag": "h1", "text": "Hello"}
	})
	defer func() { _ = conn.Close() }()

	// Single query.
	results, err := s.QueryDOM("page-q", "h1", false)
	if err != nil {
		t.Fatalf("QueryDOM single: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0]["tag"] != "h1" {
		t.Fatalf("expected tag h1, got %v", results[0]["tag"])
	}

	// All query.
	results, err = s.QueryDOM("page-q", "h1", true)
	if err != nil {
		t.Fatalf("QueryDOM all: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestClickElement(t *testing.T) {
	s := NewBridgeServer("127.0.0.1:0")
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = s.Stop() }()

	var receivedMethod string
	var receivedParams map[string]any

	conn := mockBridgeClient(t, s.Addr(), "page-c", func(msg BridgeMessage) any {
		receivedMethod = msg.Method
		_ = json.Unmarshal(msg.Params, &receivedParams)
		return map[string]any{"clicked": true}
	})
	defer func() { _ = conn.Close() }()

	if err := s.ClickElement("page-c", "#btn"); err != nil {
		t.Fatalf("ClickElement: %v", err)
	}

	if receivedMethod != "dom.click" {
		t.Fatalf("expected method dom.click, got %s", receivedMethod)
	}
	if receivedParams["selector"] != "#btn" {
		t.Fatalf("expected selector #btn, got %v", receivedParams["selector"])
	}
}

func TestInsertHTML(t *testing.T) {
	s := NewBridgeServer("127.0.0.1:0")
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = s.Stop() }()

	var receivedParams map[string]any

	conn := mockBridgeClient(t, s.Addr(), "page-i", func(msg BridgeMessage) any {
		_ = json.Unmarshal(msg.Params, &receivedParams)
		return map[string]any{"inserted": true}
	})
	defer func() { _ = conn.Close() }()

	if err := s.InsertHTML("page-i", "#container", "afterend", "<p>new</p>"); err != nil {
		t.Fatalf("InsertHTML: %v", err)
	}

	if receivedParams["position"] != "afterend" {
		t.Fatalf("expected position afterend, got %v", receivedParams["position"])
	}
	if receivedParams["html"] != "<p>new</p>" {
		t.Fatalf("expected html <p>new</p>, got %v", receivedParams["html"])
	}
	if receivedParams["selector"] != "#container" {
		t.Fatalf("expected selector #container, got %v", receivedParams["selector"])
	}
}

func TestListTabs(t *testing.T) {
	s := NewBridgeServer("127.0.0.1:0")
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = s.Stop() }()

	conn := mockBridgeClient(t, s.Addr(), "ext-bg", func(msg BridgeMessage) any {
		if msg.Method != "tab.list" {
			return map[string]any{"error": "unexpected method"}
		}
		return map[string]any{
			"tabs": []map[string]any{
				{"id": 1, "url": "https://example.com", "title": "Example", "active": true},
				{"id": 2, "url": "https://test.com", "title": "Test", "active": false},
			},
		}
	})
	defer func() { _ = conn.Close() }()

	tabs, err := s.ListTabs()
	if err != nil {
		t.Fatalf("ListTabs: %v", err)
	}
	if len(tabs) != 2 {
		t.Fatalf("expected 2 tabs, got %d", len(tabs))
	}
}

func TestBridgeCommandsNilServer(t *testing.T) {
	var s *BridgeServer

	if _, err := s.QueryDOM("p", "h1", false); err == nil {
		t.Fatal("expected error from nil QueryDOM")
	}
	if err := s.ClickElement("p", "h1"); err == nil {
		t.Fatal("expected error from nil ClickElement")
	}
	if err := s.TypeText("p", "h1", "text"); err == nil {
		t.Fatal("expected error from nil TypeText")
	}
	if err := s.InsertHTML("p", "h1", "afterend", "<p>"); err == nil {
		t.Fatal("expected error from nil InsertHTML")
	}
	if err := s.RemoveElement("p", "h1"); err == nil {
		t.Fatal("expected error from nil RemoveElement")
	}
	if err := s.ModifyAttribute("p", "h1", "class", "x"); err == nil {
		t.Fatal("expected error from nil ModifyAttribute")
	}
	if _, err := s.GetClipboard("p"); err == nil {
		t.Fatal("expected error from nil GetClipboard")
	}
	if err := s.SetClipboard("p", "text"); err == nil {
		t.Fatal("expected error from nil SetClipboard")
	}
	if _, err := s.ListTabs(); err == nil {
		t.Fatal("expected error from nil ListTabs")
	}
	if err := s.CloseTab(1); err == nil {
		t.Fatal("expected error from nil CloseTab")
	}
	if _, err := s.ConsoleMessages("p"); err == nil {
		t.Fatal("expected error from nil ConsoleMessages")
	}
	if err := s.ObserveDOM("p", "body"); err == nil {
		t.Fatal("expected error from nil ObserveDOM")
	}
	if err := s.StartConsoleCapture("p"); err == nil {
		t.Fatal("expected error from nil StartConsoleCapture")
	}
	if err := s.AutoFillForm("p", "form", map[string]string{"a": "b"}); err == nil {
		t.Fatal("expected error from nil AutoFillForm")
	}
	if _, err := s.DownloadFile("p", "http://x"); err == nil {
		t.Fatal("expected error from nil DownloadFile")
	}
}

func TestTypeText(t *testing.T) {
	s := NewBridgeServer("127.0.0.1:0")
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = s.Stop() }()

	var receivedParams map[string]any

	conn := mockBridgeClient(t, s.Addr(), "page-t", func(msg BridgeMessage) any {
		_ = json.Unmarshal(msg.Params, &receivedParams)
		return map[string]any{"typed": true}
	})
	defer func() { _ = conn.Close() }()

	if err := s.TypeText("page-t", "#input", "hello world"); err != nil {
		t.Fatalf("TypeText: %v", err)
	}

	if receivedParams["selector"] != "#input" {
		t.Fatalf("expected selector #input, got %v", receivedParams["selector"])
	}
	if receivedParams["text"] != "hello world" {
		t.Fatalf("expected text 'hello world', got %v", receivedParams["text"])
	}
}

func TestRemoveElement(t *testing.T) {
	s := NewBridgeServer("127.0.0.1:0")
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = s.Stop() }()

	conn := mockBridgeClient(t, s.Addr(), "page-r", func(msg BridgeMessage) any {
		return map[string]any{"removed": true}
	})
	defer func() { _ = conn.Close() }()

	if err := s.RemoveElement("page-r", ".ad-banner"); err != nil {
		t.Fatalf("RemoveElement: %v", err)
	}
}

func TestConsoleMessages(t *testing.T) {
	s := NewBridgeServer("127.0.0.1:0")
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = s.Stop() }()

	conn := mockBridgeClient(t, s.Addr(), "page-con", func(msg BridgeMessage) any {
		if msg.Method == "console.capture" {
			return map[string]any{"capturing": true}
		}
		if msg.Method == "console.get" {
			return map[string]any{
				"messages": []map[string]any{
					{"level": "log", "text": "hello", "ts": 1234567890},
					{"level": "error", "text": "oops", "ts": 1234567891},
				},
			}
		}
		return map[string]any{}
	})
	defer func() { _ = conn.Close() }()

	// Start capture.
	if err := s.StartConsoleCapture("page-con"); err != nil {
		t.Fatalf("StartConsoleCapture: %v", err)
	}

	// Get messages.
	msgs, err := s.ConsoleMessages("page-con")
	if err != nil {
		t.Fatalf("ConsoleMessages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0]["level"] != "log" {
		t.Fatalf("expected level log, got %v", msgs[0]["level"])
	}
}

func TestCloseTab(t *testing.T) {
	s := NewBridgeServer("127.0.0.1:0")
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = s.Stop() }()

	conn := mockBridgeClient(t, s.Addr(), "ext-close", func(msg BridgeMessage) any {
		return map[string]any{"closed": true}
	})
	defer func() { _ = conn.Close() }()

	if err := s.CloseTab(42); err != nil {
		t.Fatalf("CloseTab: %v", err)
	}
}

func TestObserveDOM(t *testing.T) {
	s := NewBridgeServer("127.0.0.1:0")
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = s.Stop() }()

	conn := mockBridgeClient(t, s.Addr(), "page-o", func(msg BridgeMessage) any {
		return map[string]any{"observing": true}
	})
	defer func() { _ = conn.Close() }()

	if err := s.ObserveDOM("page-o", "#content"); err != nil {
		t.Fatalf("ObserveDOM: %v", err)
	}
}

func TestModifyAttribute(t *testing.T) {
	s := NewBridgeServer("127.0.0.1:0")
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = s.Stop() }()

	var receivedParams map[string]any

	conn := mockBridgeClient(t, s.Addr(), "page-a", func(msg BridgeMessage) any {
		_ = json.Unmarshal(msg.Params, &receivedParams)
		return map[string]any{"modified": true}
	})
	defer func() { _ = conn.Close() }()

	if err := s.ModifyAttribute("page-a", "#el", "data-id", "123"); err != nil {
		t.Fatalf("ModifyAttribute: %v", err)
	}

	if receivedParams["attribute"] != "data-id" {
		t.Fatalf("expected attribute data-id, got %v", receivedParams["attribute"])
	}
	if receivedParams["value"] != "123" {
		t.Fatalf("expected value 123, got %v", receivedParams["value"])
	}
}

func TestAutoFillForm(t *testing.T) {
	s := NewBridgeServer("127.0.0.1:0")
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = s.Stop() }()

	var receivedParams map[string]any

	conn := mockBridgeClient(t, s.Addr(), "page-af", func(msg BridgeMessage) any {
		_ = json.Unmarshal(msg.Params, &receivedParams)
		if msg.Method != "form.autofill" {
			return map[string]any{"error": "unexpected method: " + msg.Method}
		}
		return map[string]any{"filled": 2, "total": 2}
	})
	defer func() { _ = conn.Close() }()

	data := map[string]string{
		"username": "alice",
		"password": "secret123",
	}
	if err := s.AutoFillForm("page-af", "#login-form", data); err != nil {
		t.Fatalf("AutoFillForm: %v", err)
	}

	if receivedParams["selector"] != "#login-form" {
		t.Fatalf("expected selector #login-form, got %v", receivedParams["selector"])
	}

	dataMap, ok := receivedParams["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data map, got %T", receivedParams["data"])
	}
	if dataMap["username"] != "alice" {
		t.Fatalf("expected username alice, got %v", dataMap["username"])
	}
	if dataMap["password"] != "secret123" {
		t.Fatalf("expected password secret123, got %v", dataMap["password"])
	}
}

func TestAutoFillFormError(t *testing.T) {
	s := NewBridgeServer("127.0.0.1:0")
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = s.Stop() }()

	conn := mockBridgeClient(t, s.Addr(), "page-afe", func(msg BridgeMessage) any {
		return map[string]any{"error": "form not found: #missing"}
	})
	defer func() { _ = conn.Close() }()

	err := s.AutoFillForm("page-afe", "#missing", map[string]string{"a": "b"})
	if err == nil {
		t.Fatal("expected error for missing form")
	}
}

func TestDownloadFile(t *testing.T) {
	s := NewBridgeServer("127.0.0.1:0")
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = s.Stop() }()

	// "hello world" base64-encoded.
	expectedData := "aGVsbG8gd29ybGQ="

	conn := mockBridgeClient(t, s.Addr(), "page-dl", func(msg BridgeMessage) any {
		if msg.Method != "fetch.download" {
			return map[string]any{"error": "unexpected method: " + msg.Method}
		}
		var p map[string]any
		_ = json.Unmarshal(msg.Params, &p)
		if p["url"] != "https://example.com/file.bin" {
			return map[string]any{"error": "wrong url"}
		}
		return map[string]any{"data": expectedData, "status": 200, "size": 11}
	})
	defer func() { _ = conn.Close() }()

	data, err := s.DownloadFile("page-dl", "https://example.com/file.bin")
	if err != nil {
		t.Fatalf("DownloadFile: %v", err)
	}
	if string(data) != "hello world" {
		t.Fatalf("expected 'hello world', got %q", string(data))
	}
}

func TestDownloadFileError(t *testing.T) {
	s := NewBridgeServer("127.0.0.1:0")
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = s.Stop() }()

	conn := mockBridgeClient(t, s.Addr(), "page-dle", func(msg BridgeMessage) any {
		return map[string]any{"error": "fetch failed: HTTP 404"}
	})
	defer func() { _ = conn.Close() }()

	_, err := s.DownloadFile("page-dle", "https://example.com/missing")
	if err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestDownloadFileEmptyBody(t *testing.T) {
	s := NewBridgeServer("127.0.0.1:0")
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = s.Stop() }()

	conn := mockBridgeClient(t, s.Addr(), "page-dle2", func(msg BridgeMessage) any {
		return map[string]any{"data": "", "status": 204, "size": 0}
	})
	defer func() { _ = conn.Close() }()

	data, err := s.DownloadFile("page-dle2", "https://example.com/empty")
	if err != nil {
		t.Fatalf("DownloadFile empty: %v", err)
	}
	if len(data) != 0 {
		t.Fatalf("expected empty data, got %d bytes", len(data))
	}
}

func TestRemoteError(t *testing.T) {
	s := NewBridgeServer("127.0.0.1:0")
	if err := s.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = s.Stop() }()

	conn := mockBridgeClient(t, s.Addr(), "page-err", func(msg BridgeMessage) any {
		return map[string]any{"error": "element not found: #missing"}
	})
	defer func() { _ = conn.Close() }()

	err := s.ClickElement("page-err", "#missing")
	if err == nil {
		t.Fatal("expected error for missing element")
	}

	// Suppress unused variable warning - give mock time to process.
	_ = conn
	time.Sleep(10 * time.Millisecond)
}
