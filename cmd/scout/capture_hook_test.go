package main

import "testing"

func TestFirstPositional(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{[]string{"version"}, "version"},
		{[]string{"--verbose", "gather", "https://x"}, "gather"},
		{[]string{"-v", "scrape"}, "scrape"},
		{nil, ""},
		{[]string{"--flag"}, ""},
	}

	for _, tc := range cases {
		if got := firstPositional(tc.in); got != tc.want {
			t.Errorf("firstPositional(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
