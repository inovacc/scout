package discovery

import (
	"net"
	"testing"
)

// TestAnnouncerStopNilSafe verifies Stop() is nil-safe and idempotent across
// the receiver-nil and server-nil branches of the guard `a != nil && a.server != nil`.
// These branches contain real guard logic and must never panic.
func TestAnnouncerStopNilSafe(t *testing.T) {
	tests := []struct {
		name string
		a    *Announcer
	}{
		{
			name: "nil receiver",
			a:    nil,
		},
		{
			name: "non-nil receiver with nil server",
			a:    &Announcer{},
		},
		{
			name: "zero-value announcer via composite literal",
			a:    new(Announcer),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("Stop() panicked: %v", r)
				}
			}()

			// First call must not panic.
			tt.a.Stop()
			// Second call must remain safe (idempotent).
			tt.a.Stop()
		})
	}
}

// TestPeerFields verifies the Peer value type carries the fields the discovery
// goroutine populates and that an empty Peer has the expected zero values.
func TestPeerFields(t *testing.T) {
	ip4 := net.IPv4(192, 168, 1, 10)
	ip6 := net.ParseIP("fe80::1")

	p := Peer{
		DeviceID: "abc1234",
		Host:     "scout-host.local.",
		Port:     9551,
		Addrs:    []net.IP{ip4, ip6},
	}

	if p.DeviceID != "abc1234" {
		t.Errorf("DeviceID = %q, want %q", p.DeviceID, "abc1234")
	}
	if p.Host != "scout-host.local." {
		t.Errorf("Host = %q, want %q", p.Host, "scout-host.local.")
	}
	if p.Port != 9551 {
		t.Errorf("Port = %d, want %d", p.Port, 9551)
	}
	if got := len(p.Addrs); got != 2 {
		t.Fatalf("len(Addrs) = %d, want 2", got)
	}
	if !p.Addrs[0].Equal(ip4) {
		t.Errorf("Addrs[0] = %v, want %v", p.Addrs[0], ip4)
	}
	if !p.Addrs[1].Equal(ip6) {
		t.Errorf("Addrs[1] = %v, want %v", p.Addrs[1], ip6)
	}
}

// TestPeerZeroValue pins the zero value of Peer so callers relying on the empty
// state (e.g. a not-yet-populated peer) stay in sync with the struct shape.
func TestPeerZeroValue(t *testing.T) {
	var p Peer

	if p.DeviceID != "" {
		t.Errorf("zero DeviceID = %q, want empty", p.DeviceID)
	}
	if p.Host != "" {
		t.Errorf("zero Host = %q, want empty", p.Host)
	}
	if p.Port != 0 {
		t.Errorf("zero Port = %d, want 0", p.Port)
	}
	if p.Addrs != nil {
		t.Errorf("zero Addrs = %v, want nil", p.Addrs)
	}
}

// TestServiceTypeConstant guards the mDNS service type string used by both
// Announce and Discover. A drift here silently breaks announce/discover parity.
func TestServiceTypeConstant(t *testing.T) {
	if serviceType != "_scout._tcp" {
		t.Errorf("serviceType = %q, want %q", serviceType, "_scout._tcp")
	}
}
