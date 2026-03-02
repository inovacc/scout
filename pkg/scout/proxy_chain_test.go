package scout

import (
	"testing"
)

func TestValidateProxyChain(t *testing.T) {
	tests := []struct {
		name    string
		chain   *ProxyChain
		wantErr bool
	}{
		{"nil chain", nil, true},
		{"empty hops", &ProxyChain{Hops: nil}, true},
		{"valid socks5", &ProxyChain{Hops: []ProxyHop{{URL: "socks5://localhost:1080"}}}, false},
		{"valid http", &ProxyChain{Hops: []ProxyHop{{URL: "http://proxy.example.com:8080"}}}, false},
		{"valid multi-hop", &ProxyChain{Hops: []ProxyHop{
			{URL: "socks5://hop1:1080"},
			{URL: "http://hop2:8080"},
		}}, false},
		{"valid with auth", &ProxyChain{Hops: []ProxyHop{
			{URL: "socks5://user:pass@localhost:1080"},
		}}, false},
		{"bad scheme", &ProxyChain{Hops: []ProxyHop{{URL: "ftp://host:21"}}}, true},
		{"no host", &ProxyChain{Hops: []ProxyHop{{URL: "socks5://:1080"}}}, true},
		{"invalid URL", &ProxyChain{Hops: []ProxyHop{{URL: "://bad"}}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProxyChain(tt.chain)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateProxyChain() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWithProxyChain(t *testing.T) {
	o := defaults()
	WithProxyChain(ProxyHop{URL: "socks5://localhost:1080"})(o)

	if o.proxy != "socks5://localhost:1080" {
		t.Fatalf("expected proxy set, got %q", o.proxy)
	}

	if o.proxyChain == nil || len(o.proxyChain.Hops) != 1 {
		t.Fatal("expected proxy chain with 1 hop")
	}
}

func TestWithProxyChainAuth(t *testing.T) {
	o := defaults()
	WithProxyChain(ProxyHop{URL: "socks5://user:secret@localhost:1080"})(o)

	if o.proxyAuth == nil {
		t.Fatal("expected proxy auth extracted")
	}

	if o.proxyAuth.username != "user" || o.proxyAuth.password != "secret" {
		t.Fatalf("unexpected auth: %+v", o.proxyAuth)
	}
	// Proxy URL should have auth stripped (handled via CDP).
	if o.proxy != "socks5://localhost:1080" {
		t.Fatalf("expected auth stripped from proxy URL, got %q", o.proxy)
	}
}

func TestWithProxyChainEmpty(t *testing.T) {
	o := defaults()
	WithProxyChain()(o) // no hops

	if o.proxy != "" {
		t.Fatalf("expected empty proxy, got %q", o.proxy)
	}
}

func TestProxyChainDescription(t *testing.T) {
	tests := []struct {
		name  string
		chain *ProxyChain
		want  string
	}{
		{"nil", nil, "direct"},
		{"single", &ProxyChain{Hops: []ProxyHop{{URL: "socks5://host:1080"}}}, "socks5://host:1080"},
		{"multi", &ProxyChain{Hops: []ProxyHop{
			{URL: "socks5://hop1:1080"},
			{URL: "http://hop2:8080"},
		}}, "socks5://hop1:1080 → http://hop2:8080"},
		{"masked auth", &ProxyChain{Hops: []ProxyHop{
			{URL: "socks5://user:pass@host:1080"},
		}}, "socks5://***@host:1080"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ProxyChainDescription(tt.chain)
			if got != tt.want {
				t.Errorf("ProxyChainDescription() = %q, want %q", got, tt.want)
			}
		})
	}
}
