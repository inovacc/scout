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
}

var browserCmd = &cobra.Command{
	Use:   "browser",
	Short: "Manage browser installations",
}

var browserListCmd = &cobra.Command{
	Use:   "list",
	Short: "List detected and downloaded browsers",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Detected browsers (local install):")

		for _, bt := range []scout.BrowserType{scout.BrowserChrome, scout.BrowserBrave, scout.BrowserEdge} {
			path, err := scout.LookupBrowserPublic(bt)
			if err != nil {
				fmt.Printf("  %-8s  not found\n", bt)
			} else if path == "" {
				fmt.Printf("  %-8s  auto-detect (rod)\n", bt)
			} else {
				fmt.Printf("  %-8s  %s\n", bt, path)
			}
		}

		fmt.Println()
		fmt.Println("Downloaded browsers (~/.scout/browsers/):")

		browsers, err := scout.ListDownloadedBrowsers()
		if err != nil {
			return err
		}

		if len(browsers) == 0 {
			fmt.Println("  (none)")
		} else {
			for _, b := range browsers {
				fmt.Printf("  %s\n", b)
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
			fmt.Println("Downloading Brave browser...")
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Minute)
			defer cancel()
			path, err := scout.DownloadBrave(ctx)
			if err != nil {
				return fmt.Errorf("download brave: %w", err)
			}
			fmt.Printf("Brave downloaded to: %s\n", path)
			return nil
		default:
			return fmt.Errorf("unsupported browser %q (supported: brave)", name)
		}
	},
}
