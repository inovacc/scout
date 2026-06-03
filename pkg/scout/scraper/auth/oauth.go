package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"sync"
)

// OAuthResult holds the outcome of an OAuth2 callback.
type OAuthResult struct {
	Code  string
	State string
	Error string
}

// OAuthServer runs a local HTTP server to receive OAuth2 callbacks.
type OAuthServer struct {
	listener net.Listener
	server   *http.Server
	result   chan OAuthResult
	state    string
	once     sync.Once
}

// NewOAuthServer starts a local callback server on a random port.
func NewOAuthServer() (*OAuthServer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("auth: oauth: listen: %w", err)
	}

	state, err := newState()
	if err != nil {
		_ = listener.Close()
		return nil, err
	}

	s := &OAuthServer{
		listener: listener,
		result:   make(chan OAuthResult, 1),
		state:    state,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", s.handleCallback)

	s.server = &http.Server{Handler: mux} //nolint:gosec

	go func() { _ = s.server.Serve(listener) }()

	return s, nil
}

// CallbackURL returns the full callback URL (e.g. http://127.0.0.1:12345/callback).
func (s *OAuthServer) CallbackURL() string {
	return fmt.Sprintf("http://%s/callback", s.listener.Addr().String())
}

// Wait blocks until a callback is received or the context expires.
func (s *OAuthServer) Wait(ctx context.Context) (OAuthResult, error) {
	select {
	case r := <-s.result:
		return r, nil
	case <-ctx.Done():
		return OAuthResult{}, ctx.Err()
	}
}

// Close shuts down the callback server.
func (s *OAuthServer) Close() {
	s.once.Do(func() {
		_ = s.server.Close()
	})
}

// State returns the random CSRF state the caller MUST include as the `state`
// parameter in the authorization URL. The callback handler rejects any
// callback whose state does not match.
func (s *OAuthServer) State() string { return s.state }

func (s *OAuthServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	// Reject callbacks whose state does not match the one we issued (CSRF /
	// authorization-code-injection protection). Providers echo state even on
	// error, so a missing/wrong state is always rejected.
	gotState := q.Get("state")
	if subtle.ConstantTimeCompare([]byte(gotState), []byte(s.state)) != 1 {
		http.Error(w, "state mismatch", http.StatusBadRequest)
		select {
		case s.result <- OAuthResult{Error: "state mismatch"}:
		default:
		}
		return
	}

	result := OAuthResult{
		Code:  q.Get("code"),
		State: gotState,
		Error: q.Get("error"),
	}

	_, _ = fmt.Fprintf(w, "<html><body><h2>Authentication complete</h2><p>You can close this window.</p></body></html>")

	select {
	case s.result <- result:
	default:
	}
}

// newState generates a random CSRF state token.
func newState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("auth: oauth: generate state: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}

// PKCEChallenge generates a PKCE code verifier and challenge pair.
func PKCEChallenge() (verifier, challenge string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("auth: pkce: generate verifier: %w", err)
	}

	verifier = base64.RawURLEncoding.EncodeToString(b)

	h := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(h[:])

	return verifier, challenge, nil
}
