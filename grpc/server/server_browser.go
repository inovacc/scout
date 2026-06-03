package server

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	pb "github.com/inovacc/scout/grpc/scoutpb"
	"github.com/inovacc/scout/pkg/scout"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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
