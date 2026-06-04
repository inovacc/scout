package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	pb "github.com/inovacc/scout/grpc/scoutpb"
	"github.com/inovacc/scout/pkg/scout"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// sanitizeSessionRel validates a caller-supplied artifact output path
// (HarOut/HijackOut on CreateSession). These are joined under the per-session
// directory at flush time, so only a bare filename is acceptable — an absolute
// path or a `..` segment would let a gRPC client write the HAR/hijack artifact
// to an arbitrary location on the daemon host. Empty is allowed (the caller
// falls back to the default name). Returns InvalidArgument for anything that is
// not a single, non-traversing filename. Cross-platform: rejects both `/` and
// `\` separators regardless of the daemon's OS.
func sanitizeSessionRel(p string) (string, error) {
	if p == "" {
		return "", nil
	}

	if p == "." || p == ".." ||
		filepath.IsAbs(p) ||
		strings.ContainsAny(p, `/\`) ||
		strings.Contains(p, "..") {
		return "", status.Errorf(codes.InvalidArgument,
			"scout: invalid output path %q: a bare filename is required (no path separators, no '..', not absolute)", p)
	}

	return p, nil
}

// ════════════════════════ Session Lifecycle ════════════════════════

func (s *ScoutServer) CreateSession(ctx context.Context, req *pb.CreateSessionRequest) (*pb.CreateSessionResponse, error) {
	s.touchIdle()

	// Validate caller-supplied artifact output paths up front — before any
	// browser is launched or session stored — so a path-traversal request is
	// rejected cleanly with no side effects. These names are joined under the
	// session dir at HAR/hijack flush time.
	harPath, perr := sanitizeSessionRel(req.GetHarOut())
	if perr != nil {
		return nil, perr
	}
	if harPath == "" {
		harPath = "har.json"
	}

	hijackPath, perr := sanitizeSessionRel(req.GetHijackOut())
	if perr != nil {
		return nil, perr
	}
	if hijackPath == "" {
		hijackPath = "hijack.jsonl"
	}

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
		// harPath / hijackPath were validated + defaulted at the top of
		// CreateSession (path-traversal rejected there).
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

func (s *ScoutServer) DestroySession(ctx context.Context, req *pb.SessionRequest) (*pb.Empty, error) {
	// Recover from any panic in teardown so a single bad session cannot
	// crash the RPC handler (mirrors DestroyAllSessions per-session guard).
	defer func() {
		if r := recover(); r != nil {
			slog.Warn("scout: DestroySession: teardown panicked",
				"session", req.GetSessionId(), "panic", r)
		}
	}()

	sess, err := s.getSession(ctx, req.GetSessionId())
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
				// Re-validate the persisted path defensively: monitors.json lives
				// on disk and could be tampered. Fall back to the default name on
				// anything that is not a bare filename.
				if cfg != nil {
					if rel, perr := sanitizeSessionRel(cfg.HAR.Path); perr == nil && rel != "" {
						outPath = filepath.Join(scout.SessionDir(engineID), rel)
					}
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
							// Defensive re-validation (see DestroySession).
							if cfg != nil {
								if rel, perr := sanitizeSessionRel(cfg.HAR.Path); perr == nil && rel != "" {
									outPath = filepath.Join(scout.SessionDir(engineID), rel)
								}
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
