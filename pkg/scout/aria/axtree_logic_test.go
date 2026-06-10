package aria_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	proto "github.com/inovacc/scout/internal/engine/lib/proto"
	"github.com/inovacc/scout/pkg/scout/aria"
)

// axVal builds an AccessibilityAXValue wrapping v, mirroring what CDP returns
// for role/name/value computed properties. It populates the gson.JSON field the
// same way the engine does — by unmarshalling JSON — so the test needs no direct
// gson import (forbidden in the aria layer by the depguard layering rule).
func axVal(v any) *proto.AccessibilityAXValue {
	raw, err := json.Marshal(map[string]any{"value": v})
	if err != nil {
		panic(err) // unreachable: string/int always marshal
	}
	var av proto.AccessibilityAXValue
	if err := json.Unmarshal(raw, &av); err != nil {
		panic(err) // unreachable: well-formed JSON into a known struct
	}
	return &av
}

func TestDefaultCfg(t *testing.T) {
	t.Parallel()
	maxNodes, timeout := aria.DefaultCfgForTest()
	if maxNodes != 10000 {
		t.Errorf("default maxNodes = %d, want 10000", maxNodes)
	}
	if timeout != 5*time.Second {
		t.Errorf("default timeout = %v, want 5s", timeout)
	}
}

func TestWithMaxNodes_And_WithCaptureTimeout(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		opts        []aria.Option
		wantMax     int
		wantTimeout time.Duration
	}{
		{
			name:        "no options keeps defaults",
			opts:        nil,
			wantMax:     10000,
			wantTimeout: 5 * time.Second,
		},
		{
			name:        "WithMaxNodes only",
			opts:        []aria.Option{aria.WithMaxNodes(42)},
			wantMax:     42,
			wantTimeout: 5 * time.Second,
		},
		{
			name:        "WithCaptureTimeout only",
			opts:        []aria.Option{aria.WithCaptureTimeout(250 * time.Millisecond)},
			wantMax:     10000,
			wantTimeout: 250 * time.Millisecond,
		},
		{
			name:        "both options compose",
			opts:        []aria.Option{aria.WithMaxNodes(7), aria.WithCaptureTimeout(time.Minute)},
			wantMax:     7,
			wantTimeout: time.Minute,
		},
		{
			name:        "last WithMaxNodes wins",
			opts:        []aria.Option{aria.WithMaxNodes(7), aria.WithMaxNodes(13)},
			wantMax:     13,
			wantTimeout: 5 * time.Second,
		},
		{
			name:        "zero maxNodes is honored",
			opts:        []aria.Option{aria.WithMaxNodes(0)},
			wantMax:     0,
			wantTimeout: 5 * time.Second,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotMax, gotTimeout := aria.ApplyOptionsForTest(tc.opts...)
			if gotMax != tc.wantMax {
				t.Errorf("maxNodes = %d, want %d", gotMax, tc.wantMax)
			}
			if gotTimeout != tc.wantTimeout {
				t.Errorf("timeout = %v, want %v", gotTimeout, tc.wantTimeout)
			}
		})
	}
}

func TestAXStrings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                   string
		ax                     *proto.AccessibilityAXNode
		wantRole, wantName, wantVal string
	}{
		{
			name:     "all fields present",
			ax:       &proto.AccessibilityAXNode{Role: axVal("button"), Name: axVal("Submit"), Value: axVal("v1")},
			wantRole: "button", wantName: "Submit", wantVal: "v1",
		},
		{
			name:     "all nil yields empty strings",
			ax:       &proto.AccessibilityAXNode{},
			wantRole: "", wantName: "", wantVal: "",
		},
		{
			name:     "only role set",
			ax:       &proto.AccessibilityAXNode{Role: axVal("textbox")},
			wantRole: "textbox", wantName: "", wantVal: "",
		},
		{
			name:     "name and value without role",
			ax:       &proto.AccessibilityAXNode{Name: axVal("Email"), Value: axVal("a@b.com")},
			wantRole: "", wantName: "Email", wantVal: "a@b.com",
		},
		{
			name: "non-string role value stringifies via fmt",
			ax: &proto.AccessibilityAXNode{
				Role: axVal(42),
			},
			wantRole: "42", wantName: "", wantVal: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			role, name, val := aria.AXStringsForTest(tc.ax)
			if role != tc.wantRole || name != tc.wantName || val != tc.wantVal {
				t.Errorf("axStrings = (%q, %q, %q), want (%q, %q, %q)",
					role, name, val, tc.wantRole, tc.wantName, tc.wantVal)
			}
		})
	}
}

func TestConvertAXNodes_RefNumberingAndChildren(t *testing.T) {
	t.Parallel()
	// root -> {a, b}; a has no children; b -> c.
	in := []*proto.AccessibilityAXNode{
		{NodeID: "root", BackendDOMNodeID: 100, Role: axVal("main"), ChildIDs: []proto.AccessibilityAXNodeID{"a", "b"}},
		{NodeID: "a", BackendDOMNodeID: 101, Role: axVal("link"), Name: axVal("Home")},
		{NodeID: "b", BackendDOMNodeID: 102, Role: axVal("group"), ChildIDs: []proto.AccessibilityAXNodeID{"c"}},
		{NodeID: "c", BackendDOMNodeID: 103, Role: axVal("button"), Name: axVal("Go"), Value: axVal("x")},
	}
	counter := 0
	out, truncated := aria.ConvertWithPrefixForTest(in, 1000, "", &counter)
	if truncated {
		t.Fatalf("truncated = true, want false")
	}
	if len(out) != 4 {
		t.Fatalf("len(out) = %d, want 4", len(out))
	}
	// Refs are e1..e4 in input order, counter advanced to 4.
	wantRefs := []string{"e1", "e2", "e3", "e4"}
	for i, n := range out {
		if n.Ref != wantRefs[i] {
			t.Errorf("node %d ref = %q, want %q", i, n.Ref, wantRefs[i])
		}
	}
	if counter != 4 {
		t.Errorf("counter = %d, want 4", counter)
	}
	// root's children resolve to indices 1 and 2.
	if got := out[0].Children; len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Errorf("root children = %v, want [1 2]", got)
	}
	// b's child c resolves to index 3.
	if got := out[2].Children; len(got) != 1 || got[0] != 3 {
		t.Errorf("b children = %v, want [3]", got)
	}
	// Leaf node fields fully populated.
	if out[3].Role != "button" || out[3].Name != "Go" || out[3].Value != "x" || out[3].BackendNodeID != 103 {
		t.Errorf("leaf node = %+v", out[3])
	}
}

func TestConvertAXNodes_FramePrefixAndUnknownChildDropped(t *testing.T) {
	t.Parallel()
	// One child id points at a node not present in the slice; it must be dropped.
	in := []*proto.AccessibilityAXNode{
		{NodeID: "p", BackendDOMNodeID: 1, Role: axVal("region"), ChildIDs: []proto.AccessibilityAXNodeID{"known", "ghost"}},
		{NodeID: "known", BackendDOMNodeID: 2, Role: axVal("paragraph")},
	}
	counter := 5 // pre-seeded counter; refs should continue from e6.
	out, _ := aria.ConvertWithPrefixForTest(in, 1000, "f2", &counter)
	if len(out) != 2 {
		t.Fatalf("len(out) = %d, want 2", len(out))
	}
	if out[0].Ref != "f2:e6" || out[1].Ref != "f2:e7" {
		t.Errorf("refs = %q,%q want f2:e6,f2:e7", out[0].Ref, out[1].Ref)
	}
	if out[0].FrameID != "f2" {
		t.Errorf("FrameID = %q, want f2", out[0].FrameID)
	}
	// Only the resolvable child ("known" -> index 1) survives; "ghost" dropped.
	if got := out[0].Children; len(got) != 1 || got[0] != 1 {
		t.Errorf("children = %v, want [1] (ghost dropped)", got)
	}
}

func TestConvertAXNodes_TruncationViaConvertForTest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name          string
		count         int
		maxNodes      int
		wantLen       int
		wantTruncated bool
	}{
		{name: "under cap", count: 3, maxNodes: 10, wantLen: 3, wantTruncated: false},
		{name: "exactly at cap", count: 5, maxNodes: 5, wantLen: 5, wantTruncated: false},
		{name: "over cap truncates", count: 9, maxNodes: 4, wantLen: 4, wantTruncated: true},
		{name: "empty input", count: 0, maxNodes: 10, wantLen: 0, wantTruncated: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			in := make([]*proto.AccessibilityAXNode, 0, tc.count)
			for i := 0; i < tc.count; i++ {
				in = append(in, &proto.AccessibilityAXNode{
					NodeID:           proto.AccessibilityAXNodeID(strings.Repeat("x", i+1)),
					BackendDOMNodeID: proto.DOMBackendNodeID(i),
				})
			}
			out, truncated := aria.ConvertForTest(in, tc.maxNodes)
			if len(out) != tc.wantLen {
				t.Errorf("len = %d, want %d", len(out), tc.wantLen)
			}
			if truncated != tc.wantTruncated {
				t.Errorf("truncated = %v, want %v", truncated, tc.wantTruncated)
			}
		})
	}
}

func frame(id string, children ...*proto.PageFrameTree) *proto.PageFrameTree {
	return &proto.PageFrameTree{
		Frame:       &proto.PageFrame{ID: proto.PageFrameID(id)},
		ChildFrames: children,
	}
}

func TestCollectChildFrameIDs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		tree *proto.PageFrameTree
		want []string
	}{
		{
			name: "nil tree",
			tree: nil,
			want: nil,
		},
		{
			name: "root with no children",
			tree: frame("root"),
			want: nil,
		},
		{
			name: "single level children, root excluded",
			tree: frame("root", frame("c1"), frame("c2")),
			want: []string{"c1", "c2"},
		},
		{
			name: "nested depth-first order",
			tree: frame("root",
				frame("c1", frame("c1a"), frame("c1b")),
				frame("c2"),
			),
			want: []string{"c1", "c1a", "c1b", "c2"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := aria.CollectChildFrameIDsForTest(tc.tree)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestRenderYAML_NilSnapshot(t *testing.T) {
	t.Parallel()
	var s *aria.Snapshot
	err := s.RenderYAML(io.Discard)
	if err == nil {
		t.Fatal("RenderYAML(nil) err = nil, want error")
	}
	if !strings.Contains(err.Error(), "nil snapshot") {
		t.Errorf("err = %v, want nil-snapshot message", err)
	}
}

func TestRenderYAML_RootsValuesAndNesting(t *testing.T) {
	t.Parallel()
	// Two roots: node 0 (with child node 1) and node 2 (standalone w/ value).
	snap := &aria.Snapshot{
		Nodes: []aria.Node{
			{Ref: "e1", Role: "form", Name: "Login", Children: []int{1}},
			{Ref: "e2", Role: "textbox", Name: "User", Value: "alice"},
			{Ref: "e3", Role: "button"}, // no name, no value
		},
	}
	var buf bytes.Buffer
	if err := snap.RenderYAML(&buf); err != nil {
		t.Fatalf("RenderYAML err = %v", err)
	}
	want := strings.Join([]string{
		`- form "Login" [ref=e1]`,
		`  - textbox "User" [ref=e2] value="alice"`,
		`- button [ref=e3]`,
		"",
	}, "\n")
	if got := buf.String(); got != want {
		t.Errorf("render mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderYAML_OutOfRangeChildSkipped(t *testing.T) {
	t.Parallel()
	// Child index 99 is out of range and must be skipped without panic.
	snap := &aria.Snapshot{
		Nodes: []aria.Node{
			{Ref: "e1", Role: "list", Children: []int{99, -1, 1}},
			{Ref: "e2", Role: "listitem", Name: "Item"},
		},
	}
	var buf bytes.Buffer
	if err := snap.RenderYAML(&buf); err != nil {
		t.Fatalf("RenderYAML err = %v", err)
	}
	got := buf.String()
	if !strings.Contains(got, "- list [ref=e1]") {
		t.Errorf("missing root line in:\n%s", got)
	}
	if !strings.Contains(got, `  - listitem "Item" [ref=e2]`) {
		t.Errorf("missing nested valid child in:\n%s", got)
	}
}

// errWriter fails after allowing n successful writes, to exercise the error
// propagation path in renderNode / RenderYAML.
type errWriter struct {
	remaining int
}

func (w *errWriter) Write(p []byte) (int, error) {
	if w.remaining <= 0 {
		return 0, errors.New("forced write failure")
	}
	w.remaining--
	return len(p), nil
}

func TestRenderYAML_WriteErrorPropagates(t *testing.T) {
	t.Parallel()
	snap := &aria.Snapshot{
		Nodes: []aria.Node{
			{Ref: "e1", Role: "main", Children: []int{1}},
			{Ref: "e2", Role: "button", Name: "Click"},
		},
	}
	// Fail on the very first write (the root line).
	err := snap.RenderYAML(&errWriter{remaining: 0})
	if err == nil {
		t.Fatal("expected write error, got nil")
	}
	if !strings.Contains(err.Error(), "render") {
		t.Errorf("err = %v, want wrapped render error", err)
	}

	// Fail on the nested child write (after the root line succeeds).
	err = snap.RenderYAML(&errWriter{remaining: 1})
	if err == nil {
		t.Fatal("expected nested write error, got nil")
	}
}
