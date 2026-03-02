package scout

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// mockVPNProvider implements VPNProvider for testing.
type mockVPNProvider struct {
	mu           sync.Mutex
	connectCalls []string // country args passed to Connect
	disconnects  int
	connectErr   error
}

func (m *mockVPNProvider) Name() string { return "mock" }

func (m *mockVPNProvider) Servers(_ context.Context) ([]VPNServer, error) {
	return []VPNServer{
		{Host: "us.mock.vpn", Country: "us"},
		{Host: "de.mock.vpn", Country: "de"},
		{Host: "jp.mock.vpn", Country: "jp"},
	}, nil
}

func (m *mockVPNProvider) Connect(_ context.Context, country string) (*VPNConnection, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.connectErr != nil {
		return nil, m.connectErr
	}

	m.connectCalls = append(m.connectCalls, country)

	host := country + ".mock.vpn"
	if country == "" {
		host = "auto.mock.vpn"
	}

	return &VPNConnection{
		Server:   VPNServer{Host: host, Country: country},
		Protocol: "socks5",
		Port:     1080,
	}, nil
}

func (m *mockVPNProvider) Disconnect(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.disconnects++

	return nil
}

func (m *mockVPNProvider) Status(_ context.Context) (*VPNStatus, error) {
	return &VPNStatus{Connected: true}, nil
}

func TestVPNRotator_New(t *testing.T) {
	p := &mockVPNProvider{}
	cfg := VPNRotationConfig{
		Countries: []string{"us", "de", "jp"},
		PerPage:   true,
	}

	r := newVPNRotator(p, cfg)
	if r == nil {
		t.Fatal("expected non-nil rotator")
	}

	if len(r.countries) != 3 {
		t.Fatalf("expected 3 countries, got %d", len(r.countries))
	}

	if r.index != 0 {
		t.Fatalf("expected index 0, got %d", r.index)
	}
}

func TestVPNRotator_New_EmptyCountries(t *testing.T) {
	p := &mockVPNProvider{}
	cfg := VPNRotationConfig{PerPage: true}

	r := newVPNRotator(p, cfg)
	if len(r.countries) != 1 || r.countries[0] != "" {
		t.Fatalf("expected single empty country, got %v", r.countries)
	}
}

func TestVPNRotator_ShouldRotate_PerPage(t *testing.T) {
	r := newVPNRotator(&mockVPNProvider{}, VPNRotationConfig{PerPage: true})
	if !r.shouldRotate() {
		t.Fatal("expected shouldRotate=true for PerPage")
	}
	// Should always return true for PerPage.
	if !r.shouldRotate() {
		t.Fatal("expected shouldRotate=true on second call")
	}
}

func TestVPNRotator_ShouldRotate_Interval(t *testing.T) {
	r := newVPNRotator(&mockVPNProvider{}, VPNRotationConfig{Interval: 50 * time.Millisecond})

	// Just created — interval hasn't passed.
	if r.shouldRotate() {
		t.Fatal("expected shouldRotate=false immediately after creation")
	}

	// Wait for interval to elapse.
	time.Sleep(60 * time.Millisecond)

	if !r.shouldRotate() {
		t.Fatal("expected shouldRotate=true after interval elapsed")
	}
}

func TestVPNRotator_ShouldRotate_NoConfig(t *testing.T) {
	r := newVPNRotator(&mockVPNProvider{}, VPNRotationConfig{})
	if r.shouldRotate() {
		t.Fatal("expected shouldRotate=false with no PerPage and no Interval")
	}
}

func TestVPNRotator_Next_RoundRobin(t *testing.T) {
	r := newVPNRotator(&mockVPNProvider{}, VPNRotationConfig{
		Countries: []string{"us", "de", "jp"},
	})

	expected := []string{"us", "de", "jp", "us", "de", "jp"}
	for i, want := range expected {
		got := r.next()
		if got != want {
			t.Fatalf("call %d: expected %q, got %q", i, want, got)
		}
	}
}

func TestVPNRotator_Next_SingleCountry(t *testing.T) {
	r := newVPNRotator(&mockVPNProvider{}, VPNRotationConfig{
		Countries: []string{"br"},
	})
	for i := range 5 {
		if got := r.next(); got != "br" {
			t.Fatalf("call %d: expected %q, got %q", i, "br", got)
		}
	}
}

func TestVPNRotator_RotateIfNeeded(t *testing.T) {
	mp := &mockVPNProvider{}
	r := newVPNRotator(mp, VPNRotationConfig{
		Countries: []string{"us", "de"},
		PerPage:   true,
	})

	conn, err := r.rotateIfNeeded(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if conn == nil {
		t.Fatal("expected non-nil connection")
	}

	if conn.Server.Country != "us" {
		t.Fatalf("expected country us, got %q", conn.Server.Country)
	}

	// Second call should rotate to de.
	conn, err = r.rotateIfNeeded(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if conn.Server.Country != "de" {
		t.Fatalf("expected country de, got %q", conn.Server.Country)
	}

	if mp.disconnects != 2 {
		t.Fatalf("expected 2 disconnects, got %d", mp.disconnects)
	}
}

func TestVPNRotator_RotateIfNeeded_NoRotation(t *testing.T) {
	r := newVPNRotator(&mockVPNProvider{}, VPNRotationConfig{
		Interval: 1 * time.Hour, // Won't trigger.
	})

	conn, err := r.rotateIfNeeded(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if conn != nil {
		t.Fatal("expected nil connection when no rotation needed")
	}
}

func TestVPNRotator_RotateIfNeeded_NilRotator(t *testing.T) {
	var r *vpnRotator

	conn, err := r.rotateIfNeeded(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if conn != nil {
		t.Fatal("expected nil connection for nil rotator")
	}
}

func TestVPNRotator_RotateIfNeeded_ConnectError(t *testing.T) {
	mp := &mockVPNProvider{connectErr: fmt.Errorf("network down")}
	r := newVPNRotator(mp, VPNRotationConfig{PerPage: true})

	conn, err := r.rotateIfNeeded(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}

	if conn != nil {
		t.Fatal("expected nil connection on error")
	}
}
