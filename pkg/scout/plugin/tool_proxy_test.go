package plugin

import (
	"testing"
)

func TestToolProxy_NamespacePrefix(t *testing.T) {
	proxy := &ToolProxy{
		entry:    ToolEntry{Name: "greet", Description: "Say hello"},
		manifest: &Manifest{Name: "example"},
	}

	wantName := "plugin_example_greet"
	gotName := "plugin_" + proxy.manifest.Name + "_" + proxy.entry.Name

	if gotName != wantName {
		t.Errorf("tool name = %q, want %q", gotName, wantName)
	}
}
