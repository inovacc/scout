package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
	"github.com/google/uuid"
	"github.com/inovacc/scout/pkg/scout"
	pb "github.com/inovacc/scout/grpc/scoutpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// session holds a browser instance, page, recorder, and event subscribers.
type session struct {
	id       string
	browser  *scout.Browser
	page     *scout.Page
	recorder *scout.NetworkRecorder
	subs     map[string]chan *pb.BrowserEvent
	mu       sync.RWMutex
}

func (s *session) broadcast(ev *pb.BrowserEvent) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ev.SessionId = s.id
	ev.Timestamp = time.Now().UnixMilli()
	for _, ch := range s.subs {
		select {
		case ch <- ev:
		default: // drop if subscriber is slow
		}
	}
}

func (s *session) subscribe(id string) chan *pb.BrowserEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch := make(chan *pb.BrowserEvent, 256)
	s.subs[id] = ch
	return ch
}

func (s *session) unsubscribe(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ch, ok := s.subs[id]; ok {
		close(ch)
		delete(s.subs, id)
	}
}

// findElement finds an element by CSS selector or XPath depending on the request.
func (s *session) findElement(selector string, xpath bool) (*scout.Element, error) {
	if xpath {
		return s.page.ElementByXPath(selector)
	}
	return s.page.Element(selector)
}

// ScoutServer implements the gRPC ScoutService.
type ScoutServer struct {
	pb.UnimplementedScoutServiceServer
	sessions sync.Map // map[string]*session
}

// New creates a new ScoutServer.
func New() *ScoutServer {
	return &ScoutServer{}
}

func (s *ScoutServer) getSession(id string) (*session, error) {
	v, ok := s.sessions.Load(id)
	if !ok {
		return nil, status.Errorf(codes.NotFound, "session %q not found", id)
	}
	return v.(*session), nil
}

// ════════════════════════ Session Lifecycle ════════════════════════

func (s *ScoutServer) CreateSession(_ context.Context, req *pb.CreateSessionRequest) (*pb.CreateSessionResponse, error) {
	opts := []scout.Option{
		scout.WithHeadless(req.Headless),
	}
	if req.Stealth {
		opts = append(opts, scout.WithStealth())
	}
	if req.Proxy != "" {
		opts = append(opts, scout.WithProxy(req.Proxy))
	}
	if req.UserAgent != "" {
		opts = append(opts, scout.WithUserAgent(req.UserAgent))
	}
	if req.Width > 0 && req.Height > 0 {
		opts = append(opts, scout.WithWindowSize(int(req.Width), int(req.Height)))
	}

	browser, err := scout.New(opts...)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "browser launch failed: %v", err)
	}

	url := "about:blank"
	if req.InitialUrl != "" {
		url = req.InitialUrl
	}

	page, err := browser.NewPage(url)
	if err != nil {
		_ = browser.Close()
		return nil, status.Errorf(codes.Internal, "page creation failed: %v", err)
	}

	sess := &session{
		id:      uuid.NewString(),
		browser: browser,
		page:    page,
		subs:    make(map[string]chan *pb.BrowserEvent),
	}

	// Wire CDP events to broadcast
	s.wireEvents(sess)

	// Start recording if requested
	if req.Record {
		recOpts := []scout.RecorderOption{}
		if req.CaptureBody {
			recOpts = append(recOpts, scout.WithCaptureBody(true))
		}
		sess.recorder = scout.NewNetworkRecorder(page, recOpts...)
	}

	s.sessions.Store(sess.id, sess)

	title, _ := page.Title()
	currentURL, _ := page.URL()

	return &pb.CreateSessionResponse{
		SessionId: sess.id,
		Url:       currentURL,
		Title:     title,
	}, nil
}

func (s *ScoutServer) DestroySession(_ context.Context, req *pb.SessionRequest) (*pb.Empty, error) {
	sess, err := s.getSession(req.SessionId)
	if err != nil {
		return nil, err
	}
	if sess.recorder != nil {
		sess.recorder.Stop()
	}
	_ = sess.browser.Close()
	s.sessions.Delete(req.SessionId)
	return &pb.Empty{}, nil
}

// ════════════════════════ Navigation ════════════════════════

func (s *ScoutServer) Navigate(_ context.Context, req *pb.NavigateRequest) (*pb.NavigateResponse, error) {
	sess, err := s.getSession(req.SessionId)
	if err != nil {
		return nil, err
	}

	if err := sess.page.Navigate(req.Url); err != nil {
		return nil, status.Errorf(codes.Internal, "navigate failed: %v", err)
	}

	if req.WaitStable {
		_ = sess.page.WaitStable(500 * time.Millisecond)
	}

	title, _ := sess.page.Title()
	url, _ := sess.page.URL()

	return &pb.NavigateResponse{
		Url:   url,
		Title: title,
	}, nil
}

func (s *ScoutServer) Reload(_ context.Context, req *pb.SessionRequest) (*pb.Empty, error) {
	sess, err := s.getSession(req.SessionId)
	if err != nil {
		return nil, err
	}
	if err := sess.page.Reload(); err != nil {
		return nil, status.Errorf(codes.Internal, "reload failed: %v", err)
	}
	return &pb.Empty{}, nil
}

func (s *ScoutServer) GoBack(_ context.Context, req *pb.SessionRequest) (*pb.Empty, error) {
	sess, err := s.getSession(req.SessionId)
	if err != nil {
		return nil, err
	}
	if err := sess.page.NavigateBack(); err != nil {
		return nil, status.Errorf(codes.Internal, "go back failed: %v", err)
	}
	return &pb.Empty{}, nil
}

func (s *ScoutServer) GoForward(_ context.Context, req *pb.SessionRequest) (*pb.Empty, error) {
	sess, err := s.getSession(req.SessionId)
	if err != nil {
		return nil, err
	}
	if err := sess.page.NavigateForward(); err != nil {
		return nil, status.Errorf(codes.Internal, "go forward failed: %v", err)
	}
	return &pb.Empty{}, nil
}

// ════════════════════════ Element Interaction ════════════════════════

func (s *ScoutServer) Click(_ context.Context, req *pb.ElementRequest) (*pb.Empty, error) {
	sess, err := s.getSession(req.SessionId)
	if err != nil {
		return nil, err
	}
	el, err := sess.findElement(req.Selector, req.Xpath)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "element %q not found: %v", req.Selector, err)
	}
	if err := el.Click(); err != nil {
		return nil, status.Errorf(codes.Internal, "click failed: %v", err)
	}
	return &pb.Empty{}, nil
}

func (s *ScoutServer) DoubleClick(_ context.Context, req *pb.ElementRequest) (*pb.Empty, error) {
	sess, err := s.getSession(req.SessionId)
	if err != nil {
		return nil, err
	}
	el, err := sess.findElement(req.Selector, req.Xpath)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "element not found: %v", err)
	}
	if err := el.DoubleClick(); err != nil {
		return nil, status.Errorf(codes.Internal, "double-click failed: %v", err)
	}
	return &pb.Empty{}, nil
}

func (s *ScoutServer) RightClick(_ context.Context, req *pb.ElementRequest) (*pb.Empty, error) {
	sess, err := s.getSession(req.SessionId)
	if err != nil {
		return nil, err
	}
	el, err := sess.findElement(req.Selector, req.Xpath)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "element not found: %v", err)
	}
	if err := el.RightClick(); err != nil {
		return nil, status.Errorf(codes.Internal, "right-click failed: %v", err)
	}
	return &pb.Empty{}, nil
}

func (s *ScoutServer) Hover(_ context.Context, req *pb.ElementRequest) (*pb.Empty, error) {
	sess, err := s.getSession(req.SessionId)
	if err != nil {
		return nil, err
	}
	el, err := sess.findElement(req.Selector, req.Xpath)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "element not found: %v", err)
	}
	if err := el.Hover(); err != nil {
		return nil, status.Errorf(codes.Internal, "hover failed: %v", err)
	}
	return &pb.Empty{}, nil
}

func (s *ScoutServer) Type(_ context.Context, req *pb.TypeRequest) (*pb.Empty, error) {
	sess, err := s.getSession(req.SessionId)
	if err != nil {
		return nil, err
	}
	el, err := sess.page.Element(req.Selector)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "element not found: %v", err)
	}
	if req.ClearFirst {
		_ = el.Clear()
	}
	if err := el.Input(req.Text); err != nil {
		return nil, status.Errorf(codes.Internal, "type failed: %v", err)
	}
	return &pb.Empty{}, nil
}

func (s *ScoutServer) SelectOption(_ context.Context, req *pb.SelectRequest) (*pb.Empty, error) {
	sess, err := s.getSession(req.SessionId)
	if err != nil {
		return nil, err
	}
	el, err := sess.page.Element(req.Selector)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "element not found: %v", err)
	}
	if err := el.SelectOption(req.Value); err != nil {
		return nil, status.Errorf(codes.Internal, "select option failed: %v", err)
	}
	return &pb.Empty{}, nil
}

func (s *ScoutServer) PressKey(_ context.Context, req *pb.KeyRequest) (*pb.Empty, error) {
	sess, err := s.getSession(req.SessionId)
	if err != nil {
		return nil, err
	}
	key := mapKey(req.Key)
	if err := sess.page.KeyPress(key); err != nil {
		return nil, status.Errorf(codes.Internal, "press key failed: %v", err)
	}
	return &pb.Empty{}, nil
}

// ════════════════════════ Query ════════════════════════

func (s *ScoutServer) GetText(_ context.Context, req *pb.ElementRequest) (*pb.TextResponse, error) {
	sess, err := s.getSession(req.SessionId)
	if err != nil {
		return nil, err
	}
	el, err := sess.findElement(req.Selector, req.Xpath)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "element not found: %v", err)
	}
	text, err := el.Text()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get text failed: %v", err)
	}
	return &pb.TextResponse{Text: text}, nil
}

func (s *ScoutServer) GetAttribute(_ context.Context, req *pb.AttributeRequest) (*pb.TextResponse, error) {
	sess, err := s.getSession(req.SessionId)
	if err != nil {
		return nil, err
	}
	el, err := sess.page.Element(req.Selector)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "element not found: %v", err)
	}
	val, _, err := el.Attribute(req.Attribute)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get attribute failed: %v", err)
	}
	return &pb.TextResponse{Text: val}, nil
}

func (s *ScoutServer) GetTitle(_ context.Context, req *pb.SessionRequest) (*pb.TextResponse, error) {
	sess, err := s.getSession(req.SessionId)
	if err != nil {
		return nil, err
	}
	title, err := sess.page.Title()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get title failed: %v", err)
	}
	return &pb.TextResponse{Text: title}, nil
}

func (s *ScoutServer) GetURL(_ context.Context, req *pb.SessionRequest) (*pb.TextResponse, error) {
	sess, err := s.getSession(req.SessionId)
	if err != nil {
		return nil, err
	}
	url, err := sess.page.URL()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get url failed: %v", err)
	}
	return &pb.TextResponse{Text: url}, nil
}

func (s *ScoutServer) Eval(_ context.Context, req *pb.EvalRequest) (*pb.EvalResponse, error) {
	sess, err := s.getSession(req.SessionId)
	if err != nil {
		return nil, err
	}
	result, err := sess.page.Eval(req.Script)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "eval failed: %v", err)
	}
	data, _ := json.Marshal(result)
	return &pb.EvalResponse{Result: string(data)}, nil
}

func (s *ScoutServer) ElementExists(_ context.Context, req *pb.ElementRequest) (*pb.BoolResponse, error) {
	sess, err := s.getSession(req.SessionId)
	if err != nil {
		return nil, err
	}
	var exists bool
	if req.Xpath {
		exists, _ = sess.page.HasXPath(req.Selector)
	} else {
		exists, _ = sess.page.Has(req.Selector)
	}
	return &pb.BoolResponse{Value: exists}, nil
}

// ════════════════════════ Capture ════════════════════════

func (s *ScoutServer) Screenshot(_ context.Context, req *pb.ScreenshotRequest) (*pb.ScreenshotResponse, error) {
	sess, err := s.getSession(req.SessionId)
	if err != nil {
		return nil, err
	}
	var data []byte
	if req.FullPage {
		data, err = sess.page.FullScreenshot()
	} else {
		data, err = sess.page.Screenshot()
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "screenshot failed: %v", err)
	}
	return &pb.ScreenshotResponse{
		Data:   data,
		Format: "png",
	}, nil
}

func (s *ScoutServer) PDF(_ context.Context, req *pb.SessionRequest) (*pb.PDFResponse, error) {
	sess, err := s.getSession(req.SessionId)
	if err != nil {
		return nil, err
	}
	data, err := sess.page.PDF()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "pdf failed: %v", err)
	}
	return &pb.PDFResponse{Data: data}, nil
}

// ════════════════════════ Forensic Recording ════════════════════════

func (s *ScoutServer) StartRecording(_ context.Context, req *pb.RecordingRequest) (*pb.Empty, error) {
	sess, err := s.getSession(req.SessionId)
	if err != nil {
		return nil, err
	}
	if sess.recorder != nil {
		return nil, status.Error(codes.AlreadyExists, "recording already active")
	}
	recOpts := []scout.RecorderOption{}
	if req.CaptureBody {
		recOpts = append(recOpts, scout.WithCaptureBody(true))
	}
	sess.recorder = scout.NewNetworkRecorder(sess.page, recOpts...)
	return &pb.Empty{}, nil
}

func (s *ScoutServer) StopRecording(_ context.Context, req *pb.SessionRequest) (*pb.Empty, error) {
	sess, err := s.getSession(req.SessionId)
	if err != nil {
		return nil, err
	}
	if sess.recorder != nil {
		sess.recorder.Stop()
		sess.recorder = nil
	}
	return &pb.Empty{}, nil
}

func (s *ScoutServer) ExportHAR(_ context.Context, req *pb.SessionRequest) (*pb.HARResponse, error) {
	sess, err := s.getSession(req.SessionId)
	if err != nil {
		return nil, err
	}
	if sess.recorder == nil {
		return nil, status.Error(codes.FailedPrecondition, "no active recording")
	}
	data, count, err := sess.recorder.ExportHAR()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "export failed: %v", err)
	}
	return &pb.HARResponse{
		Data:       data,
		EntryCount: int32(count),
	}, nil
}

// ════════════════════════ Event Streaming ════════════════════════

func (s *ScoutServer) StreamEvents(req *pb.SessionRequest, stream pb.ScoutService_StreamEventsServer) error {
	sess, err := s.getSession(req.SessionId)
	if err != nil {
		return err
	}

	subID := uuid.NewString()
	ch := sess.subscribe(subID)
	defer sess.unsubscribe(subID)

	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return nil
			}
			if err := stream.Send(ev); err != nil {
				return err
			}
		case <-stream.Context().Done():
			return nil
		}
	}
}

// ════════════════════════ Bidirectional Interactive ════════════════════════

func (s *ScoutServer) Interactive(stream pb.ScoutService_InteractiveServer) error {
	var sess *session
	var subID string
	var eventCh chan *pb.BrowserEvent

	for {
		cmd, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		// Lazy session binding on first command
		if sess == nil {
			sess, err = s.getSession(cmd.SessionId)
			if err != nil {
				return err
			}
			subID = uuid.NewString()
			eventCh = sess.subscribe(subID)
			defer sess.unsubscribe(subID)
			_ = subID // used in defer above

			// Goroutine to forward events to client
			go func() {
				for ev := range eventCh {
					if err := stream.Send(ev); err != nil {
						return
					}
				}
			}()
		}

		// Execute the command
		if err := s.executeCommand(sess, cmd); err != nil {
			// Send error as event instead of breaking stream
			_ = stream.Send(&pb.BrowserEvent{
				SessionId: sess.id,
				Timestamp: time.Now().UnixMilli(),
				Event: &pb.BrowserEvent_Error{
					Error: &pb.ErrorEvent{
						Message: err.Error(),
						Source:  fmt.Sprintf("command:%s", cmd.RequestId),
					},
				},
			})
		}
	}
}

func (s *ScoutServer) executeCommand(sess *session, cmd *pb.Command) error {
	switch action := cmd.Action.(type) {
	case *pb.Command_Navigate:
		return sess.page.Navigate(action.Navigate.Url)

	case *pb.Command_Click:
		el, err := sess.page.Element(action.Click.Selector)
		if err != nil {
			return fmt.Errorf("element %q not found: %w", action.Click.Selector, err)
		}
		return el.Click()

	case *pb.Command_Type:
		el, err := sess.page.Element(action.Type.Selector)
		if err != nil {
			return fmt.Errorf("element %q not found: %w", action.Type.Selector, err)
		}
		return el.Input(action.Type.Text)

	case *pb.Command_PressKey:
		key := mapKey(action.PressKey.Key)
		return sess.page.KeyPress(key)

	case *pb.Command_Eval:
		_, err := sess.page.Eval(action.Eval.Script)
		return err

	case *pb.Command_Screenshot:
		var data []byte
		var err error
		if action.Screenshot.FullPage {
			data, err = sess.page.FullScreenshot()
		} else {
			data, err = sess.page.Screenshot()
		}
		if err != nil {
			return err
		}
		_ = data // screenshot data available via ExportHAR or separate RPC
		return nil

	case *pb.Command_Wait:
		_, err := sess.page.Element(action.Wait.Selector)
		return err

	case *pb.Command_Scroll:
		script := fmt.Sprintf("window.scrollTo(%d, %d)", action.Scroll.X, action.Scroll.Y)
		_, err := sess.page.Eval(script)
		return err

	default:
		return fmt.Errorf("unknown command type")
	}
}

// ════════════════════════ CDP Event Wiring ════════════════════════

func (s *ScoutServer) wireEvents(sess *session) {
	page := sess.page.RodPage()

	go page.EachEvent(
		func(e *proto.NetworkRequestWillBeSent) {
			headers := make(map[string]string)
			for k, v := range e.Request.Headers {
				headers[k] = v.String()
			}

			sess.broadcast(&pb.BrowserEvent{
				Event: &pb.BrowserEvent_RequestSent{
					RequestSent: &pb.NetworkRequestEvent{
						RequestId:    string(e.RequestID),
						Method:       e.Request.Method,
						Url:          e.Request.URL,
						Headers:      headers,
						PostData:     e.Request.PostData,
						ResourceType: string(e.Type),
					},
				},
			})
		},
		func(e *proto.NetworkResponseReceived) {
			headers := make(map[string]string)
			for k, v := range e.Response.Headers {
				headers[k] = v.String()
			}

			var timeMs float64
			if e.Response.Timing != nil {
				timeMs = e.Response.Timing.ReceiveHeadersEnd
			}

			sess.broadcast(&pb.BrowserEvent{
				Event: &pb.BrowserEvent_ResponseReceived{
					ResponseReceived: &pb.NetworkResponseEvent{
						RequestId:  string(e.RequestID),
						Url:        e.Response.URL,
						Status:     int32(e.Response.Status),
						StatusText: e.Response.StatusText,
						Headers:    headers,
						MimeType:   e.Response.MIMEType,
						RemoteIp:   e.Response.RemoteIPAddress,
						TimeMs:     timeMs,
					},
				},
			})
		},
		func(e *proto.RuntimeConsoleAPICalled) {
			var msg string
			for _, arg := range e.Args {
				if !arg.Value.Nil() {
					msg += fmt.Sprintf("%v ", arg.Value.Val())
				}
			}
			sess.broadcast(&pb.BrowserEvent{
				Event: &pb.BrowserEvent_Console{
					Console: &pb.ConsoleEvent{
						Level:   string(e.Type),
						Message: msg,
					},
				},
			})
		},
		func(e *proto.PageLoadEventFired) {
			_ = e // suppress unused warning
			url, _ := sess.page.URL()
			sess.broadcast(&pb.BrowserEvent{
				Event: &pb.BrowserEvent_PageEvent{
					PageEvent: &pb.PageEvent{
						Type: "load",
						Url:  url,
					},
				},
			})
		},
	)()
}

// mapKey converts a string key name to an input.Key constant.
func mapKey(key string) input.Key {
	switch key {
	case "Enter":
		return input.Enter
	case "Tab":
		return input.Tab
	case "Escape":
		return input.Escape
	case "Space":
		return input.Space
	case "Backspace":
		return input.Backspace
	case "Delete":
		return input.Delete
	case "ArrowUp":
		return input.ArrowUp
	case "ArrowDown":
		return input.ArrowDown
	case "ArrowLeft":
		return input.ArrowLeft
	case "ArrowRight":
		return input.ArrowRight
	case "Home":
		return input.Home
	case "End":
		return input.End
	case "PageUp":
		return input.PageUp
	case "PageDown":
		return input.PageDown
	default:
		if len(key) == 1 {
			return input.Key(key[0])
		}
		return 0
	}
}
