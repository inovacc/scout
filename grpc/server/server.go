package server

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"sync"
	"time"

	pb "github.com/inovacc/scout/grpc/scoutpb"
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
