package capture

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

// maxFrame is the native-messaging per-message cap (browser limit is 1 MiB).
const maxFrame = 1 << 20

// Msg is the union of every wire message (extension <-> host). Secret-bearing
// fields (Cookies/Storage/Password) appear only on inbound capture_* messages and
// are NEVER echoed back.
type Msg struct {
	V        int                          `json:"v"`
	Type     string                       `json:"type"`
	ExtID    string                       `json:"ext_id,omitempty"`
	Nonce    string                       `json:"nonce,omitempty"`
	Site     string                       `json:"site,omitempty"`
	Cookies  []WireCookie                 `json:"cookies,omitempty"`
	Storage  map[string]WireOriginStorage `json:"storage,omitempty"`
	Username string                       `json:"username,omitempty"`
	Password string                       `json:"password,omitempty"`
	At       string                       `json:"at,omitempty"`
	// host -> ext only:
	ID          string `json:"id,omitempty"`
	HostVersion string `json:"host_version,omitempty"`
	Code        string `json:"code,omitempty"`
	Message     string `json:"message,omitempty"`
}

// WireCookie mirrors the fields the extension can supply for a cookie.
type WireCookie struct {
	Name, Value, Domain, Path string
	Secure, HTTPOnly          bool
}

// WireOriginStorage holds per-origin web storage for one origin.
type WireOriginStorage struct {
	Local   map[string]string `json:"local,omitempty"`
	Session map[string]string `json:"session,omitempty"`
}

// ReadFrame reads one length-prefixed JSON frame ([uint32 LE len][JSON]).
func ReadFrame(r io.Reader) (Msg, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return Msg{}, err // io.EOF on clean close
	}
	n := binary.LittleEndian.Uint32(hdr[:])
	if n == 0 || n > maxFrame {
		return Msg{}, fmt.Errorf("scout: capture: frame length %d out of range", n)
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(r, body); err != nil {
		return Msg{}, fmt.Errorf("scout: capture: short frame body: %w", err)
	}
	var m Msg
	dec := json.NewDecoder(jsonReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return Msg{}, fmt.Errorf("scout: capture: decode frame: %w", err)
	}
	return m, nil
}

// WriteFrame writes one length-prefixed JSON frame.
func WriteFrame(w io.Writer, m Msg) error {
	body, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("scout: capture: encode frame: %w", err)
	}
	if len(body) > maxFrame {
		return fmt.Errorf("scout: capture: outbound frame too large (%d)", len(body))
	}
	var hdr [4]byte
	binary.LittleEndian.PutUint32(hdr[:], uint32(len(body)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err = w.Write(body)
	return err
}

// Validate enforces version + type allowlist + required fields. It NEVER includes
// secret values (password/cookies/storage) in the returned error.
func Validate(m Msg) error {
	if m.V != 1 {
		return fmt.Errorf("scout: capture: unsupported version %d", m.V)
	}
	switch m.Type {
	case "hello":
		if m.ExtID == "" || m.Nonce == "" {
			return fmt.Errorf("scout: capture: hello missing ext_id/nonce")
		}
	case "capture_session":
		if m.Site == "" {
			return fmt.Errorf("scout: capture: capture_session missing site")
		}
	case "capture_login":
		if m.Site == "" || m.Username == "" || m.Password == "" {
			return fmt.Errorf("scout: capture: capture_login missing site/username/password")
		}
	default:
		return fmt.Errorf("scout: capture: unknown message type %q", m.Type)
	}
	return nil
}

func jsonReader(b []byte) io.Reader { return bytesReader(b) }
