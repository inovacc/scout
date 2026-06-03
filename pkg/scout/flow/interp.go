package flow

import (
	"fmt"
	"regexp"
	"strings"
)

// SecretResolver yields a secret value by name. Implemented at run time by a
// vault-backed resolver (secrets.go); nil when a spec uses no ${secret.*}.
type SecretResolver interface {
	Secret(name string) ([]byte, error)
}

var placeholderRe = regexp.MustCompile(`\$\{([^}]+)\}`)

// Interpolate replaces ${...} placeholders in s. Namespaces:
//   - ${secret.NAME} → secrets.Secret(NAME) (errors if secrets is nil)
//   - ${var.NAME} or bare ${NAME} → vars[NAME]
//
// An unknown placeholder is an error (fail closed — never emit a literal ${...}).
func Interpolate(s string, vars map[string]string, secrets SecretResolver) (string, error) {
	var firstErr error
	out := placeholderRe.ReplaceAllStringFunc(s, func(match string) string {
		name := placeholderRe.FindStringSubmatch(match)[1]
		if rest, ok := strings.CutPrefix(name, "secret."); ok {
			if secrets == nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("scout: flow: interp: ${secret.%s} but no vault profile bound", rest)
				}
				return match
			}
			b, err := secrets.Secret(rest)
			if err != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("scout: flow: interp: secret %q: %w", rest, err)
				}
				return match
			}
			return string(b) // transient; the surrounding request bytes are discarded after send
		}
		key := strings.TrimPrefix(name, "var.")
		v, ok := vars[key]
		if !ok {
			if firstErr == nil {
				firstErr = fmt.Errorf("scout: flow: interp: unknown variable %q", key)
			}
			return match
		}
		return v
	})
	if firstErr != nil {
		return "", firstErr
	}
	return out, nil
}
