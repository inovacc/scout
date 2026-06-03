package urlpolicy

import (
	"net"
	"testing"
)

func TestParseAllowTargets(t *testing.T) {
	hosts, cidrs := ParseAllowTargets([]string{
		"192.168.1.0/24", // CIDR
		"127.0.0.1",      // bare IPv4 → /32
		"::1",            // bare IPv6 → /128
		"box.local",      // hostname
		"  ",             // ignored
	})

	if len(hosts) != 1 || hosts[0] != "box.local" {
		t.Errorf("hosts = %v, want [box.local]", hosts)
	}
	if len(cidrs) != 3 {
		t.Fatalf("cidrs = %d, want 3", len(cidrs))
	}
	if !cidrs[1].Contains(net.ParseIP("127.0.0.1")) {
		t.Errorf("bare IPv4 not parsed to containing CIDR")
	}
}

func TestFromEnv(t *testing.T) {
	t.Setenv("SCOUT_ALLOW_LOCAL_TARGETS", "1")
	t.Setenv("SCOUT_ALLOW_TARGETS", "10.0.0.0/8, host.example")
	p := FromEnv()
	if !p.AllowLocal {
		t.Error("AllowLocal = false, want true")
	}
	if len(p.AllowCIDRs) != 1 || len(p.AllowHosts) != 1 {
		t.Errorf("got %d cidrs %d hosts, want 1/1", len(p.AllowCIDRs), len(p.AllowHosts))
	}
}
