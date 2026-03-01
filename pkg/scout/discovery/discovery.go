// Package discovery provides mDNS-based service discovery for scout instances.
package discovery

import (
	"context"
	"fmt"
	"net"

	"github.com/grandcat/zeroconf"
)

const serviceType = "_scout._tcp"

// Peer represents a discovered scout instance.
type Peer struct {
	DeviceID string
	Host     string
	Port     int
	Addrs    []net.IP
}

// Announcer broadcasts a scout instance on the local network via mDNS.
type Announcer struct {
	server *zeroconf.Server
}

// Announce registers a scout instance as an mDNS service with the device ID in a TXT record.
func Announce(port int, deviceID string) (*Announcer, error) {
	srv, err := zeroconf.Register(
		"scout-"+deviceID[:7], // instance name (short ID)
		serviceType,
		"local.",
		port,
		[]string{"deviceid=" + deviceID},
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("discovery: announce: %w", err)
	}

	return &Announcer{server: srv}, nil
}

// Stop stops the mDNS announcement.
func (a *Announcer) Stop() {
	if a != nil && a.server != nil {
		a.server.Shutdown()
	}
}

// Discover browses the local network for scout instances via mDNS.
// The returned channel receives discovered peers until the context is cancelled.
func Discover(ctx context.Context) (<-chan Peer, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, fmt.Errorf("discovery: create resolver: %w", err)
	}

	entries := make(chan *zeroconf.ServiceEntry)
	peers := make(chan Peer)

	go func() {
		defer close(peers)
		for entry := range entries {
			var deviceID string
			for _, txt := range entry.Text {
				if len(txt) > 9 && txt[:9] == "deviceid=" {
					deviceID = txt[9:]
				}
			}
			if deviceID == "" {
				continue
			}

			addrs := make([]net.IP, 0, len(entry.AddrIPv4)+len(entry.AddrIPv6))
			addrs = append(addrs, entry.AddrIPv4...)
			addrs = append(addrs, entry.AddrIPv6...)

			peers <- Peer{
				DeviceID: deviceID,
				Host:     entry.HostName,
				Port:     entry.Port,
				Addrs:    addrs,
			}
		}
	}()

	if err := resolver.Browse(ctx, serviceType, "local.", entries); err != nil {
		close(entries)
		return nil, fmt.Errorf("discovery: browse: %w", err)
	}

	return peers, nil
}
