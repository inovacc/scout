package knowledge_test

import (
	"strings"
	"testing"

	"github.com/inovacc/scout/pkg/okf"
	"github.com/inovacc/scout/pkg/scout/knowledge"
)

// pageByID returns the concept with the given ID from the bundle, or nil.
func pageByID(b *okf.Bundle, id string) *okf.Concept {
	for i := range b.Concepts {
		if b.Concepts[i].ID == id {
			return &b.Concepts[i]
		}
	}
	return nil
}

// TestBuildTwoPages verifies that when page A links to page B, the bundle
// contains an index concept, A's concept body has a "## Related" section with
// a link to B's concept, and the bundle passes Validate().
func TestBuildTwoPages(t *testing.T) {
	seedURL := "https://example.com/"
	pageA := knowledge.PageInput{
		URL:        "https://example.com/",
		Title:      "Home",
		Markdown:   "# Home\n\nWelcome.",
		Frameworks: []string{"React"},
		Links:      []string{"https://example.com/about"},
		Timestamp:  "2024-01-01T00:00:00Z",
	}
	pageB := knowledge.PageInput{
		URL:        "https://example.com/about",
		Title:      "About",
		Markdown:   "# About\n\nLearn more.",
		Frameworks: []string{},
		Links:      []string{},
		Timestamp:  "2024-01-01T00:00:01Z",
	}

	b, err := knowledge.Build(seedURL, []knowledge.PageInput{pageA, pageB}, "2024-01-01T00:00:00Z")
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	// Bundle must be valid.
	if err := b.Validate(); err != nil {
		t.Fatalf("Validate() failed: %v", err)
	}

	// Must have index + 2 page concepts.
	if got := len(b.Concepts); got != 3 {
		t.Fatalf("expected 3 concepts, got %d", got)
	}

	// Index concept.
	idx := pageByID(b, "index")
	if idx == nil {
		t.Fatal("index concept missing")
	}
	if idx.Type != "Bundle" {
		t.Errorf("index.Type = %q, want %q", idx.Type, "Bundle")
	}
	if idx.Resource != seedURL {
		t.Errorf("index.Resource = %q, want %q", idx.Resource, seedURL)
	}

	// Determine concept IDs (they are deterministic) by matching Resource on
	// Page-type concepts only (the index concept also carries Resource=seedURL).
	idA := pageConceptByURL(b, pageA.URL)
	idB := pageConceptByURL(b, pageB.URL)
	if idA == nil {
		t.Fatal("concept for page A not found")
	}
	if idB == nil {
		t.Fatal("concept for page B not found")
	}

	// Index body must contain bundle-relative links to both pages.
	if !strings.Contains(idx.Body, "/"+idA.ID+".md") {
		t.Errorf("index body missing link to page A concept %q; body:\n%s", idA.ID, idx.Body)
	}
	if !strings.Contains(idx.Body, "/"+idB.ID+".md") {
		t.Errorf("index body missing link to page B concept %q; body:\n%s", idB.ID, idx.Body)
	}

	// Page A must have a ## Related section linking to B.
	if !strings.Contains(idA.Body, "## Related") {
		t.Errorf("page A body missing ## Related section; body:\n%s", idA.Body)
	}
	if !strings.Contains(idA.Body, "/"+idB.ID+".md") {
		t.Errorf("page A Related section missing link to B %q; body:\n%s", idB.ID, idA.Body)
	}

	// Page A's Type.
	if idA.Type != "Page" {
		t.Errorf("page A type = %q, want Page", idA.Type)
	}

	// Tags are Frameworks.
	if len(idA.Tags) != 1 || idA.Tags[0] != "React" {
		t.Errorf("page A tags = %v, want [React]", idA.Tags)
	}
}

// TestBuildExternalLinkExcluded verifies that a link in A.Links that is NOT
// in the crawled set is NOT added to Related, so Validate() still passes.
func TestBuildExternalLinkExcluded(t *testing.T) {
	seedURL := "https://example.com/"
	pageA := knowledge.PageInput{
		URL:       "https://example.com/",
		Title:     "Home",
		Markdown:  "# Home",
		Links:     []string{"https://external.org/foo", "https://example.com/about"},
		Timestamp: "2024-01-01T00:00:00Z",
	}
	pageB := knowledge.PageInput{
		URL:       "https://example.com/about",
		Title:     "About",
		Markdown:  "# About",
		Links:     []string{},
		Timestamp: "2024-01-01T00:00:01Z",
	}

	b, err := knowledge.Build(seedURL, []knowledge.PageInput{pageA, pageB}, "2024-01-01T00:00:00Z")
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if err := b.Validate(); err != nil {
		t.Fatalf("Validate() failed (dangling link?): %v", err)
	}

	// external.org should NOT appear in any concept body as a bundle link.
	for _, c := range b.Concepts {
		if strings.Contains(c.Body, "external.org") && strings.Contains(c.Body, ".md") {
			// A bundle-relative link to external.org would be a bug.
			// Raw mention is fine; but a /<id>.md link is not expected.
			if strings.Contains(c.Body, "/pages/external") {
				t.Errorf("concept %q contains bundle link to external domain", c.ID)
			}
		}
	}
}

// TestBuildSinglePage verifies the degenerate case: one page → index + 1 page
// concept, no Related section needed, Validate passes.
func TestBuildSinglePage(t *testing.T) {
	seed := "https://solo.example.com/"
	page := knowledge.PageInput{
		URL:       seed,
		Title:     "Solo",
		Markdown:  "# Solo\n\nOnly page.",
		Links:     []string{},
		Timestamp: "2024-06-01T12:00:00Z",
	}

	b, err := knowledge.Build(seed, []knowledge.PageInput{page}, "2024-06-01T12:00:00Z")
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if err := b.Validate(); err != nil {
		t.Fatalf("Validate() failed: %v", err)
	}
	if got := len(b.Concepts); got != 2 {
		t.Fatalf("expected 2 concepts (index + 1 page), got %d", got)
	}

	idx := pageByID(b, "index")
	if idx == nil || idx.Type != "Bundle" {
		t.Fatal("index Bundle concept missing or wrong type")
	}
}

// TestConceptIDDeterminism verifies that two calls with identical URLs produce
// the same concept ID, and that a URL with special characters is sanitised.
func TestConceptIDDeterminism(t *testing.T) {
	tests := []struct {
		name     string
		urlA     string
		urlB     string
		sameID   bool   // whether the two URLs should map to the same ID
		contains string // substring the ID must contain
	}{
		{
			name:     "identical URLs produce same ID",
			urlA:     "https://example.com/path",
			urlB:     "https://example.com/path",
			sameID:   true,
			contains: "pages/",
		},
		{
			name:     "different paths produce different IDs",
			urlA:     "https://example.com/foo",
			urlB:     "https://example.com/bar",
			sameID:   false,
			contains: "pages/",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Build bundles with each URL to extract the generated ID.
			b1, err := knowledge.Build(tc.urlA, []knowledge.PageInput{
				{URL: tc.urlA, Title: "T", Markdown: "body", Timestamp: "2024-01-01T00:00:00Z"},
			}, "2024-01-01T00:00:00Z")
			if err != nil {
				t.Fatalf("Build A: %v", err)
			}
			b2, err := knowledge.Build(tc.urlB, []knowledge.PageInput{
				{URL: tc.urlB, Title: "T", Markdown: "body", Timestamp: "2024-01-01T00:00:00Z"},
			}, "2024-01-01T00:00:00Z")
			if err != nil {
				t.Fatalf("Build B: %v", err)
			}

			id1 := nonIndexID(b1)
			id2 := nonIndexID(b2)

			if tc.sameID && id1 != id2 {
				t.Errorf("expected same ID: got %q and %q", id1, id2)
			}
			if !tc.sameID && id1 == id2 {
				t.Errorf("expected different IDs: both got %q", id1)
			}
			if !strings.HasPrefix(id1, tc.contains) {
				t.Errorf("ID %q does not start with %q", id1, tc.contains)
			}
		})
	}
}

// TestConceptIDSanitization verifies that URLs with special characters produce
// filesystem-safe IDs containing only [a-z0-9/_-].
func TestConceptIDSanitization(t *testing.T) {
	rawURL := "https://Example.COM/Some Path?q=hello&lang=en#section"
	b, err := knowledge.Build(rawURL, []knowledge.PageInput{
		{URL: rawURL, Title: "Test", Markdown: "body", Timestamp: "2024-01-01T00:00:00Z"},
	}, "2024-01-01T00:00:00Z")
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	id := nonIndexID(b)
	for _, ch := range id {
		if !((ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '/' || ch == '_' || ch == '-') {
			t.Errorf("concept ID %q contains unsafe character %q", id, string(ch))
		}
	}

	if err := b.Validate(); err != nil {
		t.Fatalf("Validate() failed: %v", err)
	}
}

// helpers

// pageConceptByURL returns the Page-type concept whose Resource matches url.
// It skips the "Bundle"-type index concept which also carries the seed URL.
func pageConceptByURL(b *okf.Bundle, rawURL string) *okf.Concept {
	for i := range b.Concepts {
		if b.Concepts[i].Resource == rawURL && b.Concepts[i].Type == "Page" {
			return &b.Concepts[i]
		}
	}
	return nil
}

// nonIndexID returns the ID of the first non-"index" concept.
func nonIndexID(b *okf.Bundle) string {
	for _, c := range b.Concepts {
		if c.ID != "index" {
			return c.ID
		}
	}
	return ""
}
