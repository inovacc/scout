package vpn

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Provider is the interface for pluggable VPN backends.
type Provider interface {
	// Name returns the provider name (e.g. "surfshark", "nordvpn").
	Name() string
	// Servers returns available server locations.
	Servers(ctx context.Context) ([]Server, error)
	// Connect connects to a server in the given country (ISO 2-letter code).
	// If country is empty, connects to the optimal/nearest server.
	Connect(ctx context.Context, country string) (*Connection, error)
	// Disconnect disconnects from the current server.
	Disconnect(ctx context.Context) error
	// Status returns the current connection status.
	Status(ctx context.Context) (*Status, error)
}

// Server represents a VPN server location.
type Server struct {
	Host        string   `json:"host"`
	Country     string   `json:"country"` // ISO 2-letter code
	CountryName string   `json:"country_name"`
	City        string   `json:"city"`
	Load        int      `json:"load"` // server load percentage 0-100
	Tags        []string `json:"tags"` // e.g. "p2p", "static", "obfuscated"
}

// Connection holds active connection details.
type Connection struct {
	Server   Server `json:"server"`
	Protocol string `json:"protocol"` // "https", "socks5"
	Port     int    `json:"port"`
}

// Status represents current VPN state.
type Status struct {
	Connected  bool        `json:"connected"`
	Connection *Connection `json:"connection,omitempty"`
	PublicIP   string      `json:"public_ip,omitempty"`
}

// RotationConfig configures automatic server rotation.
type RotationConfig struct {
	Countries []string      // rotate through these countries
	Interval  time.Duration // rotate every N duration (0 = per-page)
	PerPage   bool          // rotate on every NewPage() call
}

// DirectProxyOption configures a DirectProxy.
type DirectProxyOption func(*DirectProxy)

// DirectProxy implements Provider for a single static proxy endpoint.
type DirectProxy struct {
	Host     string
	ProxyPort int
	Scheme   string // "https" or "socks5"
	Username string
	Password string
}

// NewDirectProxy creates a DirectProxy for the given host and port.
// Default scheme is "socks5". Use WithDirectProxyScheme to change.
func NewDirectProxy(host string, port int, opts ...DirectProxyOption) *DirectProxy {
	dp := &DirectProxy{
		Host:      host,
		ProxyPort: port,
		Scheme:    "socks5",
	}
	for _, fn := range opts {
		fn(dp)
	}

	return dp
}

// WithDirectProxyScheme sets the proxy protocol ("https" or "socks5").
func WithDirectProxyScheme(scheme string) DirectProxyOption {
	return func(dp *DirectProxy) { dp.Scheme = scheme }
}

// WithDirectProxyAuth sets username/password for proxy authentication.
func WithDirectProxyAuth(user, pass string) DirectProxyOption {
	return func(dp *DirectProxy) {
		dp.Username = user
		dp.Password = pass
	}
}

// Name returns "direct-proxy".
func (dp *DirectProxy) Name() string {
	return "direct-proxy"
}

// Servers returns a single-element list with the configured proxy.
func (dp *DirectProxy) Servers(_ context.Context) ([]Server, error) {
	if dp == nil {
		return nil, fmt.Errorf("scout: vpn: proxy is nil")
	}

	return []Server{dp.server()}, nil
}

// Connect returns the configured proxy connection. The country parameter is ignored.
func (dp *DirectProxy) Connect(_ context.Context, _ string) (*Connection, error) {
	if dp == nil {
		return nil, fmt.Errorf("scout: vpn: proxy is nil")
	}

	return &Connection{
		Server:   dp.server(),
		Protocol: dp.Scheme,
		Port:     dp.ProxyPort,
	}, nil
}

// Disconnect is a no-op for a static proxy.
func (dp *DirectProxy) Disconnect(_ context.Context) error {
	return nil
}

// Status returns connected=true for a configured proxy.
func (dp *DirectProxy) Status(_ context.Context) (*Status, error) {
	if dp == nil {
		return nil, fmt.Errorf("scout: vpn: proxy is nil")
	}

	conn := &Connection{
		Server:   dp.server(),
		Protocol: dp.Scheme,
		Port:     dp.ProxyPort,
	}

	return &Status{
		Connected:  true,
		Connection: conn,
	}, nil
}

// ProxyURL returns the proxy URL string suitable for WithProxy.
func (dp *DirectProxy) ProxyURL() string {
	if dp == nil {
		return ""
	}

	if dp.Username != "" {
		return fmt.Sprintf("%s://%s:%s@%s:%d", dp.Scheme, dp.Username, dp.Password, dp.Host, dp.ProxyPort)
	}

	return fmt.Sprintf("%s://%s:%d", dp.Scheme, dp.Host, dp.ProxyPort)
}

func (dp *DirectProxy) server() Server {
	return Server{
		Host: fmt.Sprintf("%s:%d", dp.Host, dp.ProxyPort),
		Tags: []string{dp.Scheme},
	}
}

// FilterServersByCountry returns servers matching the given country code.
func FilterServersByCountry(servers []Server, country string) []Server {
	country = strings.ToLower(country)

	var result []Server

	for _, s := range servers {
		if s.Country == country {
			result = append(result, s)
		}
	}

	return result
}
