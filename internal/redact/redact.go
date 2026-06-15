// Package redact masks secret material in values persisted to logs and
// interaction captures. It is best-effort and pattern-based.
package redact

import (
	"net/url"
	"regexp"
	"strings"
)

// Placeholder replaces a redacted value.
const Placeholder = "***REDACTED***"

// Pattern matches flag/field/header/query names whose values are secret.
var Pattern = regexp.MustCompile(`(?i)(api[-_]?key|passphrase|password|secret|token|credential|cookie|bearer|authorization|auth|sig)`)

// Args returns a copy of args with the values of sensitive flags masked. It
// handles "--flag value", "--flag=value" and short "-f value" forms. The input
// is never mutated.
func Args(args []string) []string {
	if len(args) == 0 {
		return args
	}

	out := make([]string, len(args))
	copy(out, args)

	for i := 0; i < len(out); i++ {
		a := out[i]

		dashes := len(a) - len(strings.TrimLeft(a, "-"))
		if dashes == 0 {
			continue
		}

		body := a[dashes:]
		if eq := strings.IndexByte(body, '='); eq >= 0 {
			if Pattern.MatchString(body[:eq]) {
				out[i] = a[:dashes] + body[:eq] + "=" + Placeholder
			}

			continue
		}

		if Pattern.MatchString(body) && i+1 < len(out) && !strings.HasPrefix(out[i+1], "-") {
			out[i+1] = Placeholder
		}
	}

	return out
}

// Map returns a shallow copy of m with values masked for keys matching Pattern;
// nested maps are redacted recursively. The input is never mutated.
func Map(m map[string]any) map[string]any {
	if len(m) == 0 {
		return m
	}

	out := make(map[string]any, len(m))
	for k, v := range m {
		switch {
		case Pattern.MatchString(k):
			out[k] = Placeholder
		default:
			if nested, ok := v.(map[string]any); ok {
				out[k] = Map(nested)
			} else {
				out[k] = v
			}
		}
	}

	return out
}

// URL masks the values of sensitive query params, preserving scheme/host/path.
// A non-URL string is returned unchanged.
func URL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw
	}

	q := u.Query()
	changed := false

	for key := range q {
		if Pattern.MatchString(key) {
			q[key] = []string{Placeholder}
			changed = true
		}
	}

	if !changed {
		return raw
	}

	// Build raw query manually so the placeholder is not percent-encoded.
	parts := make([]string, 0, len(q))
	for k, vs := range q {
		for _, v := range vs {
			if v == Placeholder {
				parts = append(parts, url.QueryEscape(k)+"="+Placeholder)
			} else {
				parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(v))
			}
		}
	}

	u.RawQuery = strings.Join(parts, "&")

	return u.String()
}

// Header returns value masked when name is a sensitive header.
func Header(name, value string) string {
	if Pattern.MatchString(name) {
		return Placeholder
	}

	return value
}

// Body truncates b to max bytes, returning the string and whether it truncated.
func Body(b []byte, max int) (string, bool) {
	if len(b) <= max {
		return string(b), false
	}

	return string(b[:max]), true
}
