package server

import (
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

// ConnectedPeer represents a connected client instance.
type ConnectedPeer struct {
	DeviceID    string
	ShortID     string
	Addr        string
	ConnectedAt time.Time
	Sessions    int
}

// ServerInfo holds server metadata for display.
type ServerInfo struct {
	DeviceID   string
	ListenAddr string
	Insecure   bool
	LocalIPs   []string
}

// PrintServerTable renders a box-drawing table with server info and connected peers.
func PrintServerTable(w io.Writer, info ServerInfo, peers []ConnectedPeer) {
	const width = 82

	mode := "mTLS"
	if info.Insecure {
		mode = "Insecure"
	}

	ips := "(none)"
	if len(info.LocalIPs) > 0 {
		ips = strings.Join(info.LocalIPs, ", ")
	}

	deviceID := info.DeviceID
	if deviceID == "" {
		deviceID = "(none)"
	}

	// Header
	_, _ = fmt.Fprintf(w, "┌%s┐\n", strings.Repeat("─", width))
	_, _ = fmt.Fprintf(w, "│ %-*s│\n", width-1, "Scout Server")
	_, _ = fmt.Fprintf(w, "├%s┬%s┤\n", strings.Repeat("─", 14), strings.Repeat("─", width-15))

	// Info rows
	printKV(w, width, "Device ID", truncate(deviceID, width-18))
	printKV(w, width, "Listen", info.ListenAddr)
	printKV(w, width, "Local IPs", truncate(ips, width-18))
	printKV(w, width, "Mode", mode)

	// Connected instances section
	_, _ = fmt.Fprintf(w, "├%s┴%s┤\n", strings.Repeat("─", 14), strings.Repeat("─", width-15))
	_, _ = fmt.Fprintf(w, "│ %-*s│\n", width-1, fmt.Sprintf("Connected Instances (%d)", len(peers)))

	if len(peers) > 0 {
		// Column widths: Short ID=10, Device ID=35, Address=17, Sessions=remainder
		const (
			colShort   = 10
			colDevice  = 35
			colAddr    = 17
			colBorders = 3 // 3 inner borders (┬/┼/┴ between 4 columns)
		)
		colSess := width - colShort - colDevice - colAddr - colBorders

		_, _ = fmt.Fprintf(w, "├%s┬%s┬%s┬%s┤\n",
			strings.Repeat("─", colShort), strings.Repeat("─", colDevice), strings.Repeat("─", colAddr), strings.Repeat("─", colSess))
		_, _ = fmt.Fprintf(w, "│ %-*s│ %-*s│ %-*s│ %-*s│\n",
			colShort-1, "Short ID", colDevice-1, "Device ID", colAddr-1, "Address", colSess-1, "Sessions")
		_, _ = fmt.Fprintf(w, "├%s┼%s┼%s┼%s┤\n",
			strings.Repeat("─", colShort), strings.Repeat("─", colDevice), strings.Repeat("─", colAddr), strings.Repeat("─", colSess))

		for _, p := range peers {
			_, _ = fmt.Fprintf(w, "│ %-*s│ %-*s│ %-*s│ %-*d│\n",
				colShort-1, p.ShortID,
				colDevice-1, truncate(p.DeviceID, colDevice-2),
				colAddr-1, truncate(p.Addr, colAddr-2),
				colSess-1, p.Sessions)
		}

		_, _ = fmt.Fprintf(w, "└%s┴%s┴%s┴%s┘\n",
			strings.Repeat("─", colShort), strings.Repeat("─", colDevice), strings.Repeat("─", colAddr), strings.Repeat("─", colSess))
	} else {
		_, _ = fmt.Fprintf(w, "└%s┘\n", strings.Repeat("─", width))
	}
}

func printKV(w io.Writer, width int, key, value string) {
	_, _ = fmt.Fprintf(w, "│ %-12s │ %-*s│\n", key, width-16, value)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// GetLocalIPs returns non-loopback IPv4 addresses.
func GetLocalIPs() []string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}

	var ips []string
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		ip := ipNet.IP
		if ip.IsLoopback() || ip.To4() == nil {
			continue
		}
		ips = append(ips, ip.String())
	}
	return ips
}
