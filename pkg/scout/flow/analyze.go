package flow

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"strings"
	"time"

	"github.com/inovacc/scout/internal/engine/llm"
)

// AnalyzeOptions configures analysis.
type AnalyzeOptions struct {
	Name    string
	Timeout time.Duration // per-pass; 0 → 60s
}

// Report is the human-review surface emitted alongside the spec.
type Report struct {
	Degraded bool     `json:"degraded"` // true if the LLM failed and a raw skeleton was emitted
	Dropped  []int    `json:"dropped"`  // capture entry indexes classified as noise
	Chains   []Chain  `json:"chains"`   // inferred correlations (review these)
	Notes    []string `json:"notes"`
}

// Chain is one inferred value correlation: extract a value from from_entry's
// response and inject it into to_entry's request.
type Chain struct {
	FromEntry  int     `json:"from_entry"`
	From       string  `json:"from"`
	Path       string  `json:"path"`
	ToEntry    int     `json:"to_entry"`
	Into       string  `json:"into"`
	Name       string  `json:"name"`
	Template   string  `json:"template"`
	Var        string  `json:"var"`
	Confidence float64 `json:"confidence"`
}

// Analyze turns a Capture into a FlowSpec + Report using staged LLM passes.
func Analyze(capt *Capture, provider llm.Provider, opts AnalyzeOptions) (*FlowSpec, *Report, error) {
	if capt == nil || len(capt.Entries) == 0 {
		return nil, nil, fmt.Errorf("scout: flow: analyze: empty capture")
	}
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	spec := skeleton(capt, opts.Name)
	report := &Report{}

	keep, dropped, classifyOK := passClassify(provider, capt, timeout)
	if !classifyOK {
		report.Degraded = true
		report.Notes = append(report.Notes, "LLM classify failed; emitted raw skeleton — review every step")
		report.Notes = append(report.Notes, sanitizeSpec(spec)...)
		return spec, report, nil
	}
	report.Dropped = dropped
	spec = skeletonKeep(capt, opts.Name, keep)

	chains, correlateOK := passCorrelate(provider, capt, timeout)
	if correlateOK {
		report.Chains = chains
		applyChains(spec, report, keep, chains)
	} else {
		report.Notes = append(report.Notes, "LLM correlate failed; no chains inferred — add extract/inject manually")
	}

	if name, nameOK := passName(provider, capt, timeout); nameOK && name != "" {
		spec.Name = name
	}
	report.Notes = append(report.Notes, sanitizeSpec(spec)...)
	return spec, report, nil
}

func skeleton(capt *Capture, name string) *FlowSpec {
	idx := make([]int, len(capt.Entries))
	for i := range idx {
		idx[i] = i
	}
	return skeletonKeep(capt, name, idx)
}

func skeletonKeep(capt *Capture, name string, keep []int) *FlowSpec {
	f := &FlowSpec{Version: "1", Name: name}
	for n, ei := range keep {
		if ei < 0 || ei >= len(capt.Entries) {
			continue
		}
		e := capt.Entries[ei]
		step := FlowStep{
			ID:      fmt.Sprintf("step%d", n+1),
			Request: Request{Method: e.Method, URL: e.URL, Headers: cloneHeaders(e.ReqHeaders)},
		}
		if e.Status != 0 {
			step.Expect = &Expect{Status: e.Status}
		}
		f.Steps = append(f.Steps, step)
	}
	return f
}

func cloneHeaders(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	maps.Copy(out, in)
	return out
}

func applyChains(spec *FlowSpec, report *Report, keep []int, chains []Chain) {
	pos := make(map[int]int, len(keep))
	for stepIdx, entryIdx := range keep {
		pos[entryIdx] = stepIdx
	}
	for _, c := range chains {
		src, okS := pos[c.FromEntry]
		dst, okD := pos[c.ToEntry]
		if !okS || !okD {
			report.Notes = append(report.Notes, fmt.Sprintf("chain %q skipped: references a dropped/unknown entry (from=%d to=%d)", c.Var, c.FromEntry, c.ToEntry))
			continue
		}
		spec.Steps[src].Extract = append(spec.Steps[src].Extract, Extract{Var: c.Var, From: c.From, Path: c.Path})
		if c.Into == "header" {
			if spec.Steps[dst].Request.Headers == nil {
				spec.Steps[dst].Request.Headers = map[string]string{}
			}
			spec.Steps[dst].Request.Headers[c.Name] = c.Template
		} else {
			report.Notes = append(report.Notes, fmt.Sprintf("chain %q: into=%q not auto-applied — wire %q manually", c.Var, c.Into, c.Name))
		}
	}
}

// sanitizeSpec replaces raw secret-bearing values the skeleton copied from the
// capture (sensitive request headers, URL query tokens) with ${secret.*}
// placeholders, so the emitted spec is secret-free (design §10) yet still runnable
// once the user adds the named secrets to a vault profile. It returns review notes.
func sanitizeSpec(spec *FlowSpec) []string {
	var notes []string
	// Track the secret names sanitization itself introduces, so any OTHER
	// ${secret.*} reference (smuggled in from raw capture data or an inferred
	// chain template) can be flagged for human review below.
	introduced := map[string]bool{}
	for i := range spec.Steps {
		st := &spec.Steps[i]
		for name, val := range st.Request.Headers {
			if val == "" || isTemplate(val) {
				continue
			}
			if isSensitiveHeader(name) {
				sn := secretName(name)
				st.Request.Headers[name] = "${secret." + sn + "}"
				introduced[sn] = true
				notes = append(notes, fmt.Sprintf("step %q: header %q parameterized to ${secret.%s} — add %s to your vault profile", st.ID, name, sn, sn))
			}
		}
		if u, changed := parameterizeURLSecrets(st.Request.URL); len(changed) > 0 {
			st.Request.URL = u
			for _, k := range changed {
				introduced[secretName(k)] = true
			}
			notes = append(notes, fmt.Sprintf("step %q: URL query %v parameterized to ${secret.*} — provide via vault/vars", st.ID, changed))
		}
	}
	return append(notes, flagForeignSecretRefs(spec, introduced)...)
}

// flagForeignSecretRefs surfaces any ${secret.NAME} reference that sanitizeSpec
// did not itself introduce. Such a reference comes from raw captured data or an
// LLM-inferred chain template and, at replay, would resolve a vault secret and
// send it to the step's URL — a potential exfiltration vector. We do not delete
// it (that could break a legitimate inferred chain) but make it loud in review.
func flagForeignSecretRefs(spec *FlowSpec, introduced map[string]bool) []string {
	var notes []string
	for i := range spec.Steps {
		st := &spec.Steps[i]
		seen := map[string]bool{}
		flag := func(where, sn string) {
			if introduced[sn] || seen[sn] {
				return
			}
			seen[sn] = true
			notes = append(notes, fmt.Sprintf("SECURITY step %q: %s references ${secret.%s} not introduced by sanitization (from capture or inferred chain) — verify this is intended before running", st.ID, where, sn))
		}
		for name, val := range st.Request.Headers {
			for _, sn := range secretRefs(val) {
				flag(fmt.Sprintf("header %q", name), sn)
			}
		}
		for _, sn := range secretRefs(st.Request.URL) {
			flag("URL", sn)
		}
	}
	return notes
}

// secretRefs extracts the NAMEs from every ${secret.NAME} reference in s.
func secretRefs(s string) []string {
	const open = "${secret."
	var names []string
	for rest := s; ; {
		i := strings.Index(rest, open)
		if i < 0 {
			return names
		}
		rest = rest[i+len(open):]
		j := strings.Index(rest, "}")
		if j < 0 {
			return names
		}
		if name := rest[:j]; name != "" {
			names = append(names, name)
		}
		rest = rest[j+1:]
	}
}

func isTemplate(s string) bool { return strings.Contains(s, "${") }

// sensitiveQueryKeys are query-parameter names that conventionally carry secrets.
var sensitiveQueryKeys = map[string]bool{
	"code": true, "token": true, "access_token": true, "refresh_token": true,
	"id_token": true, "secret": true, "client_secret": true, "signature": true,
	"sig": true, "key": true, "apikey": true, "api_key": true, "password": true,
	"state": true, "session": true, "auth": true,
}

// parameterizeURLSecrets rewrites sensitive query-param VALUES to ${secret.KEY}
// placeholders, preserving the rest of the URL verbatim (NOT url-encoding, so the
// runtime's ${...} interpolation still matches). Returns the rewritten URL and the
// list of parameterized keys.
func parameterizeURLSecrets(raw string) (string, []string) {
	base, query, found := strings.Cut(raw, "?")
	if !found {
		return raw, nil
	}
	parts := strings.Split(query, "&")
	var changed []string
	for idx, p := range parts {
		k, _, ok := strings.Cut(p, "=")
		if !ok {
			continue
		}
		if sensitiveQueryKeys[strings.ToLower(k)] {
			parts[idx] = k + "=${secret." + secretName(k) + "}"
			changed = append(changed, k)
		}
	}
	if len(changed) == 0 {
		return raw, nil
	}
	return base + "?" + strings.Join(parts, "&"), changed
}

// secretName normalizes a header/query name into a ${secret.NAME}-safe token.
func secretName(s string) string {
	s = strings.ToUpper(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

const (
	classifySys  = "You classify captured HTTP entries. Drop static assets/telemetry; keep API calls. Return JSON: {\"keep\":[indexes],\"drop\":[indexes]}."
	correlateSys = "You correlate captured HTTP entries. Find values from one response reused in a later request (auth tokens, CSRF, IDs). Return JSON {\"chains\":[{from_entry,from,path,to_entry,into,name,template,var,confidence}]}."
	nameSys      = "Name this API flow in kebab-case. Return JSON {\"name\":\"...\"}."
)

func passClassify(p llm.Provider, capt *Capture, timeout time.Duration) (keep, dropped []int, ok bool) {
	out, err := complete(p, classifySys, captureDigest(capt), timeout)
	if err != nil {
		return nil, nil, false
	}
	var r struct {
		Keep []int `json:"keep"`
		Drop []int `json:"drop"`
	}
	if json.Unmarshal([]byte(out), &r) != nil || len(r.Keep) == 0 {
		return nil, nil, false
	}
	return r.Keep, r.Drop, true
}

func passCorrelate(p llm.Provider, capt *Capture, timeout time.Duration) ([]Chain, bool) {
	out, err := complete(p, correlateSys, captureDigest(capt), timeout)
	if err != nil {
		return nil, false
	}
	var r struct {
		Chains []Chain `json:"chains"`
	}
	if json.Unmarshal([]byte(out), &r) != nil {
		return nil, false
	}
	return r.Chains, true
}

func passName(p llm.Provider, capt *Capture, timeout time.Duration) (string, bool) {
	out, err := complete(p, nameSys, captureDigest(capt), timeout)
	if err != nil {
		return "", false
	}
	var r struct {
		Name string `json:"name"`
	}
	if json.Unmarshal([]byte(out), &r) != nil {
		return "", false
	}
	return r.Name, true
}

func complete(p llm.Provider, sys, user string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return p.Complete(ctx, sys, user)
}

// captureDigest renders a compact, SECRET-REDACTED view of the capture for the LLM.
func captureDigest(capt *Capture) string {
	type de struct {
		I      int               `json:"i"`
		Method string            `json:"method"`
		URL    string            `json:"url"`
		Req    map[string]string `json:"req_headers,omitempty"`
		Status int               `json:"status"`
		Resp   string            `json:"resp_excerpt,omitempty"`
	}
	ds := make([]de, 0, len(capt.Entries))
	for i, e := range capt.Entries {
		// ReqBody is deliberately excluded — it can hold passwords / credential grants.
		ds = append(ds, de{I: i, Method: e.Method, URL: redactURL(e.URL), Req: redactHeaders(e.ReqHeaders), Status: e.Status, Resp: redactBody(e.RespBody)})
	}
	b, err := json.Marshal(ds)
	if err != nil {
		return ""
	}
	return string(b)
}

// isSensitiveHeader reports whether a header name conventionally carries a secret.
func isSensitiveHeader(name string) bool {
	n := strings.ToLower(name)
	for _, s := range []string{"authorization", "cookie", "token", "csrf", "auth", "session", "secret", "api-key", "apikey", "signature", "password"} {
		if strings.Contains(n, s) {
			return true
		}
	}
	return false
}

// safeStructuralHeader reports whether a header's VALUE is conventionally
// non-secret and therefore safe to show the analyzer as structural context.
func safeStructuralHeader(name string) bool {
	switch strings.ToLower(name) {
	case "accept", "accept-encoding", "accept-language", "content-type",
		"content-length", "content-encoding", "connection", "cache-control",
		"user-agent", "host", "origin", "referer", "date", "server",
		"transfer-encoding", "vary", "x-content-type-options", "x-frame-options":
		return true
	default:
		return false
	}
}

// redactHeaders keeps header NAMES but only passes through values from an
// allowlist of known non-secret structural headers. Every other value (custom
// headers, anything sensitive by name, short tokens) is replaced with a length
// placeholder — a length-based heuristic would leak short credentials.
func redactHeaders(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		if safeStructuralHeader(k) && !isSensitiveHeader(k) {
			out[k] = v
		} else {
			out[k] = fmt.Sprintf("<redacted:%d>", len(v))
		}
	}
	return out
}

// redactURL keeps scheme/host/path but redacts every query-parameter VALUE
// (tokens ride in query strings: ?code=, ?access_token=, ?Signature=).
func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		if base, _, found := strings.Cut(raw, "?"); found {
			return base + "?<redacted>"
		}
		return raw
	}
	if u.RawQuery == "" {
		return raw
	}
	q := u.Query()
	for k := range q {
		q[k] = []string{"<redacted>"}
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// redactBody returns a secret-free structural view of a response body for the LLM:
// JSON keys + shape are kept; every JSON string VALUE is replaced with a length
// placeholder; non-JSON bodies are never shipped raw.
func redactBody(s string) string {
	if s == "" {
		return ""
	}
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return fmt.Sprintf("<redacted-body:%d>", len(s))
	}
	b, err := json.Marshal(redactJSONValues(v))
	if err != nil {
		return fmt.Sprintf("<redacted-body:%d>", len(s))
	}
	return excerpt(string(b))
}

func redactJSONValues(v any) any {
	switch t := v.(type) {
	case string:
		return fmt.Sprintf("<redacted:%d>", len(t))
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = redactJSONValues(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = redactJSONValues(val)
		}
		return out
	default:
		return v
	}
}

// excerpt caps a string to keep the digest compact.
func excerpt(s string) string {
	const maxLen = 400
	if len(s) > maxLen {
		return s[:maxLen]
	}
	return s
}
