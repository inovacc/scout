// Package auth provides a generic authentication framework for web scrapers.
// It extracts the auth-then-scrape pattern into reusable components that any
// scraper mode (Slack, Teams, Discord, etc.) can implement.
package auth

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/inovacc/scout/pkg/scout"
)

// Session holds all captured browser session data for any provider.
type Session struct {
	Provider       string            `json:"provider"`
	Version        string            `json:"version"`
	Timestamp      time.Time         `json:"timestamp"`
	URL            string            `json:"url"`
	Cookies        []scout.Cookie    `json:"cookies"`
	Tokens         map[string]string `json:"tokens,omitempty"`
	LocalStorage   map[string]string `json:"local_storage,omitempty"`
	SessionStorage map[string]string `json:"session_storage,omitempty"`
	Extra          map[string]string `json:"extra,omitempty"`
	ExpiresAt      time.Time         `json:"expires_at,omitzero"`
}

// Provider defines the interface that each auth provider must implement.
type Provider interface {
	// Name returns the unique provider identifier (e.g. "slack", "teams").
	Name() string

	// LoginURL returns the URL to open for user authentication.
	LoginURL() string

	// DetectAuth checks if the page has valid authentication state.
	// It is called repeatedly while polling for login completion.
	DetectAuth(ctx context.Context, page *scout.Page) (bool, error)

	// CaptureSession extracts provider-specific session data from an authenticated page.
	// Called after DetectAuth returns true, right before browser close.
	CaptureSession(ctx context.Context, page *scout.Page) (*Session, error)

	// ValidateSession checks if a previously captured session is still valid.
	ValidateSession(ctx context.Context, session *Session) error
}

// Registry holds registered auth providers.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

// DefaultRegistry is the global provider registry.
var DefaultRegistry = &Registry{
	providers: make(map[string]Provider),
}

// Register adds a provider to the registry.
func (r *Registry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.providers[p.Name()] = p
}

// Get returns a provider by name.
func (r *Registry) Get(name string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("auth: unknown provider %q", name)
	}

	return p, nil
}

// List returns all registered provider names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}

	return names
}

// Register adds a provider to the default registry.
func Register(p Provider) {
	DefaultRegistry.Register(p)
}

// Get returns a provider from the default registry.
func Get(name string) (Provider, error) {
	return DefaultRegistry.Get(name)
}

// List returns all provider names from the default registry.
func List() []string {
	return DefaultRegistry.List()
}
