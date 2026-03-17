// Package strategy provides declarative YAML/JSON strategy files for
// multi-step browser automation workflows. A strategy file describes browser
// configuration, authentication, scrape steps, and output destinations.
//
// Usage:
//
//	s, err := strategy.LoadFile("strategy.yaml")
//	if err != nil { ... }
//	if err := strategy.Validate(s); err != nil { ... }
//	if err := strategy.Execute(ctx, s); err != nil { ... }
package strategy

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Strategy is the top-level declaration for a multi-step workflow.
type Strategy struct {
	Name    string        `yaml:"name" json:"name"`
	Version string        `yaml:"version" json:"version"`
	Browser BrowserConfig `yaml:"browser" json:"browser"`
	Auth    *AuthConfig   `yaml:"auth,omitempty" json:"auth,omitempty"`
	Steps   []Step        `yaml:"steps" json:"steps"`
	Output  OutputConfig  `yaml:"output" json:"output"`
}

// BrowserConfig maps to scout.Option functional options.
type BrowserConfig struct {
	Type       string `yaml:"type,omitempty" json:"type,omitempty"`             // chrome, brave, edge
	Stealth    bool   `yaml:"stealth,omitempty" json:"stealth,omitempty"`       // WithStealth()
	Headless   *bool  `yaml:"headless,omitempty" json:"headless,omitempty"`     // WithHeadless(); nil = default (true)
	Proxy      string `yaml:"proxy,omitempty" json:"proxy,omitempty"`           // WithProxy()
	UserAgent  string `yaml:"user_agent,omitempty" json:"user_agent,omitempty"` // WithUserAgent()
	WindowSize []int  `yaml:"window_size,omitempty" json:"window_size,omitempty"`
}

// AuthConfig defines how to authenticate before scraping.
type AuthConfig struct {
	Provider       string `yaml:"provider" json:"provider"`                                     // auth provider name (slack, linkedin, etc.)
	Session        string `yaml:"session,omitempty" json:"session,omitempty"`                    // path to session file
	Passphrase     string `yaml:"passphrase,omitempty" json:"passphrase,omitempty"`              // session passphrase (supports ${ENV})
	CaptureOnClose bool   `yaml:"capture_on_close,omitempty" json:"capture_on_close,omitempty"` // capture session on close
	Timeout        string `yaml:"timeout,omitempty" json:"timeout,omitempty"`                   // e.g. "5m"
}

// Step is one unit of work in a strategy.
type Step struct {
	Name    string         `yaml:"name" json:"name"`
	Mode    string         `yaml:"mode,omitempty" json:"mode,omitempty"` // scraper mode name
	URL     string         `yaml:"url,omitempty" json:"url,omitempty"`   // direct URL for non-mode steps
	Targets []string       `yaml:"targets,omitempty" json:"targets,omitempty"`
	Limit   int            `yaml:"limit,omitempty" json:"limit,omitempty"`
	Timeout string         `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	When    map[string]any `yaml:"when,omitempty" json:"when,omitempty"` // conditional execution
}

// OutputConfig defines result destinations.
type OutputConfig struct {
	Sinks  []SinkConfig `yaml:"sinks" json:"sinks"`
	Report bool         `yaml:"report,omitempty" json:"report,omitempty"` // generate AI report
}

// SinkConfig defines a single output destination.
type SinkConfig struct {
	Type   string         `yaml:"type" json:"type"`                       // json-file, csv, ndjson
	Path   string         `yaml:"path,omitempty" json:"path,omitempty"`   // output path (supports templates)
	Config map[string]any `yaml:"config,omitempty" json:"config,omitempty"` // sink-specific config
}

// LoadFile reads a strategy from a YAML or JSON file.
// Environment variables in the form ${VAR} are expanded before parsing.
func LoadFile(path string) (*Strategy, error) {
	data, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("strategy: read file: %w", err)
	}

	return Parse(data)
}

// Parse parses a strategy from raw bytes (YAML or JSON).
// Environment variables in the form ${VAR} are expanded.
func Parse(data []byte) (*Strategy, error) {
	expanded := expandEnv(string(data))

	var s Strategy

	// Try YAML first (superset of JSON).
	if err := yaml.Unmarshal([]byte(expanded), &s); err != nil {
		// Fallback to JSON for better error messages on JSON files.
		if jsonErr := json.Unmarshal([]byte(expanded), &s); jsonErr != nil {
			return nil, fmt.Errorf("strategy: parse: %w", err)
		}
	}

	return &s, nil
}

// expandEnv replaces ${VAR} references with environment variable values.
// Unset variables expand to empty string.
func expandEnv(s string) string {
	return os.Expand(s, func(key string) string {
		return os.Getenv(key)
	})
}

// ParseTimeout parses a duration string, returning the fallback on empty or error.
func ParseTimeout(s string, fallback time.Duration) time.Duration {
	if s == "" {
		return fallback
	}

	d, err := time.ParseDuration(s)
	if err != nil {
		return fallback
	}

	return d
}

// IsHeadless returns the headless setting, defaulting to true if nil.
func (b BrowserConfig) IsHeadless() bool {
	if b.Headless == nil {
		return true
	}

	return *b.Headless
}

// String implements fmt.Stringer.
func (s *Strategy) String() string {
	steps := make([]string, len(s.Steps))
	for i, st := range s.Steps {
		steps[i] = st.Name
	}

	return fmt.Sprintf("Strategy{name=%s steps=[%s]}", s.Name, strings.Join(steps, ", "))
}
