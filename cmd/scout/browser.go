package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(browserCmd)
	browserCmd.AddCommand(browserListCmd)
	browserCmd.AddCommand(browserDownloadCmd)

	browserListCmd.Flags().Bool("detect", false, "Run full browser detection with version probing")
}

var browserCmd = &cobra.Command{
	Use:   "browser",
	Short: "Manage browser installations",
}

var browserListCmd = &cobra.Command{
	Use:   "list",
	Short: "List detected and downloaded browsers",
	RunE: func(cmd *cobra.Command, args []string) error {
		detect, _ := cmd.Flags().GetBool("detect")

		if detect {
			fmt.Println("Detected browsers (full scan):") //nolint:forbidigo

			detected := scout.DetectBrowsers()
			if len(detected) == 0 {
				fmt.Println("  (none found)") //nolint:forbidigo
			} else {
				for _, d := range detected {
					ver := d.Version
					if ver == "" {
						ver = "unknown"
					}

					fmt.Printf("  %-20s  %-8s  %s  (%s)\n", d.Name, d.Type, d.Path, ver) //nolint:forbidigo
				}
			}

			fmt.Println() //nolint:forbidigo
		} else {
			fmt.Println("Detected browsers (local install):") //nolint:forbidigo

			for _, bt := range []scout.BrowserType{scout.BrowserChrome, scout.BrowserBrave, scout.BrowserEdge} {
				path, err := scout.LookupBrowserPublic(bt)
				if err != nil {
					fmt.Printf("  %-8s  not found\n", bt) //nolint:forbidigo
				} else if path == "" {
					fmt.Printf("  %-8s  auto-detect (rod)\n", bt) //nolint:forbidigo
				} else {
					fmt.Printf("  %-8s  %s\n", bt, path) //nolint:forbidigo
				}
			}

			fmt.Println() //nolint:forbidigo
		}

		fmt.Println("Downloaded browsers (~/.scout/browsers/):") //nolint:forbidigo

		browsers, err := scout.ListDownloadedBrowsers()
		if err != nil {
			return err
		}

		if len(browsers) == 0 {
			fmt.Println("  (none)") //nolint:forbidigo
		} else {
			for _, b := range browsers {
				fmt.Printf("  %s\n", b) //nolint:forbidigo
			}
		}

		return nil
	},
}

var browserDownloadCmd = &cobra.Command{
	Use:   "download <browser>",
	Short: "Download a browser for local/container use",
	Long:  "Download and cache a browser binary. Supported: brave.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := strings.ToLower(args[0])
		switch name {
		case "brave":
			fmt.Println("Downloading Brave browser...") //nolint:forbidigo

			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Minute)
			defer cancel()

			path, err := scout.DownloadBrave(ctx)
			if err != nil {
				return fmt.Errorf("download brave: %w", err)
			}

			fmt.Printf("Brave downloaded to: %s\n", path) //nolint:forbidigo

			return nil
		default:
			return fmt.Errorf("unsupported browser %q (supported: brave)", name)
		}
	},
}
