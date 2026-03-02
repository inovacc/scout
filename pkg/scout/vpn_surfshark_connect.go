package scout

import (
	"context"
	"fmt"
	"sync"
)

// proxyAuthConfig holds proxy authentication credentials for the browser.
type proxyAuthConfig struct {
	username string
	password string
}

// vpnState tracks VPN connection state on a Browser instance.
type vpnState struct {
	mu         sync.Mutex
	conn       *VPNConnection
	auth       *proxyAuthConfig
	authCancel func() error // cleanup for HandleAuth
}

// ConnectVPN sets up VPN proxy at the browser level.
// It configures proxy authentication via rod's HandleAuth and stores the
// connection state on the Browser for status queries.
//
// For a full workflow, use SurfsharkProvider.Connect() first to get the
// connection details and proxy credentials, then call this method.
func (b *Browser) ConnectVPN(_ context.Context, conn *VPNConnection, username, password string) error {
	if b == nil || b.browser == nil {
		return fmt.Errorf("scout: browser is nil")
	}

	if conn == nil {
		return fmt.Errorf("scout: vpn: connection is nil")
	}

	if b.vpn == nil {
		b.vpn = &vpnState{}
	}

	b.vpn.mu.Lock()
	defer b.vpn.mu.Unlock()

	// Clean up any previous auth handler.
	if b.vpn.authCancel != nil {
		_ = b.vpn.authCancel()
		b.vpn.authCancel = nil
	}

	// Set up proxy authentication using rod's HandleAuth (uses CDP Fetch.AuthRequired).
	waitAuth := b.browser.HandleAuth(username, password)
	b.vpn.authCancel = waitAuth

	b.vpn.conn = conn
	b.vpn.auth = &proxyAuthConfig{
		username: username,
		password: password,
	}

	return nil
}

// DisconnectVPN clears VPN connection state and proxy auth handlers.
func (b *Browser) DisconnectVPN() error {
	if b == nil || b.browser == nil {
		return fmt.Errorf("scout: browser is nil")
	}

	if b.vpn == nil {
		return nil
	}

	b.vpn.mu.Lock()
	defer b.vpn.mu.Unlock()

	if b.vpn.authCancel != nil {
		_ = b.vpn.authCancel()
		b.vpn.authCancel = nil
	}

	b.vpn.conn = nil
	b.vpn.auth = nil

	return nil
}

// VPNStatus returns the current VPN connection status for the browser.
func (b *Browser) VPNStatus() *VPNStatus {
	if b == nil || b.vpn == nil {
		return &VPNStatus{Connected: false}
	}

	b.vpn.mu.Lock()
	defer b.vpn.mu.Unlock()

	if b.vpn.conn == nil {
		return &VPNStatus{Connected: false}
	}

	return &VPNStatus{
		Connected:  true,
		Connection: b.vpn.conn,
	}
}
