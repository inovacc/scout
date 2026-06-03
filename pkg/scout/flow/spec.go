package flow

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type FlowSpec struct {
	Version string            `yaml:"version" json:"version"`
	Name    string            `yaml:"name" json:"name"`
	Auth    *AuthRef          `yaml:"auth,omitempty" json:"auth,omitempty"`
	Vars    map[string]string `yaml:"vars,omitempty" json:"vars,omitempty"`
	Steps   []FlowStep        `yaml:"steps" json:"steps"`
}

type AuthRef struct {
	Profile string `yaml:"profile" json:"profile"`
}

type FlowStep struct {
	ID      string    `yaml:"id" json:"id"`
	Request Request   `yaml:"request" json:"request"`
	Extract []Extract `yaml:"extract,omitempty" json:"extract,omitempty"`
	Expect  *Expect   `yaml:"expect,omitempty" json:"expect,omitempty"`
}

type Request struct {
	Method  string            `yaml:"method" json:"method"`
	URL     string            `yaml:"url" json:"url"`
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	JSON    map[string]any    `yaml:"json,omitempty" json:"json,omitempty"`
	GraphQL *GraphQL          `yaml:"graphql,omitempty" json:"graphql,omitempty"`
}

type GraphQL struct {
	OperationName string         `yaml:"operationName,omitempty" json:"operationName,omitempty"`
	Query         string         `yaml:"query" json:"query"`
	Variables     map[string]any `yaml:"variables,omitempty" json:"variables,omitempty"`
}

// Extract binds a named var from a step's response.
// From is one of: response.json | response.header | response.body.
type Extract struct {
	Var  string `yaml:"var" json:"var"`
	From string `yaml:"from" json:"from"`
	Path string `yaml:"path" json:"path"`
}

type Expect struct {
	Status int `yaml:"status,omitempty" json:"status,omitempty"`
}

// LoadFile reads a flow spec from disk (YAML or JSON).
func LoadFile(path string) (*FlowSpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("scout: flow: read file: %w", err)
	}
	return Parse(data)
}

// Parse unmarshals a flow spec. YAML is a superset of JSON, so yaml.v3 handles
// both; on YAML failure it retries encoding/json for a clearer error. Parse does
// NOT validate (call Validate separately) and does NOT expand ${...} (those are
// resolved at run time against vars + the vault).
func Parse(data []byte) (*FlowSpec, error) {
	var f FlowSpec
	if err := yaml.Unmarshal(data, &f); err != nil {
		if jErr := json.Unmarshal(data, &f); jErr != nil {
			return nil, fmt.Errorf("scout: flow: parse: %w", err)
		}
	}
	return &f, nil
}

// Validate collects all problems and returns them as one error.
func Validate(f *FlowSpec) error {
	var errs []string
	if f.Version == "" {
		errs = append(errs, "version is required")
	}
	if len(f.Steps) == 0 {
		errs = append(errs, "at least one step is required")
	}
	seen := map[string]bool{}
	for i, s := range f.Steps {
		where := fmt.Sprintf("step %d", i+1)
		if s.ID != "" {
			where = fmt.Sprintf("step %d (%q)", i+1, s.ID)
		}
		if s.ID == "" {
			errs = append(errs, where+": id is required")
		} else if seen[s.ID] {
			errs = append(errs, where+": duplicate step id")
		}
		seen[s.ID] = true
		if s.Request.Method == "" {
			errs = append(errs, where+": request.method is required")
		}
		if s.Request.URL == "" {
			errs = append(errs, where+": request.url is required")
		}
		for j, e := range s.Extract {
			if e.Var == "" || e.From == "" || e.Path == "" {
				errs = append(errs, fmt.Sprintf("%s: extract %d: var/from/path are all required", where, j+1))
			}
			switch e.From {
			case "response.json", "response.header", "response.body":
			default:
				errs = append(errs, fmt.Sprintf("%s: extract %d: from must be response.json|response.header|response.body", where, j+1))
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("scout: flow: validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}
