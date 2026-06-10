package aria_test

import (
	"strings"
	"testing"

	"github.com/inovacc/scout/pkg/scout/aria"
)

func snapOf(nodes ...aria.Node) *aria.Snapshot {
	return &aria.Snapshot{Nodes: nodes}
}

func TestDiff_AddRemoveChange(t *testing.T) {
	t.Parallel()
	before := snapOf(
		aria.Node{Ref: "e1", Role: "button", Name: "Old"},
		aria.Node{Ref: "e2", Role: "textbox", Name: "Keep"},
		aria.Node{Ref: "e3", Role: "link", Name: "Gone"},
	)
	after := snapOf(
		aria.Node{Ref: "e1", Role: "button", Name: "New"},     // changed name
		aria.Node{Ref: "e2", Role: "textbox", Name: "Keep"},   // unchanged
		aria.Node{Ref: "e4", Role: "heading", Name: "Fresh"},  // added
	)
	got := aria.Diff(before, after)

	if len(got.Added) != 1 || got.Added[0].Ref != "e4" {
		t.Errorf("Added = %+v, want [e4]", got.Added)
	}
	if len(got.Removed) != 1 || got.Removed[0].Ref != "e3" {
		t.Errorf("Removed = %+v, want [e3]", got.Removed)
	}
	if len(got.Changed) != 1 || got.Changed[0].Ref != "e1" {
		t.Errorf("Changed = %+v, want [e1]", got.Changed)
	}
}

func TestDiff_ValueChangeCountsAsChanged(t *testing.T) {
	t.Parallel()
	// Same role and name, but Value differs -> Changed.
	before := snapOf(aria.Node{Ref: "e1", Role: "textbox", Name: "Email", Value: "old@x.com"})
	after := snapOf(aria.Node{Ref: "e1", Role: "textbox", Name: "Email", Value: "new@x.com"})
	got := aria.Diff(before, after)
	if len(got.Changed) != 1 || len(got.Added) != 0 || len(got.Removed) != 0 {
		t.Fatalf("want only one Changed, got %+v", got)
	}
}

func TestDiff_NilSnapshots(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name             string
		before, after    *aria.Snapshot
		wantAdded        int
		wantRemoved      int
	}{
		{
			name:   "both nil",
			before: nil, after: nil,
			wantAdded: 0, wantRemoved: 0,
		},
		{
			name:   "nil before -> all added",
			before: nil,
			after:  snapOf(aria.Node{Ref: "e1", Role: "button"}, aria.Node{Ref: "e2", Role: "link"}),
			wantAdded: 2, wantRemoved: 0,
		},
		{
			name:   "nil after -> all removed",
			before: snapOf(aria.Node{Ref: "e1", Role: "button"}),
			after:  nil,
			wantAdded: 0, wantRemoved: 1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := aria.Diff(tc.before, tc.after)
			if len(got.Added) != tc.wantAdded {
				t.Errorf("Added = %d, want %d", len(got.Added), tc.wantAdded)
			}
			if len(got.Removed) != tc.wantRemoved {
				t.Errorf("Removed = %d, want %d", len(got.Removed), tc.wantRemoved)
			}
		})
	}
}

func TestDiff_DeterministicSortByRef(t *testing.T) {
	t.Parallel()
	// Added refs supplied out of order; result must be sorted by Ref.
	before := snapOf()
	after := snapOf(
		aria.Node{Ref: "e3", Role: "c"},
		aria.Node{Ref: "e1", Role: "a"},
		aria.Node{Ref: "e2", Role: "b"},
	)
	got := aria.Diff(before, after)
	gotRefs := []string{got.Added[0].Ref, got.Added[1].Ref, got.Added[2].Ref}
	want := []string{"e1", "e2", "e3"}
	for i := range want {
		if gotRefs[i] != want[i] {
			t.Fatalf("Added order = %v, want %v", gotRefs, want)
		}
	}
}

func TestSummaryString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		summary aria.Summary
		want    string
	}{
		{
			name:    "no changes",
			summary: aria.Summary{},
			want:    "no changes",
		},
		{
			name: "single added (singular, no plural s)",
			summary: aria.Summary{
				Added: []aria.NodeChange{{Ref: "e1", Role: "button", Name: "OK"}},
			},
			want: `1 element added (ref=e1 button "OK")`,
		},
		{
			name: "two added (plural s)",
			summary: aria.Summary{
				Added: []aria.NodeChange{
					{Ref: "e1", Role: "button"},
					{Ref: "e2", Role: "link", Name: "Home"},
				},
			},
			want: `2 elements added (ref=e1 button, ref=e2 link "Home")`,
		},
		{
			name: "added removed and changed joined with semicolons",
			summary: aria.Summary{
				Added:   []aria.NodeChange{{Ref: "e1", Role: "button"}},
				Removed: []aria.NodeChange{{Ref: "e2", Role: "link"}},
				Changed: []aria.NodeChange{{Ref: "e3", Role: "textbox", Name: "Email"}},
			},
			want: `1 element added (ref=e1 button); 1 removed (ref=e2 link); 1 changed (ref=e3 textbox "Email")`,
		},
		{
			name: "more than three truncates with and N more",
			summary: aria.Summary{
				Added: []aria.NodeChange{
					{Ref: "e1", Role: "a"},
					{Ref: "e2", Role: "b"},
					{Ref: "e3", Role: "c"},
					{Ref: "e4", Role: "d"},
					{Ref: "e5", Role: "e"},
				},
			},
			want: `5 elements added (ref=e1 a, ref=e2 b, ref=e3 c, and 2 more)`,
		},
		{
			name: "removed only, no added prefix",
			summary: aria.Summary{
				Removed: []aria.NodeChange{{Ref: "e9", Role: "img", Name: "logo"}},
			},
			want: `1 removed (ref=e9 img "logo")`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.summary.String(); got != tc.want {
				t.Errorf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSummaryString_ExactlyThreeNoTruncation(t *testing.T) {
	t.Parallel()
	s := aria.Summary{
		Changed: []aria.NodeChange{
			{Ref: "e1", Role: "a"},
			{Ref: "e2", Role: "b"},
			{Ref: "e3", Role: "c"},
		},
	}
	got := s.String()
	if strings.Contains(got, "more") {
		t.Errorf("exactly 3 should not truncate: %q", got)
	}
	want := "3 changed (ref=e1 a, ref=e2 b, ref=e3 c)"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestDiff_String_RoundTrip(t *testing.T) {
	t.Parallel()
	// Exercise Diff -> String end to end on a small structural delta.
	before := snapOf(
		aria.Node{Ref: "e1", Role: "button", Name: "Save"},
		aria.Node{Ref: "e2", Role: "link", Name: "Old"},
	)
	after := snapOf(
		aria.Node{Ref: "e1", Role: "button", Name: "Save"}, // unchanged
		aria.Node{Ref: "e3", Role: "heading", Name: "New"}, // added
	)
	got := aria.Diff(before, after).String()
	// e2 removed, e3 added; both single, deterministic.
	want := `1 element added (ref=e3 heading "New"); 1 removed (ref=e2 link "Old")`
	if got != want {
		t.Errorf("round trip = %q, want %q", got, want)
	}
}
