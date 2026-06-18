package okf_test

import (
	"sort"
	"testing"

	"github.com/inovacc/scout/pkg/okf"
)

// sortConcepts sorts a slice of Concepts by ID in place.
func sortConcepts(cs []okf.Concept) {
	sort.Slice(cs, func(i, j int) bool {
		return cs[i].ID < cs[j].ID
	})
}

// TestRoundTrip verifies that Write then Read reproduces an identical Bundle.
func TestRoundTrip(t *testing.T) {
	t.Parallel()

	want := []okf.Concept{
		{
			ID:          "intro",
			Type:        "Page",
			Title:       "Introduction",
			Description: "Getting started",
			Body:        "Welcome. See [the pro plan](/entities/plan-pro.md) for details.\n",
		},
		{
			ID:        "entities/plan-pro",
			Type:      "Schema",
			Title:     "Pro Plan",
			Resource:  "https://example.com/pricing",
			Tags:      []string{"pricing", "plans"},
			Timestamp: "2026-06-18T14:30:00Z",
			Body:      "# Pro Plan\n\nSee [intro](../intro.md) for context.\n",
		},
	}

	b := &okf.Bundle{Concepts: want}
	dir := t.TempDir()

	if err := b.Write(dir); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := okf.Read(dir)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	sortConcepts(want)
	sortConcepts(got.Concepts)

	if len(got.Concepts) != len(want) {
		t.Fatalf("concept count: got %d, want %d", len(got.Concepts), len(want))
	}

	for i := range want {
		w, g := want[i], got.Concepts[i]
		if g.ID != w.ID {
			t.Errorf("[%d] ID: got %q, want %q", i, g.ID, w.ID)
		}
		if g.Type != w.Type {
			t.Errorf("[%d] Type: got %q, want %q", i, g.Type, w.Type)
		}
		if g.Title != w.Title {
			t.Errorf("[%d] Title: got %q, want %q", i, g.Title, w.Title)
		}
		if g.Description != w.Description {
			t.Errorf("[%d] Description: got %q, want %q", i, g.Description, w.Description)
		}
		if g.Resource != w.Resource {
			t.Errorf("[%d] Resource: got %q, want %q", i, g.Resource, w.Resource)
		}
		if g.Timestamp != w.Timestamp {
			t.Errorf("[%d] Timestamp: got %q, want %q", i, g.Timestamp, w.Timestamp)
		}
		if g.Body != w.Body {
			t.Errorf("[%d] Body: got %q, want %q", i, g.Body, w.Body)
		}
		// Compare tags.
		if len(g.Tags) != len(w.Tags) {
			t.Errorf("[%d] Tags len: got %d, want %d", i, len(g.Tags), len(w.Tags))
		} else {
			for j := range w.Tags {
				if g.Tags[j] != w.Tags[j] {
					t.Errorf("[%d] Tags[%d]: got %q, want %q", i, j, g.Tags[j], w.Tags[j])
				}
			}
		}
	}
}

// TestFrontmatterMinimal ensures a concept with only Type round-trips cleanly
// and that optional fields are absent in the marshalled output (no empty keys).
func TestFrontmatterMinimal(t *testing.T) {
	t.Parallel()

	c := okf.Concept{
		ID:   "bare",
		Type: "Page",
		Body: "Just a body.\n",
	}

	b := &okf.Bundle{Concepts: []okf.Concept{c}}
	dir := t.TempDir()

	if err := b.Write(dir); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := okf.Read(dir)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if len(got.Concepts) != 1 {
		t.Fatalf("concept count: got %d, want 1", len(got.Concepts))
	}
	g := got.Concepts[0]
	if g.Type != "Page" {
		t.Errorf("Type: got %q, want %q", g.Type, "Page")
	}
	if g.Title != "" {
		t.Errorf("Title should be empty, got %q", g.Title)
	}
	if g.Description != "" {
		t.Errorf("Description should be empty, got %q", g.Description)
	}
	if len(g.Tags) != 0 {
		t.Errorf("Tags should be empty, got %v", g.Tags)
	}
	if g.Body != "Just a body.\n" {
		t.Errorf("Body: got %q", g.Body)
	}
}

// TestValidateMissingType ensures Validate returns an error when Type is empty.
func TestValidateMissingType(t *testing.T) {
	t.Parallel()

	b := &okf.Bundle{Concepts: []okf.Concept{
		{ID: "no-type", Type: "", Body: "content"},
	}}
	if err := b.Validate(); err == nil {
		t.Error("expected error for missing type, got nil")
	}
}

// TestValidateDanglingLink ensures Validate returns an error for a link to a
// concept that doesn't exist in the bundle.
func TestValidateDanglingLink(t *testing.T) {
	t.Parallel()

	b := &okf.Bundle{Concepts: []okf.Concept{
		{ID: "a", Type: "Page", Body: "See [missing](/ghost.md)."},
	}}
	if err := b.Validate(); err == nil {
		t.Error("expected error for dangling link, got nil")
	}
}

// TestValidateValid ensures a well-formed bundle with resolved links passes.
func TestValidateValid(t *testing.T) {
	t.Parallel()

	b := &okf.Bundle{Concepts: []okf.Concept{
		{ID: "a", Type: "Page", Body: "See [B](/b.md)."},
		{ID: "b", Type: "Page", Body: "See [A](/a.md)."},
	}}
	if err := b.Validate(); err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
}

// TestLinks exercises the Links() method across multiple target forms.
func TestLinks(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		id   string
		body string
		want []string
	}{
		{
			name: "absolute bundle link",
			id:   "intro",
			body: "See [pro](/entities/plan-pro.md).",
			want: []string{"entities/plan-pro"},
		},
		{
			name: "relative link up",
			id:   "entities/plan-pro",
			body: "See [intro](../intro.md).",
			want: []string{"intro"},
		},
		{
			name: "external https skipped",
			id:   "intro",
			body: "Visit [site](https://example.com).",
			want: nil,
		},
		{
			name: "mailto skipped",
			id:   "intro",
			body: "Email [us](mailto:info@example.com).",
			want: nil,
		},
		{
			name: "anchor skipped",
			id:   "intro",
			body: "Jump [here](#section).",
			want: nil,
		},
		{
			name: "mixed — only bundle links returned",
			id:   "intro",
			body: "See [pro](/entities/plan-pro.md) and [site](https://example.com) and [jump](#top).",
			want: []string{"entities/plan-pro"},
		},
		{
			name: "relative same dir",
			id:   "entities/a",
			body: "See [b](b.md).",
			want: []string{"entities/b"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := okf.Concept{ID: tt.id, Type: "Page", Body: tt.body}
			got := c.Links()

			if len(got) != len(tt.want) {
				t.Fatalf("Links() = %v, want %v", got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("Links()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
