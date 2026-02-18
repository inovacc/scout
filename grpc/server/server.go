package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	pb "github.com/inovacc/scout/grpc/scoutpb"
	"github.com/inovacc/scout/pkg/identity"
	"github.com/inovacc/scout/pkg/rod/lib/input"
	"github.com/inovacc/scout/pkg/rod/lib/proto"
	"github.com/inovacc/scout/pkg/scout"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
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

// SessionEvent records a server-side activity for the event log.
type SessionEvent struct {
	Time      time.Time
	Type      string // "connect", "disconnect", "navigate", "screenshot", etc.
	SessionID string
	DeviceID  string
	Detail    string
}

// maxEvents is the maximum number of events kept in the ring buffer.
const maxEvents = 50

// ScoutServer implements the gRPC ScoutService.
type ScoutServer struct {
	pb.UnimplementedScoutServiceServer

	sessions sync.Map // map[string]*session
	peers    sync.Map // map[deviceID]*ConnectedPeer

	// sessionPeer tracks which device owns each session.
	sessionPeer sync.Map // map[sessionID]deviceID

	// OnPeerChange is called when peer list changes. Set by the CLI server command.
	OnPeerChange func(peers []ConnectedPeer)

	// OnStatsChange is called when stats or events change.
	OnStatsChange func()

	stats struct {
		sync.Mutex
		totalSessions int64
		totalRequests int64
		events        []SessionEvent
	}
}

// New creates a new ScoutServer.
func New() *ScoutServer {
	return &ScoutServer{}
}

// Stats returns cumulative session/request counts.
func (s *ScoutServer) Stats() (totalSessions, totalRequests int64) {
	s.stats.Lock()
	defer s.stats.Unlock()
	return s.stats.totalSessions, s.stats.totalRequests
}

// Events returns a copy of the recent event log.
func (s *ScoutServer) Events() []SessionEvent {
	s.stats.Lock()
	defer s.stats.Unlock()
	result := make([]SessionEvent, len(s.stats.events))
	copy(result, s.stats.events)
	return result
}

func (s *ScoutServer) recordEvent(typ, sessionID, deviceID, detail string) {
	s.stats.Lock()
	s.stats.events = append(s.stats.events, SessionEvent{
		Time:      time.Now(),
		Type:      typ,
		SessionID: sessionID,
		DeviceID:  deviceID,
		Detail:    detail,
	})
	if len(s.stats.events) > maxEvents {
		s.stats.events = s.stats.events[len(s.stats.events)-maxEvents:]
	}
	s.stats.totalRequests++
	s.stats.Unlock()

	if s.OnStatsChange != nil {
		s.OnStatsChange()
	}
}

// pathSanitizer matches local filesystem paths that should not be exposed to clients.
var pathSanitizer = regexp.MustCompile(`(?i)([A-Z]:\\[^\s"']+|/(?:home|Users|tmp|var|root|etc)[^\s"']+|/\w+/\.\w+[^\s"']+)`)

// sanitizeError strips local filesystem paths from error messages.
func sanitizeError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	sanitized := pathSanitizer.ReplaceAllString(msg, "[path-redacted]")
	if sanitized == msg {
		return err
	}
	return fmt.Errorf("%s", sanitized)
}

// Peers returns a snapshot of all connected peers.
func (s *ScoutServer) Peers() []ConnectedPeer {
	var result []ConnectedPeer
	s.peers.Range(func(_, v any) bool {
		p := v.(*ConnectedPeer)
		result = append(result, *p)
		return true
	})
	return result
}

func (s *ScoutServer) trackPeer(ctx context.Context, sessionID string) {
	deviceID := "unknown"
	addr := "unknown"

	if p, ok := peer.FromContext(ctx); ok {
		addr = p.Addr.String()
		if tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo); ok {
			if len(tlsInfo.State.PeerCertificates) > 0 {
				deviceID = identity.DeviceIDFromCert(tlsInfo.State.PeerCertificates[0])
			}
		}
	}

	s.sessionPeer.Store(sessionID, deviceID)

	shortID := identity.ShortID(deviceID)

	if v, ok := s.peers.Load(deviceID); ok {
		p := v.(*ConnectedPeer)
		p.Sessions++
	} else {
		s.peers.Store(deviceID, &ConnectedPeer{
			DeviceID:    deviceID,
			ShortID:     shortID,
			Addr:        addr,
			ConnectedAt: time.Now(),
			Sessions:    1,
		})
	}

	s.stats.Lock()
	s.stats.totalSessions++
	s.stats.Unlock()

	s.recordEvent("connect", sessionID, shortID, "session "+sessionID[:8])
	s.notifyPeerChange()
}

func (s *ScoutServer) untrackPeer(sessionID string) {
	v, ok := s.sessionPeer.LoadAndDelete(sessionID)
	if !ok {
		return
	}
	deviceID := v.(string)
	shortID := identity.ShortID(deviceID)

	if v, ok := s.peers.Load(deviceID); ok {
		p := v.(*ConnectedPeer)
		p.Sessions--
		if p.Sessions <= 0 {
			s.peers.Delete(deviceID)
		}
	}

	s.recordEvent("disconnect", sessionID, shortID, "session "+sessionID[:8])
	s.notifyPeerChange()
}

func (s *ScoutServer) notifyPeerChange() {
	if s.OnPeerChange != nil {
		s.OnPeerChange(s.Peers())
	}
}

// NotifyPeerChange triggers a peer change notification externally (e.g. after pairing).
func (s *ScoutServer) NotifyPeerChange() {
	s.notifyPeerChange()
}

func (s *ScoutServer) peerShortID(sessionID string) string {
	v, ok := s.sessionPeer.Load(sessionID)
	if !ok {
		return "unknown"
	}
	return identity.ShortID(v.(string))
}

func (s *ScoutServer) getSession(id string) (*session, error) {
	v, ok := s.sessions.Load(id)
	if !ok {
		return nil, status.Errorf(codes.NotFound, "session %q not found", id)
	}

	return v.(*session), nil
}

// ════════════════════════ Session Lifecycle ════════════════════════

func (s *ScoutServer) CreateSession(ctx context.Context, req *pb.CreateSessionRequest) (*pb.CreateSessionResponse, error) {
	opts := platformSessionDefaults()
	opts = append(opts, scout.WithHeadless(req.GetHeadless()))

	if req.GetStealth() {
		opts = append(opts, scout.WithStealth())
	}

	if req.GetProxy() != "" {
		opts = append(opts, scout.WithProxy(req.GetProxy()))
	}

	if req.GetUserAgent() != "" {
		opts = append(opts, scout.WithUserAgent(req.GetUserAgent()))
	}

	if req.GetWidth() > 0 && req.GetHeight() > 0 {
		opts = append(opts, scout.WithWindowSize(int(req.GetWidth()), int(req.GetHeight())))
	}

	if req.GetMaximized() {
		opts = append(opts, scout.WithMaximized())
	}

	if req.GetDevtools() {
		opts = append(opts, scout.WithDevTools())
	}

	if req.GetNoSandbox() {
		opts = append(opts, scout.WithNoSandbox())
	}

	browser, err := scout.New(opts...)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "browser launch failed: %v", sanitizeError(err))
	}

	url := "about:blank"
	if req.GetInitialUrl() != "" {
		url = req.GetInitialUrl()
	}

	page, err := browser.NewPage(url)
	if err != nil {
		_ = browser.Close()
		return nil, status.Errorf(codes.Internal, "page creation failed: %v", sanitizeError(err))
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
	if req.GetRecord() {
		recOpts := []scout.RecorderOption{}
		if req.GetCaptureBody() {
			recOpts = append(recOpts, scout.WithCaptureBody(true))
		}

		sess.recorder = scout.NewNetworkRecorder(page, recOpts...)
	}

	s.sessions.Store(sess.id, sess)
	s.trackPeer(ctx, sess.id)

	title, _ := page.Title()
	currentURL, _ := page.URL()

	return &pb.CreateSessionResponse{
		SessionId: sess.id,
		Url:       currentURL,
		Title:     title,
	}, nil
}

func (s *ScoutServer) DestroySession(_ context.Context, req *pb.SessionRequest) (*pb.Empty, error) {
	sess, err := s.getSession(req.GetSessionId())
	if err != nil {
		return nil, err
	}

	if sess.recorder != nil {
		sess.recorder.Stop()
	}

	_ = sess.browser.Close()

	s.sessions.Delete(req.GetSessionId())
	s.untrackPeer(req.GetSessionId())

	return &pb.Empty{}, nil
}

// ════════════════════════ Navigation ════════════════════════

func (s *ScoutServer) Navigate(_ context.Context, req *pb.NavigateRequest) (*pb.NavigateResponse, error) {
	sess, err := s.getSession(req.GetSessionId())
	if err != nil {
		return nil, err
	}

	if err := sess.page.Navigate(req.GetUrl()); err != nil {
		return nil, status.Errorf(codes.Internal, "navigate failed: %v", sanitizeError(err))
	}

	if req.GetWaitStable() {
		_ = sess.page.WaitStable(500 * time.Millisecond)
	}

	title, _ := sess.page.Title()
	url, _ := sess.page.URL()

	s.recordEvent("navigate", req.GetSessionId(), s.peerShortID(req.GetSessionId()), req.GetUrl())

	return &pb.NavigateResponse{
		Url:   url,
		Title: title,
	}, nil
}

func (s *ScoutServer) Reload(_ context.Context, req *pb.SessionRequest) (*pb.Empty, error) {
	sess, err := s.getSession(req.GetSessionId())
	if err != nil {
		return nil, err
	}

	if err := sess.page.Reload(); err != nil {
		return nil, status.Errorf(codes.Internal, "reload failed: %v", sanitizeError(err))
	}

	return &pb.Empty{}, nil
}

func (s *ScoutServer) GoBack(_ context.Context, req *pb.SessionRequest) (*pb.Empty, error) {
	sess, err := s.getSession(req.GetSessionId())
	if err != nil {
		return nil, err
	}

	if err := sess.page.NavigateBack(); err != nil {
		return nil, status.Errorf(codes.Internal, "go back failed: %v", sanitizeError(err))
	}

	return &pb.Empty{}, nil
}

func (s *ScoutServer) GoForward(_ context.Context, req *pb.SessionRequest) (*pb.Empty, error) {
	sess, err := s.getSession(req.GetSessionId())
	if err != nil {
		return nil, err
	}

	if err := sess.page.NavigateForward(); err != nil {
		return nil, status.Errorf(codes.Internal, "go forward failed: %v", sanitizeError(err))
	}

	return &pb.Empty{}, nil
}

// ════════════════════════ Element Interaction ════════════════════════

func (s *ScoutServer) Click(_ context.Context, req *pb.ElementRequest) (*pb.Empty, error) {
	sess, err := s.getSession(req.GetSessionId())
	if err != nil {
		return nil, err
	}

	el, err := sess.findElement(req.GetSelector(), req.GetXpath())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "element %q not found: %v", req.GetSelector(), sanitizeError(err))
	}

	if err := el.Click(); err != nil {
		return nil, status.Errorf(codes.Internal, "click failed: %v", sanitizeError(err))
	}

	return &pb.Empty{}, nil
}

func (s *ScoutServer) DoubleClick(_ context.Context, req *pb.ElementRequest) (*pb.Empty, error) {
	sess, err := s.getSession(req.GetSessionId())
	if err != nil {
		return nil, err
	}

	el, err := sess.findElement(req.GetSelector(), req.GetXpath())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "element not found: %v", sanitizeError(err))
	}

	if err := el.DoubleClick(); err != nil {
		return nil, status.Errorf(codes.Internal, "double-click failed: %v", sanitizeError(err))
	}

	return &pb.Empty{}, nil
}

func (s *ScoutServer) RightClick(_ context.Context, req *pb.ElementRequest) (*pb.Empty, error) {
	sess, err := s.getSession(req.GetSessionId())
	if err != nil {
		return nil, err
	}

	el, err := sess.findElement(req.GetSelector(), req.GetXpath())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "element not found: %v", sanitizeError(err))
	}

	if err := el.RightClick(); err != nil {
		return nil, status.Errorf(codes.Internal, "right-click failed: %v", sanitizeError(err))
	}

	return &pb.Empty{}, nil
}

func (s *ScoutServer) Hover(_ context.Context, req *pb.ElementRequest) (*pb.Empty, error) {
	sess, err := s.getSession(req.GetSessionId())
	if err != nil {
		return nil, err
	}

	el, err := sess.findElement(req.GetSelector(), req.GetXpath())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "element not found: %v", sanitizeError(err))
	}

	if err := el.Hover(); err != nil {
		return nil, status.Errorf(codes.Internal, "hover failed: %v", sanitizeError(err))
	}

	return &pb.Empty{}, nil
}

func (s *ScoutServer) Type(_ context.Context, req *pb.TypeRequest) (*pb.Empty, error) {
	sess, err := s.getSession(req.GetSessionId())
	if err != nil {
		return nil, err
	}

	el, err := sess.page.Element(req.GetSelector())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "element not found: %v", sanitizeError(err))
	}

	if req.GetClearFirst() {
		_ = el.Clear()
	}

	if err := el.Input(req.GetText()); err != nil {
		return nil, status.Errorf(codes.Internal, "type failed: %v", sanitizeError(err))
	}

	return &pb.Empty{}, nil
}

func (s *ScoutServer) SelectOption(_ context.Context, req *pb.SelectRequest) (*pb.Empty, error) {
	sess, err := s.getSession(req.GetSessionId())
	if err != nil {
		return nil, err
	}

	el, err := sess.page.Element(req.GetSelector())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "element not found: %v", sanitizeError(err))
	}

	if err := el.SelectOption(req.GetValue()); err != nil {
		return nil, status.Errorf(codes.Internal, "select option failed: %v", sanitizeError(err))
	}

	return &pb.Empty{}, nil
}

func (s *ScoutServer) PressKey(_ context.Context, req *pb.KeyRequest) (*pb.Empty, error) {
	sess, err := s.getSession(req.GetSessionId())
	if err != nil {
		return nil, err
	}

	key := mapKey(req.GetKey())
	if err := sess.page.KeyPress(key); err != nil {
		return nil, status.Errorf(codes.Internal, "press key failed: %v", sanitizeError(err))
	}

	return &pb.Empty{}, nil
}

// ════════════════════════ Query ════════════════════════

func (s *ScoutServer) GetText(_ context.Context, req *pb.ElementRequest) (*pb.TextResponse, error) {
	sess, err := s.getSession(req.GetSessionId())
	if err != nil {
		return nil, err
	}

	el, err := sess.findElement(req.GetSelector(), req.GetXpath())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "element not found: %v", sanitizeError(err))
	}

	text, err := el.Text()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get text failed: %v", sanitizeError(err))
	}

	return &pb.TextResponse{Text: text}, nil
}

func (s *ScoutServer) GetAttribute(_ context.Context, req *pb.AttributeRequest) (*pb.TextResponse, error) {
	sess, err := s.getSession(req.GetSessionId())
	if err != nil {
		return nil, err
	}

	el, err := sess.page.Element(req.GetSelector())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "element not found: %v", sanitizeError(err))
	}

	val, _, err := el.Attribute(req.GetAttribute())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get attribute failed: %v", sanitizeError(err))
	}

	return &pb.TextResponse{Text: val}, nil
}

func (s *ScoutServer) GetTitle(_ context.Context, req *pb.SessionRequest) (*pb.TextResponse, error) {
	sess, err := s.getSession(req.GetSessionId())
	if err != nil {
		return nil, err
	}

	title, err := sess.page.Title()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get title failed: %v", sanitizeError(err))
	}

	return &pb.TextResponse{Text: title}, nil
}

func (s *ScoutServer) GetURL(_ context.Context, req *pb.SessionRequest) (*pb.TextResponse, error) {
	sess, err := s.getSession(req.GetSessionId())
	if err != nil {
		return nil, err
	}

	url, err := sess.page.URL()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get url failed: %v", sanitizeError(err))
	}

	return &pb.TextResponse{Text: url}, nil
}

func (s *ScoutServer) Eval(_ context.Context, req *pb.EvalRequest) (*pb.EvalResponse, error) {
	sess, err := s.getSession(req.GetSessionId())
	if err != nil {
		return nil, err
	}

	result, err := sess.page.Eval(req.GetScript())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "eval failed: %v", sanitizeError(err))
	}

	data, err2 := json.Marshal(result) //nolint:musttag // result is dynamic eval output
	if err2 != nil {
		return nil, status.Errorf(codes.Internal, "marshal result failed: %v", err2)
	}

	return &pb.EvalResponse{Result: string(data)}, nil
}

func (s *ScoutServer) ElementExists(_ context.Context, req *pb.ElementRequest) (*pb.BoolResponse, error) {
	sess, err := s.getSession(req.GetSessionId())
	if err != nil {
		return nil, err
	}

	var exists bool
	if req.GetXpath() {
		exists, _ = sess.page.HasXPath(req.GetSelector())
	} else {
		exists, _ = sess.page.Has(req.GetSelector())
	}

	return &pb.BoolResponse{Value: exists}, nil
}

// ════════════════════════ Capture ════════════════════════

func (s *ScoutServer) Screenshot(_ context.Context, req *pb.ScreenshotRequest) (*pb.ScreenshotResponse, error) {
	sess, err := s.getSession(req.GetSessionId())
	if err != nil {
		return nil, err
	}

	var data []byte

	if req.GetFullPage() {
		data, err = sess.page.FullScreenshot()
	} else {
		data, err = sess.page.Screenshot()
	}

	if err != nil {
		return nil, status.Errorf(codes.Internal, "screenshot failed: %v", sanitizeError(err))
	}

	mode := "viewport"
	if req.GetFullPage() {
		mode = "fullpage"
	}
	s.recordEvent("screenshot", req.GetSessionId(), s.peerShortID(req.GetSessionId()), fmt.Sprintf("%s %dKB", mode, len(data)/1024))

	return &pb.ScreenshotResponse{
		Data:   data,
		Format: "png",
	}, nil
}

func (s *ScoutServer) PDF(_ context.Context, req *pb.SessionRequest) (*pb.PDFResponse, error) {
	sess, err := s.getSession(req.GetSessionId())
	if err != nil {
		return nil, err
	}

	data, err := sess.page.PDF()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "pdf failed: %v", sanitizeError(err))
	}

	return &pb.PDFResponse{Data: data}, nil
}

// ════════════════════════ Forensic Recording ════════════════════════

func (s *ScoutServer) StartRecording(_ context.Context, req *pb.RecordingRequest) (*pb.Empty, error) {
	sess, err := s.getSession(req.GetSessionId())
	if err != nil {
		return nil, err
	}

	if sess.recorder != nil {
		return nil, status.Error(codes.AlreadyExists, "recording already active")
	}

	recOpts := []scout.RecorderOption{}
	if req.GetCaptureBody() {
		recOpts = append(recOpts, scout.WithCaptureBody(true))
	}

	sess.recorder = scout.NewNetworkRecorder(sess.page, recOpts...)

	return &pb.Empty{}, nil
}

func (s *ScoutServer) StopRecording(_ context.Context, req *pb.SessionRequest) (*pb.Empty, error) {
	sess, err := s.getSession(req.GetSessionId())
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
	sess, err := s.getSession(req.GetSessionId())
	if err != nil {
		return nil, err
	}

	if sess.recorder == nil {
		return nil, status.Error(codes.FailedPrecondition, "no active recording")
	}

	data, count, err := sess.recorder.ExportHAR()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "export failed: %v", sanitizeError(err))
	}

	return &pb.HARResponse{
		Data:       data,
		EntryCount: int32(count),
	}, nil
}

// ════════════════════════ Event Streaming ════════════════════════

func (s *ScoutServer) StreamEvents(req *pb.SessionRequest, stream pb.ScoutService_StreamEventsServer) error {
	sess, err := s.getSession(req.GetSessionId())
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
			sess, err = s.getSession(cmd.GetSessionId())
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
