package aria

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"time"

	"github.com/inovacc/scout/internal/engine"
	proto2 "github.com/inovacc/scout/internal/engine/lib/proto"
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

var versionCounter atomic.Uint64

// Option configures a Capture call.
type Option func(*captureCfg)

type captureCfg struct {
	maxNodes int
	timeout  time.Duration
}

func defaultCfg() *captureCfg {
	return &captureCfg{maxNodes: 10000, timeout: 5 * time.Second}
}

// WithMaxNodes limits the number of nodes captured from the AX tree.
func WithMaxNodes(n int) Option { return func(c *captureCfg) { c.maxNodes = n } }

// WithCaptureTimeout sets a deadline for the CDP call.
func WithCaptureTimeout(d time.Duration) Option { return func(c *captureCfg) { c.timeout = d } }

// Capture runs CDP Accessibility.getFullAXTree on the page (root frame) and
// converts the result to a *Snapshot.
func Capture(ctx context.Context, page *engine.Page, opts ...Option) (*Snapshot, error) {
	cfg := defaultCfg()
	for _, opt := range opts {
		opt(cfg)
	}

	ctx, cancel := context.WithTimeout(ctx, cfg.timeout)
	defer cancel()

	type axResult struct {
		res *proto2.AccessibilityGetFullAXTreeResult
		err error
	}
	resultCh := make(chan axResult, 1)
	go func() {
		res, err := proto2.AccessibilityGetFullAXTree{}.Call(page.RodPage())
		resultCh <- axResult{res, err}
	}()

	var result *proto2.AccessibilityGetFullAXTreeResult
	select {
	case r := <-resultCh:
		if r.err != nil {
			return nil, fmt.Errorf("scout: aria: capture: getFullAXTree: %w", r.err)
		}
		result = r.res
	case <-ctx.Done():
		return nil, fmt.Errorf("scout: aria: capture: %w", ctx.Err())
	}

	counter := 0
	nodes, truncated := convertAXNodes(result.Nodes, cfg, "", &counter)

	pageID := string(page.RodPage().TargetID)
	version := versionCounter.Add(1)
	return &Snapshot{
		PageID:     pageID,
		Version:    version,
		Nodes:      nodes,
		URI:        fmt.Sprintf("scout://snapshot/%s?v=%d", pageID, version),
		CapturedAt: time.Now(),
		Truncated:  truncated,
	}, nil
}

func convertAXNodes(in []*proto2.AccessibilityAXNode, cfg *captureCfg, framePrefix string, counter *int) ([]Node, bool) {
	truncated := false
	if len(in) > cfg.maxNodes {
		in = in[:cfg.maxNodes]
		truncated = true
	}
	idByAX := make(map[proto2.AccessibilityAXNodeID]int, len(in))
	for i, ax := range in {
		idByAX[ax.NodeID] = i
	}
	out := make([]Node, 0, len(in))
	for _, ax := range in {
		*counter++
		ref := fmt.Sprintf("e%d", *counter)
		if framePrefix != "" {
			ref = framePrefix + ":" + ref
		}
		role, name, val := axStrings(ax)
		children := make([]int, 0, len(ax.ChildIDs))
		for _, cid := range ax.ChildIDs {
			if ci, ok := idByAX[cid]; ok {
				children = append(children, ci)
			}
		}
		out = append(out, Node{
			Ref:           ref,
			BackendNodeID: int64(ax.BackendDOMNodeID),
			Role:          role,
			Name:          name,
			Value:         val,
			Children:      children,
			FrameID:       framePrefix,
		})
	}
	return out, truncated
}

func axStrings(ax *proto2.AccessibilityAXNode) (role, name, val string) {
	if ax.Role != nil {
		role = ax.Role.Value.Str()
	}
	if ax.Name != nil {
		name = ax.Name.Value.Str()
	}
	if ax.Value != nil {
		val = ax.Value.Value.Str()
	}
	return
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
