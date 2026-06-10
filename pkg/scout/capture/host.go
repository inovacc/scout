package capture

import (
	"encoding/json"
	"errors"
	"io"
)

// hostVersion is reported in hello_ack.
const hostVersion = "1.0.0"

// maxCapturesPerConn bounds how many captures a single native-messaging
// connection may spool, so a paired extension cannot fill the disk unboundedly.
const maxCapturesPerConn = 10000

// HostConfig configures one RunHost session.
type HostConfig struct {
	Pub          *[32]byte
	SpoolDir     string
	AllowedExtID string
	NoncePath    string
}

func (c HostConfig) nonceOK(got string) bool {
	return VerifyNonce(c.NoncePath, got)
}

// RunHost reads length-prefixed frames from r until EOF, validating and spooling
// captures, writing ack/error frames to w. It returns nil on clean EOF; a
// transport/encode error otherwise. It NEVER writes secret values back to w.
func RunHost(r io.Reader, w io.Writer, cfg HostConfig) error {
	paired := false
	captures := 0
	for {
		m, err := ReadFrame(r)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			// Malformed/oversized frame: report and stop (stream is desynced).
			_ = WriteFrame(w, Msg{V: 1, Type: "error", Code: "bad_frame", Message: "malformed frame"})
			return nil
		}
		if verr := Validate(m); verr != nil {
			if werr := WriteFrame(w, Msg{V: 1, Type: "error", Code: "invalid", Message: verr.Error()}); werr != nil {
				return werr
			}
			continue
		}
		switch m.Type {
		case "hello":
			if m.ExtID != cfg.AllowedExtID || !cfg.nonceOK(m.Nonce) {
				if werr := WriteFrame(w, Msg{V: 1, Type: "error", Code: "unauthorized", Message: "origin/nonce rejected"}); werr != nil {
					return werr
				}
				continue
			}
			paired = true
			if werr := WriteFrame(w, Msg{V: 1, Type: "hello_ack", HostVersion: hostVersion}); werr != nil {
				return werr
			}
		case "capture_session", "capture_login":
			if !paired {
				if werr := WriteFrame(w, Msg{V: 1, Type: "error", Code: "not_paired", Message: "send hello first"}); werr != nil {
					return werr
				}
				continue
			}
			captures++
			if captures > maxCapturesPerConn {
				_ = WriteFrame(w, Msg{V: 1, Type: "error", Code: "rate_limited", Message: "capture limit reached for this connection"})
				return nil
			}
			payload, _ := json.Marshal(m) // re-marshal the validated message as the spool record
			id, serr := WriteSpool(cfg.SpoolDir, cfg.Pub, payload)
			if serr != nil {
				if werr := WriteFrame(w, Msg{V: 1, Type: "error", Code: "spool", Message: "could not store capture"}); werr != nil {
					return werr
				}
				continue
			}
			if werr := WriteFrame(w, Msg{V: 1, Type: "ack", ID: id}); werr != nil {
				return werr
			}
		}
	}
}
