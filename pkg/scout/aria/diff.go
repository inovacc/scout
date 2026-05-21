package aria

import (
	"fmt"
	"sort"
	"strings"
)

// NodeChange describes a single node that was added, removed, or changed.
type NodeChange struct {
	Ref  string
	Role string
	Name string
}

// Summary is the structural delta between two Snapshots.
type Summary struct {
	Added   []NodeChange
	Removed []NodeChange
	Changed []NodeChange
}

// String returns a deterministic human-readable summary of the delta.
// Format is intentionally stable — Phase B action tools pin this string.
func (s Summary) String() string {
	if len(s.Added) == 0 && len(s.Removed) == 0 && len(s.Changed) == 0 {
		return "no changes"
	}
	var b strings.Builder
	if n := len(s.Added); n > 0 {
		_, _ = fmt.Fprintf(&b, "%d element%s added", n, plural(n))
		writeDetails(&b, s.Added)
	}
	if n := len(s.Removed); n > 0 {
		if b.Len() > 0 {
			b.WriteString("; ")
		}
		_, _ = fmt.Fprintf(&b, "%d removed", n)
		writeDetails(&b, s.Removed)
	}
	if n := len(s.Changed); n > 0 {
		if b.Len() > 0 {
			b.WriteString("; ")
		}
		_, _ = fmt.Fprintf(&b, "%d changed", n)
		writeDetails(&b, s.Changed)
	}
	return b.String()
}

func writeDetails(b *strings.Builder, changes []NodeChange) {
	const max = 3
	if len(changes) <= max {
		b.WriteString(" (")
		for i, c := range changes {
			if i > 0 {
				b.WriteString(", ")
			}
			writeChange(b, c)
		}
		b.WriteString(")")
		return
	}
	b.WriteString(" (")
	for i := range max {
		if i > 0 {
			b.WriteString(", ")
		}
		writeChange(b, changes[i])
	}
	_, _ = fmt.Fprintf(b, ", and %d more)", len(changes)-max)
}

func writeChange(b *strings.Builder, c NodeChange) {
	_, _ = fmt.Fprintf(b, "ref=%s %s", c.Ref, c.Role)
	if c.Name != "" {
		_, _ = fmt.Fprintf(b, " %q", c.Name)
	}
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// Diff compares two snapshots and returns a structural Summary. Comparison
// keys on Ref; same ref + changed role/name/value is Changed; refs present
// only in before are Removed; refs present only in after are Added.
// Result lists are sorted by Ref for deterministic Summary.String output.
// Both before and after may be nil.
func Diff(before, after *Snapshot) Summary {
	beforeByRef := map[string]Node{}
	if before != nil {
		for _, n := range before.Nodes {
			beforeByRef[n.Ref] = n
		}
	}
	afterByRef := map[string]Node{}
	if after != nil {
		for _, n := range after.Nodes {
			afterByRef[n.Ref] = n
		}
	}

	var s Summary
	for ref, n := range afterByRef {
		old, ok := beforeByRef[ref]
		if !ok {
			s.Added = append(s.Added, NodeChange{Ref: ref, Role: n.Role, Name: n.Name})
			continue
		}
		if old.Role != n.Role || old.Name != n.Name || old.Value != n.Value {
			s.Changed = append(s.Changed, NodeChange{Ref: ref, Role: n.Role, Name: n.Name})
		}
	}
	for ref, n := range beforeByRef {
		if _, ok := afterByRef[ref]; !ok {
			s.Removed = append(s.Removed, NodeChange{Ref: ref, Role: n.Role, Name: n.Name})
		}
	}

	sort.Slice(s.Added, func(i, j int) bool { return s.Added[i].Ref < s.Added[j].Ref })
	sort.Slice(s.Removed, func(i, j int) bool { return s.Removed[i].Ref < s.Removed[j].Ref })
	sort.Slice(s.Changed, func(i, j int) bool { return s.Changed[i].Ref < s.Changed[j].Ref })
	return s
}
