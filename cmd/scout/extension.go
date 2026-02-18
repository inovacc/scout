package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(extensionCmd)
	extensionCmd.AddCommand(extLoadCmd, extTestCmd, extListCmd, extDownloadCmd, extRemoveCmd)

	extLoadCmd.Flags().StringSlice("path", nil, "path(s) to unpacked extension directory (required, repeatable)")
	extLoadCmd.Flags().String("url", "", "URL to navigate to after loading")

	extTestCmd.Flags().StringSlice("path", nil, "path(s) to unpacked extension directory (required, repeatable)")
	extTestCmd.Flags().String("url", "chrome://extensions", "URL to navigate to")
	extTestCmd.Flags().String("screenshot", "", "capture screenshot to file")
	extTestCmd.Flags().Duration("timeout", 30*time.Second, "timeout before exit")

	extListCmd.Flags().String("url", "chrome://extensions", "URL to navigate to")
}

var extensionCmd = &cobra.Command{
	Use:   "extension",
	Short: "Manage, load, and test Chrome extensions",
	Long:  `Commands for downloading, loading, and testing Chrome extensions in Scout-controlled browsers.`,
}

var extDownloadCmd = &cobra.Command{
	Use:   "download <extension-id>",
	Short: "Download an extension from the Chrome Web Store",
	Long:  `Download a Chrome extension by its Web Store ID, unpack the CRX3 file, and store it locally in ~/.scout/extensions/.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Downloading extension %s...\n", id)

		info, err := scout.DownloadExtension(id)
		if err != nil {
			return err
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Downloaded: %s v%s\n  ID:   %s\n  Path: %s\n", info.Name, info.Version, info.ID, info.Path)
		return nil
	},
}

var extRemoveCmd = &cobra.Command{
	Use:   "remove <extension-id>",
	Short: "Remove a locally downloaded extension",
	Long:  `Delete a previously downloaded extension from ~/.scout/extensions/.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		if err := scout.RemoveExtension(id); err != nil {
			return err
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Removed extension %s\n", id)
		return nil
	},
}

var extLoadCmd = &cobra.Command{
	Use:   "load",
	Short: "Load an unpacked extension and open the browser",
	Long: `Load one or more unpacked Chrome extensions into a non-headless browser.
Navigates to the given URL (or chrome://extensions) and blocks until Ctrl+C.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		paths, _ := cmd.Flags().GetStringSlice("path")
		if len(paths) == 0 {
			return fmt.Errorf("scout: --path is required")
		}

		for _, p := range paths {
			if _, err := os.Stat(p); err != nil {
				return fmt.Errorf("scout: extension path %q: %w", p, err)
			}
		}

		urlFlag, _ := cmd.Flags().GetString("url")
		if urlFlag == "" {
			urlFlag = "chrome://extensions"
		}

		browser, err := scout.New(
			scout.WithHeadless(false),
			scout.WithNoSandbox(),
			scout.WithExtension(paths...),
			browserOpt(cmd),
		)
		if err != nil {
			return fmt.Errorf("scout: launch browser: %w", err)
		}
		defer func() { _ = browser.Close() }()

		page, err := browser.NewPage(urlFlag)
		if err != nil {
			return fmt.Errorf("scout: navigate: %w", err)
		}
		if err := page.WaitLoad(); err != nil {
			return fmt.Errorf("scout: wait load: %w", err)
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Extension loaded. Browser open at %s\nPress Ctrl+C to exit.\n", urlFlag)

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		<-ctx.Done()

		return nil
	},
}

var extTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Load an extension in headless mode and capture results",
	Long: `Load one or more unpacked Chrome extensions into a headless browser for testing.
Optionally capture a screenshot and list detected extensions.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		paths, _ := cmd.Flags().GetStringSlice("path")
		if len(paths) == 0 {
			return fmt.Errorf("scout: --path is required")
		}

		for _, p := range paths {
			if _, err := os.Stat(p); err != nil {
				return fmt.Errorf("scout: extension path %q: %w", p, err)
			}
		}

		urlFlag, _ := cmd.Flags().GetString("url")
		screenshotFile, _ := cmd.Flags().GetString("screenshot")
		timeout, _ := cmd.Flags().GetDuration("timeout")

		browser, err := scout.New(
			scout.WithHeadless(isHeadless(cmd)),
			scout.WithNoSandbox(),
			scout.WithTimeout(timeout),
			scout.WithExtension(paths...),
			browserOpt(cmd),
		)
		if err != nil {
			return fmt.Errorf("scout: launch browser: %w", err)
		}
		defer func() { _ = browser.Close() }()

		page, err := browser.NewPage(urlFlag)
		if err != nil {
			return fmt.Errorf("scout: navigate: %w", err)
		}
		if err := page.WaitLoad(); err != nil {
			return fmt.Errorf("scout: wait load: %w", err)
		}

		if screenshotFile != "" {
			data, err := page.Screenshot()
			if err != nil {
				return fmt.Errorf("scout: screenshot: %w", err)
			}
			dest, err := writeOutput(cmd, data, screenshotFile)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Screenshot saved to %s\n", dest)
		}

		// Try to list extensions via chrome.management API (requires management permission
		// or chrome://extensions page context).
		result, err := page.Eval(`() => {
			if (typeof chrome !== 'undefined' && chrome.management && chrome.management.getAll) {
				return new Promise((resolve) => {
					chrome.management.getAll((exts) => {
						resolve(exts.map(e => ({ name: e.name, id: e.id, version: e.version, enabled: e.enabled })));
					});
				});
			}
			return [];
		}`)
		if err != nil {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Could not query chrome.management API: %v\n", err)
			return nil
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Loaded extensions:")
		val := result.Value
		if arr, ok := val.([]interface{}); ok {
			for _, item := range arr {
				if m, ok := item.(map[string]interface{}); ok {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  - %s (id: %s, version: %s, enabled: %v)\n",
						m["name"], m["id"], m["version"], m["enabled"])
				}
			}
		}

		return nil
	},
}

var extListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed and downloaded extensions",
	Long:  `List locally downloaded extensions from ~/.scout/extensions/ and optionally query the browser for loaded extensions.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		// Show locally downloaded extensions first.
		localExts, err := scout.ListLocalExtensions()
		if err != nil {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Warning: could not list local extensions: %v\n", err)
		} else {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Downloaded extensions (~/.scout/extensions/):")
			if len(localExts) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  (none)")
			}
			for _, ext := range localExts {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  - %s v%s (id: %s)\n", ext.Name, ext.Version, ext.ID)
			}
		}

		urlFlag, _ := cmd.Flags().GetString("url")

		browser, err := scout.New(
			scout.WithHeadless(isHeadless(cmd)),
			scout.WithNoSandbox(),
			browserOpt(cmd),
		)
		if err != nil {
			return fmt.Errorf("scout: launch browser: %w", err)
		}
		defer func() { _ = browser.Close() }()

		page, err := browser.NewPage(urlFlag)
		if err != nil {
			return fmt.Errorf("scout: navigate: %w", err)
		}
		if err := page.WaitLoad(); err != nil {
			return fmt.Errorf("scout: wait load: %w", err)
		}

		// Try chrome.management API first.
		result, err := page.Eval(`() => {
			if (typeof chrome !== 'undefined' && chrome.management && chrome.management.getAll) {
				return new Promise((resolve) => {
					chrome.management.getAll((exts) => {
						resolve(exts.map(e => ({ name: e.name, id: e.id, version: e.version, enabled: e.enabled })));
					});
				});
			}
			return [];
		}`)
		if err != nil {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Could not query chrome.management API: %v\n", err)
			return nil
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "\nBrowser extensions:")
		val := result.Value
		if arr, ok := val.([]interface{}); ok {
			if len(arr) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "  (none)")
			}
			for _, item := range arr {
				if m, ok := item.(map[string]interface{}); ok {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  - %s (id: %s, version: %s, enabled: %v)\n",
						m["name"], m["id"], m["version"], m["enabled"])
				}
			}
		}

		return nil
	},
}
