package vpn

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	SurfsharkAPIBase     = "https://ext.surfshark.com"
	SurfsharkLoginPath   = "/v1/auth/login"
	SurfsharkRenewPath   = "/v1/auth/renew"
	SurfsharkUserPath    = "/v1/server/user"
	SurfsharkServersPath = "/v5/server/clusters/all"
)

// SurfsharkLoginRequest is the JSON body for POST /v1/auth/login.
type SurfsharkLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// SurfsharkLoginResponse is the JSON response from POST /v1/auth/login.
type SurfsharkLoginResponse struct {
	Token      string `json:"token"`
	RenewToken string `json:"renewToken"`
}

// SurfsharkProxyCreds is the JSON response from GET /v1/server/user.
type SurfsharkProxyCreds struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// SurfsharkCluster is a single entry from GET /v5/server/clusters/all.
type SurfsharkCluster struct {
	Host        string   `json:"connectionName"`
	CountryCode string   `json:"country"`
	City        string   `json:"location"`
	Load        float64  `json:"load"`
	Tags        []string `json:"tags"`
}

// SurfsharkProvider implements Provider for the Surfshark VPN service.
type SurfsharkProvider struct {
	Email      string
	Passwd     string
	Token      string
	RenewToken string
	ProxyUser  string
	ProxyPass  string
	Servers_   []Server
	Connected  *Connection
	HTTPClient *http.Client
	APIBase    string // overridable for testing
	mu         sync.Mutex
}

// NewSurfsharkProvider creates a new Surfshark VPN provider.
func NewSurfsharkProvider(email, password string) *SurfsharkProvider {
	return &SurfsharkProvider{
		Email:  email,
		Passwd: password,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		APIBase: SurfsharkAPIBase,
	}
}

// Name returns "surfshark".
func (s *SurfsharkProvider) Name() string {
	return "surfshark"
}

// Servers fetches the full server list from Surfshark. Results are cached.
func (s *SurfsharkProvider) Servers(ctx context.Context) ([]Server, error) {
	if s == nil {
		return nil, fmt.Errorf("scout: vpn: surfshark: provider is nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.Servers_) > 0 {
		return s.Servers_, nil
	}

	if err := s.EnsureAuthenticated(ctx); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.APIBase+SurfsharkServersPath, nil)
	if err != nil {
		return nil, fmt.Errorf("scout: vpn: surfshark: create server request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.Token)

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("scout: vpn: surfshark: fetch servers: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("scout: vpn: surfshark: fetch servers: status %d: %s", resp.StatusCode, string(body))
	}

	var clusters []SurfsharkCluster
	if err := json.NewDecoder(resp.Body).Decode(&clusters); err != nil {
		return nil, fmt.Errorf("scout: vpn: surfshark: parse servers: %w", err)
	}

	servers := make([]Server, len(clusters))
	for i, c := range clusters {
		servers[i] = Server{
			Host:    c.Host,
			Country: strings.ToLower(c.CountryCode),
			City:    c.City,
			Load:    int(c.Load),
			Tags:    c.Tags,
		}
	}

	s.Servers_ = servers

	return servers, nil
}

// Connect authenticates, fetches proxy credentials, selects the lowest-load
// server in the given country, and returns the connection details.
func (s *SurfsharkProvider) Connect(ctx context.Context, country string) (*Connection, error) {
	if s == nil {
		return nil, fmt.Errorf("scout: vpn: surfshark: provider is nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.EnsureAuthenticated(ctx); err != nil {
		return nil, err
	}

	if err := s.FetchProxyCredentials(ctx); err != nil {
		return nil, err
	}

	// Ensure servers are loaded (unlock not needed, already holding lock).
	if len(s.Servers_) == 0 {
		s.mu.Unlock()
		servers, err := s.Servers(ctx)
		s.mu.Lock()
		if err != nil {
			return nil, err
		}

		s.Servers_ = servers
	}

	server, err := s.PickServer(country)
	if err != nil {
		return nil, err
	}

	conn := &Connection{
		Server:   *server,
		Protocol: "https",
		Port:     443,
	}
	s.Connected = conn

	return conn, nil
}

// Disconnect clears the active connection state.
func (s *SurfsharkProvider) Disconnect(_ context.Context) error {
	if s == nil {
		return fmt.Errorf("scout: vpn: surfshark: provider is nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.Connected = nil

	return nil
}

// Status returns the current connection status.
func (s *SurfsharkProvider) Status(_ context.Context) (*Status, error) {
	if s == nil {
		return nil, fmt.Errorf("scout: vpn: surfshark: provider is nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return &Status{
		Connected:  s.Connected != nil,
		Connection: s.Connected,
	}, nil
}

// ProxyCredentials returns the proxy username and password.
// Must be called after Connect.
func (s *SurfsharkProvider) ProxyCredentials() (username, password string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.ProxyUser, s.ProxyPass
}

// Authenticate performs POST /v1/auth/login and stores the JWT tokens.
func (s *SurfsharkProvider) Authenticate(ctx context.Context) error {
	body, err := json.Marshal(SurfsharkLoginRequest{
		Email:    s.Email,
		Password: s.Passwd,
	})
	if err != nil {
		return fmt.Errorf("scout: vpn: surfshark: marshal login: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.APIBase+SurfsharkLoginPath, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("scout: vpn: surfshark: create login request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("scout: vpn: surfshark: login request: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("scout: vpn: surfshark: login failed: status %d: %s", resp.StatusCode, string(respBody))
	}

	var loginResp SurfsharkLoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return fmt.Errorf("scout: vpn: surfshark: parse login response: %w", err)
	}

	if loginResp.Token == "" {
		return fmt.Errorf("scout: vpn: surfshark: login returned empty token")
	}

	s.Token = loginResp.Token
	s.RenewToken = loginResp.RenewToken

	return nil
}

// FetchProxyCredentials fetches proxy auth credentials from GET /v1/server/user.
func (s *SurfsharkProvider) FetchProxyCredentials(ctx context.Context) error {
	if s.ProxyUser != "" {
		return nil // already fetched
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.APIBase+SurfsharkUserPath, nil)
	if err != nil {
		return fmt.Errorf("scout: vpn: surfshark: create creds request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.Token)

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("scout: vpn: surfshark: fetch proxy creds: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("scout: vpn: surfshark: fetch proxy creds: status %d: %s", resp.StatusCode, string(body))
	}

	var creds SurfsharkProxyCreds
	if err := json.NewDecoder(resp.Body).Decode(&creds); err != nil {
		return fmt.Errorf("scout: vpn: surfshark: parse proxy creds: %w", err)
	}

	if creds.Username == "" {
		return fmt.Errorf("scout: vpn: surfshark: proxy credentials empty")
	}

	s.ProxyUser = creds.Username
	s.ProxyPass = creds.Password

	return nil
}

// EnsureAuthenticated authenticates if no token is present.
func (s *SurfsharkProvider) EnsureAuthenticated(ctx context.Context) error {
	if s.Token != "" {
		return nil
	}

	return s.Authenticate(ctx)
}

// PickServer selects the lowest-load server matching the given country code.
func (s *SurfsharkProvider) PickServer(country string) (*Server, error) {
	country = strings.ToLower(country)

	var best *Server

	for i := range s.Servers_ {
		sv := &s.Servers_[i]
		if sv.Country != country {
			continue
		}

		if best == nil || sv.Load < best.Load {
			best = sv
		}
	}

	if best == nil {
		return nil, fmt.Errorf("scout: vpn: surfshark: no servers found for country %q", country)
	}

	return best, nil
}

// ParseSurfsharkClusters parses the raw JSON response from
// GET /v5/server/clusters/all into Server slices.
func ParseSurfsharkClusters(data []byte) ([]Server, error) {
	var clusters []SurfsharkCluster
	if err := json.Unmarshal(data, &clusters); err != nil {
		return nil, fmt.Errorf("scout: vpn: surfshark: parse clusters: %w", err)
	}

	servers := make([]Server, len(clusters))
	for i, c := range clusters {
		servers[i] = Server{
			Host:    c.Host,
			Country: strings.ToLower(c.CountryCode),
			City:    c.City,
			Load:    int(c.Load),
			Tags:    c.Tags,
		}
	}

	return servers, nil
}
