package proxy

import "testing"

// TestGuardProxyBind verifies the proxy fails closed on a non-loopback bind
// unless a token is configured (or the explicit opt-in env is set).
func TestGuardProxyBind(t *testing.T) {
	t.Setenv("SCOUT_PROXY_ALLOW_REMOTE", "")

	cases := []struct {
		addr    string
		token   string
		wantErr bool
	}{
		{"127.0.0.1:8080", "", false},
		{"localhost:8080", "", false},
		{"[::1]:8080", "", false},
		{"0.0.0.0:8080", "", true},
		{":8080", "", true},
		{"10.0.0.1:8080", "", true},
		{"0.0.0.0:8080", "secret", false},
	}

	for _, tc := range cases {
		if err := guardProxyBind(tc.addr, tc.token); (err != nil) != tc.wantErr {
			t.Errorf("guardProxyBind(%q, token=%q) err=%v wantErr=%v", tc.addr, tc.token, err, tc.wantErr)
		}
	}

	t.Setenv("SCOUT_PROXY_ALLOW_REMOTE", "1")
	if err := guardProxyBind("0.0.0.0:8080", ""); err != nil {
		t.Errorf("opt-in env still rejected: %v", err)
	}
}
