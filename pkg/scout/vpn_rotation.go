package scout

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// vpnRotator manages automatic VPN server rotation.
type vpnRotator struct {
	mu         sync.Mutex
	provider   VPNProvider
	config     VPNRotationConfig
	countries  []string // resolved country list
	index      int      // current country index (round-robin)
	lastRotate time.Time
	pageCount  int
}

// newVPNRotator creates a rotator from a provider and config.
func newVPNRotator(provider VPNProvider, config VPNRotationConfig) *vpnRotator {
	countries := config.Countries
	if len(countries) == 0 {
		countries = []string{""} // empty string = provider default/nearest
	}
	return &vpnRotator{
		provider:   provider,
		config:     config,
		countries:  countries,
		lastRotate: time.Now(),
	}
}

// shouldRotate returns true if it's time to rotate based on config.
func (r *vpnRotator) shouldRotate() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.config.PerPage {
		return true
	}

	if r.config.Interval > 0 {
		return time.Since(r.lastRotate) >= r.config.Interval
	}

	return false
}

// next returns the next country in round-robin order.
func (r *vpnRotator) next() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	country := r.countries[r.index%len(r.countries)]
	r.index++
	return country
}

// rotateIfNeeded connects to the next server if rotation is due.
// Returns the new connection or nil if no rotation happened.
func (r *vpnRotator) rotateIfNeeded(ctx context.Context) (*VPNConnection, error) {
	if r == nil || r.provider == nil {
		return nil, nil
	}

	if !r.shouldRotate() {
		return nil, nil
	}

	country := r.next()

	// Disconnect current connection first.
	if err := r.provider.Disconnect(ctx); err != nil {
		return nil, fmt.Errorf("scout: vpn: rotate: disconnect: %w", err)
	}

	conn, err := r.provider.Connect(ctx, country)
	if err != nil {
		return nil, fmt.Errorf("scout: vpn: rotate: connect %q: %w", country, err)
	}

	r.mu.Lock()
	r.lastRotate = time.Now()
	r.pageCount++
	r.mu.Unlock()

	return conn, nil
}
