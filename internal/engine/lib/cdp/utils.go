package cdp

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"

	"github.com/inovacc/scout/internal/engine/lib/utils"
)

// Dialer interface for WebSocket connection.
type Dialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

// tlsDialer wraps tls.Dialer to satisfy the Dialer interface.
type tlsDialer struct{}

func (d *tlsDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return (&tls.Dialer{}).DialContext(ctx, network, address)
}

// MustConnectWS helper to make a websocket connection.
func MustConnectWS(wsURL string) WebSocketable {
	ws := &WebSocket{}
	utils.E(ws.Connect(context.Background(), wsURL, nil))

	return ws
}

// MustStartWithURL helper for ConnectURL.
func MustStartWithURL(ctx context.Context, u string, h http.Header) *Client {
	c, err := StartWithURL(ctx, u, h)
	utils.E(err)

	return c
}

// StartWithURL helper to connect to the u with the default websocket lib.
func StartWithURL(ctx context.Context, u string, h http.Header) (*Client, error) {
	ws := &WebSocket{}

	err := ws.Connect(ctx, u, h)
	if err != nil {
		return nil, err
	}

	return New().Start(ws), nil
}
