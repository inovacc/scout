package vpn

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// --- DirectProxy tests ---

func TestNewDirectProxyDefaults(t *testing.T) {
	dp := NewDirectProxy("proxy.example.com", 1080)
	if dp.Host != "proxy.example.com" {
		t.Errorf("Host = %q, want proxy.example.com", dp.Host)
	}
	if dp.ProxyPort != 1080 {
		t.Errorf("ProxyPort = %d, want 1080", dp.ProxyPort)
	}
	if dp.Scheme != "socks5" {
		t.Errorf("Scheme = %q, want socks5", dp.Scheme)
	}
	if dp.Username != "" {
		t.Errorf("Username should be empty, got %q", dp.Username)
	}
}

func TestNewDirectProxyWithOptions(t *testing.T) {
	dp := NewDirectProxy("proxy.example.com", 8080,
		WithDirectProxyScheme("https"),
		WithDirectProxyAuth("user", "pass"),
	)
	if dp.Scheme != "https" {
		t.Errorf("Scheme = %q, want https", dp.Scheme)
	}
	if dp.Username != "user" || dp.Password != "pass" {
		t.Errorf("auth = %q:%q, want user:pass", dp.Username, dp.Password)
	}
}

func TestDirectProxyName(t *testing.T) {
	dp := NewDirectProxy("host", 1080)
	if dp.Name() != "direct-proxy" {
		t.Errorf("Name() = %q, want direct-proxy", dp.Name())
	}
}

func TestDirectProxyServers(t *testing.T) {
	dp := NewDirectProxy("proxy.test", 9090)
	servers, err := dp.Servers(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 1 {
		t.Fatalf("got %d servers, want 1", len(servers))
	}
	if servers[0].Host != "proxy.test:9090" {
		t.Errorf("Host = %q, want proxy.test:9090", servers[0].Host)
	}
	if len(servers[0].Tags) != 1 || servers[0].Tags[0] != "socks5" {
		t.Errorf("Tags = %v, want [socks5]", servers[0].Tags)
	}
}

func TestDirectProxyServersNil(t *testing.T) {
	var dp *DirectProxy
	_, err := dp.Servers(context.Background())
	if err == nil {
		t.Error("expected error for nil proxy")
	}
}

func TestDirectProxyConnect(t *testing.T) {
	dp := NewDirectProxy("host", 443, WithDirectProxyScheme("https"))
	conn, err := dp.Connect(context.Background(), "us")
	if err != nil {
		t.Fatal(err)
	}
	if conn.Protocol != "https" {
		t.Errorf("Protocol = %q, want https", conn.Protocol)
	}
	if conn.Port != 443 {
		t.Errorf("Port = %d, want 443", conn.Port)
	}
	if conn.Server.Host != "host:443" {
		t.Errorf("Server.Host = %q, want host:443", conn.Server.Host)
	}
}

func TestDirectProxyConnectNil(t *testing.T) {
	var dp *DirectProxy
	_, err := dp.Connect(context.Background(), "us")
	if err == nil {
		t.Error("expected error for nil proxy")
	}
}

func TestDirectProxyDisconnect(t *testing.T) {
	dp := NewDirectProxy("host", 1080)
	if err := dp.Disconnect(context.Background()); err != nil {
		t.Errorf("Disconnect() error = %v", err)
	}
}

func TestDirectProxyStatus(t *testing.T) {
	dp := NewDirectProxy("host", 1080)
	status, err := dp.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !status.Connected {
		t.Error("Connected should be true")
	}
	if status.Connection == nil {
		t.Fatal("Connection should not be nil")
	}
	if status.Connection.Protocol != "socks5" {
		t.Errorf("Protocol = %q, want socks5", status.Connection.Protocol)
	}
}

func TestDirectProxyStatusNil(t *testing.T) {
	var dp *DirectProxy
	_, err := dp.Status(context.Background())
	if err == nil {
		t.Error("expected error for nil proxy")
	}
}

func TestDirectProxyProxyURL(t *testing.T) {
	tests := []struct {
		name string
		dp   *DirectProxy
		want string
	}{
		{
			name: "without auth",
			dp:   NewDirectProxy("proxy.test", 1080),
			want: "socks5://proxy.test:1080",
		},
		{
			name: "with auth",
			dp:   NewDirectProxy("proxy.test", 8080, WithDirectProxyScheme("https"), WithDirectProxyAuth("u", "p")),
			want: "https://u:p@proxy.test:8080",
		},
		{
			name: "nil proxy",
			dp:   nil,
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.dp.ProxyURL()
			if got != tt.want {
				t.Errorf("ProxyURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- DirectProxy implements Provider interface ---

func TestDirectProxyImplementsProvider(t *testing.T) {
	var _ Provider = (*DirectProxy)(nil)
}

// --- FilterServersByCountry tests ---

func TestFilterServersByCountry(t *testing.T) {
	servers := []Server{
		{Host: "us1.example.com", Country: "us", City: "New York"},
		{Host: "de1.example.com", Country: "de", City: "Berlin"},
		{Host: "us2.example.com", Country: "us", City: "LA"},
		{Host: "br1.example.com", Country: "br", City: "Sao Paulo"},
	}

	t.Run("match multiple", func(t *testing.T) {
		got := FilterServersByCountry(servers, "us")
		if len(got) != 2 {
			t.Fatalf("got %d, want 2", len(got))
		}
	})

	t.Run("case insensitive input", func(t *testing.T) {
		got := FilterServersByCountry(servers, "US")
		if len(got) != 2 {
			t.Fatalf("got %d, want 2", len(got))
		}
	})

	t.Run("no match", func(t *testing.T) {
		got := FilterServersByCountry(servers, "jp")
		if len(got) != 0 {
			t.Errorf("got %d, want 0", len(got))
		}
	})

	t.Run("empty input", func(t *testing.T) {
		got := FilterServersByCountry(servers, "")
		if len(got) != 0 {
			t.Errorf("got %d, want 0", len(got))
		}
	})

	t.Run("empty server list", func(t *testing.T) {
		got := FilterServersByCountry(nil, "us")
		if len(got) != 0 {
			t.Errorf("got %d, want 0", len(got))
		}
	})
}

// --- Server JSON tests ---

func TestServerJSON(t *testing.T) {
	s := Server{
		Host:        "us1.vpn.com",
		Country:     "us",
		CountryName: "United States",
		City:        "New York",
		Load:        42,
		Tags:        []string{"p2p", "static"},
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}

	var decoded Server
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.Host != s.Host || decoded.Country != s.Country || decoded.Load != s.Load {
		t.Errorf("decoded = %+v, want %+v", decoded, s)
	}
}

// --- RotationConfig tests ---

func TestRotationConfigDefaults(t *testing.T) {
	cfg := RotationConfig{}
	if cfg.PerPage {
		t.Error("PerPage should default to false")
	}
	if cfg.Interval != 0 {
		t.Error("Interval should default to 0")
	}
	if len(cfg.Countries) != 0 {
		t.Error("Countries should default to empty")
	}
}

// --- Rotator tests ---

func TestNewRotatorDefaultCountries(t *testing.T) {
	dp := NewDirectProxy("host", 1080)
	r := NewRotator(dp, RotationConfig{})
	if len(r.Countries) != 1 || r.Countries[0] != "" {
		t.Errorf("Countries = %v, want [\"\"]", r.Countries)
	}
}

func TestNewRotatorWithCountries(t *testing.T) {
	dp := NewDirectProxy("host", 1080)
	r := NewRotator(dp, RotationConfig{Countries: []string{"us", "de", "br"}})
	if len(r.Countries) != 3 {
		t.Errorf("Countries len = %d, want 3", len(r.Countries))
	}
}

func TestRotatorShouldRotatePerPage(t *testing.T) {
	dp := NewDirectProxy("host", 1080)
	r := NewRotator(dp, RotationConfig{PerPage: true})
	if !r.ShouldRotate() {
		t.Error("ShouldRotate should return true for PerPage")
	}
}

func TestRotatorShouldRotateInterval(t *testing.T) {
	dp := NewDirectProxy("host", 1080)
	r := NewRotator(dp, RotationConfig{Interval: 1 * time.Millisecond})

	// Wait for interval to elapse
	time.Sleep(2 * time.Millisecond)
	if !r.ShouldRotate() {
		t.Error("ShouldRotate should return true after interval elapsed")
	}
}

func TestRotatorShouldRotateNoConfig(t *testing.T) {
	dp := NewDirectProxy("host", 1080)
	r := NewRotator(dp, RotationConfig{})
	if r.ShouldRotate() {
		t.Error("ShouldRotate should return false with no rotation config")
	}
}

func TestRotatorNext(t *testing.T) {
	dp := NewDirectProxy("host", 1080)
	r := NewRotator(dp, RotationConfig{Countries: []string{"us", "de", "br"}})

	results := make([]string, 6)
	for i := range results {
		results[i] = r.Next()
	}

	want := []string{"us", "de", "br", "us", "de", "br"}
	for i, got := range results {
		if got != want[i] {
			t.Errorf("Next()[%d] = %q, want %q", i, got, want[i])
		}
	}
}

func TestRotatorNextSingleCountry(t *testing.T) {
	dp := NewDirectProxy("host", 1080)
	r := NewRotator(dp, RotationConfig{Countries: []string{"us"}})

	for i := 0; i < 3; i++ {
		got := r.Next()
		if got != "us" {
			t.Errorf("Next() = %q, want us", got)
		}
	}
}

func TestRotatorRotateIfNeededNilRotator(t *testing.T) {
	var r *Rotator
	conn, err := r.RotateIfNeeded(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if conn != nil {
		t.Error("expected nil connection")
	}
}

func TestRotatorRotateIfNeededNilProvider(t *testing.T) {
	r := &Rotator{Provider: nil}
	conn, err := r.RotateIfNeeded(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if conn != nil {
		t.Error("expected nil connection")
	}
}

func TestRotatorRotateIfNeededNoRotation(t *testing.T) {
	dp := NewDirectProxy("host", 1080)
	r := NewRotator(dp, RotationConfig{}) // no PerPage, no Interval
	conn, err := r.RotateIfNeeded(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if conn != nil {
		t.Error("expected nil connection when no rotation needed")
	}
}

func TestRotatorRotateIfNeededPerPage(t *testing.T) {
	dp := NewDirectProxy("proxy.test", 1080)
	r := NewRotator(dp, RotationConfig{PerPage: true})

	conn, err := r.RotateIfNeeded(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if conn == nil {
		t.Fatal("expected connection, got nil")
	}
	if conn.Protocol != "socks5" {
		t.Errorf("Protocol = %q, want socks5", conn.Protocol)
	}
	if r.PageCount != 1 {
		t.Errorf("PageCount = %d, want 1", r.PageCount)
	}
}

// --- SurfsharkProvider basic tests ---

func TestNewSurfsharkProvider(t *testing.T) {
	sp := NewSurfsharkProvider("test@example.com", "secret")
	if sp.Email != "test@example.com" {
		t.Errorf("Email = %q", sp.Email)
	}
	if sp.Passwd != "secret" {
		t.Errorf("Password = %q", sp.Passwd)
	}
	if sp.APIBase != SurfsharkAPIBase {
		t.Errorf("APIBase = %q, want %q", sp.APIBase, SurfsharkAPIBase)
	}
	if sp.HTTPClient == nil {
		t.Error("HTTPClient should not be nil")
	}
}

func TestSurfsharkProviderName(t *testing.T) {
	sp := NewSurfsharkProvider("e", "p")
	if sp.Name() != "surfshark" {
		t.Errorf("Name() = %q, want surfshark", sp.Name())
	}
}

func TestSurfsharkProviderNilChecks(t *testing.T) {
	var sp *SurfsharkProvider

	_, err := sp.Servers(context.Background())
	if err == nil {
		t.Error("Servers on nil should error")
	}

	_, err = sp.Connect(context.Background(), "us")
	if err == nil {
		t.Error("Connect on nil should error")
	}

	err = sp.Disconnect(context.Background())
	if err == nil {
		t.Error("Disconnect on nil should error")
	}

	_, err = sp.Status(context.Background())
	if err == nil {
		t.Error("Status on nil should error")
	}
}

func TestSurfsharkProviderStatusDisconnected(t *testing.T) {
	sp := NewSurfsharkProvider("e", "p")
	status, err := sp.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.Connected {
		t.Error("should not be connected initially")
	}
	if status.Connection != nil {
		t.Error("connection should be nil when disconnected")
	}
}

func TestSurfsharkProviderDisconnect(t *testing.T) {
	sp := NewSurfsharkProvider("e", "p")
	sp.Connected = &Connection{
		Server:   Server{Host: "test"},
		Protocol: "https",
		Port:     443,
	}
	if err := sp.Disconnect(context.Background()); err != nil {
		t.Fatal(err)
	}
	if sp.Connected != nil {
		t.Error("Connected should be nil after disconnect")
	}
}

func TestSurfsharkProviderProxyCredentials(t *testing.T) {
	sp := NewSurfsharkProvider("e", "p")
	sp.ProxyUser = "proxy_user"
	sp.ProxyPass = "proxy_pass"

	u, p := sp.ProxyCredentials()
	if u != "proxy_user" || p != "proxy_pass" {
		t.Errorf("got %q:%q, want proxy_user:proxy_pass", u, p)
	}
}

// --- PickServer tests ---

func TestPickServerLowestLoad(t *testing.T) {
	sp := NewSurfsharkProvider("e", "p")
	sp.Servers_ = []Server{
		{Host: "us1.test", Country: "us", Load: 50},
		{Host: "us2.test", Country: "us", Load: 20},
		{Host: "us3.test", Country: "us", Load: 80},
		{Host: "de1.test", Country: "de", Load: 10},
	}

	srv, err := sp.PickServer("us")
	if err != nil {
		t.Fatal(err)
	}
	if srv.Host != "us2.test" {
		t.Errorf("picked %q, want us2.test (lowest load)", srv.Host)
	}
	if srv.Load != 20 {
		t.Errorf("Load = %d, want 20", srv.Load)
	}
}

func TestPickServerCaseInsensitive(t *testing.T) {
	sp := NewSurfsharkProvider("e", "p")
	sp.Servers_ = []Server{
		{Host: "us1.test", Country: "us", Load: 10},
	}

	srv, err := sp.PickServer("US")
	if err != nil {
		t.Fatal(err)
	}
	if srv.Host != "us1.test" {
		t.Errorf("picked %q, want us1.test", srv.Host)
	}
}

func TestPickServerNoMatch(t *testing.T) {
	sp := NewSurfsharkProvider("e", "p")
	sp.Servers_ = []Server{
		{Host: "us1.test", Country: "us", Load: 10},
	}

	_, err := sp.PickServer("jp")
	if err == nil {
		t.Error("expected error for no matching country")
	}
}

func TestPickServerEmptyList(t *testing.T) {
	sp := NewSurfsharkProvider("e", "p")
	_, err := sp.PickServer("us")
	if err == nil {
		t.Error("expected error for empty server list")
	}
}

// --- ParseSurfsharkClusters tests ---

func TestParseSurfsharkClusters(t *testing.T) {
	data := `[
		{"connectionName": "us-nyc.prod.surfshark.com", "country": "US", "location": "New York", "load": 42.5, "tags": ["p2p"]},
		{"connectionName": "de-ber.prod.surfshark.com", "country": "DE", "location": "Berlin", "load": 15.0, "tags": ["static", "obfuscated"]}
	]`

	servers, err := ParseSurfsharkClusters([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 2 {
		t.Fatalf("got %d servers, want 2", len(servers))
	}

	// First server
	if servers[0].Host != "us-nyc.prod.surfshark.com" {
		t.Errorf("Host = %q", servers[0].Host)
	}
	if servers[0].Country != "us" {
		t.Errorf("Country = %q, want us (lowercase)", servers[0].Country)
	}
	if servers[0].City != "New York" {
		t.Errorf("City = %q", servers[0].City)
	}
	if servers[0].Load != 42 {
		t.Errorf("Load = %d, want 42 (truncated from float)", servers[0].Load)
	}

	// Second server
	if servers[1].Country != "de" {
		t.Errorf("Country = %q, want de", servers[1].Country)
	}
	if len(servers[1].Tags) != 2 {
		t.Errorf("Tags = %v, want 2 tags", servers[1].Tags)
	}
}

func TestParseSurfsharkClustersEmpty(t *testing.T) {
	servers, err := ParseSurfsharkClusters([]byte("[]"))
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 0 {
		t.Errorf("got %d, want 0", len(servers))
	}
}

func TestParseSurfsharkClustersInvalidJSON(t *testing.T) {
	_, err := ParseSurfsharkClusters([]byte("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// --- SurfsharkProvider implements Provider interface ---

func TestSurfsharkProviderImplementsProvider(t *testing.T) {
	var _ Provider = (*SurfsharkProvider)(nil)
}

// --- Connection/Status JSON tests ---

func TestConnectionJSON(t *testing.T) {
	conn := Connection{
		Server:   Server{Host: "test.com", Country: "us"},
		Protocol: "https",
		Port:     443,
	}

	data, err := json.Marshal(conn)
	if err != nil {
		t.Fatal(err)
	}

	var decoded Connection
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.Protocol != "https" || decoded.Port != 443 {
		t.Errorf("decoded = %+v", decoded)
	}
}

func TestStatusJSON(t *testing.T) {
	status := Status{
		Connected: true,
		Connection: &Connection{
			Protocol: "socks5",
			Port:     1080,
		},
		PublicIP: "1.2.3.4",
	}

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}

	var decoded Status
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if !decoded.Connected || decoded.PublicIP != "1.2.3.4" {
		t.Errorf("decoded = %+v", decoded)
	}
}

// --- SurfsharkLoginRequest JSON ---

func TestSurfsharkLoginRequestJSON(t *testing.T) {
	req := SurfsharkLoginRequest{Email: "test@test.com", Password: "pw"}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); got != `{"email":"test@test.com","password":"pw"}` {
		t.Errorf("got %s", got)
	}
}

// --- API path constants ---

func TestSurfsharkAPIConstants(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"APIBase", SurfsharkAPIBase, "https://ext.surfshark.com"},
		{"LoginPath", SurfsharkLoginPath, "/v1/auth/login"},
		{"RenewPath", SurfsharkRenewPath, "/v1/auth/renew"},
		{"UserPath", SurfsharkUserPath, "/v1/server/user"},
		{"ServersPath", SurfsharkServersPath, "/v5/server/clusters/all"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}
}

// --- mockProvider for Rotator disconnect error ---

type mockProvider struct {
	disconnectErr error
	connectErr    error
	connectConn   *Connection
}

func (m *mockProvider) Name() string { return "mock" }
func (m *mockProvider) Servers(_ context.Context) ([]Server, error) {
	return nil, nil
}
func (m *mockProvider) Connect(_ context.Context, _ string) (*Connection, error) {
	if m.connectErr != nil {
		return nil, m.connectErr
	}
	return m.connectConn, nil
}
func (m *mockProvider) Disconnect(_ context.Context) error {
	return m.disconnectErr
}
func (m *mockProvider) Status(_ context.Context) (*Status, error) {
	return &Status{}, nil
}

func TestRotatorRotateIfNeededDisconnectError(t *testing.T) {
	mp := &mockProvider{disconnectErr: fmt.Errorf("disconnect failed")}
	r := NewRotator(mp, RotationConfig{PerPage: true})

	_, err := r.RotateIfNeeded(context.Background())
	if err == nil {
		t.Error("expected disconnect error to propagate")
	}
}

func TestRotatorRotateIfNeededConnectError(t *testing.T) {
	mp := &mockProvider{connectErr: fmt.Errorf("connect failed")}
	r := NewRotator(mp, RotationConfig{PerPage: true})

	_, err := r.RotateIfNeeded(context.Background())
	if err == nil {
		t.Error("expected connect error to propagate")
	}
}

func TestRotatorRotateIfNeededSuccess(t *testing.T) {
	mp := &mockProvider{
		connectConn: &Connection{
			Server:   Server{Host: "new.server", Country: "de"},
			Protocol: "https",
			Port:     443,
		},
	}
	r := NewRotator(mp, RotationConfig{
		PerPage:   true,
		Countries: []string{"de", "br"},
	})

	conn, err := r.RotateIfNeeded(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if conn == nil {
		t.Fatal("expected connection")
	}
	if conn.Server.Host != "new.server" {
		t.Errorf("Host = %q, want new.server", conn.Server.Host)
	}
	if r.PageCount != 1 {
		t.Errorf("PageCount = %d, want 1", r.PageCount)
	}
}

// --- EnsureAuthenticated with existing token ---

func TestEnsureAuthenticatedWithToken(t *testing.T) {
	sp := NewSurfsharkProvider("e", "p")
	sp.Token = "existing-token"

	// Should return nil immediately without making network calls
	err := sp.EnsureAuthenticated(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- FetchProxyCredentials already cached ---

func TestFetchProxyCredentialsAlreadyCached(t *testing.T) {
	sp := NewSurfsharkProvider("e", "p")
	sp.ProxyUser = "cached"

	// Should return nil immediately
	err := sp.FetchProxyCredentials(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
