package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	pb "github.com/inovacc/scout/grpc/scoutpb"
	input2 "github.com/inovacc/scout/internal/engine/lib/input"
	proto2 "github.com/inovacc/scout/internal/engine/lib/proto"
	"github.com/inovacc/scout/pkg/scout"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ════════════════════════ Session Hijacking ════════════════════════

func (s *session) broadcastHijack(ev *pb.HijackedEvent) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ev.SessionId = s.id
	ev.Timestamp = time.Now().UnixMilli()

	for _, ch := range s.hijackSubs {
		select {
		case ch <- ev:
		default:
		}
	}
}

func (s *session) subscribeHijack(id string) chan *pb.HijackedEvent {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.hijackSubs == nil {
		s.hijackSubs = make(map[string]chan *pb.HijackedEvent)
	}

	ch := make(chan *pb.HijackedEvent, 256)
	s.hijackSubs[id] = ch

	return ch
}

func (s *session) unsubscribeHijack(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ch, ok := s.hijackSubs[id]; ok {
		close(ch)
		delete(s.hijackSubs, id)
	}
}

func (s *ScoutServer) StartHijack(ctx context.Context, req *pb.HijackRequest) (*pb.Empty, error) {
	sess, err := s.getSession(ctx, req.GetSessionId())
	if err != nil {
		return nil, err
	}

	if sess.hijacker != nil {
		return nil, status.Error(codes.AlreadyExists, "hijack already active")
	}

	var opts []scout.HijackOption
	if req.GetCaptureBody() {
		opts = append(opts, scout.WithHijackBodyCapture())
	}

	if len(req.GetUrlPatterns()) > 0 {
		opts = append(opts, scout.WithHijackURLFilter(req.GetUrlPatterns()...))
	}

	hijacker, err := sess.page.NewSessionHijacker(opts...)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "hijack start failed: %v", sanitizeError(err))
	}

	sess.hijacker = hijacker

	// Fan-out goroutine: read from hijacker channel and broadcast to gRPC subscribers.
	go func() {
		for ev := range hijacker.Events() {
			pbEv := hijackEventToProto(ev)
			sess.broadcastHijack(pbEv)
		}
	}()

	return &pb.Empty{}, nil
}

func (s *ScoutServer) StopHijack(ctx context.Context, req *pb.SessionRequest) (*pb.Empty, error) {
	sess, err := s.getSession(ctx, req.GetSessionId())
	if err != nil {
		return nil, err
	}

	if sess.hijacker != nil {
		sess.hijacker.Stop()
		sess.hijacker = nil
	}

	return &pb.Empty{}, nil
}

func (s *ScoutServer) StreamHijack(req *pb.SessionRequest, stream pb.ScoutService_StreamHijackServer) error {
	sess, err := s.getSession(stream.Context(), req.GetSessionId())
	if err != nil {
		return err
	}

	subID := uuid.NewString()

	ch := sess.subscribeHijack(subID)
	defer sess.unsubscribeHijack(subID)

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

func hijackEventToProto(ev scout.HijackEvent) *pb.HijackedEvent {
	pbEv := &pb.HijackedEvent{}

	switch ev.Type { //nolint:exhaustive // all cases handled
	case scout.HijackEventRequest:
		if ev.Request != nil {
			pbEv.Event = &pb.HijackedEvent_Request{
				Request: &pb.HijackedRequestEvent{
					RequestId:    ev.Request.RequestID,
					Method:       ev.Request.Method,
					Url:          ev.Request.URL,
					Headers:      ev.Request.Headers,
					Body:         ev.Request.Body,
					ResourceType: ev.Request.ResourceType,
				},
			}
		}
	case scout.HijackEventResponse:
		if ev.Response != nil {
			pbEv.Event = &pb.HijackedEvent_Response{
				Response: &pb.HijackedResponseEvent{
					RequestId: ev.Response.RequestID,
					Url:       ev.Response.URL,
					Status:    int32(ev.Response.Status),
					Headers:   ev.Response.Headers,
					Body:      ev.Response.Body,
					MimeType:  ev.Response.MimeType,
					ElapsedMs: ev.Response.ElapsedMs,
				},
			}
		}
	case scout.HijackWSSent, scout.HijackWSReceived, scout.HijackWSOpened, scout.HijackWSClosed:
		if ev.Frame != nil {
			pbEv.Event = &pb.HijackedEvent_WsFrame{
				WsFrame: &pb.WebSocketFrameEvent{
					RequestId: ev.Frame.RequestID,
					Url:       ev.Frame.URL,
					Direction: ev.Frame.Direction,
					Opcode:    ev.Frame.Opcode,
					Payload:   ev.Frame.Payload,
					Masked:    ev.Frame.Masked,
				},
			}
		}
	}

	return pbEv
}

// ════════════════════════ Profile ════════════════════════

func (s *ScoutServer) CaptureProfile(ctx context.Context, req *pb.CaptureProfileRequest) (*pb.CaptureProfileResponse, error) {
	sess, err := s.getSession(ctx, req.GetSessionId())
	if err != nil {
		return nil, err
	}

	prof, err := scout.CaptureProfile(sess.page)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "capture profile failed: %v", sanitizeError(err))
	}

	data, err := json.Marshal(prof)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshal profile failed: %v", err)
	}

	s.recordEvent("capture_profile", req.GetSessionId(), s.peerShortID(req.GetSessionId()), "profile captured")

	return &pb.CaptureProfileResponse{ProfileJson: string(data)}, nil
}

// LoadProfile is a compatibility stub. Post secret-migration, UserProfile carries no
// secrets and Page.ApplyProfile is a no-op, so this RPC no longer applies
// cookies/headers/storage to the session. Flag for deprecation/removal in Phase 2
// (after 2026-07-02). Local secret apply is done via `scout vault use`.
func (s *ScoutServer) LoadProfile(ctx context.Context, req *pb.LoadProfileRequest) (*pb.LoadProfileResponse, error) {
	sess, err := s.getSession(ctx, req.GetSessionId())
	if err != nil {
		return nil, err
	}

	var prof scout.UserProfile
	if err := json.Unmarshal([]byte(req.GetProfileJson()), &prof); err != nil {
		return &pb.LoadProfileResponse{Success: false, Error: fmt.Sprintf("unmarshal profile: %v", err)}, nil
	}

	if err := sess.page.ApplyProfile(&prof); err != nil {
		return &pb.LoadProfileResponse{Success: false, Error: fmt.Sprintf("apply profile: %v", sanitizeError(err))}, nil
	}

	s.recordEvent("load_profile", req.GetSessionId(), s.peerShortID(req.GetSessionId()), "profile loaded")

	return &pb.LoadProfileResponse{Success: true}, nil
}

// ════════════════════════ Event Streaming ════════════════════════

func (s *ScoutServer) StreamEvents(req *pb.SessionRequest, stream pb.ScoutService_StreamEventsServer) error {
	sess, err := s.getSession(stream.Context(), req.GetSessionId())
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
	var (
		sess    *session
		subID   string
		eventCh chan *pb.BrowserEvent
	)

	for { //nolint:wsl
		cmd, err := stream.Recv()
		if err == io.EOF {
			return nil
		}

		if err != nil {
			return err
		}

		// Lazy session binding on first command
		if sess == nil {
			sess, err = s.getSession(stream.Context(), cmd.GetSessionId())
			if err != nil {
				return err
			}

			subID = uuid.NewString()
			eventCh = sess.subscribe(subID)

			defer sess.unsubscribe(subID)

			_ = subID   // used in defer above
			_ = eventCh // used in goroutine below

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
						Source:  fmt.Sprintf("command:%s", cmd.GetRequestId()),
					},
				},
			})
		}
	}
}

func (s *ScoutServer) executeCommand(sess *session, cmd *pb.Command) error {
	switch action := cmd.Action.(type) { //nolint:protogetter // type switch requires field access
	case *pb.Command_Navigate:
		return sess.page.Navigate(action.Navigate.GetUrl())

	case *pb.Command_Click:
		el, err := sess.page.Element(action.Click.GetSelector())
		if err != nil {
			return fmt.Errorf("element %q not found: %w", action.Click.GetSelector(), err)
		}

		return el.Click()

	case *pb.Command_Type:
		el, err := sess.page.Element(action.Type.GetSelector())
		if err != nil {
			return fmt.Errorf("element %q not found: %w", action.Type.GetSelector(), err)
		}

		return el.Input(action.Type.GetText())

	case *pb.Command_PressKey:
		key := mapKey(action.PressKey.GetKey())
		return sess.page.KeyPress(key)

	case *pb.Command_Eval:
		_, err := sess.page.Eval(action.Eval.GetScript())
		return err

	case *pb.Command_Screenshot:
		var (
			data []byte
			err  error
		)

		if action.Screenshot.GetFullPage() {
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
		_, err := sess.page.Element(action.Wait.GetSelector())
		return err

	case *pb.Command_Scroll:
		script := fmt.Sprintf("window.scrollTo(%d, %d)", action.Scroll.GetX(), action.Scroll.GetY())
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
		func(e *proto2.NetworkRequestWillBeSent) {
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
		func(e *proto2.NetworkResponseReceived) {
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
		func(e *proto2.RuntimeConsoleAPICalled) {
			var sb strings.Builder

			for _, arg := range e.Args {
				if !arg.Value.Nil() {
					_, _ = fmt.Fprintf(&sb, "%v ", arg.Value.Val())
				}
			}

			sess.broadcast(&pb.BrowserEvent{
				Event: &pb.BrowserEvent_Console{
					Console: &pb.ConsoleEvent{
						Level:   string(e.Type),
						Message: sb.String(),
					},
				},
			})
		},
		func(e *proto2.PageLoadEventFired) {
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
func mapKey(key string) input2.Key {
	switch key {
	case "Enter":
		return input2.Enter
	case "Tab":
		return input2.Tab
	case "Escape":
		return input2.Escape
	case "Space":
		return input2.Space
	case "Backspace":
		return input2.Backspace
	case "Delete":
		return input2.Delete
	case "ArrowUp":
		return input2.ArrowUp
	case "ArrowDown":
		return input2.ArrowDown
	case "ArrowLeft":
		return input2.ArrowLeft
	case "ArrowRight":
		return input2.ArrowRight
	case "Home":
		return input2.Home
	case "End":
		return input2.End
	case "PageUp":
		return input2.PageUp
	case "PageDown":
		return input2.PageDown
	default:
		if len(key) == 1 {
			return input2.Key(key[0])
		}

		return 0
	}
}
