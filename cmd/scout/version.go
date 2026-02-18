package main

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags.
var Version = "dev"

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print scout version and build info",
	RunE: func(cmd *cobra.Command, _ []string) error {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "scout %s\n", Version)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  go:   %s\n", runtime.Version())
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  os:   %s/%s\n", runtime.GOOS, runtime.GOARCH)
		return nil
	},
}
