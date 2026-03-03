package vpn

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Rotator manages automatic VPN server rotation.
type Rotator struct {
	mu         sync.Mutex
	Provider   Provider
	Config     RotationConfig
	Countries  []string // resolved country list
	Index      int      // current country index (round-robin)
	LastRotate time.Time
	PageCount  int
}

// NewRotator creates a rotator from a provider and config.
func NewRotator(provider Provider, config RotationConfig) *Rotator {
	countries := config.Countries
	if len(countries) == 0 {
		countries = []string{""} // empty string = provider default/nearest
	}

	return &Rotator{
		Provider:   provider,
		Config:     config,
		Countries:  countries,
		LastRotate: time.Now(),
	}
}

// ShouldRotate returns true if it's time to rotate based on config.
func (r *Rotator) ShouldRotate() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.Config.PerPage {
		return true
	}

	if r.Config.Interval > 0 {
		return time.Since(r.LastRotate) >= r.Config.Interval
	}

	return false
}

// Next returns the next country in round-robin order.
func (r *Rotator) Next() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	country := r.Countries[r.Index%len(r.Countries)]
	r.Index++

	return country
}

// RotateIfNeeded connects to the next server if rotation is due.
// Returns the new connection or nil if no rotation happened.
func (r *Rotator) RotateIfNeeded(ctx context.Context) (*Connection, error) {
	if r == nil || r.Provider == nil {
		return nil, nil //nolint:nilnil // no rotation needed when provider is nil
	}

	if !r.ShouldRotate() {
		return nil, nil //nolint:nilnil
	}

	country := r.Next()

	// Disconnect current connection first.
	if err := r.Provider.Disconnect(ctx); err != nil {
		return nil, fmt.Errorf("scout: vpn: rotate: disconnect: %w", err)
	}

	conn, err := r.Provider.Connect(ctx, country)
	if err != nil {
		return nil, fmt.Errorf("scout: vpn: rotate: connect %q: %w", country, err)
	}

	r.mu.Lock()
	r.LastRotate = time.Now()
	r.PageCount++
	r.mu.Unlock()

	return conn, nil
}
