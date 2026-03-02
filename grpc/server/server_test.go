package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	pb "github.com/inovacc/scout/grpc/scoutpb"
)

func TestCreateDestroySession(t *testing.T) {
	env := setupTestServer(t)
	ctx := context.Background()

	resp, err := env.client.CreateSession(ctx, &pb.CreateSessionRequest{
		Headless:  true,
		NoSandbox: true,
	})
	if err != nil {
		t.Skipf("browser unavailable: %v", err)
	}

	if resp.GetSessionId() == "" {
		t.Fatal("empty session ID")
	}

	// Destroy should succeed
	_, err = env.client.DestroySession(ctx, &pb.SessionRequest{SessionId: resp.GetSessionId()})
	if err != nil {
		t.Fatalf("destroy: %v", err)
	}

	// Double destroy should fail (not found)
	_, err = env.client.DestroySession(ctx, &pb.SessionRequest{SessionId: resp.GetSessionId()})
	if err == nil {
		t.Fatal("expected error on double destroy")
	}
}

func TestNavigate(t *testing.T) {
	env := setupTestServer(t)
	sid := env.createSession(t)

	resp := env.navigate(t, sid, "/")
	if resp.GetTitle() != "Test Page" {
		t.Errorf("title = %q, want %q", resp.GetTitle(), "Test Page")
	}

	if !strings.HasSuffix(resp.GetUrl(), "/") {
		t.Errorf("url = %q, want suffix /", resp.GetUrl())
	}
}

func TestGoBackForward(t *testing.T) {
	env := setupTestServer(t)
	sid := env.createSession(t)
	ctx := context.Background()

	env.navigate(t, sid, "/")
	env.navigate(t, sid, "/page2")

	// Go back
	_, err := env.client.GoBack(ctx, &pb.SessionRequest{SessionId: sid})
	if err != nil {
		t.Fatalf("go back: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	titleResp, _ := env.client.GetTitle(ctx, &pb.SessionRequest{SessionId: sid})
	if titleResp.GetText() != "Test Page" {
		t.Errorf("after back: title = %q, want %q", titleResp.GetText(), "Test Page")
	}

	// Go forward
	_, err = env.client.GoForward(ctx, &pb.SessionRequest{SessionId: sid})
	if err != nil {
		t.Fatalf("go forward: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	titleResp, _ = env.client.GetTitle(ctx, &pb.SessionRequest{SessionId: sid})
	if titleResp.GetText() != "Page Two" {
		t.Errorf("after forward: title = %q, want %q", titleResp.GetText(), "Page Two")
	}
}

func TestReload(t *testing.T) {
	env := setupTestServer(t)
	sid := env.createSession(t)
	ctx := context.Background()

	env.navigate(t, sid, "/")

	_, err := env.client.Reload(ctx, &pb.SessionRequest{SessionId: sid})
	if err != nil {
		t.Fatalf("reload: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	titleResp, _ := env.client.GetTitle(ctx, &pb.SessionRequest{SessionId: sid})
	if titleResp.GetText() != "Test Page" {
		t.Errorf("after reload: title = %q, want %q", titleResp.GetText(), "Test Page")
	}
}

func TestClick(t *testing.T) {
	env := setupTestServer(t)
	sid := env.createSession(t)
	ctx := context.Background()

	env.navigate(t, sid, "/click-target")

	_, err := env.client.Click(ctx, &pb.ElementRequest{SessionId: sid, Selector: "#btn"})
	if err != nil {
		t.Fatalf("click: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	textResp, _ := env.client.GetText(ctx, &pb.ElementRequest{SessionId: sid, Selector: "#output"})
	if textResp.GetText() != "clicked" {
		t.Errorf("after click: text = %q, want %q", textResp.GetText(), "clicked")
	}
}

func TestDoubleClick(t *testing.T) {
	env := setupTestServer(t)
	sid := env.createSession(t)
	ctx := context.Background()

	env.navigate(t, sid, "/click-target")

	_, err := env.client.DoubleClick(ctx, &pb.ElementRequest{SessionId: sid, Selector: "#dbl"})
	if err != nil {
		t.Fatalf("double click: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	textResp, _ := env.client.GetText(ctx, &pb.ElementRequest{SessionId: sid, Selector: "#output"})
	if textResp.GetText() != "double-clicked" {
		t.Errorf("after dblclick: text = %q, want %q", textResp.GetText(), "double-clicked")
	}
}

func TestRightClick(t *testing.T) {
	env := setupTestServer(t)
	sid := env.createSession(t)
	ctx := context.Background()

	env.navigate(t, sid, "/click-target")

	// RightClick should not error (no DOM side-effect to verify easily)
	_, err := env.client.RightClick(ctx, &pb.ElementRequest{SessionId: sid, Selector: "#btn"})
	if err != nil {
		t.Fatalf("right click: %v", err)
	}
}

func TestHover(t *testing.T) {
	env := setupTestServer(t)
	sid := env.createSession(t)
	ctx := context.Background()

	env.navigate(t, sid, "/click-target")

	_, err := env.client.Hover(ctx, &pb.ElementRequest{SessionId: sid, Selector: "#hover-zone"})
	if err != nil {
		t.Fatalf("hover: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	textResp, _ := env.client.GetText(ctx, &pb.ElementRequest{SessionId: sid, Selector: "#output"})
	if textResp.GetText() != "hovered" {
		t.Errorf("after hover: text = %q, want %q", textResp.GetText(), "hovered")
	}
}

func TestType(t *testing.T) {
	env := setupTestServer(t)
	sid := env.createSession(t)
	ctx := context.Background()

	env.navigate(t, sid, "/")

	// Type into the input (has default value "default")
	_, err := env.client.Type(ctx, &pb.TypeRequest{
		SessionId:  sid,
		Selector:   "#name",
		Text:       "hello",
		ClearFirst: true,
	})
	if err != nil {
		t.Fatalf("type: %v", err)
	}

	// Read back via eval (rod's Input sets .value via JS)
	evalResp, _ := env.client.Eval(ctx, &pb.EvalRequest{
		SessionId: sid,
		Script:    "() => document.getElementById('name').value",
	})
	if !strings.Contains(evalResp.GetResult(), "hello") {
		t.Errorf("after type: value = %q, want contains 'hello'", evalResp.GetResult())
	}

	// Type without clear — appends
	_, err = env.client.Type(ctx, &pb.TypeRequest{
		SessionId: sid,
		Selector:  "#name",
		Text:      " world",
	})
	if err != nil {
		t.Fatalf("type append: %v", err)
	}

	evalResp, _ = env.client.Eval(ctx, &pb.EvalRequest{
		SessionId: sid,
		Script:    "() => document.getElementById('name').value",
	})
	if !strings.Contains(evalResp.GetResult(), "world") {
		t.Errorf("after append type: value = %q, want contains 'world'", evalResp.GetResult())
	}
}

func TestSelectOption(t *testing.T) {
	env := setupTestServer(t)
	sid := env.createSession(t)
	ctx := context.Background()

	env.navigate(t, sid, "/")

	// SelectOption uses text matching (not value attribute)
	_, err := env.client.SelectOption(ctx, &pb.SelectRequest{
		SessionId: sid,
		Selector:  "#sel",
		Value:     "Beta",
	})
	if err != nil {
		t.Fatalf("select option: %v", err)
	}

	// Verify via eval
	evalResp, _ := env.client.Eval(ctx, &pb.EvalRequest{
		SessionId: sid,
		Script:    "() => document.getElementById('sel').value",
	})
	if !strings.Contains(evalResp.GetResult(), "b") {
		t.Errorf("after select: result = %q, want contains 'b'", evalResp.GetResult())
	}
}

func TestPressKey(t *testing.T) {
	env := setupTestServer(t)
	sid := env.createSession(t)
	ctx := context.Background()

	env.navigate(t, sid, "/")

	// PressKey should not error
	_, err := env.client.PressKey(ctx, &pb.KeyRequest{SessionId: sid, Key: "Tab"})
	if err != nil {
		t.Fatalf("press key: %v", err)
	}
}

func TestGetText(t *testing.T) {
	env := setupTestServer(t)
	sid := env.createSession(t)
	ctx := context.Background()

	env.navigate(t, sid, "/")

	// CSS selector
	resp, err := env.client.GetText(ctx, &pb.ElementRequest{SessionId: sid, Selector: "#info"})
	if err != nil {
		t.Fatalf("get text: %v", err)
	}

	if resp.GetText() != "Some text" {
		t.Errorf("text = %q, want %q", resp.GetText(), "Some text")
	}

	// XPath
	resp, err = env.client.GetText(ctx, &pb.ElementRequest{SessionId: sid, Selector: "//h1", Xpath: true})
	if err != nil {
		t.Fatalf("get text xpath: %v", err)
	}

	if resp.GetText() != "Hello World" {
		t.Errorf("text = %q, want %q", resp.GetText(), "Hello World")
	}
}

func TestGetAttribute(t *testing.T) {
	env := setupTestServer(t)
	sid := env.createSession(t)
	ctx := context.Background()

	env.navigate(t, sid, "/")

	resp, err := env.client.GetAttribute(ctx, &pb.AttributeRequest{
		SessionId: sid,
		Selector:  "#name",
		Attribute: "type",
	})
	if err != nil {
		t.Fatalf("get attribute: %v", err)
	}

	if resp.GetText() != "text" {
		t.Errorf("attr = %q, want %q", resp.GetText(), "text")
	}
}

func TestGetTitleURL(t *testing.T) {
	env := setupTestServer(t)
	sid := env.createSession(t)
	ctx := context.Background()

	env.navigate(t, sid, "/")

	titleResp, err := env.client.GetTitle(ctx, &pb.SessionRequest{SessionId: sid})
	if err != nil {
		t.Fatalf("get title: %v", err)
	}

	if titleResp.GetText() != "Test Page" {
		t.Errorf("title = %q, want %q", titleResp.GetText(), "Test Page")
	}

	urlResp, err := env.client.GetURL(ctx, &pb.SessionRequest{SessionId: sid})
	if err != nil {
		t.Fatalf("get url: %v", err)
	}

	if !strings.Contains(urlResp.GetText(), env.baseURL) {
		t.Errorf("url = %q, want contains %q", urlResp.GetText(), env.baseURL)
	}
}

func TestEval(t *testing.T) {
	env := setupTestServer(t)
	sid := env.createSession(t)
	ctx := context.Background()

	env.navigate(t, sid, "/")

	// Rod's Eval expects function expressions: () => expr
	resp, err := env.client.Eval(ctx, &pb.EvalRequest{SessionId: sid, Script: "() => 1 + 1"})
	if err != nil {
		t.Fatalf("eval: %v", err)
	}

	// Result is JSON-encoded EvalResult struct with Type/Value/Subtype fields
	var val struct {
		Type  string `json:"Type"`
		Value any    `json:"Value"`
	}
	if err := json.Unmarshal([]byte(resp.GetResult()), &val); err != nil {
		t.Fatalf("unmarshal eval result: %v", err)
	}

	if num, ok := val.Value.(float64); !ok || num != 2 {
		t.Errorf("eval 1+1 = %v, want 2", val.Value)
	}

	// String eval
	resp, err = env.client.Eval(ctx, &pb.EvalRequest{SessionId: sid, Script: "() => document.title"})
	if err != nil {
		t.Fatalf("eval title: %v", err)
	}

	if !strings.Contains(resp.GetResult(), "Test Page") {
		t.Errorf("eval title = %q, want contains 'Test Page'", resp.GetResult())
	}
}

func TestElementExists(t *testing.T) {
	env := setupTestServer(t)
	sid := env.createSession(t)
	ctx := context.Background()

	env.navigate(t, sid, "/")

	// CSS — exists
	resp, err := env.client.ElementExists(ctx, &pb.ElementRequest{SessionId: sid, Selector: "#info"})
	if err != nil {
		t.Fatalf("element exists: %v", err)
	}

	if !resp.GetValue() {
		t.Error("expected #info to exist")
	}

	// CSS — not exists
	resp, err = env.client.ElementExists(ctx, &pb.ElementRequest{SessionId: sid, Selector: "#nonexistent"})
	if err != nil {
		t.Fatalf("element exists: %v", err)
	}

	if resp.GetValue() {
		t.Error("expected #nonexistent to not exist")
	}

	// XPath — exists
	resp, err = env.client.ElementExists(ctx, &pb.ElementRequest{SessionId: sid, Selector: "//h1", Xpath: true})
	if err != nil {
		t.Fatalf("element exists xpath: %v", err)
	}

	if !resp.GetValue() {
		t.Error("expected //h1 to exist")
	}

	// XPath — not exists
	resp, err = env.client.ElementExists(ctx, &pb.ElementRequest{SessionId: sid, Selector: "//nonexistent", Xpath: true})
	if err != nil {
		t.Fatalf("element exists xpath: %v", err)
	}

	if resp.GetValue() {
		t.Error("expected //nonexistent to not exist")
	}
}

func TestScreenshot(t *testing.T) {
	env := setupTestServer(t)
	sid := env.createSession(t)
	ctx := context.Background()

	env.navigate(t, sid, "/")

	// Viewport screenshot
	resp, err := env.client.Screenshot(ctx, &pb.ScreenshotRequest{SessionId: sid})
	if err != nil {
		t.Fatalf("screenshot: %v", err)
	}

	if len(resp.GetData()) == 0 {
		t.Error("empty screenshot data")
	}

	if resp.GetFormat() != "png" {
		t.Errorf("format = %q, want png", resp.GetFormat())
	}

	// Full page screenshot
	resp, err = env.client.Screenshot(ctx, &pb.ScreenshotRequest{SessionId: sid, FullPage: true})
	if err != nil {
		t.Fatalf("full screenshot: %v", err)
	}

	if len(resp.GetData()) == 0 {
		t.Error("empty full-page screenshot data")
	}
}

func TestPDF(t *testing.T) {
	env := setupTestServer(t)
	sid := env.createSession(t)
	ctx := context.Background()

	env.navigate(t, sid, "/")

	resp, err := env.client.PDF(ctx, &pb.SessionRequest{SessionId: sid})
	if err != nil {
		t.Fatalf("pdf: %v", err)
	}

	if len(resp.GetData()) == 0 {
		t.Error("empty PDF data")
	}
	// PDF magic bytes
	if len(resp.GetData()) > 4 && string(resp.GetData()[:5]) != "%PDF-" {
		t.Error("data does not start with PDF header")
	}
}

func TestHARRecording(t *testing.T) {
	env := setupTestServer(t)
	sid := env.createSession(t)
	ctx := context.Background()

	// Start recording
	_, err := env.client.StartRecording(ctx, &pb.RecordingRequest{SessionId: sid, CaptureBody: true})
	if err != nil {
		t.Fatalf("start recording: %v", err)
	}

	// Navigate to trigger network activity
	env.navigate(t, sid, "/recorder-page")
	time.Sleep(1 * time.Second) // let fetch complete

	// Export HAR (while still recording)
	harResp, err := env.client.ExportHAR(ctx, &pb.SessionRequest{SessionId: sid})
	if err != nil {
		t.Fatalf("export har: %v", err)
	}

	if len(harResp.GetData()) == 0 {
		t.Error("empty HAR data")
	}

	if harResp.GetEntryCount() == 0 {
		t.Error("zero HAR entries")
	}

	// Stop recording
	_, err = env.client.StopRecording(ctx, &pb.SessionRequest{SessionId: sid})
	if err != nil {
		t.Fatalf("stop recording: %v", err)
	}

	// Double start should work (recorder was stopped and nilled)
	_, err = env.client.StartRecording(ctx, &pb.RecordingRequest{SessionId: sid})
	if err != nil {
		t.Fatalf("re-start recording: %v", err)
	}
}

func TestStreamEvents(t *testing.T) {
	env := setupTestServer(t)
	sid := env.createSession(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, err := env.client.StreamEvents(ctx, &pb.SessionRequest{SessionId: sid})
	if err != nil {
		t.Fatalf("stream events: %v", err)
	}

	// Navigate to trigger events (use separate context to avoid stream cancellation)
	go func() {
		time.Sleep(200 * time.Millisecond)

		navCtx, navCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer navCancel()

		_, _ = env.client.Navigate(navCtx, &pb.NavigateRequest{
			SessionId:  sid,
			Url:        env.baseURL + "/",
			WaitStable: true,
		})
	}()

	// Collect at least one event
	ev, err := stream.Recv()
	if err != nil {
		t.Fatalf("recv event: %v", err)
	}

	if ev.GetSessionId() != sid {
		t.Errorf("event session = %q, want %q", ev.GetSessionId(), sid)
	}

	if ev.GetTimestamp() == 0 {
		t.Error("event timestamp is zero")
	}
}

func TestErrorPaths(t *testing.T) {
	env := setupTestServer(t)
	ctx := context.Background()

	// Operations on invalid session
	_, err := env.client.Navigate(ctx, &pb.NavigateRequest{SessionId: "nonexistent", Url: "http://x"})
	if err == nil {
		t.Error("expected error for invalid session navigate")
	}

	_, err = env.client.Click(ctx, &pb.ElementRequest{SessionId: "nonexistent", Selector: "#x"})
	if err == nil {
		t.Error("expected error for invalid session click")
	}

	_, err = env.client.Screenshot(ctx, &pb.ScreenshotRequest{SessionId: "nonexistent"})
	if err == nil {
		t.Error("expected error for invalid session screenshot")
	}

	_, err = env.client.GetTitle(ctx, &pb.SessionRequest{SessionId: "nonexistent"})
	if err == nil {
		t.Error("expected error for invalid session title")
	}

	// Element not found — use a short timeout so rod doesn't retry forever
	sid := env.createSession(t)
	env.navigate(t, sid, "/")

	shortCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err = env.client.Click(shortCtx, &pb.ElementRequest{SessionId: sid, Selector: "#nonexistent"})
	if err == nil {
		t.Error("expected error for nonexistent element click")
	}

	shortCtx2, cancel2 := context.WithTimeout(ctx, 5*time.Second)
	defer cancel2()

	_, err = env.client.GetText(shortCtx2, &pb.ElementRequest{SessionId: sid, Selector: "#nonexistent"})
	if err == nil {
		t.Error("expected error for nonexistent element text")
	}

	// ExportHAR without recording
	_, err = env.client.ExportHAR(ctx, &pb.SessionRequest{SessionId: sid})
	if err == nil {
		t.Error("expected error for export without recording")
	}

	// Double start recording
	_, err = env.client.StartRecording(ctx, &pb.RecordingRequest{SessionId: sid})
	if err != nil {
		t.Fatalf("start recording: %v", err)
	}

	_, err = env.client.StartRecording(ctx, &pb.RecordingRequest{SessionId: sid})
	if err == nil {
		t.Error("expected error for double start recording")
	}

	_, _ = env.client.StopRecording(ctx, &pb.SessionRequest{SessionId: sid})
}

func TestStatsAfterOperations(t *testing.T) {
	env := setupTestServer(t)
	sid := env.createSession(t)

	// After creating a session, stats should show totalSessions >= 1
	// (we can't directly access server stats via gRPC, but we verify
	// the session creates/navigates succeed - the stats tracking was
	// verified in sanitize_test.go unit tests)

	env.navigate(t, sid, "/")

	// Screenshot to trigger event — use timeout to avoid flaky hangs
	// on closed connections during test cleanup
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, err := env.client.Screenshot(ctx, &pb.ScreenshotRequest{SessionId: sid})
	if err != nil {
		t.Fatalf("screenshot: %v", err)
	}
}

func TestInteractive(t *testing.T) {
	env := setupTestServer(t)
	sid := env.createSession(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	stream, err := env.client.Interactive(ctx)
	if err != nil {
		t.Fatalf("interactive: %v", err)
	}

	// Send navigate command
	err = stream.Send(&pb.Command{
		SessionId: sid,
		RequestId: "1",
		Action: &pb.Command_Navigate{
			Navigate: &pb.NavigateAction{Url: env.baseURL + "/click-target"},
		},
	})
	if err != nil {
		t.Fatalf("send navigate: %v", err)
	}

	time.Sleep(1 * time.Second)

	// Send click command
	err = stream.Send(&pb.Command{
		SessionId: sid,
		RequestId: "2",
		Action: &pb.Command_Click{
			Click: &pb.ClickAction{Selector: "#btn"},
		},
	})
	if err != nil {
		t.Fatalf("send click: %v", err)
	}

	// Read at least one event (network or page event from navigation)
	ev, err := stream.Recv()
	if err != nil {
		t.Fatalf("recv: %v", err)
	}

	if ev.GetSessionId() != sid {
		t.Errorf("event session = %q, want %q", ev.GetSessionId(), sid)
	}

	// Close send direction
	_ = stream.CloseSend()
}

func TestInteractive_Type(t *testing.T) {
	env := setupTestServer(t)
	sid := env.createSession(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	stream, err := env.client.Interactive(ctx)
	if err != nil {
		t.Fatalf("interactive: %v", err)
	}

	// Navigate first
	err = stream.Send(&pb.Command{
		SessionId: sid,
		RequestId: "1",
		Action: &pb.Command_Navigate{
			Navigate: &pb.NavigateAction{Url: env.baseURL + "/"},
		},
	})
	if err != nil {
		t.Fatalf("send navigate: %v", err)
	}

	time.Sleep(1 * time.Second)

	// Type into input
	err = stream.Send(&pb.Command{
		SessionId: sid,
		RequestId: "2",
		Action: &pb.Command_Type{
			Type: &pb.TypeAction{Selector: "#name", Text: "test input"},
		},
	})
	if err != nil {
		t.Fatalf("send type: %v", err)
	}

	// Collect at least one event
	ev, err := stream.Recv()
	if err != nil {
		t.Fatalf("recv: %v", err)
	}

	if ev.GetSessionId() != sid {
		t.Errorf("event session = %q, want %q", ev.GetSessionId(), sid)
	}

	_ = stream.CloseSend()
}

func TestInteractive_PressKey(t *testing.T) {
	env := setupTestServer(t)
	sid := env.createSession(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	stream, err := env.client.Interactive(ctx)
	if err != nil {
		t.Fatalf("interactive: %v", err)
	}

	// Navigate first
	err = stream.Send(&pb.Command{
		SessionId: sid,
		RequestId: "1",
		Action: &pb.Command_Navigate{
			Navigate: &pb.NavigateAction{Url: env.baseURL + "/"},
		},
	})
	if err != nil {
		t.Fatalf("send navigate: %v", err)
	}

	time.Sleep(1 * time.Second)

	// Press Enter
	err = stream.Send(&pb.Command{
		SessionId: sid,
		RequestId: "2",
		Action: &pb.Command_PressKey{
			PressKey: &pb.KeyAction{Key: "Enter"},
		},
	})
	if err != nil {
		t.Fatalf("send press key: %v", err)
	}

	ev, err := stream.Recv()
	if err != nil {
		t.Fatalf("recv: %v", err)
	}

	if ev.GetSessionId() != sid {
		t.Errorf("event session = %q, want %q", ev.GetSessionId(), sid)
	}

	_ = stream.CloseSend()
}

func TestInteractive_Eval(t *testing.T) {
	env := setupTestServer(t)
	sid := env.createSession(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	stream, err := env.client.Interactive(ctx)
	if err != nil {
		t.Fatalf("interactive: %v", err)
	}

	// Navigate first
	err = stream.Send(&pb.Command{
		SessionId: sid,
		RequestId: "1",
		Action: &pb.Command_Navigate{
			Navigate: &pb.NavigateAction{Url: env.baseURL + "/"},
		},
	})
	if err != nil {
		t.Fatalf("send navigate: %v", err)
	}

	time.Sleep(1 * time.Second)

	// Eval
	err = stream.Send(&pb.Command{
		SessionId: sid,
		RequestId: "2",
		Action: &pb.Command_Eval{
			Eval: &pb.EvalAction{Script: "() => 1 + 1"},
		},
	})
	if err != nil {
		t.Fatalf("send eval: %v", err)
	}

	ev, err := stream.Recv()
	if err != nil {
		t.Fatalf("recv: %v", err)
	}

	if ev.GetSessionId() != sid {
		t.Errorf("event session = %q, want %q", ev.GetSessionId(), sid)
	}

	_ = stream.CloseSend()
}

func TestInteractive_Scroll(t *testing.T) {
	env := setupTestServer(t)
	sid := env.createSession(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	stream, err := env.client.Interactive(ctx)
	if err != nil {
		t.Fatalf("interactive: %v", err)
	}

	err = stream.Send(&pb.Command{
		SessionId: sid,
		RequestId: "1",
		Action: &pb.Command_Navigate{
			Navigate: &pb.NavigateAction{Url: env.baseURL + "/"},
		},
	})
	if err != nil {
		t.Fatalf("send navigate: %v", err)
	}

	time.Sleep(1 * time.Second)

	err = stream.Send(&pb.Command{
		SessionId: sid,
		RequestId: "2",
		Action: &pb.Command_Scroll{
			Scroll: &pb.ScrollAction{X: 0, Y: 500},
		},
	})
	if err != nil {
		t.Fatalf("send scroll: %v", err)
	}

	ev, err := stream.Recv()
	if err != nil {
		t.Fatalf("recv: %v", err)
	}

	if ev.GetSessionId() != sid {
		t.Errorf("event session = %q, want %q", ev.GetSessionId(), sid)
	}

	_ = stream.CloseSend()
}

func TestInteractive_Wait(t *testing.T) {
	env := setupTestServer(t)
	sid := env.createSession(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	stream, err := env.client.Interactive(ctx)
	if err != nil {
		t.Fatalf("interactive: %v", err)
	}

	err = stream.Send(&pb.Command{
		SessionId: sid,
		RequestId: "1",
		Action: &pb.Command_Navigate{
			Navigate: &pb.NavigateAction{Url: env.baseURL + "/"},
		},
	})
	if err != nil {
		t.Fatalf("send navigate: %v", err)
	}

	time.Sleep(1 * time.Second)

	// Wait for an element that exists
	err = stream.Send(&pb.Command{
		SessionId: sid,
		RequestId: "2",
		Action: &pb.Command_Wait{
			Wait: &pb.WaitAction{Selector: "#info"},
		},
	})
	if err != nil {
		t.Fatalf("send wait: %v", err)
	}

	ev, err := stream.Recv()
	if err != nil {
		t.Fatalf("recv: %v", err)
	}

	if ev.GetSessionId() != sid {
		t.Errorf("event session = %q, want %q", ev.GetSessionId(), sid)
	}

	_ = stream.CloseSend()
}

func TestStreamHijack(t *testing.T) {
	env := setupTestServer(t)
	sid := env.createSession(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Navigate first so the page has a URL
	env.navigate(t, sid, "/recorder-page")

	// Start hijack with body capture
	_, err := env.client.StartHijack(ctx, &pb.HijackRequest{
		SessionId:   sid,
		CaptureBody: true,
	})
	if err != nil {
		t.Fatalf("start hijack: %v", err)
	}

	// Open the stream
	stream, err := env.client.StreamHijack(ctx, &pb.SessionRequest{SessionId: sid})
	if err != nil {
		t.Fatalf("stream hijack: %v", err)
	}

	// Navigate to trigger network events that the hijacker captures
	go func() {
		time.Sleep(200 * time.Millisecond)

		navCtx, navCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer navCancel()

		_, _ = env.client.Navigate(navCtx, &pb.NavigateRequest{
			SessionId:  sid,
			Url:        env.baseURL + "/recorder-page",
			WaitStable: true,
		})
	}()

	// Collect at least one hijack event
	ev, err := stream.Recv()
	if err != nil {
		t.Fatalf("recv hijack event: %v", err)
	}

	if ev == nil {
		t.Fatal("received nil hijack event")
	}

	// Should have either a request or response event
	if ev.GetRequest() == nil && ev.GetResponse() == nil && ev.GetWsFrame() == nil {
		t.Error("hijack event has no request, response, or ws_frame")
	}

	// Stop hijack
	_, err = env.client.StopHijack(ctx, &pb.SessionRequest{SessionId: sid})
	if err != nil {
		t.Fatalf("stop hijack: %v", err)
	}
}

func TestStreamHijack_InvalidSession(t *testing.T) {
	env := setupTestServer(t)
	ctx := context.Background()

	stream, err := env.client.StreamHijack(ctx, &pb.SessionRequest{SessionId: "nonexistent"})
	if err != nil {
		// Some gRPC implementations return error on call
		return
	}

	// Others return error on first Recv
	_, err = stream.Recv()
	if err == nil {
		t.Error("expected error for invalid session stream hijack")
	}
}

func TestHijack_DoubleStart(t *testing.T) {
	env := setupTestServer(t)
	sid := env.createSession(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	env.navigate(t, sid, "/")

	_, err := env.client.StartHijack(ctx, &pb.HijackRequest{SessionId: sid})
	if err != nil {
		t.Fatalf("start hijack: %v", err)
	}

	// Double start should fail
	_, err = env.client.StartHijack(ctx, &pb.HijackRequest{SessionId: sid})
	if err == nil {
		t.Error("expected error for double start hijack")
	}

	// Stop should succeed
	_, err = env.client.StopHijack(ctx, &pb.SessionRequest{SessionId: sid})
	if err != nil {
		t.Fatalf("stop hijack: %v", err)
	}

	// Stop again should be idempotent
	_, err = env.client.StopHijack(ctx, &pb.SessionRequest{SessionId: sid})
	if err != nil {
		t.Fatalf("double stop hijack: %v", err)
	}
}

func TestCreateSession_WithOptions(t *testing.T) {
	env := setupTestServer(t)
	ctx := context.Background()

	resp, err := env.client.CreateSession(ctx, &pb.CreateSessionRequest{
		Headless:  true,
		NoSandbox: true,
		Stealth:   true,
		UserAgent: "ScoutTestAgent/1.0",
		Width:     1024,
		Height:    768,
	})
	if err != nil {
		t.Skipf("browser unavailable: %v", err)
	}

	if resp.GetSessionId() == "" {
		t.Fatal("empty session ID")
	}

	t.Cleanup(func() {
		ctx2, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, _ = env.client.DestroySession(ctx2, &pb.SessionRequest{SessionId: resp.GetSessionId()})
	})
}

func TestCreateSession_WithInitialURL(t *testing.T) {
	env := setupTestServer(t)
	ctx := context.Background()

	resp, err := env.client.CreateSession(ctx, &pb.CreateSessionRequest{
		Headless:   true,
		NoSandbox:  true,
		InitialUrl: env.baseURL + "/page2",
	})
	if err != nil {
		t.Skipf("browser unavailable: %v", err)
	}

	if resp.GetSessionId() == "" {
		t.Fatal("empty session ID")
	}

	if resp.GetTitle() != "Page Two" {
		t.Errorf("title = %q, want %q", resp.GetTitle(), "Page Two")
	}

	t.Cleanup(func() {
		ctx2, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, _ = env.client.DestroySession(ctx2, &pb.SessionRequest{SessionId: resp.GetSessionId()})
	})
}
