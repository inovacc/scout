package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "scout",
	Short: "Headless browser automation, scraping, and forensic capture",
	Long: `Scout is a CLI for headless browser automation, web scraping, search, crawling,
and forensic capture. It wraps go-rod and exposes all features as commands.

Commands communicate with a background gRPC daemon for session persistence,
or run standalone for one-shot operations.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().String("addr", "localhost:50051", "gRPC daemon address")
	rootCmd.PersistentFlags().Bool("standalone", false, "run without daemon (one-shot browser)")
	rootCmd.PersistentFlags().String("session", "", "session ID to use")
	rootCmd.PersistentFlags().StringP("output", "o", "", "output file path")
	rootCmd.PersistentFlags().String("format", "text", "output format (text, json)")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)

		os.Exit(1)
	}
}
