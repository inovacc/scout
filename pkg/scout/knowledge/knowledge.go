// Package knowledge maps gathered web pages into an Open Knowledge Format bundle.
package knowledge

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/inovacc/scout/pkg/okf"
)

// PageInput holds the gathered data for a single web page.
type PageInput struct {
	URL        string   // absolute URL of the page
	Title      string   // page title
	Markdown   string   // page body as markdown
	Frameworks []string // detected tech stacks, become Tags on the concept
	Links      []string // outgoing absolute link URLs found on the page
	Timestamp  string   // ISO-8601 collection timestamp
}

// Build maps gathered pages into an okf.Bundle.
//
// It produces:
//   - one "index" concept (Type "Bundle"): overview listing bundle-relative links
//     to every page concept; Resource = seedURL; Timestamp = generatedAt.
//   - one "pages/<slug>" concept per page (Type "Page"): Title, Resource=URL,
//     Tags=Frameworks, Timestamp, Body=Markdown plus a trailing "## Related"
//     section with bundle-relative links to concepts of OTHER pages whose URLs
//     appear in this page's Links list and are within the crawled set.
//
// External links (not in the crawled set) are omitted from Related so that
// okf.Bundle.Validate() passes (no dangling bundle links).
func Build(seedURL string, pages []PageInput, generatedAt string) (*okf.Bundle, error) {
	if seedURL == "" {
		return nil, fmt.Errorf("knowledge: build: seedURL required")
	}

	// Build URL→conceptID lookup for the crawled set.
	urlToID := make(map[string]string, len(pages))
	for _, p := range pages {
		id := conceptID(p.URL)
		urlToID[p.URL] = id
	}

	concepts := make([]okf.Concept, 0, len(pages)+1)

	// --- index concept ---
	var indexBody strings.Builder
	indexBody.WriteString("# Knowledge Bundle\n\n")
	indexBody.WriteString(fmt.Sprintf("Seed: %s\n\n", seedURL))
	indexBody.WriteString("## Pages\n\n")
	for _, p := range pages {
		id := urlToID[p.URL]
		title := p.Title
		if title == "" {
			title = p.URL
		}
		indexBody.WriteString(fmt.Sprintf("- [%s](/%s.md)\n", title, id))
	}

	concepts = append(concepts, okf.Concept{
		ID:        "index",
		Type:      "Bundle",
		Title:     "Knowledge Bundle",
		Resource:  seedURL,
		Timestamp: generatedAt,
		Body:      indexBody.String(),
	})

	// --- per-page concepts ---
	for _, p := range pages {
		id := urlToID[p.URL]
		title := p.Title
		if title == "" {
			title = p.URL
		}

		body := p.Markdown

		// Collect Related: links that resolve to OTHER pages in the crawled set.
		var related []string
		seen := make(map[string]bool)
		for _, link := range p.Links {
			targetID, ok := urlToID[link]
			if !ok {
				continue // external — skip
			}
			if targetID == id {
				continue // self-link — skip
			}
			if seen[targetID] {
				continue
			}
			seen[targetID] = true
			related = append(related, targetID)
		}

		if len(related) > 0 {
			var rel strings.Builder
			rel.WriteString("\n\n## Related\n\n")
			for _, relID := range related {
				// Find the title for this related concept.
				relTitle := relID
				for _, rp := range pages {
					if urlToID[rp.URL] == relID {
						if rp.Title != "" {
							relTitle = rp.Title
						}
						break
					}
				}
				rel.WriteString(fmt.Sprintf("- [%s](/%s.md)\n", relTitle, relID))
			}
			body += rel.String()
		}

		concepts = append(concepts, okf.Concept{
			ID:        id,
			Type:      "Page",
			Title:     title,
			Resource:  p.URL,
			Tags:      p.Frameworks,
			Timestamp: p.Timestamp,
			Body:      body,
		})
	}

	b := &okf.Bundle{Concepts: concepts}
	return b, nil
}

// nonSafeRe matches characters that are not filesystem-safe in concept IDs.
var nonSafeRe = regexp.MustCompile(`[^a-z0-9/_-]+`)

// conceptID converts a raw URL into a deterministic, filesystem-safe concept ID
// of the form "pages/<host>-<path>".
//
// Rules:
//   - Lowercased.
//   - Host and path are separated by "-" (not "/") so the host never becomes
//     a directory component.
//   - Non-[a-z0-9/_-] characters → "-".
//   - Runs of "-" are collapsed to one.
//   - Trailing and leading "-" per segment are trimmed.
//   - An empty or "/" path becomes "home".
func conceptID(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		// Fall back to sanitising the raw string.
		safe := nonSafeRe.ReplaceAllString(strings.ToLower(rawURL), "-")
		safe = strings.Trim(safe, "-")
		return "pages/" + safe
	}

	host := strings.ToLower(u.Hostname())
	// Remove port from host portion.
	host = nonSafeRe.ReplaceAllString(host, "-")
	host = strings.Trim(host, "-")

	pathPart := strings.ToLower(u.Path)
	pathPart = strings.Trim(pathPart, "/")
	if pathPart == "" {
		pathPart = "home"
	} else {
		pathPart = nonSafeRe.ReplaceAllString(pathPart, "-")
		pathPart = strings.Trim(pathPart, "-")
		// Collapse consecutive dashes.
		for strings.Contains(pathPart, "--") {
			pathPart = strings.ReplaceAll(pathPart, "--", "-")
		}
	}

	slug := host + "-" + pathPart
	// Collapse consecutive dashes in the combined slug.
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	slug = strings.Trim(slug, "-")

	return "pages/" + slug
}
