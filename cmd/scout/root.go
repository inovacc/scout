package main

import (
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
	rootCmd.PersistentFlags().String("addr", "localhost:9551", "gRPC daemon address (deprecated, use --target)")
	rootCmd.PersistentFlags().StringSlice("target", nil, "target server address(es), repeatable")
	rootCmd.PersistentFlags().Bool("standalone", false, "run without daemon (one-shot browser)")
	rootCmd.PersistentFlags().String("session", "", "session ID to use")
	rootCmd.PersistentFlags().StringP("output", "o", "", "output file path")
	rootCmd.PersistentFlags().String("format", "text", "output format (text, json)")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().Bool("headless", true, "run browser in headless mode")
	rootCmd.PersistentFlags().String("browser", "chrome", "browser type: chrome, brave, edge")
	rootCmd.PersistentFlags().Bool("maximized", false, "start browser window maximized")
	rootCmd.PersistentFlags().Bool("devtools", false, "open Chrome DevTools automatically")
	rootCmd.PersistentFlags().Bool("insecure", false, "disable mTLS for client connections")
}

func main() {
	cobra.CheckErr(rootCmd.Execute())
}
