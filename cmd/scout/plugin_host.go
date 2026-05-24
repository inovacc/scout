// Cross-host plugin packager command tree (`scout plugin install|uninstall|status|extract`).
// Mirrors unravel's aihost dispatcher. Host-specific install rituals
// (marketplace patches, settings.json, claude CLI shell-outs) live in
// pkg/scout/aihost/<host>/. This file only routes flags to the
// selected host. The existing subprocess JSON-RPC plugin commands moved
// to `scout subplugin *` (cmd/scout/plugin.go).
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/inovacc/scout/pkg/scout/aihost"
	_ "github.com/inovacc/scout/pkg/scout/aihost/all" // register hosts

	"github.com/spf13/cobra"
)

const defaultHost = "claude"

var hostPluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Install / uninstall / inspect the scout AI-host plugin",
	Long: `Manage the scout plugin for any registered AI host (Claude Code, Codex
CLI, Gemini CLI). Host selection: --host=claude|codex|gemini, or legacy
--claude flag (implies --host=claude). Default: claude.

Subcommands:
  install   [--host H] [--target PATH]   Render assets + patch host's marketplace/settings.
  uninstall [--host H]                    Reverse of install.
  status    [--host H]                    Report current install state.
  extract   [--host H] [--target PATH]    Dump rendered assets without JSON patches.
  doctor    [--host H]                    Run health checks; emit JSON if --json.
  hosts                                   List registered hosts.

For Scout's subprocess JSON-RPC plugin runtime (install third-party
plugin binaries that extend MCP / scraper modes), use ` + "`" + `scout subplugin` + "`" + `.`,
}

func init() {
	rootCmd.AddCommand(hostPluginCmd)
	hostPluginCmd.AddCommand(pluginHostInstallCmd, pluginHostUninstallCmd, pluginHostStatusCmd, pluginHostExtractCmd, pluginHostDoctorCmd, pluginHostHostsCmd)

	for _, c := range []*cobra.Command{pluginHostInstallCmd, pluginHostUninstallCmd, pluginHostStatusCmd, pluginHostExtractCmd, pluginHostDoctorCmd} {
		c.Flags().String("host", "", "AI host: claude (default), codex, gemini")
		c.Flags().Bool("claude", false, "[alias] equivalent to --host=claude")
	}
	pluginHostInstallCmd.Flags().String("target", "", "override install target directory (default: host's canonical path)")
	pluginHostInstallCmd.Flags().Bool("no-restart-hint", false, "suppress 'restart host' message")
	pluginHostExtractCmd.Flags().String("target", "", "target directory to write rendered assets (default: ./<host>-plugin)")
	pluginHostDoctorCmd.Flags().Bool("json", false, "emit DoctorReport as JSON")
}

func selectedHost(cmd *cobra.Command) (aihost.Host, error) {
	hostFlag, _ := cmd.Flags().GetString("host")
	claudeFlag, _ := cmd.Flags().GetBool("claude")
	switch {
	case hostFlag != "":
	case claudeFlag:
		hostFlag = "claude"
	default:
		hostFlag = defaultHost
	}
	h, ok := aihost.ByName(hostFlag)
	if !ok {
		return nil, fmt.Errorf("unknown host %q (run `scout plugin hosts` to list registered hosts)", hostFlag)
	}
	return h, nil
}

var pluginHostInstallCmd = &cobra.Command{
	Use:   "install [--host claude|codex|gemini|all] [--target PATH]",
	Short: "Install the scout plugin into the selected AI host (use --host=all to install into every host with an installer)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		hostFlag, _ := cmd.Flags().GetString("host")
		if hostFlag == "all" {
			return installAllHosts(cmd)
		}
		h, err := selectedHost(cmd)
		if err != nil {
			return err
		}
		target, _ := cmd.Flags().GetString("target")
		if target == "" {
			t, err := h.InstallTarget()
			if err != nil {
				return fmt.Errorf("resolve install target: %w", err)
			}
			target = t
		}
		inst, ok := h.(aihost.Installer)
		if !ok {
			return fmt.Errorf("host %q has no installer (extract assets manually with `scout plugin extract --host %s`)", h.Name(), h.Name())
		}
		n, err := inst.Install(target)
		if err != nil {
			return fmt.Errorf("install: %w", err)
		}
		errOut := cmd.ErrOrStderr()
		_, _ = fmt.Fprintf(errOut, "[install] host=%s target=%s files=%d\n", h.Name(), target, n)
		if statusCarrier, ok := h.(aihost.Status); ok {
			_, _ = fmt.Fprintln(errOut, "")
			_, _ = fmt.Fprintln(errOut, "[install] status:")
			if f, ok := errOut.(*os.File); ok {
				_ = statusCarrier.PrintStatus(f)
			} else {
				_ = statusCarrier.PrintStatus(os.Stderr)
			}
			_, _ = fmt.Fprintln(errOut, "")
		}
		if noHint, _ := cmd.Flags().GetBool("no-restart-hint"); !noHint {
			_, _ = fmt.Fprintf(errOut, "[install] done — restart %s to load the plugin\n", h.Name())
		}
		return nil
	},
}

var pluginHostUninstallCmd = &cobra.Command{
	Use:   "uninstall [--host claude|codex|gemini]",
	Short: "Uninstall the scout plugin from the selected AI host",
	RunE: func(cmd *cobra.Command, _ []string) error {
		h, err := selectedHost(cmd)
		if err != nil {
			return err
		}
		target, err := h.InstallTarget()
		if err != nil {
			return err
		}
		inst, ok := h.(aihost.Installer)
		if !ok {
			return fmt.Errorf("host %q has no installer; remove %s manually", h.Name(), target)
		}
		if err := inst.Uninstall(target); err != nil {
			return fmt.Errorf("uninstall: %w", err)
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "uninstalled host=%s from %s\n", h.Name(), target)
		return nil
	},
}

var pluginHostStatusCmd = &cobra.Command{
	Use:   "status [--host claude|codex|gemini]",
	Short: "Report current install state for the selected host",
	RunE: func(cmd *cobra.Command, _ []string) error {
		h, err := selectedHost(cmd)
		if err != nil {
			return err
		}
		st, ok := h.(aihost.Status)
		if !ok {
			return fmt.Errorf("host %q has no status reporter", h.Name())
		}
		return st.PrintStatus(os.Stdout)
	},
}

var pluginHostExtractCmd = &cobra.Command{
	Use:   "extract [--host claude|codex|gemini] [--target PATH]",
	Short: "Dump rendered plugin assets to a directory (no JSON patches)",
	RunE: func(cmd *cobra.Command, _ []string) error {
		h, err := selectedHost(cmd)
		if err != nil {
			return err
		}
		target, _ := cmd.Flags().GetString("target")
		if target == "" {
			target = filepath.Join(".", h.Name()+"-plugin")
		}
		if err := os.MkdirAll(target, 0o755); err != nil {
			return err
		}
		count := 0
		writeOne := func(rel string, data []byte) error {
			dst := filepath.Join(target, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(dst, data, 0o644); err != nil {
				return err
			}
			count++
			return nil
		}
		if err := h.Walk(writeOne); err != nil {
			return fmt.Errorf("walk assets: %w", err)
		}
		gen, err := h.ManifestFiles()
		if err != nil {
			return fmt.Errorf("manifest files: %w", err)
		}
		for rel, data := range gen {
			if err := writeOne(rel, data); err != nil {
				return err
			}
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "extracted %d files for host=%s into %s\n", count, h.Name(), target)
		return nil
	},
}

var pluginHostDoctorCmd = &cobra.Command{
	Use:   "doctor [--host claude|codex|gemini] [--json]",
	Short: "Run health checks for the selected host's install state",
	RunE: func(cmd *cobra.Command, _ []string) error {
		h, err := selectedHost(cmd)
		if err != nil {
			return err
		}
		d, ok := h.(aihost.Doctor)
		if !ok {
			return fmt.Errorf("host %q has no doctor", h.Name())
		}
		report := d.Doctor()
		if asJSON, _ := cmd.Flags().GetBool("json"); asJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(report)
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "host=%s target=%s verdict=%s\n", report.Host, report.Target, report.Verdict)
		for _, c := range report.Checks {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s — %s\n", c.Verdict, c.Name, c.Detail)
			if c.Fix != "" {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "        fix: %s\n", c.Fix)
			}
		}
		return nil
	},
}

// installAllHosts iterates every registered host and installs into the
// ones that satisfy aihost.Installer. Reports per-host outcome.
// Skipped hosts (no installer yet — codex/gemini stubs) are logged with
// the manual extract command. Fails fast on the first install error.
func installAllHosts(cmd *cobra.Command) error {
	errOut := cmd.ErrOrStderr()
	noHint, _ := cmd.Flags().GetBool("no-restart-hint")
	hosts := aihost.All()
	if len(hosts) == 0 {
		return fmt.Errorf("no hosts registered")
	}
	installed, skipped := 0, 0
	for _, h := range hosts {
		target, err := h.InstallTarget()
		if err != nil {
			return fmt.Errorf("[%s] resolve target: %w", h.Name(), err)
		}
		inst, ok := h.(aihost.Installer)
		if !ok {
			_, _ = fmt.Fprintf(errOut, "[install] skipping host=%s — no Installer (try `scout plugin extract --host %s`)\n", h.Name(), h.Name())
			skipped++
			continue
		}
		n, err := inst.Install(target)
		if err != nil {
			return fmt.Errorf("[%s] install: %w", h.Name(), err)
		}
		_, _ = fmt.Fprintf(errOut, "[install] host=%s target=%s files=%d\n", h.Name(), target, n)
		installed++
	}
	_, _ = fmt.Fprintf(errOut, "\n[install] summary: installed=%d skipped=%d total=%d\n", installed, skipped, len(hosts))
	if !noHint && installed > 0 {
		_, _ = fmt.Fprintln(errOut, "[install] done — restart each host to load the plugin")
	}
	return nil
}

var pluginHostHostsCmd = &cobra.Command{
	Use:   "hosts",
	Short: "List registered AI hosts",
	RunE: func(cmd *cobra.Command, _ []string) error {
		for _, h := range aihost.All() {
			target, _ := h.InstallTarget()
			caps := []string{"walk", "manifest"}
			if _, ok := h.(aihost.Installer); ok {
				caps = append(caps, "install")
			}
			if _, ok := h.(aihost.Status); ok {
				caps = append(caps, "status")
			}
			if _, ok := h.(aihost.Doctor); ok {
				caps = append(caps, "doctor")
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %-10s %v  →  %s\n", h.Name(), caps, target)
		}
		return nil
	},
}
