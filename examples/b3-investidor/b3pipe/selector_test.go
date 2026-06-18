package b3pipe

import "testing"

func TestSelectEngine(t *testing.T) {
	cases := []struct {
		name string
		in   RefreshTokenInfo
		want Engine
	}{
		{"none", RefreshTokenInfo{Found: false}, EngineFallback},
		{"localStorage", RefreshTokenInfo{Found: true, InLocalStorage: true}, EngineA},
		{"sessionStorage", RefreshTokenInfo{Found: true, InSessionStorage: true}, EngineA},
		{"readable cookie", RefreshTokenInfo{Found: true, InReadableCookie: true}, EngineA},
		{"httpOnly only", RefreshTokenInfo{Found: true, InHTTPOnlyCookie: true}, EngineB},
		{"httpOnly + localStorage prefers A", RefreshTokenInfo{Found: true, InHTTPOnlyCookie: true, InLocalStorage: true}, EngineA},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := SelectEngine(c.in); got != c.want {
				t.Errorf("SelectEngine(%+v) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
