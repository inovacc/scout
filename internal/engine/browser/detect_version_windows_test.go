//go:build windows

package browser

import "testing"

// TestPSQuote pins the PowerShell single-quoted-literal escape: a single quote
// in a browser path is doubled (so it cannot break out of the literal and
// inject PowerShell), while safe paths — including backslashes and spaces —
// pass through unchanged (backslashes are literal inside single-quoted strings).
func TestPSQuote(t *testing.T) {
	cases := []struct{ in, want string }{
		{`C:\Program Files\Google\Chrome\Application\chrome.exe`, `C:\Program Files\Google\Chrome\Application\chrome.exe`},
		{`C:\tmp\a'b`, `C:\tmp\a''b`},
		{`C:\x'; Remove-Item C:\y; '`, `C:\x''; Remove-Item C:\y; ''`},
	}
	for _, c := range cases {
		if got := psQuote(c.in); got != c.want {
			t.Errorf("psQuote(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
