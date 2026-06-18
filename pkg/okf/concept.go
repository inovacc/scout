package okf

import (
	"path"
	"regexp"
	"strings"
)

// Concept is a single knowledge unit stored as one Markdown file inside a Bundle.
// Its ID is the bundle-relative path without the ".md" extension, using forward slashes.
type Concept struct {
	ID          string   // bundle-relative path, no ".md", forward slashes
	Type        string   // REQUIRED
	Title       string   // optional
	Description string   // optional
	Resource    string   // optional source URI/URL
	Tags        []string // optional
	Timestamp   string   // optional, ISO-8601
	Body        string   // markdown below the frontmatter
}

// linkRe matches Markdown inline links: [text](target)
var linkRe = regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)

// Links returns the bundle-relative concept IDs referenced by bundle-internal
// Markdown links found in c.Body.
//
// External links (http://, https://, mailto:) and pure anchors (#...) are
// excluded. Absolute bundle targets (starting with /) are resolved from the
// bundle root; relative targets are resolved against the concept's own directory.
// The ".md" extension is stripped from the result.
func (c Concept) Links() []string {
	matches := linkRe.FindAllStringSubmatch(c.Body, -1)
	if len(matches) == 0 {
		return nil
	}

	base := path.Dir(c.ID) // directory of this concept within the bundle

	var result []string
	for _, m := range matches {
		target := m[1]

		// Strip fragment: split on '#' — use only the path part.
		// But first check if it's a pure anchor.
		if strings.HasPrefix(target, "#") {
			continue
		}

		// Strip inline fragment suffix (e.g. "foo.md#section" → "foo.md").
		if idx := strings.IndexByte(target, '#'); idx >= 0 {
			target = target[:idx]
		}

		// Skip external links.
		if strings.HasPrefix(target, "http://") ||
			strings.HasPrefix(target, "https://") ||
			strings.HasPrefix(target, "mailto:") {
			continue
		}

		// Skip if empty after stripping fragment.
		if target == "" {
			continue
		}

		// Resolve absolute vs relative.
		var resolved string
		if strings.HasPrefix(target, "/") {
			// Bundle-root-relative: strip leading slash.
			resolved = strings.TrimPrefix(target, "/")
		} else {
			// Relative to this concept's directory.
			resolved = path.Join(base, target)
		}

		// Strip .md extension.
		resolved = strings.TrimSuffix(resolved, ".md")

		// Clean path (removes ./ and resolves ..).
		resolved = path.Clean(resolved)

		result = append(result, resolved)
	}
	return result
}
