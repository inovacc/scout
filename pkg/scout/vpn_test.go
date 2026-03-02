package scout

import (
	"context"
	"testing"
)

func TestDirectProxy_NewAndConnect(t *testing.T) {
	dp := NewDirectProxy("proxy.example.com", 1080)

	conn, err := dp.Connect(context.Background(), "")
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	if conn.Protocol != "socks5" {
		t.Errorf("protocol = %q, want socks5", conn.Protocol)
	}

	if conn.Port != 1080 {
		t.Errorf("port = %d, want 1080", conn.Port)
	}

	if conn.Server.Host != "proxy.example.com:1080" {
		t.Errorf("host = %q, want proxy.example.com:1080", conn.Server.Host)
	}
}

func TestDirectProxy_Scheme(t *testing.T) {
	dp := NewDirectProxy("proxy.example.com", 8080, WithDirectProxyScheme("https"))

	conn, err := dp.Connect(context.Background(), "us")
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	if conn.Protocol != "https" {
		t.Errorf("protocol = %q, want https", conn.Protocol)
	}
}

func TestDirectProxy_Auth(t *testing.T) {
	dp := NewDirectProxy("proxy.example.com", 1080,
		WithDirectProxyAuth("user", "pass"),
	)
	url := dp.ProxyURL()

	want := "socks5://user:pass@proxy.example.com:1080"
	if url != want {
		t.Errorf("ProxyURL = %q, want %q", url, want)
	}
}

func TestDirectProxy_ProxyURL_NoAuth(t *testing.T) {
	dp := NewDirectProxy("proxy.example.com", 1080)
	url := dp.ProxyURL()

	want := "socks5://proxy.example.com:1080"
	if url != want {
		t.Errorf("ProxyURL = %q, want %q", url, want)
	}
}

func TestDirectProxy_Servers(t *testing.T) {
	dp := NewDirectProxy("proxy.example.com", 1080)

	servers, err := dp.Servers(context.Background())
	if err != nil {
		t.Fatalf("Servers: %v", err)
	}

	if len(servers) != 1 {
		t.Fatalf("len(servers) = %d, want 1", len(servers))
	}

	if servers[0].Host != "proxy.example.com:1080" {
		t.Errorf("host = %q", servers[0].Host)
	}
}

func TestDirectProxy_Status(t *testing.T) {
	dp := NewDirectProxy("proxy.example.com", 1080)

	st, err := dp.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	if !st.Connected {
		t.Error("expected connected=true")
	}

	if st.Connection == nil {
		t.Fatal("expected non-nil connection")
	}
}

func TestDirectProxy_Disconnect(t *testing.T) {
	dp := NewDirectProxy("proxy.example.com", 1080)
	if err := dp.Disconnect(context.Background()); err != nil {
		t.Fatalf("Disconnect: %v", err)
	}
}

func TestDirectProxy_NilSafety(t *testing.T) {
	var dp *DirectProxy
	if _, err := dp.Servers(context.Background()); err == nil {
		t.Error("expected error for nil Servers")
	}

	if _, err := dp.Connect(context.Background(), ""); err == nil {
		t.Error("expected error for nil Connect")
	}

	if _, err := dp.Status(context.Background()); err == nil {
		t.Error("expected error for nil Status")
	}

	if url := dp.ProxyURL(); url != "" {
		t.Errorf("expected empty ProxyURL for nil, got %q", url)
	}
}

func TestVPNServer_Fields(t *testing.T) {
	s := VPNServer{
		Host:        "us1.example.com",
		Country:     "US",
		CountryName: "United States",
		City:        "New York",
		Load:        42,
		Tags:        []string{"p2p", "static"},
	}
	if s.Country != "US" {
		t.Errorf("Country = %q", s.Country)
	}

	if len(s.Tags) != 2 {
		t.Errorf("Tags len = %d", len(s.Tags))
	}
}

func TestVPNRotationConfig(t *testing.T) {
	cfg := VPNRotationConfig{
		Countries: []string{"US", "DE", "JP"},
		PerPage:   true,
	}
	if len(cfg.Countries) != 3 {
		t.Errorf("Countries len = %d", len(cfg.Countries))
	}

	if !cfg.PerPage {
		t.Error("expected PerPage=true")
	}
}

func TestDirectProxy_Name(t *testing.T) {
	dp := NewDirectProxy("proxy.example.com", 1080)
	if dp.Name() != "direct-proxy" {
		t.Errorf("Name = %q, want direct-proxy", dp.Name())
	}
}

// TestVPN_DirectProxy_Integration tests that WithVPN+DirectProxy sets proxy options correctly.
func TestVPN_DirectProxy_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires browser")
	}

	ts := newTestServer()
	defer ts.Close()

	proxy := NewDirectProxy("127.0.0.1", 8080, WithDirectProxyAuth("user", "pass"))

	// Verify provider integration with options
	b, err := New(
		WithHeadless(true),
		WithNoSandbox(),
		WithVPN(proxy),
		WithoutBridge(),
	)
	if err != nil {
		t.Skipf("browser unavailable: %v", err)
	}

	defer func() { _ = b.Close() }()

	// Verify VPN status
	status := b.VPNStatus()
	if status == nil {
		t.Log("VPN status is nil (proxy set at launch level, not CDP level)")
	}

	// Verify proxy was configured in options
	if b.opts.proxy == "" {
		t.Error("expected proxy to be set in browser options")
	} else {
		t.Logf("proxy configured: %s", b.opts.proxy)
	}

	if b.opts.proxyAuth == nil {
		t.Error("expected proxyAuth to be set")
	}
}

// TestVPN_RotationConfig_Integration tests rotation config is applied to browser.
func TestVPN_RotationConfig_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: requires browser")
	}

	proxy := NewDirectProxy("127.0.0.1", 8080)

	b, err := New(
		WithHeadless(true),
		WithNoSandbox(),
		WithVPN(proxy),
		WithVPNRotation(VPNRotationConfig{
			Countries: []string{"us", "de", "jp"},
			PerPage:   true,
		}),
		WithoutBridge(),
	)
	if err != nil {
		t.Skipf("browser unavailable: %v", err)
	}

	defer func() { _ = b.Close() }()

	if b.vpnRot == nil {
		t.Error("expected vpnRotator to be initialized")
	} else {
		t.Logf("rotator initialized with %d countries", len(b.vpnRot.countries))
	}
}
