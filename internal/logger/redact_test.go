package logger

import (
	"reflect"
	"testing"
)

func TestRedactArgs(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"separate value", []string{"--api-key", "SECRET"}, []string{"--api-key", redactedValue}},
		{"equals form", []string{"--api-key=SECRET"}, []string{"--api-key=" + redactedValue}},
		{"underscore key", []string{"--api_key=SECRET"}, []string{"--api_key=" + redactedValue}},
		{"passphrase mid", []string{"scrape", "--passphrase", "p@ss", "--headless"}, []string{"scrape", "--passphrase", redactedValue, "--headless"}},
		{"token then positional", []string{"--token=abc", "next"}, []string{"--token=" + redactedValue, "next"}},
		{"bearer short stream", []string{"--bearer", "xyz"}, []string{"--bearer", redactedValue}},
		{"non-sensitive untouched", []string{"--url", "http://x", "--depth", "3"}, []string{"--url", "http://x", "--depth", "3"}},
		{"sensitive flag at end no value", []string{"run", "--secret"}, []string{"run", "--secret"}},
		{"sensitive flag followed by flag", []string{"--password", "--headless"}, []string{"--password", "--headless"}},
		{"empty", nil, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := append([]string(nil), tc.in...)
			got := redactArgs(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("redactArgs(%v) = %v, want %v", tc.in, got, tc.want)
			}
			if !reflect.DeepEqual(tc.in, in) {
				t.Fatalf("redactArgs mutated input: got %v, want %v", tc.in, in)
			}
		})
	}
}
