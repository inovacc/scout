package engine

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSurfsharkProvider_Name(t *testing.T) {
	p := NewSurfsharkProvider("test@example.com", "pass")
	if got := p.Name(); got != "surfshark" {
		t.Errorf("Name() = %q, want %q", got, "surfshark")
	}
}

func TestSurfsharkProvider_NewProvider(t *testing.T) {
	p := NewSurfsharkProvider("a@b.com", "secret")
	if p == nil {
		t.Fatal("NewSurfsharkProvider returned nil")
	}

	if p.Email != "a@b.com" {
		t.Errorf("email = %q, want %q", p.Email, "a@b.com")
	}

	if p.Passwd != "secret" {
		t.Errorf("password = %q, want %q", p.Passwd, "secret")
	}

	if p.HTTPClient == nil {
		t.Error("httpClient is nil")
	}

	if p.APIBase != surfsharkAPIBase {
		t.Errorf("apiBase = %q, want %q", p.APIBase, surfsharkAPIBase)
	}
}

func TestSurfsharkProvider_ParseServers(t *testing.T) {
	raw := `[
		{"connectionName": "us-nyc.prod.surfshark.com", "country": "US", "location": "New York", "load": 42.5, "tags": ["p2p"]},
		{"connectionName": "de-ber.prod.surfshark.com", "country": "DE", "location": "Berlin", "load": 15.0, "tags": []},
		{"connectionName": "us-lax.prod.surfshark.com", "country": "US", "location": "Los Angeles", "load": 80.0, "tags": ["static"]}
	]`

	servers, err := ParseSurfsharkClusters([]byte(raw))
	if err != nil {
		t.Fatalf("ParseSurfsharkClusters() error: %v", err)
	}

	if len(servers) != 3 {
		t.Fatalf("got %d servers, want 3", len(servers))
	}

	tests := []struct {
		idx     int
		host    string
		country string
		city    string
		load    int
	}{
		{0, "us-nyc.prod.surfshark.com", "us", "New York", 42},
		{1, "de-ber.prod.surfshark.com", "de", "Berlin", 15},
		{2, "us-lax.prod.surfshark.com", "us", "Los Angeles", 80},
	}

	for _, tt := range tests {
		s := servers[tt.idx]
		if s.Host != tt.host {
			t.Errorf("[%d] Host = %q, want %q", tt.idx, s.Host, tt.host)
		}

		if s.Country != tt.country {
			t.Errorf("[%d] Country = %q, want %q", tt.idx, s.Country, tt.country)
		}

		if s.City != tt.city {
			t.Errorf("[%d] City = %q, want %q", tt.idx, s.City, tt.city)
		}

		if s.Load != tt.load {
			t.Errorf("[%d] Load = %d, want %d", tt.idx, s.Load, tt.load)
		}
	}
}

func TestSurfsharkProvider_ParseServers_Invalid(t *testing.T) {
	_, err := ParseSurfsharkClusters([]byte(`not json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestSurfsharkProvider_FilterByCountry(t *testing.T) {
	servers := []VPNServer{
		{Host: "us-nyc", Country: "us", City: "New York"},
		{Host: "de-ber", Country: "de", City: "Berlin"},
		{Host: "us-lax", Country: "us", City: "Los Angeles"},
		{Host: "gb-lon", Country: "gb", City: "London"},
	}

	tests := []struct {
		country string
		want    int
	}{
		{"us", 2},
		{"US", 2},
		{"de", 1},
		{"gb", 1},
		{"jp", 0},
	}

	for _, tt := range tests {
		got := FilterServersByCountry(servers, tt.country)
		if len(got) != tt.want {
			t.Errorf("FilterServersByCountry(%q) = %d servers, want %d", tt.country, len(got), tt.want)
		}
	}
}

func TestSurfsharkProvider_Authenticate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != surfsharkLoginPath {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		var req surfsharkLoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		if req.Email != "test@example.com" || req.Password != "hunter2" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"invalid credentials"}`))

			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token":"jwt-token-123","renewToken":"renew-456"}`))
	}))
	defer srv.Close()

	p := NewSurfsharkProvider("test@example.com", "hunter2")
	p.APIBase = srv.URL

	if err := p.Authenticate(context.Background()); err != nil {
		t.Fatalf("authenticate() error: %v", err)
	}

	if p.Token != "jwt-token-123" {
		t.Errorf("token = %q, want %q", p.Token, "jwt-token-123")
	}

	if p.RenewToken != "renew-456" {
		t.Errorf("renewToken = %q, want %q", p.RenewToken, "renew-456")
	}
}

func TestSurfsharkProvider_Authenticate_BadCredentials(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"invalid credentials"}`))
	}))
	defer srv.Close()

	p := NewSurfsharkProvider("bad@example.com", "wrong")
	p.APIBase = srv.URL

	if err := p.Authenticate(context.Background()); err == nil {
		t.Error("expected error for bad credentials")
	}
}

func TestSurfsharkProvider_FetchCredentials(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != surfsharkUserPath {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		if r.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"username":"proxy-user","password":"proxy-pass"}`))
	}))
	defer srv.Close()

	p := NewSurfsharkProvider("a@b.com", "pass")
	p.APIBase = srv.URL
	p.Token = "test-token"

	if err := p.FetchProxyCredentials(context.Background()); err != nil {
		t.Fatalf("fetchProxyCredentials() error: %v", err)
	}

	if p.ProxyUser != "proxy-user" {
		t.Errorf("proxyUser = %q, want %q", p.ProxyUser, "proxy-user")
	}

	if p.ProxyPass != "proxy-pass" {
		t.Errorf("proxyPass = %q, want %q", p.ProxyPass, "proxy-pass")
	}

	// Second call should be cached (no-op).
	if err := p.FetchProxyCredentials(context.Background()); err != nil {
		t.Fatalf("second fetchProxyCredentials() error: %v", err)
	}
}

func TestSurfsharkProvider_FetchServers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case surfsharkLoginPath:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"token":"tok","renewToken":"ren"}`))
		case surfsharkServersPath:
			if r.Header.Get("Authorization") != "Bearer tok" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[
				{"connectionName":"us-nyc.prod.surfshark.com","country":"US","location":"New York","load":30,"tags":["p2p"]},
				{"connectionName":"de-ber.prod.surfshark.com","country":"DE","location":"Berlin","load":10,"tags":[]}
			]`))
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := NewSurfsharkProvider("a@b.com", "pass")
	p.APIBase = srv.URL

	servers, err := p.Servers(context.Background())
	if err != nil {
		t.Fatalf("Servers() error: %v", err)
	}

	if len(servers) != 2 {
		t.Fatalf("got %d servers, want 2", len(servers))
	}

	if servers[0].Host != "us-nyc.prod.surfshark.com" {
		t.Errorf("servers[0].Host = %q", servers[0].Host)
	}

	// Second call should return cached.
	servers2, err := p.Servers(context.Background())
	if err != nil {
		t.Fatalf("cached Servers() error: %v", err)
	}

	if len(servers2) != 2 {
		t.Errorf("cached servers count = %d, want 2", len(servers2))
	}
}

func TestSurfsharkProvider_ConnectFlow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case surfsharkLoginPath:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"token":"tok","renewToken":"ren"}`))
		case surfsharkUserPath:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"username":"puser","password":"ppass"}`))
		case surfsharkServersPath:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[
				{"connectionName":"us-nyc.prod.surfshark.com","country":"US","location":"New York","load":50,"tags":[]},
				{"connectionName":"us-lax.prod.surfshark.com","country":"US","location":"Los Angeles","load":20,"tags":[]},
				{"connectionName":"de-ber.prod.surfshark.com","country":"DE","location":"Berlin","load":10,"tags":[]}
			]`))
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := NewSurfsharkProvider("a@b.com", "pass")
	p.APIBase = srv.URL

	ctx := context.Background()

	// Connect to US -- should pick lowest load (Los Angeles, 20).
	conn, err := p.Connect(ctx, "us")
	if err != nil {
		t.Fatalf("Connect() error: %v", err)
	}

	if conn.Server.Host != "us-lax.prod.surfshark.com" {
		t.Errorf("connected to %q, want us-lax (lowest load)", conn.Server.Host)
	}

	if conn.Protocol != "https" {
		t.Errorf("Protocol = %q, want %q", conn.Protocol, "https")
	}

	if conn.Port != 443 {
		t.Errorf("Port = %d, want 443", conn.Port)
	}

	// Check proxy credentials.
	user, pass := p.ProxyCredentials()
	if user != "puser" || pass != "ppass" {
		t.Errorf("ProxyCredentials() = (%q, %q), want (%q, %q)", user, pass, "puser", "ppass")
	}

	// Check status.
	status, err := p.Status(ctx)
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}

	if !status.Connected {
		t.Error("Status().Connected = false, want true")
	}

	// Disconnect.
	if err := p.Disconnect(ctx); err != nil {
		t.Fatalf("Disconnect() error: %v", err)
	}

	status, err = p.Status(ctx)
	if err != nil {
		t.Fatalf("Status() after disconnect error: %v", err)
	}

	if status.Connected {
		t.Error("Status().Connected = true after disconnect, want false")
	}
}

func TestSurfsharkProvider_Connect_NoServers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case surfsharkLoginPath:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"token":"tok","renewToken":"ren"}`))
		case surfsharkUserPath:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"username":"u","password":"p"}`))
		case surfsharkServersPath:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"connectionName":"de-ber.prod.surfshark.com","country":"DE","location":"Berlin","load":10,"tags":[]}]`))
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer srv.Close()

	p := NewSurfsharkProvider("a@b.com", "pass")
	p.APIBase = srv.URL

	_, err := p.Connect(context.Background(), "jp")
	if err == nil {
		t.Error("expected error for country with no servers")
	}
}

func TestSurfsharkProvider_NilSafety(t *testing.T) {
	var p *SurfsharkProvider

	if _, err := p.Servers(context.Background()); err == nil {
		t.Error("expected error from nil Servers()")
	}

	if _, err := p.Connect(context.Background(), "us"); err == nil {
		t.Error("expected error from nil Connect()")
	}

	if err := p.Disconnect(context.Background()); err == nil {
		t.Error("expected error from nil Disconnect()")
	}

	if _, err := p.Status(context.Background()); err == nil {
		t.Error("expected error from nil Status()")
	}
}

func TestBrowser_VPNStatus_NilSafety(t *testing.T) {
	var b *Browser

	status := b.VPNStatus()
	if status.Connected {
		t.Error("nil browser should return not connected")
	}

	b = &Browser{}

	status = b.VPNStatus()
	if status.Connected {
		t.Error("browser with no vpn state should return not connected")
	}
}
