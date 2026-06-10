package vpn

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestProvider returns a SurfsharkProvider whose APIBase points at the given
// httptest server and whose HTTPClient is the server's client (no real network).
func newTestProvider(srv *httptest.Server) *SurfsharkProvider {
	sp := NewSurfsharkProvider("user@example.com", "secret")
	sp.APIBase = srv.URL
	sp.HTTPClient = srv.Client()

	return sp
}

// --- Authenticate ---

func TestAuthenticateSuccess(t *testing.T) {
	var gotMethod, gotPath, gotContentType, gotBody string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		gotBody = string(buf)

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"token":"jwt-token","renewToken":"renew-token"}`))
	}))
	defer srv.Close()

	sp := newTestProvider(srv)

	if err := sp.Authenticate(context.Background()); err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}

	if sp.Token != "jwt-token" {
		t.Errorf("Token = %q, want jwt-token", sp.Token)
	}
	if sp.RenewToken != "renew-token" {
		t.Errorf("RenewToken = %q, want renew-token", sp.RenewToken)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotPath != SurfsharkLoginPath {
		t.Errorf("path = %q, want %q", gotPath, SurfsharkLoginPath)
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotContentType)
	}
	if !strings.Contains(gotBody, `"email":"user@example.com"`) || !strings.Contains(gotBody, `"password":"secret"`) {
		t.Errorf("request body = %q, missing credentials", gotBody)
	}
}

func TestAuthenticateErrors(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantSub string
	}{
		{
			name:    "non-200 status",
			status:  http.StatusUnauthorized,
			body:    `{"error":"bad credentials"}`,
			wantSub: "login failed: status 401",
		},
		{
			name:    "invalid json",
			status:  http.StatusOK,
			body:    `not json`,
			wantSub: "parse login response",
		},
		{
			name:    "empty token",
			status:  http.StatusOK,
			body:    `{"token":"","renewToken":"r"}`,
			wantSub: "empty token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			sp := newTestProvider(srv)

			err := sp.Authenticate(context.Background())
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("error = %q, want substring %q", err.Error(), tt.wantSub)
			}
		})
	}
}

func TestAuthenticateRequestError(t *testing.T) {
	// Server closed immediately -> HTTPClient.Do returns a transport error.
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	client := srv.Client()
	url := srv.URL
	srv.Close()

	sp := NewSurfsharkProvider("e", "p")
	sp.APIBase = url
	sp.HTTPClient = client

	err := sp.Authenticate(context.Background())
	if err == nil {
		t.Fatal("expected transport error after server closed")
	}
	if !strings.Contains(err.Error(), "login request") {
		t.Errorf("error = %q, want substring 'login request'", err.Error())
	}
}

// --- EnsureAuthenticated (network path) ---

func TestEnsureAuthenticatedFetchesToken(t *testing.T) {
	var calls int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != SurfsharkLoginPath {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"token":"fresh-token"}`))
	}))
	defer srv.Close()

	sp := newTestProvider(srv)

	if err := sp.EnsureAuthenticated(context.Background()); err != nil {
		t.Fatalf("EnsureAuthenticated() error = %v", err)
	}
	if sp.Token != "fresh-token" {
		t.Errorf("Token = %q, want fresh-token", sp.Token)
	}
	if calls != 1 {
		t.Errorf("login called %d times, want 1", calls)
	}

	// Second call should be a no-op (token present, no extra HTTP call).
	if err := sp.EnsureAuthenticated(context.Background()); err != nil {
		t.Fatalf("second EnsureAuthenticated() error = %v", err)
	}
	if calls != 1 {
		t.Errorf("login called %d times after re-auth, want 1", calls)
	}
}

func TestEnsureAuthenticatedPropagatesError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`boom`))
	}))
	defer srv.Close()

	sp := newTestProvider(srv)

	err := sp.EnsureAuthenticated(context.Background())
	if err == nil {
		t.Fatal("expected error from failed authentication")
	}
	if !strings.Contains(err.Error(), "status 500") {
		t.Errorf("error = %q, want substring 'status 500'", err.Error())
	}
}

// --- FetchProxyCredentials ---

func TestFetchProxyCredentialsSuccess(t *testing.T) {
	var gotAuth, gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`{"username":"px_user","password":"px_pass"}`))
	}))
	defer srv.Close()

	sp := newTestProvider(srv)
	sp.Token = "bearer-token"

	if err := sp.FetchProxyCredentials(context.Background()); err != nil {
		t.Fatalf("FetchProxyCredentials() error = %v", err)
	}

	if sp.ProxyUser != "px_user" || sp.ProxyPass != "px_pass" {
		t.Errorf("creds = %q:%q, want px_user:px_pass", sp.ProxyUser, sp.ProxyPass)
	}
	if gotAuth != "Bearer bearer-token" {
		t.Errorf("Authorization = %q, want 'Bearer bearer-token'", gotAuth)
	}
	if gotPath != SurfsharkUserPath {
		t.Errorf("path = %q, want %q", gotPath, SurfsharkUserPath)
	}
}

func TestFetchProxyCredentialsErrors(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantSub string
	}{
		{
			name:    "non-200 status",
			status:  http.StatusForbidden,
			body:    `nope`,
			wantSub: "fetch proxy creds: status 403",
		},
		{
			name:    "invalid json",
			status:  http.StatusOK,
			body:    `{not-json`,
			wantSub: "parse proxy creds",
		},
		{
			name:    "empty username",
			status:  http.StatusOK,
			body:    `{"username":"","password":"x"}`,
			wantSub: "proxy credentials empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			sp := newTestProvider(srv)
			sp.Token = "t"

			err := sp.FetchProxyCredentials(context.Background())
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("error = %q, want substring %q", err.Error(), tt.wantSub)
			}
		})
	}
}

func TestFetchProxyCredentialsCachedSkipsHTTP(t *testing.T) {
	var calls int

	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		calls++
	}))
	defer srv.Close()

	sp := newTestProvider(srv)
	sp.ProxyUser = "already"

	if err := sp.FetchProxyCredentials(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 0 {
		t.Errorf("expected no HTTP call when creds cached, got %d", calls)
	}
}

// --- Servers ---

const clustersJSON = `[
	{"connectionName":"us-nyc.surfshark.com","country":"US","location":"New York","load":42.7,"tags":["p2p"]},
	{"connectionName":"de-ber.surfshark.com","country":"DE","location":"Berlin","load":10.0,"tags":["static"]}
]`

func TestServersSuccess(t *testing.T) {
	var loginCalls, serverCalls int
	var serverAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case SurfsharkLoginPath:
			loginCalls++
			_, _ = w.Write([]byte(`{"token":"tok"}`))
		case SurfsharkServersPath:
			serverCalls++
			serverAuth = r.Header.Get("Authorization")
			_, _ = w.Write([]byte(clustersJSON))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	sp := newTestProvider(srv)

	servers, err := sp.Servers(context.Background())
	if err != nil {
		t.Fatalf("Servers() error = %v", err)
	}
	if len(servers) != 2 {
		t.Fatalf("got %d servers, want 2", len(servers))
	}
	if servers[0].Host != "us-nyc.surfshark.com" || servers[0].Country != "us" {
		t.Errorf("server[0] = %+v", servers[0])
	}
	if servers[0].Load != 42 {
		t.Errorf("Load = %d, want 42 (truncated)", servers[0].Load)
	}
	if serverAuth != "Bearer tok" {
		t.Errorf("Authorization = %q, want 'Bearer tok'", serverAuth)
	}
	if loginCalls != 1 {
		t.Errorf("login called %d times, want 1 (auto-auth)", loginCalls)
	}

	// Cached: a second call must not hit the network again.
	cached, err := sp.Servers(context.Background())
	if err != nil {
		t.Fatalf("cached Servers() error = %v", err)
	}
	if len(cached) != 2 {
		t.Errorf("cached len = %d, want 2", len(cached))
	}
	if serverCalls != 1 {
		t.Errorf("server endpoint called %d times, want 1 (cached)", serverCalls)
	}
}

func TestServersAuthFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == SurfsharkLoginPath {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`denied`))
			return
		}
		t.Errorf("servers endpoint should not be reached, path=%q", r.URL.Path)
	}))
	defer srv.Close()

	sp := newTestProvider(srv)

	_, err := sp.Servers(context.Background())
	if err == nil {
		t.Fatal("expected error when authentication fails")
	}
	if !strings.Contains(err.Error(), "status 401") {
		t.Errorf("error = %q, want substring 'status 401'", err.Error())
	}
}

func TestServersFetchErrors(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantSub string
	}{
		{
			name:    "non-200 status",
			status:  http.StatusBadGateway,
			body:    `upstream down`,
			wantSub: "fetch servers: status 502",
		},
		{
			name:    "invalid json",
			status:  http.StatusOK,
			body:    `[ broken`,
			wantSub: "parse servers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == SurfsharkServersPath {
					w.WriteHeader(tt.status)
					_, _ = w.Write([]byte(tt.body))
					return
				}
				_, _ = w.Write([]byte(`{"token":"tok"}`))
			}))
			defer srv.Close()

			sp := newTestProvider(srv)

			_, err := sp.Servers(context.Background())
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("error = %q, want substring %q", err.Error(), tt.wantSub)
			}
		})
	}
}

// --- Connect ---

func TestConnectSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case SurfsharkLoginPath:
			_, _ = w.Write([]byte(`{"token":"tok"}`))
		case SurfsharkUserPath:
			_, _ = w.Write([]byte(`{"username":"pu","password":"pp"}`))
		case SurfsharkServersPath:
			_, _ = w.Write([]byte(clustersJSON))
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	sp := newTestProvider(srv)

	conn, err := sp.Connect(context.Background(), "DE")
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if conn == nil {
		t.Fatal("expected connection, got nil")
	}
	if conn.Server.Country != "de" {
		t.Errorf("Country = %q, want de", conn.Server.Country)
	}
	if conn.Server.Host != "de-ber.surfshark.com" {
		t.Errorf("Host = %q, want de-ber.surfshark.com", conn.Server.Host)
	}
	if conn.Protocol != "https" || conn.Port != 443 {
		t.Errorf("conn = %+v, want https:443", conn)
	}

	// Provider records the active connection + proxy creds.
	if sp.Connected == nil {
		t.Error("Connected should be set after Connect")
	}
	u, p := sp.ProxyCredentials()
	if u != "pu" || p != "pp" {
		t.Errorf("proxy creds = %q:%q, want pu:pp", u, p)
	}

	st, err := sp.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !st.Connected {
		t.Error("Status should report connected after Connect")
	}
}

func TestConnectNoServerForCountry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case SurfsharkLoginPath:
			_, _ = w.Write([]byte(`{"token":"tok"}`))
		case SurfsharkUserPath:
			_, _ = w.Write([]byte(`{"username":"pu","password":"pp"}`))
		case SurfsharkServersPath:
			_, _ = w.Write([]byte(clustersJSON))
		}
	}))
	defer srv.Close()

	sp := newTestProvider(srv)

	_, err := sp.Connect(context.Background(), "jp")
	if err == nil {
		t.Fatal("expected error for country with no servers")
	}
	if !strings.Contains(err.Error(), "no servers found") {
		t.Errorf("error = %q, want substring 'no servers found'", err.Error())
	}
	if sp.Connected != nil {
		t.Error("Connected should remain nil when server selection fails")
	}
}

func TestConnectProxyCredsFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case SurfsharkLoginPath:
			_, _ = w.Write([]byte(`{"token":"tok"}`))
		case SurfsharkUserPath:
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`denied`))
		default:
			t.Errorf("path %q should not be reached after creds failure", r.URL.Path)
		}
	}))
	defer srv.Close()

	sp := newTestProvider(srv)

	_, err := sp.Connect(context.Background(), "de")
	if err == nil {
		t.Fatal("expected error when proxy credential fetch fails")
	}
	if !strings.Contains(err.Error(), "fetch proxy creds") {
		t.Errorf("error = %q, want substring 'fetch proxy creds'", err.Error())
	}
}

func TestConnectUsesPreloadedServers(t *testing.T) {
	// When Servers_ is already populated, Connect should not call the servers
	// endpoint again.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case SurfsharkUserPath:
			_, _ = w.Write([]byte(`{"username":"pu","password":"pp"}`))
		case SurfsharkServersPath:
			t.Error("servers endpoint should not be called when preloaded")
		default:
			t.Errorf("unexpected path %q", r.URL.Path)
		}
	}))
	defer srv.Close()

	sp := newTestProvider(srv)
	sp.Token = "preset" // skip auth
	sp.Servers_ = []Server{
		{Host: "br1.test", Country: "br", Load: 30},
		{Host: "br2.test", Country: "br", Load: 5},
	}

	conn, err := sp.Connect(context.Background(), "br")
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if conn.Server.Host != "br2.test" {
		t.Errorf("Host = %q, want br2.test (lowest load)", conn.Server.Host)
	}
}
