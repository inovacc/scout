package strategy

import (
	"fmt"
	"strings"
	"time"
)

// Validate checks a strategy for correctness.
// It returns a combined error listing all issues found.
func Validate(s *Strategy) error {
	var errs []string

	if s.Name == "" {
		errs = append(errs, "name is required")
	}

	if s.Version == "" {
		errs = append(errs, "version is required")
	}

	if len(s.Steps) == 0 {
		errs = append(errs, "at least one step is required")
	}

	stepNames := make(map[string]bool)

	for i, step := range s.Steps {
		prefix := fmt.Sprintf("steps[%d]", i)

		if step.Name == "" {
			errs = append(errs, prefix+": name is required")
		} else if stepNames[step.Name] {
			errs = append(errs, prefix+": duplicate step name "+step.Name)
		} else {
			stepNames[step.Name] = true
		}

		if step.Mode == "" && step.URL == "" {
			errs = append(errs, prefix+": either mode or url is required")
		}

		if step.Mode != "" && step.URL != "" {
			errs = append(errs, prefix+": cannot specify both mode and url")
		}

		if step.Timeout != "" {
			if _, err := time.ParseDuration(step.Timeout); err != nil {
				errs = append(errs, prefix+": invalid timeout: "+step.Timeout)
			}
		}

		if step.Limit < 0 {
			errs = append(errs, prefix+": limit must be >= 0")
		}
	}

	if len(s.Output.Sinks) == 0 {
		errs = append(errs, "output: at least one sink is required")
	}

	for i, sink := range s.Output.Sinks {
		prefix := fmt.Sprintf("output.sinks[%d]", i)

		if sink.Type == "" {
			errs = append(errs, prefix+": type is required")
		}

		switch sink.Type {
		case "json-file", "ndjson", "csv":
			if sink.Path == "" {
				errs = append(errs, prefix+": path is required for "+sink.Type+" sink")
			}
		case "":
			// Already reported above.
		default:
			// Plugin or custom sinks — no validation needed.
		}
	}

	if s.Auth != nil {
		if s.Auth.Provider == "" {
			errs = append(errs, "auth: provider is required")
		}

		if s.Auth.Timeout != "" {
			if _, err := time.ParseDuration(s.Auth.Timeout); err != nil {
				errs = append(errs, "auth: invalid timeout: "+s.Auth.Timeout)
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("strategy: validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}

	return nil
}
