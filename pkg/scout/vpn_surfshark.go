package scout

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
	surfsharkAPIBase     = "https://ext.surfshark.com"
	surfsharkLoginPath   = "/v1/auth/login"
	surfsharkRenewPath   = "/v1/auth/renew"
	surfsharkUserPath    = "/v1/server/user"
	surfsharkServersPath = "/v5/server/clusters/all"
)

// surfsharkLoginRequest is the JSON body for POST /v1/auth/login.
type surfsharkLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// surfsharkLoginResponse is the JSON response from POST /v1/auth/login.
type surfsharkLoginResponse struct {
	Token      string `json:"token"`
	RenewToken string `json:"renewToken"`
}

// surfsharkProxyCreds is the JSON response from GET /v1/server/user.
type surfsharkProxyCreds struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// surfsharkCluster is a single entry from GET /v5/server/clusters/all.
type surfsharkCluster struct {
	Host        string   `json:"connectionName"`
	CountryCode string   `json:"country"`
	City        string   `json:"location"`
	Load        float64  `json:"load"`
	Tags        []string `json:"tags"`
}

// SurfsharkProvider implements VPNProvider for the Surfshark VPN service.
type SurfsharkProvider struct {
	email      string
	password   string
	token      string
	renewToken string
	proxyUser  string
	proxyPass  string
	servers    []VPNServer
	connected  *VPNConnection
	httpClient *http.Client
	apiBase    string // overridable for testing
	mu         sync.Mutex
}

// NewSurfsharkProvider creates a new Surfshark VPN provider.
func NewSurfsharkProvider(email, password string) *SurfsharkProvider {
	return &SurfsharkProvider{
		email:    email,
		password: password,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		apiBase: surfsharkAPIBase,
	}
}

// Name returns "surfshark".
func (s *SurfsharkProvider) Name() string {
	return "surfshark"
}

// Servers fetches the full server list from Surfshark. Results are cached.
func (s *SurfsharkProvider) Servers(ctx context.Context) ([]VPNServer, error) {
	if s == nil {
		return nil, fmt.Errorf("scout: vpn: surfshark: provider is nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.servers) > 0 {
		return s.servers, nil
	}

	if err := s.ensureAuthenticated(ctx); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.apiBase+surfsharkServersPath, nil)
	if err != nil {
		return nil, fmt.Errorf("scout: vpn: surfshark: create server request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.token)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("scout: vpn: surfshark: fetch servers: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("scout: vpn: surfshark: fetch servers: status %d: %s", resp.StatusCode, string(body))
	}

	var clusters []surfsharkCluster
	if err := json.NewDecoder(resp.Body).Decode(&clusters); err != nil {
		return nil, fmt.Errorf("scout: vpn: surfshark: parse servers: %w", err)
	}

	servers := make([]VPNServer, len(clusters))
	for i, c := range clusters {
		servers[i] = VPNServer{
			Host:    c.Host,
			Country: strings.ToLower(c.CountryCode),
			City:    c.City,
			Load:    int(c.Load),
			Tags:    c.Tags,
		}
	}

	s.servers = servers
	return servers, nil
}

// Connect authenticates, fetches proxy credentials, selects the lowest-load
// server in the given country, and returns the connection details.
func (s *SurfsharkProvider) Connect(ctx context.Context, country string) (*VPNConnection, error) {
	if s == nil {
		return nil, fmt.Errorf("scout: vpn: surfshark: provider is nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureAuthenticated(ctx); err != nil {
		return nil, err
	}

	if err := s.fetchProxyCredentials(ctx); err != nil {
		return nil, err
	}

	// Ensure servers are loaded (unlock not needed, already holding lock).
	if len(s.servers) == 0 {
		s.mu.Unlock()
		servers, err := s.Servers(ctx)
		s.mu.Lock()
		if err != nil {
			return nil, err
		}
		s.servers = servers
	}

	server, err := s.pickServer(country)
	if err != nil {
		return nil, err
	}

	conn := &VPNConnection{
		Server:   *server,
		Protocol: "https",
		Port:     443,
	}
	s.connected = conn
	return conn, nil
}

// Disconnect clears the active connection state.
func (s *SurfsharkProvider) Disconnect(_ context.Context) error {
	if s == nil {
		return fmt.Errorf("scout: vpn: surfshark: provider is nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.connected = nil
	return nil
}

// Status returns the current connection status.
func (s *SurfsharkProvider) Status(_ context.Context) (*VPNStatus, error) {
	if s == nil {
		return nil, fmt.Errorf("scout: vpn: surfshark: provider is nil")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return &VPNStatus{
		Connected:  s.connected != nil,
		Connection: s.connected,
	}, nil
}

// ProxyCredentials returns the proxy username and password.
// Must be called after Connect.
func (s *SurfsharkProvider) ProxyCredentials() (username, password string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.proxyUser, s.proxyPass
}

// authenticate performs POST /v1/auth/login and stores the JWT tokens.
func (s *SurfsharkProvider) authenticate(ctx context.Context) error {
	body, err := json.Marshal(surfsharkLoginRequest{
		Email:    s.email,
		Password: s.password,
	})
	if err != nil {
		return fmt.Errorf("scout: vpn: surfshark: marshal login: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.apiBase+surfsharkLoginPath, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("scout: vpn: surfshark: create login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("scout: vpn: surfshark: login request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("scout: vpn: surfshark: login failed: status %d: %s", resp.StatusCode, string(respBody))
	}

	var loginResp surfsharkLoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return fmt.Errorf("scout: vpn: surfshark: parse login response: %w", err)
	}

	if loginResp.Token == "" {
		return fmt.Errorf("scout: vpn: surfshark: login returned empty token")
	}

	s.token = loginResp.Token
	s.renewToken = loginResp.RenewToken
	return nil
}

// fetchProxyCredentials fetches proxy auth credentials from GET /v1/server/user.
func (s *SurfsharkProvider) fetchProxyCredentials(ctx context.Context) error {
	if s.proxyUser != "" {
		return nil // already fetched
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.apiBase+surfsharkUserPath, nil)
	if err != nil {
		return fmt.Errorf("scout: vpn: surfshark: create creds request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.token)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("scout: vpn: surfshark: fetch proxy creds: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("scout: vpn: surfshark: fetch proxy creds: status %d: %s", resp.StatusCode, string(body))
	}

	var creds surfsharkProxyCreds
	if err := json.NewDecoder(resp.Body).Decode(&creds); err != nil {
		return fmt.Errorf("scout: vpn: surfshark: parse proxy creds: %w", err)
	}

	if creds.Username == "" {
		return fmt.Errorf("scout: vpn: surfshark: proxy credentials empty")
	}

	s.proxyUser = creds.Username
	s.proxyPass = creds.Password
	return nil
}

// ensureAuthenticated authenticates if no token is present.
func (s *SurfsharkProvider) ensureAuthenticated(ctx context.Context) error {
	if s.token != "" {
		return nil
	}
	return s.authenticate(ctx)
}

// pickServer selects the lowest-load server matching the given country code.
func (s *SurfsharkProvider) pickServer(country string) (*VPNServer, error) {
	country = strings.ToLower(country)

	var best *VPNServer
	for i := range s.servers {
		sv := &s.servers[i]
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

// FilterServersByCountry returns servers matching the given country code.
// Exported for testing and external use.
func FilterServersByCountry(servers []VPNServer, country string) []VPNServer {
	country = strings.ToLower(country)
	var result []VPNServer
	for _, s := range servers {
		if s.Country == country {
			result = append(result, s)
		}
	}
	return result
}

// ParseSurfsharkClusters parses the raw JSON response from
// GET /v5/server/clusters/all into VPNServer slices.
// Exported for testing.
func ParseSurfsharkClusters(data []byte) ([]VPNServer, error) {
	var clusters []surfsharkCluster
	if err := json.Unmarshal(data, &clusters); err != nil {
		return nil, fmt.Errorf("scout: vpn: surfshark: parse clusters: %w", err)
	}

	servers := make([]VPNServer, len(clusters))
	for i, c := range clusters {
		servers[i] = VPNServer{
			Host:    c.Host,
			Country: strings.ToLower(c.CountryCode),
			City:    c.City,
			Load:    int(c.Load),
			Tags:    c.Tags,
		}
	}
	return servers, nil
}
