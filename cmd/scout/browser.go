package main

import (
	"fmt"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(browserCmd)
	browserCmd.AddCommand(browserListCmd)
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
