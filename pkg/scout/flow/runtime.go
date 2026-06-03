package flow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"regexp"
	"time"
)

// RunOptions configures a flow run.
type RunOptions struct {
	Client  *http.Client      // nil → a default 30s client
	Secrets SecretResolver    // nil → ${secret.*} is an error
	Vars    map[string]string // overrides merged over FlowSpec.Vars
}

// RunResult is the per-step outcome of a run.
type RunResult struct {
	Steps []StepResult
}

// StepResult records one step's response + the vars it bound. Body holds the
// full response body verbatim, which may echo secret values a server returns;
// callers must not log a RunResult blindly.
type StepResult struct {
	ID        string
	Status    int
	Body      string
	Extracted map[string]string
}

// Run executes the flow's steps in order, threading extracted vars into later
// requests. It does NOT touch a browser. Secrets are resolved per-request via
// opts.Secrets and never retained.
func Run(ctx context.Context, f *FlowSpec, opts RunOptions) (*RunResult, error) {
	client := opts.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	// vars: spec.Vars overlaid with opts.Vars (opts wins), then grown by extracts.
	vars := map[string]string{}
	maps.Copy(vars, f.Vars)
	maps.Copy(vars, opts.Vars)

	res := &RunResult{}
	for _, step := range f.Steps {
		sr, err := runStep(ctx, client, step, vars, opts.Secrets)
		if err != nil {
			return res, fmt.Errorf("scout: flow: step %q: %w", step.ID, err)
		}
		res.Steps = append(res.Steps, *sr)
	}
	return res, nil
}

func runStep(ctx context.Context, client *http.Client, step FlowStep, vars map[string]string, secrets SecretResolver) (*StepResult, error) {
	url, err := Interpolate(step.Request.URL, vars, secrets)
	if err != nil {
		return nil, err
	}
	body, contentType, err := buildBody(step.Request, vars, secrets)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, step.Request.Method, url, body)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for k, v := range step.Request.Headers {
		hv, hErr := Interpolate(v, vars, secrets)
		if hErr != nil {
			return nil, hErr
		}
		req.Header.Set(k, hv)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if step.Expect != nil && step.Expect.Status != 0 && resp.StatusCode != step.Expect.Status {
		return nil, fmt.Errorf("expected status %d, got %d", step.Expect.Status, resp.StatusCode)
	}

	sr := &StepResult{ID: step.ID, Status: resp.StatusCode, Body: string(raw), Extracted: map[string]string{}}
	for _, e := range step.Extract {
		val, eErr := applyExtract(e, resp.Header, raw)
		if eErr != nil {
			return nil, eErr
		}
		vars[e.Var] = val
		sr.Extracted[e.Var] = val
	}
	return sr, nil
}

// buildBody returns the request body reader + Content-Type. GraphQL uses
// Request.GraphQL; REST uses Request.JSON.
func buildBody(r Request, varsMap map[string]string, secrets SecretResolver) (io.Reader, string, error) {
	if r.GraphQL != nil {
		vars := map[string]any{}
		for k, v := range r.GraphQL.Variables {
			iv, err := interpolateJSON(v, varsMap, secrets)
			if err != nil {
				return nil, "", err
			}
			vars[k] = iv
		}
		payload := map[string]any{"query": r.GraphQL.Query}
		if r.GraphQL.OperationName != "" {
			payload["operationName"] = r.GraphQL.OperationName
		}
		if len(vars) > 0 {
			payload["variables"] = vars
		}
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, "", fmt.Errorf("marshal graphql body: %w", err)
		}
		return bytes.NewReader(data), "application/json", nil
	}
	if r.JSON == nil {
		return http.NoBody, "", nil
	}
	interp, err := interpolateJSON(r.JSON, varsMap, secrets)
	if err != nil {
		return nil, "", err
	}
	data, err := json.Marshal(interp)
	if err != nil {
		return nil, "", fmt.Errorf("marshal json body: %w", err)
	}
	return bytes.NewReader(data), "application/json", nil
}

// interpolateJSON walks a decoded JSON value, interpolating ${...} in every string.
func interpolateJSON(v any, vars map[string]string, secrets SecretResolver) (any, error) {
	switch t := v.(type) {
	case string:
		return Interpolate(t, vars, secrets)
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			iv, err := interpolateJSON(val, vars, secrets)
			if err != nil {
				return nil, err
			}
			out[k] = iv
		}
		return out, nil
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			iv, err := interpolateJSON(val, vars, secrets)
			if err != nil {
				return nil, err
			}
			out[i] = iv
		}
		return out, nil
	default:
		return v, nil
	}
}

func applyExtract(e Extract, header http.Header, body []byte) (string, error) {
	switch e.From {
	case "response.json":
		return ExtractPath(body, e.Path)
	case "response.header":
		return header.Get(e.Path), nil
	case "response.body":
		return extractRegex(e.Path, body)
	default:
		return "", fmt.Errorf("unknown extract source %q", e.From)
	}
}

func extractRegex(pattern string, body []byte) (string, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", fmt.Errorf("compile regex: %w", err)
	}
	m := re.FindSubmatch(body)
	if len(m) < 2 {
		return "", fmt.Errorf("regex %q: no capture group match", pattern)
	}
	return string(m[1]), nil
}
