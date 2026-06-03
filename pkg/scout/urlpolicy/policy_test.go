package urlpolicy

import (
	"context"
	"net"
	"testing"
)

// fakeResolver maps hostnames to fixed IPs so tests need no real DNS.
type fakeResolver map[string][]net.IP

func (f fakeResolver) LookupIP(_ context.Context, host string) ([]net.IP, error) {
	if ips, ok := f[host]; ok {
		return ips, nil
	}
	return nil, &net.DNSError{Err: "not found", Name: host}
}

func TestCheck(t *testing.T) {
	res := fakeResolver{
		"example.com": {net.ParseIP("93.184.216.34")},
		"evil.test":   {net.ParseIP("127.0.0.1")},
		"mixed.test":  {net.ParseIP("93.184.216.34"), net.ParseIP("10.0.0.5")},
	}
	base := Policy{Resolver: res}

	allowed := []string{
		"https://example.com/path?q=1",
		"http://example.com",
	}
	for _, u := range allowed {
		if err := base.Check(context.Background(), u); err != nil {
			t.Errorf("Check(%q) = %v, want allowed", u, err)
		}
	}

	blocked := []string{
		"file:///etc/passwd",
		"data:text/html,<h1>x</h1>",
		"chrome://settings",
		"http://127.0.0.1",
		"http://169.254.169.254/latest/meta-data/",
		"http://10.0.0.1",
		"http://192.168.1.5:8080",
		"http://[::1]/",
		"http://evil.test",  // DNS bypass: name → loopback
		"http://mixed.test", // any internal IP in the set blocks
		"not a url ::::",
	}
	for _, u := range blocked {
		var be *BlockedError
		err := base.Check(context.Background(), u)
		if err == nil {
			t.Errorf("Check(%q) = nil, want blocked", u)
			continue
		}
		if !asBlocked(err, &be) {
			t.Errorf("Check(%q) = %v, want *BlockedError", u, err)
		}
	}
}

func TestCheckAllowLocalBypass(t *testing.T) {
	p := Policy{AllowLocal: true}
	for _, u := range []string{"file:///etc/passwd", "http://127.0.0.1", "http://169.254.169.254"} {
		if err := p.Check(context.Background(), u); err != nil {
			t.Errorf("AllowLocal Check(%q) = %v, want allowed", u, err)
		}
	}
}

func TestCheckAllowlist(t *testing.T) {
	res := fakeResolver{"box.local": {net.ParseIP("192.168.1.50")}}
	_, cidr, _ := net.ParseCIDR("192.168.1.0/24")
	p := Policy{Resolver: res, AllowCIDRs: []*net.IPNet{cidr}}
	if err := p.Check(context.Background(), "http://box.local"); err != nil {
		t.Errorf("allowlisted CIDR Check = %v, want allowed", err)
	}

	p2 := Policy{Resolver: res, AllowHosts: []string{"box.local"}}
	if err := p2.Check(context.Background(), "http://box.local"); err != nil {
		t.Errorf("allowlisted host Check = %v, want allowed", err)
	}
}

// asBlocked is a tiny errors.As helper kept local to avoid an import in the test body.
func asBlocked(err error, target **BlockedError) bool {
	be, ok := err.(*BlockedError)
	if ok {
		*target = be
	}
	return ok
}
