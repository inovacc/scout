package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure Scout as a plugin for your AI coding assistant",
	Long: `Interactive setup wizard for Scout. Configures hooks and MCP server
for your chosen AI coding assistant.

If --client is provided, skips the interactive menu:
  scout setup --client claude
  scout setup --client gemini
  scout setup --client cursor

Supported clients: claude, gemini, vscode, cursor, opencode, codex.`,
	Args: cobra.NoArgs,
	RunE: runSetup,
}

var (
	setupClient string
	setupDryRun bool
)

func init() {
	rootCmd.AddCommand(setupCmd)
	setupCmd.Flags().StringVarP(&setupClient, "client", "c", "", "target client (claude, gemini, vscode, cursor, opencode, codex)")
	setupCmd.Flags().BoolVarP(&setupDryRun, "dry-run", "n", false, "Print config without writing")
}

type platformEntry struct {
	label      string
	id         string
	settingsFn func() string
}

var platforms = []platformEntry{
	{"Claude Code", "claude-code", claudeSettingsPath},
	{"Gemini CLI", "gemini-cli", geminiSettingsPath},
	{"VS Code Copilot", "vscode-copilot", vscodeSettingsPath},
	{"Cursor", "cursor", cursorSettingsPath},
	{"OpenCode", "opencode", nil},
	{"Codex", "codex", nil},
}

var clientAliases = map[string]int{
	"claude":         0,
	"claude-code":    0,
	"gemini":         1,
	"gemini-cli":     1,
	"vscode":         2,
	"copilot":        2,
	"vscode-copilot": 2,
	"cursor":         3,
	"opencode":       4,
	"codex":          5,
}

func promptMenu() (int, error) {
	_, _ = fmt.Fprintln(os.Stderr, "")
	_, _ = fmt.Fprintln(os.Stderr, "Select your AI coding assistant:")
	_, _ = fmt.Fprintln(os.Stderr, "")

	for i, entry := range platforms {
		_, _ = fmt.Fprintf(os.Stderr, "  %d) %s\n", i+1, entry.label)
	}

	_, _ = fmt.Fprintln(os.Stderr, "")
	_, _ = fmt.Fprintf(os.Stderr, "Enter number (1-%d): ", len(platforms))

	reader := bufio.NewReader(os.Stdin)

	line, err := reader.ReadString('\n')
	if err != nil {
		return 0, fmt.Errorf("failed to read input: %w", err)
	}

	choice, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || choice < 1 || choice > len(platforms) {
		return 0, fmt.Errorf("invalid choice: %q (enter 1-%d)", strings.TrimSpace(line), len(platforms))
	}

	_, _ = fmt.Fprintf(os.Stderr, "Selected: %s\n", platforms[choice-1].label)

	return choice - 1, nil
}

func runSetup(_ *cobra.Command, _ []string) error {
	var idx int

	if setupClient != "" {
		i, ok := clientAliases[setupClient]
		if !ok {
			return fmt.Errorf("unknown client %q (supported: claude, gemini, vscode, cursor, opencode, codex)", setupClient)
		}

		idx = i
	} else {
		selected, err := promptMenu()
		if err != nil {
			return err
		}

		idx = selected
	}

	p := platforms[idx]

	if p.settingsFn == nil {
		_, _ = fmt.Fprintf(os.Stderr, "%s is MCP-only. Configure your MCP client to run: scout mcp --headless --stealth\n", p.label)
		return nil
	}

	// Build the config with hooks and MCP server.
	execPath, err := os.Executable()
	if err != nil {
		execPath = "scout"
	} else {
		execPath, _ = filepath.EvalSymlinks(execPath)
	}

	config := buildScoutConfig(p.id, execPath)

	configJSON, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if setupDryRun {
		_, _ = fmt.Fprintf(os.Stderr, "Config for %s:\n", p.label)
		_, _ = fmt.Fprintln(os.Stdout, string(configJSON))

		return nil
	}

	settingsPath := p.settingsFn()

	if err := writeSettings(settingsPath, config); err != nil {
		return err
	}

	_, _ = fmt.Fprintf(os.Stderr, "✓ Scout configured for %s\n", p.label)
	_, _ = fmt.Fprintf(os.Stderr, "  Settings: %s\n", settingsPath)

	return nil
}

func buildScoutConfig(platformID, binaryPath string) map[string]any {
	config := map[string]any{
		"mcpServers": map[string]any{
			"scout": map[string]any{
				"command": binaryPath,
				"args":    []string{"mcp", "--headless", "--stealth"},
			},
		},
	}

	// Add hooks for platforms that support them.
	switch platformID {
	case "claude-code":
		config["hooks"] = map[string]any{
			"SessionStart": []any{
				map[string]any{
					"matcher": "",
					"hooks": []any{
						map[string]any{"type": "command", "command": binaryPath + " hook sessionstart"},
					},
				},
			},
		}
	}

	return config
}

func writeSettings(settingsPath string, config map[string]any) error {
	var existing map[string]any

	data, err := os.ReadFile(settingsPath)
	if err == nil {
		_ = json.Unmarshal(data, &existing)
	}

	if existing == nil {
		existing = make(map[string]any)
	}

	if newMCP, ok := config["mcpServers"].(map[string]any); ok {
		existingMCP, _ := existing["mcpServers"].(map[string]any)
		if existingMCP == nil {
			existingMCP = make(map[string]any)
		}

		maps.Copy(existingMCP, newMCP)

		existing["mcpServers"] = existingMCP
	}

	if newHooks, ok := config["hooks"].(map[string]any); ok {
		existingHooks, _ := existing["hooks"].(map[string]any)
		if existingHooks == nil {
			existingHooks = make(map[string]any)
		}

		maps.Copy(existingHooks, newHooks)

		existing["hooks"] = existingHooks
	}

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return fmt.Errorf("create settings dir: %w", err)
	}

	output, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}

	if err := os.WriteFile(settingsPath, append(output, '\n'), 0o644); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}

	return nil
}

// Settings paths per platform.

func claudeSettingsPath() string {
	home, _ := os.UserHomeDir()

	return filepath.Join(home, ".claude", "settings.json")
}

func geminiSettingsPath() string {
	home, _ := os.UserHomeDir()

	return filepath.Join(home, ".gemini", "settings.json")
}

func vscodeSettingsPath() string {
	home, _ := os.UserHomeDir()

	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Code", "User", "settings.json")
	case "windows":
		return filepath.Join(os.Getenv("APPDATA"), "Code", "User", "settings.json")
	default:
		return filepath.Join(home, ".config", "Code", "User", "settings.json")
	}
}

func cursorSettingsPath() string {
	home, _ := os.UserHomeDir()

	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "Cursor", "User", "settings.json")
	case "windows":
		return filepath.Join(os.Getenv("APPDATA"), "Cursor", "User", "settings.json")
	default:
		return filepath.Join(home, ".config", "Cursor", "User", "settings.json")
	}
}
