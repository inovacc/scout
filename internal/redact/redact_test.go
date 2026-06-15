package redact

import (
	"reflect"
	"strings"
	"testing"
)

func TestArgs(t *testing.T) {
	cases := []struct {
		name     string
		in, want []string
	}{
		{"separate", []string{"--api-key", "S"}, []string{"--api-key", Placeholder}},
		{"equals", []string{"--token=abc"}, []string{"--token=" + Placeholder}},
		{"untouched", []string{"--url", "http://x"}, []string{"--url", "http://x"}},
		{"flag-at-end", []string{"run", "--secret"}, []string{"run", "--secret"}},
		{"empty", nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Args(tc.in); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Args(%v)=%v want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestMap(t *testing.T) {
	in := map[string]any{"url": "http://x", "password": "p", "nested": map[string]any{"token": "t", "ok": 1}}
	got := Map(in)
	if got["url"] != "http://x" || got["password"] != Placeholder {
		t.Fatalf("top-level redaction wrong: %v", got)
	}
	n := got["nested"].(map[string]any)
	if n["token"] != Placeholder || n["ok"] != 1 {
		t.Fatalf("nested redaction wrong: %v", n)
	}
	if in["password"] != "p" {
		t.Fatal("input mutated")
	}
}

func TestURL(t *testing.T) {
	got := URL("https://h/p?token=abc&q=1")
	if strings.Contains(got, "token=abc") || !strings.Contains(got, Placeholder) || !strings.Contains(got, "q=1") {
		t.Fatalf("URL redaction wrong: %s", got)
	}
	if URL("not a url") != "not a url" {
		t.Fatal("non-url changed")
	}
}

func TestHeaderAndBody(t *testing.T) {
	if Header("Authorization", "Bearer x") != Placeholder {
		t.Fatal("auth header not masked")
	}
	if Header("Accept", "text/html") != "text/html" {
		t.Fatal("benign header masked")
	}
	s, trunc := Body([]byte("hello"), 3)
	if s != "hel" || !trunc {
		t.Fatalf("body cap wrong: %q %v", s, trunc)
	}

	if s, trunc := Body([]byte("x"), 0); s != "" || !trunc {
		t.Fatalf("body max=0 wrong: %q %v", s, trunc)
	}

	if s, trunc := Body([]byte("x"), -1); s != "" || !trunc { // must not panic on negative max
		t.Fatalf("body negative max wrong: %q %v", s, trunc)
	}

	if s, trunc := Body(nil, 5); s != "" || trunc {
		t.Fatalf("empty body wrong: %q %v", s, trunc)
	}
}
