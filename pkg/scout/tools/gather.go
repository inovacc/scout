package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/inovacc/scout/pkg/scout"
)

// GatherInput is the typed input to one-shot page-intelligence gather.
// Field tags drive both cobra flag wiring (via the CLI shim) and the
// MCP server's auto-generated inputSchema.
type GatherInput struct {
	URL        string `json:"url"                  jsonschema:"the URL to gather intelligence from"`
	HTML       bool   `json:"html,omitempty"       jsonschema:"include raw HTML"`
	Markdown   bool   `json:"markdown,omitempty"   jsonschema:"include markdown rendering"`
	Screenshot bool   `json:"screenshot,omitempty" jsonschema:"include base64-encoded PNG screenshot"`
	Snapshot   bool   `json:"snapshot,omitempty"   jsonschema:"include accessibility-tree snapshot"`
	HAR        bool   `json:"har,omitempty"        jsonschema:"include HAR network recording"`
	Links      bool   `json:"links,omitempty"      jsonschema:"include extracted links"`
	Cookies    bool   `json:"cookies,omitempty"    jsonschema:"include page cookies"`
	Meta       bool   `json:"meta,omitempty"       jsonschema:"include page metadata (OG, Twitter, JSON-LD)"`
	Frameworks bool   `json:"frameworks,omitempty" jsonschema:"include detected frontend frameworks"`
	Console    bool   `json:"console,omitempty"    jsonschema:"capture console output during page load"`
	All        bool   `json:"all,omitempty"        jsonschema:"include everything (default when no flags set)"`
	Timeout    string `json:"timeout,omitempty"    jsonschema:"page load timeout as Go duration string (default 30s)"`
}

// GatherOutput is the gather result shape. Re-exports the engine type
// so adding fields upstream propagates automatically.
type GatherOutput = scout.GatherResult

// Gather drives Browser.Gather with options derived from the typed input.
// The caller owns the Browser; pass `b` for both CLI (fresh per invocation)
// and MCP (shared state).
func Gather(_ context.Context, b *scout.Browser, in GatherInput) (*GatherOutput, error) {
	if b == nil {
		return nil, fmt.Errorf("tools: gather: nil browser")
	}
	if in.URL == "" {
		return nil, fmt.Errorf("tools: gather: url required")
	}
	opts, err := in.toGatherOptions()
	if err != nil {
		return nil, err
	}
	return b.Gather(in.URL, opts...)
}

// toGatherOptions converts the typed input into engine GatherOptions.
// If no specific flags are set (and All is false), the engine's default
// "everything" behaviour applies — Browser.Gather treats nil opts as all.
func (in GatherInput) toGatherOptions() ([]scout.GatherOption, error) {
	var opts []scout.GatherOption
	anySpecific := false

	checks := []struct {
		on bool
		o  scout.GatherOption
	}{
		{in.HTML, scout.WithGatherHTML()},
		{in.Markdown, scout.WithGatherMarkdown()},
		{in.Screenshot, scout.WithGatherScreenshot()},
		{in.Snapshot, scout.WithGatherSnapshot()},
		{in.HAR, scout.WithGatherHAR()},
		{in.Links, scout.WithGatherLinks()},
		{in.Cookies, scout.WithGatherCookies()},
		{in.Meta, scout.WithGatherMeta()},
		{in.Frameworks, scout.WithGatherFrameworks()},
		{in.Console, scout.WithGatherConsole()},
	}
	for _, c := range checks {
		if c.on {
			opts = append(opts, c.o)
			anySpecific = true
		}
	}
	// --all (or no specific flags) ⇒ defer to engine defaults (everything).
	_ = anySpecific
	if in.All && len(opts) > 0 {
		// User said --all AND named specific fields; --all wins.
		opts = nil
	}
	if in.Timeout != "" {
		d, err := time.ParseDuration(in.Timeout)
		if err != nil {
			return nil, fmt.Errorf("tools: gather: parse timeout %q: %w", in.Timeout, err)
		}
		opts = append(opts, scout.WithGatherTimeout(d))
	}
	return opts, nil
}
