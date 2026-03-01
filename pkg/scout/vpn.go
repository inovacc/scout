package scout

import (
	"context"
	"fmt"
	"time"
)

// VPNProvider is the interface for pluggable VPN backends.
type VPNProvider interface {
	// Name returns the provider name (e.g. "surfshark", "nordvpn").
	Name() string
	// Servers returns available server locations.
	Servers(ctx context.Context) ([]VPNServer, error)
	// Connect connects to a server in the given country (ISO 2-letter code).
	// If country is empty, connects to the optimal/nearest server.
	Connect(ctx context.Context, country string) (*VPNConnection, error)
	// Disconnect disconnects from the current server.
	Disconnect(ctx context.Context) error
	// Status returns the current connection status.
	Status(ctx context.Context) (*VPNStatus, error)
}

// VPNServer represents a VPN server location.
type VPNServer struct {
	Host        string   `json:"host"`
	Country     string   `json:"country"` // ISO 2-letter code
	CountryName string   `json:"country_name"`
	City        string   `json:"city"`
	Load        int      `json:"load"` // server load percentage 0-100
	Tags        []string `json:"tags"` // e.g. "p2p", "static", "obfuscated"
}

// VPNConnection holds active connection details.
type VPNConnection struct {
	Server   VPNServer `json:"server"`
	Protocol string    `json:"protocol"` // "https", "socks5"
	Port     int       `json:"port"`
}

// VPNStatus represents current VPN state.
type VPNStatus struct {
	Connected  bool           `json:"connected"`
	Connection *VPNConnection `json:"connection,omitempty"`
	PublicIP   string         `json:"public_ip,omitempty"`
}

// VPNRotationConfig configures automatic server rotation.
type VPNRotationConfig struct {
	Countries []string      // rotate through these countries
	Interval  time.Duration // rotate every N duration (0 = per-page)
	PerPage   bool          // rotate on every NewPage() call
}

// DirectProxyOption configures a DirectProxy.
type DirectProxyOption func(*DirectProxy)

// DirectProxy implements VPNProvider for a single static proxy endpoint.
type DirectProxy struct {
	host     string
	port     int
	scheme   string // "https" or "socks5"
	username string
	password string
}

// NewDirectProxy creates a DirectProxy for the given host and port.
// Default scheme is "socks5". Use WithDirectProxyScheme to change.
func NewDirectProxy(host string, port int, opts ...DirectProxyOption) *DirectProxy {
	dp := &DirectProxy{
		host:   host,
		port:   port,
		scheme: "socks5",
	}
	for _, fn := range opts {
		fn(dp)
	}
	return dp
}

// WithDirectProxyScheme sets the proxy protocol ("https" or "socks5").
func WithDirectProxyScheme(scheme string) DirectProxyOption {
	return func(dp *DirectProxy) { dp.scheme = scheme }
}

// WithDirectProxyAuth sets username/password for proxy authentication.
func WithDirectProxyAuth(user, pass string) DirectProxyOption {
	return func(dp *DirectProxy) {
		dp.username = user
		dp.password = pass
	}
}

// Name returns "direct-proxy".
func (dp *DirectProxy) Name() string {
	return "direct-proxy"
}

// Servers returns a single-element list with the configured proxy.
func (dp *DirectProxy) Servers(_ context.Context) ([]VPNServer, error) {
	if dp == nil {
		return nil, fmt.Errorf("scout: vpn: proxy is nil")
	}
	return []VPNServer{dp.server()}, nil
}

// Connect returns the configured proxy connection. The country parameter is ignored.
func (dp *DirectProxy) Connect(_ context.Context, _ string) (*VPNConnection, error) {
	if dp == nil {
		return nil, fmt.Errorf("scout: vpn: proxy is nil")
	}
	return &VPNConnection{
		Server:   dp.server(),
		Protocol: dp.scheme,
		Port:     dp.port,
	}, nil
}

// Disconnect is a no-op for a static proxy.
func (dp *DirectProxy) Disconnect(_ context.Context) error {
	return nil
}

// Status returns connected=true for a configured proxy.
func (dp *DirectProxy) Status(_ context.Context) (*VPNStatus, error) {
	if dp == nil {
		return nil, fmt.Errorf("scout: vpn: proxy is nil")
	}
	conn := &VPNConnection{
		Server:   dp.server(),
		Protocol: dp.scheme,
		Port:     dp.port,
	}
	return &VPNStatus{
		Connected:  true,
		Connection: conn,
	}, nil
}

// ProxyURL returns the proxy URL string suitable for WithProxy.
func (dp *DirectProxy) ProxyURL() string {
	if dp == nil {
		return ""
	}
	if dp.username != "" {
		return fmt.Sprintf("%s://%s:%s@%s:%d", dp.scheme, dp.username, dp.password, dp.host, dp.port)
	}
	return fmt.Sprintf("%s://%s:%d", dp.scheme, dp.host, dp.port)
}

func (dp *DirectProxy) server() VPNServer {
	return VPNServer{
		Host: fmt.Sprintf("%s:%d", dp.host, dp.port),
		Tags: []string{dp.scheme},
	}
}
