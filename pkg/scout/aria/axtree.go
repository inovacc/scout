package aria

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// Node is a single entry in a Snapshot's flat node list. Children are encoded
// by index references into the parent Snapshot.Nodes slice.
type Node struct {
	Ref           string // "e15" (root) or "f2:e3" (frame 2)
	BackendNodeID int64  // CDP DOM.BackendNodeId
	Role          string // e.g. "button", "textbox"
	Name          string // accessible name
	Value         string // current value, if any
	Children      []int  // indices into Snapshot.Nodes
	FrameID       string // "" for root frame
}

// Snapshot is a captured accessibility tree at a point in time. Immutable
// once Put into the Store. Capture (added in Task 5) produces a new Snapshot
// with a fresh Version on each call.
type Snapshot struct {
	PageID     string
	Version    uint64
	Nodes      []Node
	URI        string
	CapturedAt time.Time
	Truncated  bool
}

// RenderYAML writes a YAML-like representation of the snapshot. The format
// matches the playwright-mcp ARIA dialect:
//
//	- role "accessible name" [ref=eN]
//	  - child role "name" [ref=eN+1]
//
// Roots are nodes referenced by no other node. Values are appended in the
// form: textbox "Label" [ref=eN] value="current text".
func (s *Snapshot) RenderYAML(w io.Writer) error {
	if s == nil {
		return fmt.Errorf("scout: aria: render: nil snapshot")
	}
	referenced := make([]bool, len(s.Nodes))
	for i := range s.Nodes {
		for _, ci := range s.Nodes[i].Children {
			if ci >= 0 && ci < len(referenced) {
				referenced[ci] = true
			}
		}
	}
	for i := range s.Nodes {
		if !referenced[i] {
			if err := renderNode(w, s, i, 0); err != nil {
				return fmt.Errorf("scout: aria: render: %w", err)
			}
		}
	}
	return nil
}

func renderNode(w io.Writer, s *Snapshot, idx, depth int) error {
	n := &s.Nodes[idx]
	indent := strings.Repeat("  ", depth)
	line := fmt.Sprintf("%s- %s", indent, n.Role)
	if n.Name != "" {
		line += fmt.Sprintf(" %q", n.Name)
	}
	line += fmt.Sprintf(" [ref=%s]", n.Ref)
	if n.Value != "" {
		line += fmt.Sprintf(" value=%q", n.Value)
	}
	if _, err := fmt.Fprintln(w, line); err != nil {
		return err
	}
	for _, ci := range n.Children {
		if ci < 0 || ci >= len(s.Nodes) {
			continue
		}
		if err := renderNode(w, s, ci, depth+1); err != nil {
			return err
		}
	}
	return nil
}
