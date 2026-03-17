package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// CommandEntry declares a CLI command provided by a plugin.
type CommandEntry struct {
	Name            string      `json:"name"`
	Use             string      `json:"use"`
	Short           string      `json:"short"`
	Args            CommandArgs `json:"args,omitzero"`
	Flags           []FlagEntry `json:"flags,omitempty"`
	Category        string      `json:"category,omitempty"`
	Replaces        string      `json:"replaces,omitempty"`
	RequiresBrowser bool        `json:"requires_browser,omitempty"`
}

// CommandArgs specifies min/max positional argument constraints.
type CommandArgs struct {
	Min int `json:"min,omitempty"`
	Max int `json:"max,omitempty"`
}

// FlagEntry declares a flag for a plugin command.
type FlagEntry struct {
	Name        string `json:"name"`
	Short       string `json:"short,omitempty"`
	Type        string `json:"type,omitempty"` // "string", "int", "bool", "float"
	Default     any    `json:"default,omitempty"`
	Description string `json:"description,omitempty"`
}

// CommandResult is the response from a command/execute RPC call.
type CommandResult struct {
	Output      string `json:"output"`
	ExitCode    int    `json:"exit_code"`
	ContentType string `json:"content_type,omitempty"`
}

// BrowserContext provides browser connection details to plugins that require a browser.
type BrowserContext struct {
	CDPEndpoint string `json:"cdp_endpoint"`
	SessionDir  string `json:"session_dir"`
	SessionID   string `json:"session_id"`
}

// CommandProxy synthesizes a cobra.Command that forwards execution to a plugin subprocess.
type CommandProxy struct {
	entry    CommandEntry
	manifest *Manifest
	manager  *Manager
}

// CobraCommand creates a *cobra.Command from the plugin's command entry.
func (c *CommandProxy) CobraCommand() *cobra.Command {
	use := c.entry.Use
	if use == "" {
		use = c.entry.Name
	}

	cmd := &cobra.Command{
		Use:   use,
		Short: c.entry.Short,
		Annotations: map[string]string{
			"plugin": c.manifest.Name,
		},
		RunE:         c.runE,
		SilenceUsage: true,
	}

	if c.entry.Args.Min > 0 || c.entry.Args.Max > 0 {
		cmd.Args = buildArgsValidator(c.entry.Args)
	}

	for _, f := range c.entry.Flags {
		addFlag(cmd, f)
	}

	return cmd
}

func (c *CommandProxy) runE(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	flags := make(map[string]any)

	for _, f := range c.entry.Flags {
		val, err := readFlag(cmd, f)
		if err == nil {
			flags[f.Name] = val
		}
	}

	params := map[string]any{
		"command": c.entry.Name,
		"args":    args,
		"flags":   flags,
	}

	if c.entry.RequiresBrowser {
		bc, err := c.provisionBrowser(cmd)
		if err != nil {
			return fmt.Errorf("plugin: provision browser: %w", err)
		}

		params["browser_context"] = bc
	}

	client, err := c.manager.getClient(c.manifest)
	if err != nil {
		return fmt.Errorf("plugin: start %s: %w", c.manifest.Name, err)
	}

	result, err := client.Call(ctx, "command/execute", params)
	if err != nil {
		return fmt.Errorf("plugin: command %s: %w", c.entry.Name, err)
	}

	var cmdResult CommandResult
	if err := json.Unmarshal(result, &cmdResult); err != nil {
		// Fallback: print raw result when response doesn't match CommandResult schema.
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(result))

		return nil //nolint:nilerr // intentional fallback: raw output is valid
	}

	if cmdResult.Output != "" {
		_, _ = fmt.Fprint(cmd.OutOrStdout(), cmdResult.Output)
	}

	if cmdResult.ExitCode != 0 {
		os.Exit(cmdResult.ExitCode)
	}

	return nil
}

// ErrNoBrowserProvisioner is returned when a plugin command requires a browser
// but no provisioner callback has been registered by the CLI layer.
var ErrNoBrowserProvisioner = errors.New("plugin: no browser provisioner registered")

// provisionBrowser creates a browser and returns its CDP endpoint for the plugin.
func (c *CommandProxy) provisionBrowser(cmd *cobra.Command) (*BrowserContext, error) {
	// Import cycle prevention: we use the baseOpts helper via a callback
	// registered by the CLI layer.
	if browserProvisionFunc == nil {
		return nil, ErrNoBrowserProvisioner
	}

	return browserProvisionFunc(cmd)
}

// BrowserProvisionFunc is a callback that the CLI layer sets to provision a browser
// for plugin commands. This avoids an import cycle between cmd/scout and pkg/scout/plugin.
type BrowserProvisionFunc func(cmd *cobra.Command) (*BrowserContext, error)

var browserProvisionFunc BrowserProvisionFunc //nolint:gochecknoglobals // set once at CLI init

// SetBrowserProvisioner sets the callback used to provision browsers for plugin commands.
func SetBrowserProvisioner(fn BrowserProvisionFunc) {
	browserProvisionFunc = fn
}

// RegisterCLICommands registers all plugin-provided CLI commands with the given root command.
// If a command declares "replaces", the built-in command is removed and the plugin one takes its place.
// Returns the list of command names that were replaced for logging.
func (m *Manager) RegisterCLICommands(root *cobra.Command) []string {
	var replaced []string

	for _, manifest := range m.manifests {
		if !manifest.HasCapability("cli_command") {
			continue
		}

		for _, entry := range manifest.Commands {
			proxy := &CommandProxy{
				entry:    entry,
				manifest: manifest,
				manager:  m,
			}

			pluginCmd := proxy.CobraCommand()

			if entry.Replaces != "" {
				if removeCommand(root, entry.Replaces) {
					replaced = append(replaced, entry.Replaces)
					m.logger.Info("plugin: replaced command",
						"command", entry.Replaces,
						"plugin", manifest.Name,
					)
				}
			}

			root.AddCommand(pluginCmd)
		}
	}

	return replaced
}

// ListCommands returns all plugin-provided command names.
func (m *Manager) ListCommands() []string {
	var names []string

	for _, manifest := range m.manifests {
		for _, entry := range manifest.Commands {
			names = append(names, entry.Name)
		}
	}

	return names
}

// removeCommand removes a subcommand by name from the parent. Returns true if found.
func removeCommand(parent *cobra.Command, name string) bool {
	for _, child := range parent.Commands() {
		if child.Name() == name {
			parent.RemoveCommand(child)
			return true
		}
	}

	return false
}

// buildArgsValidator returns a cobra.PositionalArgs function from CommandArgs constraints.
func buildArgsValidator(a CommandArgs) cobra.PositionalArgs {
	if a.Min > 0 && a.Max > 0 {
		return cobra.RangeArgs(a.Min, a.Max)
	}

	if a.Min > 0 {
		return cobra.MinimumNArgs(a.Min)
	}

	if a.Max > 0 {
		return cobra.MaximumNArgs(a.Max)
	}

	return nil
}

// addFlag adds a flag to the command based on the FlagEntry type.
func addFlag(cmd *cobra.Command, f FlagEntry) {
	switch f.Type {
	case "int":
		def, _ := toInt(f.Default)
		if f.Short != "" {
			cmd.Flags().IntP(f.Name, f.Short, def, f.Description)
		} else {
			cmd.Flags().Int(f.Name, def, f.Description)
		}
	case "bool":
		def, _ := f.Default.(bool)
		if f.Short != "" {
			cmd.Flags().BoolP(f.Name, f.Short, def, f.Description)
		} else {
			cmd.Flags().Bool(f.Name, def, f.Description)
		}
	case "float":
		def, _ := toFloat(f.Default)
		if f.Short != "" {
			cmd.Flags().Float64P(f.Name, f.Short, def, f.Description)
		} else {
			cmd.Flags().Float64(f.Name, def, f.Description)
		}
	default: // "string" or unspecified
		def, _ := f.Default.(string)
		if f.Short != "" {
			cmd.Flags().StringP(f.Name, f.Short, def, f.Description)
		} else {
			cmd.Flags().String(f.Name, def, f.Description)
		}
	}
}

// readFlag reads a flag value from the command based on its type.
func readFlag(cmd *cobra.Command, f FlagEntry) (any, error) {
	switch f.Type {
	case "int":
		return cmd.Flags().GetInt(f.Name)
	case "bool":
		return cmd.Flags().GetBool(f.Name)
	case "float":
		return cmd.Flags().GetFloat64(f.Name)
	default:
		return cmd.Flags().GetString(f.Name)
	}
}

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case float64:
		return int(n), true
	case json.Number:
		i, err := n.Int64()
		return int(i), err == nil
	default:
		return 0, false
	}
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}
