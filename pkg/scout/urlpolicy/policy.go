// Package urlpolicy decides whether an untrusted-supplied URL may be navigated
// to. It is enforced at untrusted ingress (the MCP server and agent REST API)
// only; the Scout library and CLI are unaffected.
package urlpolicy

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
)

// Resolver looks a host up to IPs. Injecting it lets tests avoid real DNS.
type Resolver interface {
	LookupIP(ctx context.Context, host string) ([]net.IP, error)
}

type defaultResolver struct{}

func (defaultResolver) LookupIP(ctx context.Context, host string) ([]net.IP, error) {
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	ips := make([]net.IP, 0, len(addrs))
	for _, a := range addrs {
		ips = append(ips, a.IP)
	}
	return ips, nil
}

// Policy decides whether an untrusted-supplied URL may be navigated to.
type Policy struct {
	AllowLocal bool         // blanket opt-out: allow everything, including non-http schemes
	AllowCIDRs []*net.IPNet // granular allowlist of IP ranges
	AllowHosts []string     // granular allowlist of exact hostnames (case-insensitive)
	Resolver   Resolver     // nil → net.DefaultResolver
}

// BlockedError reports why a URL was denied.
type BlockedError struct {
	Reason string // "scheme" | "internal-ip" | "parse"
	Detail string
	URL    string
}

func (e *BlockedError) Error() string {
	return fmt.Sprintf("blocked %q: %s (%s). This endpoint denies internal/local targets by default; "+
		"restart with --allow-target <host|CIDR> or --allow-local-targets to permit it.", e.URL, e.Reason, e.Detail)
}

// Check reports whether rawURL may be navigated to. nil means allowed.
func (p Policy) Check(ctx context.Context, rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return &BlockedError{Reason: "parse", Detail: err.Error(), URL: rawURL}
	}

	// The scheme gate applies UNCONDITIONALLY — only http(s) is ever a fetchable
	// target. Enforced before the AllowLocal short-circuit so file://, gopher://,
	// data://, etc. can never slip through once an operator opts into local
	// targets (AllowLocal is meant for internal *HTTP* services, not arbitrary
	// schemes / local-file reads).
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return &BlockedError{Reason: "scheme", Detail: schemeOrEmpty(scheme), URL: rawURL}
	}

	// AllowLocal: operator explicitly opted into internal/loopback HTTP(S)
	// targets (e.g. local testing). Scheme is already enforced above.
	if p.AllowLocal {
		return nil
	}

	host := u.Hostname()
	if host == "" {
		return &BlockedError{Reason: "parse", Detail: "empty host", URL: rawURL}
	}
	for _, h := range p.AllowHosts {
		if strings.EqualFold(h, host) {
			return nil
		}
	}

	ips, err := p.resolve(ctx, host)
	if err != nil {
		return &BlockedError{Reason: "internal-ip", Detail: "unresolved: " + err.Error(), URL: rawURL}
	}

	for _, ip := range ips {
		if p.allowedIP(ip) {
			continue
		}
		if isInternalIP(ip) {
			return &BlockedError{Reason: "internal-ip", Detail: ip.String(), URL: rawURL}
		}
	}

	return nil
}

func schemeOrEmpty(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}

func (p Policy) resolve(ctx context.Context, host string) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, nil
	}
	r := p.Resolver
	if r == nil {
		r = defaultResolver{}
	}
	return r.LookupIP(ctx, host)
}

func (p Policy) allowedIP(ip net.IP) bool {
	for _, n := range p.AllowCIDRs {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// isInternalIP reports whether ip is in a range an untrusted caller must not reach.
func isInternalIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified()
}
