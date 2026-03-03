package main

import (
	"log"
	"os"
	"time"

	"github.com/google/gops/agent"
	"github.com/inovacc/scout/internal/flags"
	"github.com/inovacc/scout/internal/logger"
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
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := flags.ExportFlagsToEnv(); err != nil {
			return nil // non-fatal
		}

		if flags.ShouldIgnoreCommand(cmd.Name()) {
			return nil
		}

		log := logger.Init(cmd.Name())
		if log.IsActive() {
			stdout, stderr := log.StartExecution(cmd.Name(), args, cmd.OutOrStdout(), cmd.ErrOrStderr())
			cmd.SetOut(stdout)
			cmd.SetErr(stderr)
		}

		return nil
	},
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
	rootCmd.PersistentFlags().Bool("stealth", false, "enable anti-bot-detection stealth mode")
	rootCmd.PersistentFlags().Bool("insecure", false, "disable mTLS for client connections")
	rootCmd.PersistentFlags().String("electron-app", "", "path to Electron app directory or binary")
	rootCmd.PersistentFlags().String("electron-version", "", "Electron version to download (e.g. v33.2.0)")
	rootCmd.PersistentFlags().String("electron-cdp", "", "CDP endpoint of running Electron app")
	rootCmd.PersistentFlags().Duration("idle-timeout", 5*time.Minute, "auto-shutdown after inactivity (0 to disable)")
	rootCmd.PersistentFlags().Bool("system-browser", false, "allow system-installed browsers instead of cache-only")
}

func Execute() {
	err := rootCmd.Execute()

	log := logger.Get()
	if log != nil && log.IsActive() {
		log.EndExecution(err)
		_ = log.Close()
	}

	if err != nil {
		os.Exit(1)
	}
}

func main() {
	if err := agent.Listen(agent.Options{ShutdownCleanup: true}); err != nil {
		log.Printf("scout: gops agent: %v", err)
	}

	defer agent.Close()

	Execute()
}
