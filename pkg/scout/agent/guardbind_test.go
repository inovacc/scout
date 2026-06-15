package agent

import "testing"

// TestGuardBind verifies the agent server fails closed: a non-loopback bind with
// no API key is refused unless InsecureAllowRemote is set.
func TestGuardBind(t *testing.T) {
	cases := []struct {
		addr    string
		apiKey  string
		allow   bool
		wantErr bool
	}{
		{"localhost:9000", "", false, false},
		{"127.0.0.1:9000", "", false, false},
		{"[::1]:9000", "", false, false},
		{"0.0.0.0:9000", "", false, true},
		{":9000", "", false, true},
		{"192.168.1.5:9000", "", false, true},
		{"0.0.0.0:9000", "key", false, false},
		{"0.0.0.0:9000", "", true, false},
	}

	for _, tc := range cases {
		s := &Server{config: ServerConfig{Addr: tc.addr, APIKey: tc.apiKey, InsecureAllowRemote: tc.allow}}
		if err := s.guardBind(); (err != nil) != tc.wantErr {
			t.Errorf("guardBind(addr=%q key=%q allow=%v) err=%v, wantErr=%v", tc.addr, tc.apiKey, tc.allow, err, tc.wantErr)
		}
	}
}
