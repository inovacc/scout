package scout

import (
	"fmt"
	"net/url"
	"strings"
)

// ProxyHop represents a single proxy in a chain.
type ProxyHop struct {
	// URL is the full proxy URL (e.g. "socks5://host:1080", "http://user:pass@host:8080").
	URL string
}

// ProxyChain represents an ordered list of proxies to route through.
// Chrome only supports a single --proxy-server flag, so chaining is achieved
// by nesting proxies: each hop forwards traffic to the next.
//
// For two-hop chains (SOCKS5 → HTTP or SOCKS5 → SOCKS5), use a local
// forwarding proxy or set up the chain externally and provide the entry
// point via WithProxy.
//
// WithProxyChain validates the chain and configures the browser to use the
// first hop. For true multi-hop chaining, use an external tool like
// proxychains-ng, redsocks, or gost, and point WithProxy at the local entry.
type ProxyChain struct {
	Hops []ProxyHop
}

// WithProxyChain configures a proxy chain. The browser connects through the
// first hop. If only one hop is provided, it behaves identically to WithProxy.
//
// For multi-hop chains, the last hop is the exit proxy (closest to the target).
// Intermediate hops must be configured to forward to the next hop externally,
// since Chrome's --proxy-server only accepts a single upstream proxy.
//
// Common patterns:
//   - Single hop: WithProxyChain(ProxyHop{URL: "socks5://localhost:1080"})
//   - Two-hop with gost: gost -L :1080 -F socks5://hop1:1080 -F http://hop2:8080
//     then: WithProxyChain(ProxyHop{URL: "socks5://localhost:1080"})
func WithProxyChain(hops ...ProxyHop) Option {
	return func(o *options) {
		if len(hops) == 0 {
			return
		}
		o.proxyChain = &ProxyChain{Hops: hops}
		// Set the entry proxy for Chrome's --proxy-server flag.
		o.proxy = hops[0].URL
		// Extract auth from the entry proxy URL if present.
		if u, err := url.Parse(hops[0].URL); err == nil && u.User != nil {
			pass, _ := u.User.Password()
			o.proxyAuth = &proxyAuthConfig{
				username: u.User.Username(),
				password: pass,
			}
			// Strip auth from the proxy URL for Chrome (auth is handled via CDP).
			u.User = nil
			o.proxy = u.String()
		}
	}
}

// ValidateProxyChain checks that all hops in the chain have valid URLs
// with supported schemes.
func ValidateProxyChain(chain *ProxyChain) error {
	if chain == nil || len(chain.Hops) == 0 {
		return fmt.Errorf("scout: proxy chain: empty chain")
	}

	supportedSchemes := map[string]bool{
		"http": true, "https": true, "socks4": true, "socks5": true,
	}

	for i, hop := range chain.Hops {
		u, err := url.Parse(hop.URL)
		if err != nil {
			return fmt.Errorf("scout: proxy chain: hop %d: invalid URL %q: %w", i, hop.URL, err)
		}
		scheme := strings.ToLower(u.Scheme)
		if !supportedSchemes[scheme] {
			return fmt.Errorf("scout: proxy chain: hop %d: unsupported scheme %q (use http, https, socks4, or socks5)", i, scheme)
		}
		if u.Hostname() == "" {
			return fmt.Errorf("scout: proxy chain: hop %d: missing host", i)
		}
	}

	return nil
}

// ProxyChainDescription returns a human-readable description of the chain
// for logging and debugging. Auth credentials are masked.
func ProxyChainDescription(chain *ProxyChain) string {
	if chain == nil || len(chain.Hops) == 0 {
		return "direct"
	}

	parts := make([]string, len(chain.Hops))
	for i, hop := range chain.Hops {
		u, err := url.Parse(hop.URL)
		if err != nil {
			parts[i] = hop.URL
			continue
		}
		if u.User != nil {
			u.User = url.User("***")
		}
		parts[i] = u.String()
	}

	return strings.Join(parts, " → ")
}
