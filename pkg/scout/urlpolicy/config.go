package urlpolicy

import (
	"net"
	"os"
	"strings"
)

// FromEnv builds a Policy from SCOUT_ALLOW_LOCAL_TARGETS (bool) and
// SCOUT_ALLOW_TARGETS (comma-separated host/CIDR list).
func FromEnv() *Policy {
	hosts, cidrs := ParseAllowTargets(splitList(os.Getenv("SCOUT_ALLOW_TARGETS")))
	return &Policy{
		AllowLocal: envTrue(os.Getenv("SCOUT_ALLOW_LOCAL_TARGETS")),
		AllowHosts: hosts,
		AllowCIDRs: cidrs,
	}
}

// ParseAllowTargets splits entries into exact hostnames and CIDR ranges. A bare
// IP becomes a single-address CIDR; anything else is treated as a hostname.
func ParseAllowTargets(entries []string) (hosts []string, cidrs []*net.IPNet) {
	for _, e := range entries {
		if e = strings.TrimSpace(e); e == "" {
			continue
		}
		if _, n, err := net.ParseCIDR(e); err == nil {
			cidrs = append(cidrs, n)
			continue
		}
		if ip := net.ParseIP(e); ip != nil {
			bits := 32
			if ip.To4() == nil {
				bits = 128
			}
			cidrs = append(cidrs, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
			continue
		}
		hosts = append(hosts, e)
	}
	return hosts, cidrs
}

func envTrue(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func splitList(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
