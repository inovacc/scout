package aihost

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"text/template"
)

// Kind classifies a portable plugin asset so cross-host packages can
// query the registry without caring which host originally authored it.
type Kind int

const (
	KindUnknown Kind = iota
	KindCommand      // commands/<name>.md — slash command body + frontmatter
	KindAgent        // agents/<name>.md — subagent definition
	KindSkill        // skills/<name>/SKILL.md — skill instructions
)

// String returns the lowercase identifier suitable for log lines.
func (k Kind) String() string {
	switch k {
	case KindCommand:
		return "command"
	case KindAgent:
		return "agent"
	case KindSkill:
		return "skill"
	default:
		return "unknown"
	}
}

// TemplateDelimsStart / End avoid clashing with MD `{{...}}` examples.
const (
	TemplateDelimsStart = "<%"
	TemplateDelimsEnd   = "%>"

	DefaultCreated = "2026-05-24"
)

// TemplateData feeds Render() at install time so author-side <%.Name%>
// placeholders resolve to the host's published values.
type TemplateData struct {
	Name        string
	Version     string
	Description string
	McpCommand  string
	Created     string
}

// Asset is one portable plugin file (frontmatter + body) classified by
// Kind. Render() emits on-disk markdown bytes with a `created:` marker
// injected per asset-type convention (skills → frontmatter, others →
// trailing HTML comment).
type Asset struct {
	Kind        Kind
	Path        string // forward-slash, relative to plugin root
	Frontmatter string // YAML between `---` markers, or ""
	Body        string // markdown body after frontmatter
	Created     string // YYYY-MM-DD, falls back to DefaultCreated
}

func (a Asset) created() string {
	if a.Created == "" {
		return DefaultCreated
	}
	return a.Created
}

// Render produces on-disk bytes. The caller supplies TemplateData so
// host-specific names/versions can flow into the published file.
func (a Asset) Render(d TemplateData) ([]byte, error) {
	if d.Created == "" {
		d.Created = a.created()
	}
	exec := func(label, src string, out *bytes.Buffer) error {
		t, err := template.New(label).Delims(TemplateDelimsStart, TemplateDelimsEnd).Parse(src)
		if err != nil {
			return fmt.Errorf("parse %s: %w", label, err)
		}
		if err := t.Execute(out, d); err != nil {
			return fmt.Errorf("render %s: %w", label, err)
		}
		return nil
	}

	var out bytes.Buffer
	if a.Frontmatter != "" {
		out.WriteString("---\n")
		if err := exec("fm:"+a.Path, a.Frontmatter, &out); err != nil {
			return nil, err
		}
		if a.Kind == KindSkill {
			fm := a.Frontmatter
			if !strings.Contains(fm, "\ncreated:") && !strings.HasPrefix(fm, "created:") {
				fmt.Fprintf(&out, "created: %s\n", d.Created)
			}
		}
		out.WriteString("---\n")
	}
	if err := exec("body:"+a.Path, a.Body, &out); err != nil {
		return nil, err
	}
	if a.Kind != KindSkill {
		if !bytes.HasSuffix(out.Bytes(), []byte("\n")) {
			out.WriteByte('\n')
		}
		fmt.Fprintf(&out, "\n<!-- created:%s -->\n", d.Created)
	}
	return out.Bytes(), nil
}

// Registry of every asset every host package contributes via init().
var assetRegistry []Asset

// RegisterAsset adds an asset to the global registry. Host packages
// call this from init() once per Asset literal.
func RegisterAsset(a ...Asset) { assetRegistry = append(assetRegistry, a...) }

// AssetByPath returns the asset matching (kind, path). Path is the
// plugin-tree-relative slash path (e.g. "skills/scrape/SKILL.md").
func AssetByPath(kind Kind, path string) (Asset, bool) {
	for _, a := range assetRegistry {
		if a.Kind == kind && a.Path == path {
			return a, true
		}
	}
	return Asset{}, false
}

// AssetsByKind returns every asset of the given kind, sorted by path.
func AssetsByKind(kind Kind) []Asset {
	var out []Asset
	for _, a := range assetRegistry {
		if a.Kind == kind {
			out = append(out, a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// AllAssets returns every registered asset, sorted by path.
func AllAssets() []Asset {
	out := make([]Asset, len(assetRegistry))
	copy(out, assetRegistry)
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}
