package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
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
	once     sync.Once
}

// NewOAuthServer starts a local callback server on a random port.
func NewOAuthServer() (*OAuthServer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("auth: oauth: listen: %w", err)
	}

	s := &OAuthServer{
		listener: listener,
		result:   make(chan OAuthResult, 1),
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

func (s *OAuthServer) handleCallback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	result := OAuthResult{
		Code:  q.Get("code"),
		State: q.Get("state"),
		Error: q.Get("error"),
	}

	_, _ = fmt.Fprintf(w, "<html><body><h2>Authentication complete</h2><p>You can close this window.</p></body></html>")

	select {
	case s.result <- result:
	default:
	}
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
