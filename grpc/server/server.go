package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	pb "github.com/inovacc/scout/grpc/scoutpb"
	input2 "github.com/inovacc/scout/internal/engine/lib/input"
	proto2 "github.com/inovacc/scout/internal/engine/lib/proto"
	"github.com/inovacc/scout/internal/engine/swarm"
	"github.com/inovacc/scout/internal/idle"
	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/identity"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

// session holds a browser instance, page, recorder, and event subscribers.
type session struct {
	id            string
	browser       *scout.Browser
	page          *scout.Page
	recorder      *scout.NetworkRecorder
	hijacker      *scout.SessionHijacker
	subs          map[string]chan *pb.BrowserEvent
	hijackSubs    map[string]chan *pb.HijackedEvent
	mu            sync.RWMutex
	monitorCancel func() // stops console/ws sidecar goroutines on destroy
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

	// IdleTimeout is the duration of inactivity before auto-shutdown.
	IdleTimeout time.Duration

	// OnIdleShutdown is called when idle timeout fires. Set by the CLI to
	// trigger graceful server stop.
	OnIdleShutdown func()

	idle *idle.Timer

	// swarmCoord is the optional swarm coordinator for distributed crawling.
	swarmCoord *swarm.Coordinator

	stats struct {
		sync.Mutex

		totalSessions int64
		totalRequests int64
		events        []SessionEvent
	}
}

// reapHook performs a single on-disk orphan-reaping pass and returns the
// number of holder processes killed. It is a package var so tests can stub
// it without a live browser. Production wires it to scout.ReapOnce in
// reconcile_wire.go.
var reapHook = func() int { return 0 }

// New creates a new ScoutServer.
func New() *ScoutServer {
	return &ScoutServer{}
}

// Reconcile reaps prior-instance session orphans on the disk at daemon
// startup. The in-memory session map is empty at boot, so there is nothing
// to adopt — Reconcile only kills/removes on-disk orphans left by a crashed
// or force-killed previous daemon. It returns the number of holder processes
// killed during the pass. Best-effort; never fatal.
func (s *ScoutServer) Reconcile() int {
	defer func() {
		if r := recover(); r != nil {
			slog.Warn("scout: reconcile: recovered from panic", "panic", r)
		}
	}()

	return reapHook()
}

// SetSwarm attaches a swarm coordinator for distributed crawling RPCs.
func (s *ScoutServer) SetSwarm(c *swarm.Coordinator) {
	s.swarmCoord = c
}

// StartIdleTimer initializes the idle timer if IdleTimeout > 0.
// Call after setting IdleTimeout and OnIdleShutdown.
func (s *ScoutServer) StartIdleTimer() {
	if s.IdleTimeout <= 0 {
		return
	}

	s.idle = idle.New(s.IdleTimeout, func() {
		// Full teardown of every session: stops monitor sidecars + hijacker,
		// flushes HAR, stops recorders, closes browsers, untracks peers.
		// Runs BEFORE OnIdleShutdown so artifacts are flushed before the
		// gRPC server stops accepting calls.
		s.DestroyAllSessions()

		if s.OnIdleShutdown != nil {
			s.OnIdleShutdown()
		}
	})
}

// StopIdleTimer permanently disables the idle timer.
func (s *ScoutServer) StopIdleTimer() {
	if s.idle != nil {
		s.idle.Stop()
	}
}

func (s *ScoutServer) touchIdle() {
	if s.idle != nil {
		s.idle.Reset()
	}
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
				deviceID, _ = identity.DeviceIDFromCert(tlsInfo.State.PeerCertificates[0])
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
	s.touchIdle()

	opts := platformSessionDefaults()
	// Disable per-page timeout for server sessions. Rod's Page.Timeout() creates
	// a one-shot context that expires permanently after the duration, making the
	// page unusable for long-lived sessions. The gRPC layer manages its own deadlines.
	opts = append(opts, scout.WithTimeout(0))
	// H6: daemon-managed sessions default to reusable so the engine's
	// Browser.Close path preserves the session dir for resume. Callers
	// opt into ephemeral via the Ephemeral request field — useful for
	// short-lived gather / scrape jobs where persistence is undesirable.
	if !req.GetEphemeral() {
		opts = append(opts, scout.WithReusableSession())

		if secs := req.GetExpiresInSeconds(); secs > 0 {
			opts = append(opts, scout.WithReusableLifetime(time.Duration(secs)*time.Second))
		}
	}
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

	// Block rules from CreateSessionRequest.Blocks. Each matching request
	// is aborted at the browser; pair with --record-har or --record-hijack
	// to capture the intended payload before abort.
	if blocks := req.GetBlocks(); len(blocks) > 0 {
		rules := make([]scout.BlockRule, 0, len(blocks))
		for _, b := range blocks {
			rules = append(rules, scout.BlockRule{Pattern: b.GetPattern(), Method: b.GetMethod()})
		}
		opts = append(opts, scout.WithBlockRules(rules...))
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

	// Start recording if requested. Either the legacy 'record' field
	// or the new 'record_har' field enables the network recorder.
	if req.GetRecord() || req.GetRecordHar() {
		recOpts := []scout.RecorderOption{}
		if req.GetCaptureBody() || req.GetHijackBodies() {
			recOpts = append(recOpts, scout.WithCaptureBody(true))
		}

		sess.recorder = scout.NewNetworkRecorder(page, recOpts...)
	}

	// Console + WS sidecar writers run as goroutines that drain the
	// respective event sources to files under the engine session dir.
	// sess.monitorCancel stops them at destroy time.
	if engineID := browser.SessionID(); engineID != "" {
		var stoppers []func()
		if req.GetRecordConsole() {
			if f, err := os.OpenFile(filepath.Join(scout.SessionDir(engineID), "console.log"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600); err == nil {
				cancel := page.OnConsole(func(msg scout.ConsoleMessage) {
					_, _ = fmt.Fprintf(f, "[%s] %s\n", msg.Level, msg.Text)
				})
				stoppers = append(stoppers, func() { cancel(); _ = f.Close() })
			}
		}
		if req.GetRecordWs() {
			if msgs, stop, err := page.MonitorWebSockets(); err == nil {
				f, ferr := os.OpenFile(filepath.Join(scout.SessionDir(engineID), "ws.jsonl"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
				if ferr == nil {
					done := make(chan struct{})
					go func() {
						enc := json.NewEncoder(f)
						for m := range msgs {
							_ = enc.Encode(m)
						}
						close(done)
					}()
					stoppers = append(stoppers, func() { stop(); <-done; _ = f.Close() })
				} else {
					stop()
				}
			}
		}
		if len(stoppers) > 0 {
			sess.monitorCancel = func() {
				for _, s := range stoppers {
					s()
				}
			}
		}
	}

	s.sessions.Store(sess.id, sess)
	s.trackPeer(ctx, sess.id)

	// Persist monitors.json sidecar so 'scout session destroy' and audit
	// tooling know which artifacts to finalize even if the daemon restarts
	// between create and destroy.
	if engineID := browser.SessionID(); engineID != "" {
		harPath := "har.json"
		if p := req.GetHarOut(); p != "" {
			harPath = p
		}
		hijackPath := "hijack.jsonl"
		if p := req.GetHijackOut(); p != "" {
			hijackPath = p
		}
		cfg := &scout.SessionMonitorConfig{
			HAR: scout.MonitorSink{
				Enabled:    req.GetRecord() || req.GetRecordHar(),
				Path:       harPath,
				WithBodies: req.GetCaptureBody() || req.GetHijackBodies(),
			},
			Hijack: scout.MonitorSink{
				Enabled:    req.GetRecordHijack(),
				Path:       hijackPath,
				WithBodies: req.GetHijackBodies(),
			},
			Console: scout.MonitorSink{Enabled: req.GetRecordConsole(), Path: "console.log"},
			WS:      scout.MonitorSink{Enabled: req.GetRecordWs(), Path: "ws.jsonl"},
		}
		for _, b := range req.GetBlocks() {
			cfg.Blocks = append(cfg.Blocks, scout.SessionMonitorRule{Pattern: b.GetPattern(), Method: b.GetMethod()})
		}
		_ = scout.WriteSessionMonitors(engineID, cfg)
	}

	title, _ := page.Title()
	currentURL, _ := page.URL()

	return &pb.CreateSessionResponse{
		SessionId: sess.id,
		Url:       currentURL,
		Title:     title,
	}, nil
}

func (s *ScoutServer) DestroySession(_ context.Context, req *pb.SessionRequest) (*pb.Empty, error) {
	// Recover from any panic in teardown so a single bad session cannot
	// crash the RPC handler (mirrors DestroyAllSessions per-session guard).
	defer func() {
		if r := recover(); r != nil {
			slog.Warn("scout: DestroySession: teardown panicked",
				"session", req.GetSessionId(), "panic", r)
		}
	}()

	sess, err := s.getSession(req.GetSessionId())
	if err != nil {
		return nil, err
	}

	// Stop console + WS sidecar writers first so their files flush
	// before the browser is torn down.
	if sess.monitorCancel != nil {
		sess.monitorCancel()
		sess.monitorCancel = nil
	}

	// Flush HAR sidecar before shutting down the recorder. Reads
	// monitors.json to learn the target path (default: har.json under the
	// engine session dir). Best-effort; failures don't block destroy.
	if sess.recorder != nil {
		engineID := sess.browser.SessionID()
		if engineID != "" {
			if data, _, err := sess.recorder.ExportHAR(); err == nil && len(data) > 0 {
				cfg, _ := scout.ReadSessionMonitors(engineID)
				outPath := scout.DefaultHARPath(engineID)
				if cfg != nil && cfg.HAR.Path != "" {
					outPath = filepath.Join(scout.SessionDir(engineID), cfg.HAR.Path)
				}
				if werr := os.WriteFile(outPath, data, 0o600); werr != nil {
					slog.Warn("scout: HAR flush on destroy failed", "path", outPath, "err", werr)
				}
			}
		}
		sess.recorder.Stop()
	}

	if sess.browser != nil {
		_ = sess.browser.Close()
	}

	s.sessions.Delete(req.GetSessionId())
	s.untrackPeer(req.GetSessionId())

	return &pb.Empty{}, nil
}

// DestroyAllSessions tears down every in-flight session: it stops monitor
// sidecars, flushes the HAR artifact, stops the recorder and hijacker,
// closes the browser, untracks the peer, and deletes the session from the
// map. Each session's teardown runs under its own recover() so a single
// panicking session cannot abort the sweep. Used by daemon idle/shutdown
// paths so no session is leaked when the server stops.
func (s *ScoutServer) DestroyAllSessions() {
	s.sessions.Range(func(key, value any) bool {
		func() {
			// Always delete the session from the map, even if teardown panics.
			defer s.sessions.Delete(key)

			defer func() {
				if r := recover(); r != nil {
					slog.Warn("scout: destroy all: session teardown panicked", "session", key, "panic", r)
				}
			}()

			sess, ok := value.(*session)
			if !ok || sess == nil {
				return
			}

			// Stop console + WS sidecar writers first so their files flush
			// before the browser is torn down.
			if sess.monitorCancel != nil {
				sess.monitorCancel()
				sess.monitorCancel = nil
			}

			// Flush HAR sidecar before stopping the recorder. Mirrors
			// DestroySession: reads monitors.json for the target path,
			// defaults to DefaultHARPath. Best-effort; never blocks teardown.
			if sess.recorder != nil {
				if sess.browser != nil {
					if engineID := sess.browser.SessionID(); engineID != "" {
						if data, _, err := sess.recorder.ExportHAR(); err == nil && len(data) > 0 {
							cfg, _ := scout.ReadSessionMonitors(engineID)
							outPath := scout.DefaultHARPath(engineID)
							if cfg != nil && cfg.HAR.Path != "" {
								outPath = filepath.Join(scout.SessionDir(engineID), cfg.HAR.Path)
							}
							if werr := os.WriteFile(outPath, data, 0o600); werr != nil {
								slog.Warn("scout: HAR flush on destroy-all failed", "path", outPath, "err", werr)
							}
						}
					}
				}
				sess.recorder.Stop()
			}

			// Stop the hijack fan-out goroutine if one is active.
			if sess.hijacker != nil {
				sess.hijacker.Stop()
				sess.hijacker = nil
			}

			_ = sess.browser.Close()

			s.untrackPeer(sess.id)
		}()

		return true
	})
}

// ════════════════════════ Navigation ════════════════════════

func (s *ScoutServer) Navigate(_ context.Context, req *pb.NavigateRequest) (*pb.NavigateResponse, error) {
	s.touchIdle()

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
	s.touchIdle()

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
	s.touchIdle()

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
	s.touchIdle()

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
	s.touchIdle()

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

func (s *ScoutServer) InjectJS(_ context.Context, req *pb.InjectJSRequest) (*pb.InjectJSResponse, error) {
	sess, err := s.getSession(req.GetSessionId())
	if err != nil {
		return nil, err
	}

	var script string
	if code := req.GetCode(); code != "" {
		script = code
	} else if tmplName := req.GetTemplateName(); tmplName != "" {
		tmpl, ok := scout.BuiltinTemplates[tmplName]
		if !ok {
			return &pb.InjectJSResponse{Error: fmt.Sprintf("unknown template %q", tmplName)}, nil
		}

		var data map[string]any
		if td := req.GetTemplateData(); td != "" {
			if err := json.Unmarshal([]byte(td), &data); err != nil {
				return &pb.InjectJSResponse{Error: fmt.Sprintf("invalid template_data JSON: %v", err)}, nil
			}
		}

		rendered, err := scout.RenderTemplate(tmpl, data)
		if err != nil {
			return &pb.InjectJSResponse{Error: err.Error()}, nil //nolint:nilerr
		}

		script = rendered
	} else {
		return &pb.InjectJSResponse{Error: "code or template_name required"}, nil
	}

	result, err := sess.page.Eval(script)
	if err != nil {
		return &pb.InjectJSResponse{Error: fmt.Sprintf("eval failed: %v", sanitizeError(err))}, nil
	}

	resultData, err2 := json.Marshal(result) //nolint:musttag // result is dynamic eval output
	if err2 != nil {
		return &pb.InjectJSResponse{Error: fmt.Sprintf("marshal result failed: %v", err2)}, nil
	}

	return &pb.InjectJSResponse{Result: string(resultData)}, nil
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
	s.touchIdle()

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

func (s *ScoutServer) StartHijack(_ context.Context, req *pb.HijackRequest) (*pb.Empty, error) {
	sess, err := s.getSession(req.GetSessionId())
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

func (s *ScoutServer) StopHijack(_ context.Context, req *pb.SessionRequest) (*pb.Empty, error) {
	sess, err := s.getSession(req.GetSessionId())
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
	sess, err := s.getSession(req.GetSessionId())
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

func (s *ScoutServer) CaptureProfile(_ context.Context, req *pb.CaptureProfileRequest) (*pb.CaptureProfileResponse, error) {
	sess, err := s.getSession(req.GetSessionId())
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

func (s *ScoutServer) LoadProfile(_ context.Context, req *pb.LoadProfileRequest) (*pb.LoadProfileResponse, error) {
	sess, err := s.getSession(req.GetSessionId())
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
