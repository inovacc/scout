package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/inovacc/scout/pkg/scout/plugin"
	"github.com/spf13/cobra"
)

var pluginManager *plugin.Manager

// initPluginManager lazily initializes the global plugin manager.
func initPluginManager() *plugin.Manager {
	if pluginManager != nil {
		return pluginManager
	}

	pluginManager = plugin.NewManager(plugin.DefaultDirs(), nil)
	_ = pluginManager.Discover()

	return pluginManager
}

func init() {
	rootCmd.AddCommand(pluginCmd)
	pluginCmd.AddCommand(pluginListCmd, pluginInstallCmd, pluginRemoveCmd, pluginRunCmd)
}

var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Manage Scout plugins",
}

var pluginListCmd = &cobra.Command{
	Use:   "list",
	Short: "List discovered plugins",
	RunE: func(cmd *cobra.Command, _ []string) error {
		mgr := initPluginManager()
		plugins := mgr.Plugins()

		if len(plugins) == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "no plugins found")
			return nil
		}

		for _, p := range plugins {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %-20s v%s  %s\n", p.Name, p.Version, p.Description)

			if len(p.Modes) > 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "    modes: ")
				for i, m := range p.Modes {
					if i > 0 {
						_, _ = fmt.Fprint(cmd.OutOrStdout(), ", ")
					}

					_, _ = fmt.Fprint(cmd.OutOrStdout(), m.Name)
				}

				_, _ = fmt.Fprintln(cmd.OutOrStdout())
			}

			if len(p.Tools) > 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "    tools: ")
				for i, t := range p.Tools {
					if i > 0 {
						_, _ = fmt.Fprint(cmd.OutOrStdout(), ", ")
					}

					_, _ = fmt.Fprint(cmd.OutOrStdout(), t.Name)
				}

				_, _ = fmt.Fprintln(cmd.OutOrStdout())
			}

			if len(p.Extractors) > 0 {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "    extractors: ")
				for i, e := range p.Extractors {
					if i > 0 {
						_, _ = fmt.Fprint(cmd.OutOrStdout(), ", ")
					}

					_, _ = fmt.Fprint(cmd.OutOrStdout(), e.Name)
				}

				_, _ = fmt.Fprintln(cmd.OutOrStdout())
			}
		}

		return nil
	},
}

var pluginInstallCmd = &cobra.Command{
	Use:   "install <path>",
	Short: "Install a plugin from a directory",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		srcDir := args[0]

		// Validate source has a manifest.
		manifest, err := plugin.LoadManifest(srcDir)
		if err != nil {
			return fmt.Errorf("scout: plugin install: %w", err)
		}

		// Destination: ~/.scout/plugins/<name>/
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("scout: plugin install: %w", err)
		}

		destDir := filepath.Join(home, ".scout", "plugins", manifest.Name)
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			return fmt.Errorf("scout: plugin install: %w", err)
		}

		// Copy all files from source to destination.
		entries, err := os.ReadDir(srcDir)
		if err != nil {
			return fmt.Errorf("scout: plugin install: %w", err)
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			src := filepath.Join(srcDir, entry.Name())
			dst := filepath.Join(destDir, entry.Name())

			if err := copyFile(src, dst); err != nil {
				return fmt.Errorf("scout: plugin install: copy %s: %w", entry.Name(), err)
			}
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "installed plugin %s v%s to %s\n", manifest.Name, manifest.Version, destDir)

		return nil
	},
}

var pluginRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove an installed plugin",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("scout: plugin remove: %w", err)
		}

		dir := filepath.Join(home, ".scout", "plugins", name)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return fmt.Errorf("scout: plugin remove: plugin %q not found", name)
		}

		if err := os.RemoveAll(dir); err != nil {
			return fmt.Errorf("scout: plugin remove: %w", err)
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "removed plugin %s\n", name)

		return nil
	},
}

var pluginRunCmd = &cobra.Command{
	Use:   "run <name>",
	Short: "Test-run a plugin (initialize + shutdown)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		mgr := initPluginManager()

		var manifest *plugin.Manifest
		for _, p := range mgr.Plugins() {
			if p.Name == name {
				manifest = p
				break
			}
		}

		if manifest == nil {
			return fmt.Errorf("scout: plugin run: plugin %q not found", name)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		client := plugin.NewClient(manifest, nil)
		if err := client.Start(ctx); err != nil {
			return fmt.Errorf("scout: plugin run: %w", err)
		}

		if err := client.Initialize(ctx); err != nil {
			return fmt.Errorf("scout: plugin run: initialize: %w", err)
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "plugin %s v%s initialized successfully\n", manifest.Name, manifest.Version)

		if err := client.Shutdown(ctx); err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: shutdown: %v\n", err)
		}

		return nil
	},
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}

	defer func() { _ = in.Close() }()

	info, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}

	defer func() { _ = out.Close() }()

	_, err = io.Copy(out, in)

	return err
}
