package plugin

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestCommandProxy_CobraCommand(t *testing.T) {
	entry := CommandEntry{
		Name:  "test-cmd",
		Use:   "test-cmd <url>",
		Short: "A test command",
		Args:  CommandArgs{Min: 1, Max: 1},
		Flags: []FlagEntry{
			{Name: "format", Type: "string", Default: "json", Short: "f", Description: "Output format"},
			{Name: "depth", Type: "int", Default: float64(3), Description: "Crawl depth"},
			{Name: "verbose", Type: "bool", Default: false, Description: "Verbose output"},
			{Name: "threshold", Type: "float", Default: 0.5, Description: "Threshold value"},
		},
		Category:        "content",
		RequiresBrowser: false,
	}

	manifest := &Manifest{
		Name:         "test-plugin",
		Version:      "1.0.0",
		Command:      "test-plugin",
		Capabilities: []string{"cli_command"},
		Commands:     []CommandEntry{entry},
	}

	proxy := &CommandProxy{
		entry:    entry,
		manifest: manifest,
	}

	cmd := proxy.CobraCommand()

	if cmd.Use != "test-cmd <url>" {
		t.Errorf("Use = %q, want %q", cmd.Use, "test-cmd <url>")
	}

	if cmd.Short != "A test command" {
		t.Errorf("Short = %q, want %q", cmd.Short, "A test command")
	}

	if cmd.Annotations["plugin"] != "test-plugin" {
		t.Errorf("plugin annotation = %q, want %q", cmd.Annotations["plugin"], "test-plugin")
	}

	// Check flags were registered.
	f := cmd.Flags()
	if v, _ := f.GetString("format"); v != "json" {
		t.Errorf("format default = %q, want %q", v, "json")
	}

	if v, _ := f.GetInt("depth"); v != 3 {
		t.Errorf("depth default = %d, want %d", v, 3)
	}

	if v, _ := f.GetBool("verbose"); v != false {
		t.Errorf("verbose default = %v, want %v", v, false)
	}

	if v, _ := f.GetFloat64("threshold"); v != 0.5 {
		t.Errorf("threshold default = %f, want %f", v, 0.5)
	}

	// Check shorthand.
	if sh := f.ShorthandLookup("f"); sh == nil {
		t.Error("expected shorthand 'f' for format flag")
	}
}

func TestCommandProxy_CobraCommand_DefaultUse(t *testing.T) {
	entry := CommandEntry{
		Name:  "simple",
		Short: "Simple command",
	}

	proxy := &CommandProxy{
		entry:    entry,
		manifest: &Manifest{Name: "test"},
	}

	cmd := proxy.CobraCommand()
	if cmd.Use != "simple" {
		t.Errorf("Use = %q, want %q", cmd.Use, "simple")
	}
}

func TestBuildArgsValidator(t *testing.T) {
	tests := []struct {
		name string
		args CommandArgs
		in   []string
		ok   bool
	}{
		{"range 1-1 with 1", CommandArgs{Min: 1, Max: 1}, []string{"a"}, true},
		{"range 1-1 with 0", CommandArgs{Min: 1, Max: 1}, []string{}, false},
		{"range 1-1 with 2", CommandArgs{Min: 1, Max: 1}, []string{"a", "b"}, false},
		{"min 1 with 2", CommandArgs{Min: 1}, []string{"a", "b"}, true},
		{"min 1 with 0", CommandArgs{Min: 1}, []string{}, false},
		{"max 2 with 2", CommandArgs{Max: 2}, []string{"a", "b"}, true},
		{"max 2 with 3", CommandArgs{Max: 2}, []string{"a", "b", "c"}, false},
		{"no constraint", CommandArgs{}, []string{"a", "b", "c"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := buildArgsValidator(tt.args)
			if validator == nil {
				if !tt.ok {
					t.Error("expected validator to reject args, but no validator was created")
				}

				return
			}

			cmd := &cobra.Command{}

			err := validator(cmd, tt.in)
			if tt.ok && err != nil {
				t.Errorf("expected no error, got %v", err)
			}

			if !tt.ok && err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestRemoveCommand(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	child := &cobra.Command{Use: "child"}
	root.AddCommand(child)

	if !removeCommand(root, "child") {
		t.Error("expected removeCommand to return true")
	}

	if removeCommand(root, "nonexistent") {
		t.Error("expected removeCommand to return false for nonexistent command")
	}

	// Verify child was actually removed.
	for _, c := range root.Commands() {
		if c.Name() == "child" {
			t.Error("child command should have been removed")
		}
	}
}

func TestRegisterCLICommands(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	builtin := &cobra.Command{Use: "ping", Short: "built-in ping"}
	root.AddCommand(builtin)

	mgr := NewManager(nil, nil)
	mgr.manifests["test"] = &Manifest{
		Name:         "test",
		Version:      "1.0.0",
		Command:      "test",
		Capabilities: []string{"cli_command"},
		Commands: []CommandEntry{
			{Name: "my-cmd", Short: "custom command"},
			{Name: "ping", Short: "plugin ping", Replaces: "ping"},
		},
	}

	replaced := mgr.RegisterCLICommands(root)

	// Should have replaced "ping".
	if len(replaced) != 1 || replaced[0] != "ping" {
		t.Errorf("replaced = %v, want [ping]", replaced)
	}

	// Find the ping command — should be the plugin one.
	found := false

	for _, c := range root.Commands() {
		if c.Name() == "ping" {
			if c.Short != "plugin ping" {
				t.Errorf("ping.Short = %q, want %q", c.Short, "plugin ping")
			}

			if c.Annotations["plugin"] != "test" {
				t.Errorf("ping plugin annotation = %q, want %q", c.Annotations["plugin"], "test")
			}

			found = true
		}
	}

	if !found {
		t.Error("expected plugin ping command to be registered")
	}

	// Find my-cmd.
	foundCustom := false

	for _, c := range root.Commands() {
		if c.Name() == "my-cmd" {
			foundCustom = true
		}
	}

	if !foundCustom {
		t.Error("expected my-cmd command to be registered")
	}
}

func TestListCommands(t *testing.T) {
	mgr := NewManager(nil, nil)
	mgr.manifests["a"] = &Manifest{
		Name:     "a",
		Commands: []CommandEntry{{Name: "cmd-a"}, {Name: "cmd-b"}},
	}
	mgr.manifests["b"] = &Manifest{
		Name:     "b",
		Commands: []CommandEntry{{Name: "cmd-c"}},
	}

	names := mgr.ListCommands()
	if len(names) != 3 {
		t.Errorf("ListCommands() returned %d names, want 3", len(names))
	}
}

func TestToInt(t *testing.T) {
	tests := []struct {
		name string
		val  any
		want int
		ok   bool
	}{
		{"int", 42, 42, true},
		{"float64", float64(7), 7, true},
		{"string", "nope", 0, false},
		{"nil", nil, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := toInt(tt.val)
			if ok != tt.ok || got != tt.want {
				t.Errorf("toInt(%v) = (%d, %v), want (%d, %v)", tt.val, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestToFloat(t *testing.T) {
	tests := []struct {
		name string
		val  any
		want float64
		ok   bool
	}{
		{"float64", 3.14, 3.14, true},
		{"int", 5, 5.0, true},
		{"string", "nope", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := toFloat(tt.val)
			if ok != tt.ok || got != tt.want {
				t.Errorf("toFloat(%v) = (%f, %v), want (%f, %v)", tt.val, got, ok, tt.want, tt.ok)
			}
		})
	}
}
