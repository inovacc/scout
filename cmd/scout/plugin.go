package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/inovacc/scout/pkg/scout/archive"
	"github.com/inovacc/scout/pkg/scout/plugin"
	"github.com/inovacc/scout/pkg/scout/plugin/registry"
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
	pluginCmd.AddCommand(pluginListCmd, pluginInstallCmd, pluginRemoveCmd, pluginRunCmd, pluginSearchCmd, pluginUpdateCmd)
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

			printPluginCommands(cmd, p)
		}

		return nil
	},
}

func printPluginCommands(cmd *cobra.Command, p *plugin.Manifest) {
	if len(p.Commands) == 0 {
		return
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "    commands: ")

	for i, c := range p.Commands {
		if i > 0 {
			_, _ = fmt.Fprint(cmd.OutOrStdout(), ", ")
		}

		_, _ = fmt.Fprint(cmd.OutOrStdout(), c.Name)
	}

	_, _ = fmt.Fprintln(cmd.OutOrStdout())
}

var pluginInstallCmd = &cobra.Command{
	Use:   "install <path|url|github:owner/plugin>",
	Short: "Install a plugin from a directory, URL, or GitHub release",
	Long: `Install a plugin from a local directory, archive URL, or GitHub release.

Examples:
  scout plugin install ./plugins/scout-diag
  scout plugin install https://example.com/scout-diag-linux-amd64.tar.gz
  scout plugin install github:inovacc/scout-diag`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		src := args[0]

		// GitHub shorthand: github:owner/plugin → latest release URL.
		if strings.HasPrefix(src, "github:") {
			return installPluginFromGitHub(cmd, strings.TrimPrefix(src, "github:"))
		}

		// Detect URL vs local path.
		if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
			return installPluginFromURL(cmd, src)
		}

		return installPluginFromDir(cmd, src)
	},
}

func installPluginFromDir(cmd *cobra.Command, srcDir string) error {
	manifest, err := plugin.LoadManifest(srcDir)
	if err != nil {
		return fmt.Errorf("scout: plugin install: %w", err)
	}

	destDir, err := pluginDestDir(manifest.Name)
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("scout: plugin install: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		s := filepath.Join(srcDir, entry.Name())
		d := filepath.Join(destDir, entry.Name())

		if err := copyFile(s, d); err != nil {
			return fmt.Errorf("scout: plugin install: copy %s: %w", entry.Name(), err)
		}
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "installed plugin %s v%s to %s\n", manifest.Name, manifest.Version, destDir)

	return nil
}

func installPluginFromURL(cmd *cobra.Command, url string) error {
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "downloading %s...\n", url)

	resp, err := http.Get(url) //nolint:gosec,noctx // user-provided URL
	if err != nil {
		return fmt.Errorf("scout: plugin install: download: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("scout: plugin install: download: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("scout: plugin install: read body: %w", err)
	}

	// Extract archive to temp dir.
	tmpDir, err := os.MkdirTemp("", "scout-plugin-*")
	if err != nil {
		return fmt.Errorf("scout: plugin install: %w", err)
	}

	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Determine filename from URL path.
	filename := filepath.Base(url)
	if err := archive.Extract(data, filename, tmpDir); err != nil {
		return fmt.Errorf("scout: plugin install: extract: %w", err)
	}

	// Find plugin.json in extracted contents (may be nested one level).
	manifestDir, err := findManifestDir(tmpDir)
	if err != nil {
		return err
	}

	return installPluginFromDir(cmd, manifestDir)
}

func findManifestDir(root string) (string, error) {
	// Check root first.
	if _, err := os.Stat(filepath.Join(root, "plugin.json")); err == nil {
		return root, nil
	}

	// Check one level deep.
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", fmt.Errorf("scout: plugin install: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			dir := filepath.Join(root, entry.Name())
			if _, err := os.Stat(filepath.Join(dir, "plugin.json")); err == nil {
				return dir, nil
			}
		}
	}

	return "", fmt.Errorf("scout: plugin install: no plugin.json found in archive")
}

func pluginDestDir(name string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("scout: plugin install: %w", err)
	}

	destDir := filepath.Join(home, ".scout", "plugins", name)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("scout: plugin install: %w", err)
	}

	return destDir, nil
}

// installPluginFromGitHub resolves a GitHub release URL and installs the plugin.
// Format: owner/plugin (e.g., inovacc/scout-diag)
func installPluginFromGitHub(cmd *cobra.Command, repo string) error {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("scout: plugin install: invalid github reference %q (expected owner/plugin)", repo)
	}

	owner, name := parts[0], parts[1]

	// Detect platform.
	goos := "linux"
	goarch := "amd64"

	switch {
	case strings.Contains(strings.ToLower(os.Getenv("OS")), "windows"):
		goos = "windows"
	case fileExists("/Applications"):
		goos = "darwin"
	}

	// Construct release asset URL (latest release).
	ext := "tar.gz"
	if goos == "windows" {
		ext = "zip"
	}

	url := fmt.Sprintf("https://github.com/%s/%s/releases/latest/download/%s-%s-%s.%s",
		owner, name, name, goos, goarch, ext)

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "installing %s/%s (%s/%s)...\n", owner, name, goos, goarch)

	return installPluginFromURL(cmd, url)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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

var pluginSearchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search the plugin registry",
	RunE: func(cmd *cobra.Command, args []string) error {
		query := ""
		if len(args) > 0 {
			query = strings.Join(args, " ")
		}

		idx, err := registry.FetchIndex("")
		if err != nil {
			return fmt.Errorf("scout: plugin search: %w", err)
		}

		results := idx.Search(query)
		if len(results) == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "no plugins found")
			return nil
		}

		for _, p := range results {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %-25s %s\n", p.Name, p.Description)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "    repo: %s  latest: %s\n", p.Repo, p.Latest)
		}

		return nil
	},
}

var pluginUpdateCmd = &cobra.Command{
	Use:   "update [name]",
	Short: "Update installed plugins to latest version",
	Long:  "Update a specific plugin or all installed plugins (--all) to their latest release.",
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr := initPluginManager()
		plugins := mgr.Plugins()

		if len(plugins) == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "no plugins installed")
			return nil
		}

		// Load lock file for version tracking.
		lf, err := registry.LoadLockFile()
		if err != nil {
			return fmt.Errorf("scout: plugin update: %w", err)
		}

		// Fetch registry for latest versions.
		idx, err := registry.FetchIndex("")
		if err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not fetch registry: %v\n", err)
			return nil
		}

		target := ""
		if len(args) > 0 {
			target = args[0]
		}

		updated := 0

		for _, p := range plugins {
			if target != "" && p.Name != target {
				continue
			}

			// Find in registry.
			var info *registry.PluginInfo

			for _, ri := range idx.Plugins {
				if ri.Name == p.Name {
					info = &ri

					break
				}
			}

			if info == nil {
				continue
			}

			locked := lf.Get(p.Name)
			if locked != nil && locked.Version == info.Latest {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s: up to date (%s)\n", p.Name, info.Latest)
				continue
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s: updating to %s...\n", p.Name, info.Latest)

			url := registry.LatestReleaseURL(info.Repo, p.Name)
			if err := installPluginFromURL(cmd, url); err != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "  %s: update failed: %v\n", p.Name, err)
				continue
			}

			lf.Lock(p.Name, info.Latest, "", info.Repo)
			updated++
		}

		if err := lf.Save(); err != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: save lock file: %v\n", err)
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\n%d plugin(s) updated\n", updated)

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
