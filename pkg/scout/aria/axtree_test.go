package aria_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/inovacc/scout/pkg/scout/aria"
)

type fixtureSnapshot struct {
	PageID    string        `json:"page_id"`
	Version   uint64        `json:"version"`
	Nodes     []fixtureNode `json:"nodes"`
	Truncated bool          `json:"truncated"`
}
type fixtureNode struct {
	Ref           string `json:"ref"`
	BackendNodeID int64  `json:"backend_node_id"`
	Role          string `json:"role"`
	Name          string `json:"name"`
	Value         string `json:"value,omitempty"`
	Children      []int  `json:"children"`
	FrameID       string `json:"frame_id,omitempty"`
}

func loadFixture(t *testing.T, name string) *aria.Snapshot {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name+".json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var fx fixtureSnapshot
	if err := json.Unmarshal(raw, &fx); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	nodes := make([]aria.Node, len(fx.Nodes))
	for i, fn := range fx.Nodes {
		nodes[i] = aria.Node{
			Ref: fn.Ref, BackendNodeID: fn.BackendNodeID,
			Role: fn.Role, Name: fn.Name, Value: fn.Value,
			Children: fn.Children, FrameID: fn.FrameID,
		}
	}
	return &aria.Snapshot{
		PageID: fx.PageID, Version: fx.Version, Nodes: nodes, Truncated: fx.Truncated,
	}
}

func TestRenderYAML_SimpleForm(t *testing.T) {
	t.Parallel()
	snap := loadFixture(t, "simple_form")
	var buf bytes.Buffer
	if err := snap.RenderYAML(&buf); err != nil {
		t.Fatalf("RenderYAML err=%v", err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "simple_form.yaml"))
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Errorf("rendering mismatch\n--- got ---\n%s\n--- want ---\n%s", buf.String(), want)
	}
}
