package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/inovacc/scout/pkg/scout/scraper"
	"github.com/inovacc/scout/pkg/scout/scraper/auth"
	"github.com/spf13/cobra"

	// Register all scraper modes.
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/amazon"
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/cloud"
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/confluence"
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/discord"
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/gdrive"
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/gmail"
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/gmaps"
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/grafana"
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/jira"
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/linkedin"
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/notion"
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/outlook"
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/reddit"
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/salesforce"
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/sharepoint"
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/slack"
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/teams"
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/twitter"
	_ "github.com/inovacc/scout/pkg/scout/scraper/modes/youtube"
)

func init() {
	rootCmd.AddCommand(scrapeCmd)
	scrapeCmd.AddCommand(scrapeListCmd)
	scrapeCmd.AddCommand(scrapeRunCmd)
	scrapeCmd.AddCommand(scrapeAuthCmd)

	scrapeRunCmd.Flags().StringSlice("target", nil, "extraction targets (channels, subreddits, etc.)")
	scrapeRunCmd.Flags().String("session-file", "", "path to encrypted session file")
	scrapeRunCmd.Flags().String("passphrase", "", "session file passphrase (or SCOUT_PASSPHRASE env)")
	scrapeRunCmd.Flags().String("output-dir", "", "directory for exported data")
	scrapeRunCmd.Flags().Int("limit", 0, "max items to extract (0 = unlimited)")
	scrapeRunCmd.Flags().Duration("timeout", 10*time.Minute, "max scrape duration")
	scrapeRunCmd.Flags().Bool("body", false, "capture response bodies")

	scrapeAuthCmd.Flags().String("save", "", "save session to encrypted file")
	scrapeAuthCmd.Flags().String("passphrase", "", "passphrase for session encryption (or SCOUT_PASSPHRASE env)")
	scrapeAuthCmd.Flags().Duration("timeout", 5*time.Minute, "max time to wait for login")
}

var scrapeCmd = &cobra.Command{
	Use:   "scrape",
	Short: "Extract data from authenticated web services",
	Long:  "Scrape authenticated services (Slack, Discord, Teams, Reddit) using browser automation and session hijacking.",
}

var scrapeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available scraper modes",
	RunE: func(cmd *cobra.Command, _ []string) error {
		modes := scraper.ListModes()
		if len(modes) == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "no scraper modes registered")
			return nil
		}

		for _, name := range modes {
			m, _ := scraper.GetMode(name)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %-12s %s\n", name, m.Description())
		}

		return nil
	},
}

var scrapeAuthCmd = &cobra.Command{
	Use:   "auth <mode>",
	Short: "Authenticate with a service via browser login",
	Long:  "Opens a browser for interactive login, captures session data, and optionally saves it encrypted.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		modeName := args[0]

		mode, err := scraper.GetMode(modeName)
		if err != nil {
			return fmt.Errorf("scout: scrape auth: %w", err)
		}

		timeout, _ := cmd.Flags().GetDuration("timeout")
		savePath, _ := cmd.Flags().GetString("save")
		passphrase, _ := cmd.Flags().GetString("passphrase")
		if passphrase == "" {
			passphrase = os.Getenv("SCOUT_PASSPHRASE")
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		opts := auth.DefaultBrowserAuthOptions()
		opts.Timeout = timeout
		opts.Progress = func(p auth.Progress) {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "[%s] %s\n", p.Phase, p.Message)
		}

		provider, ok := mode.AuthProvider().(auth.Provider)
		if !ok {
			return fmt.Errorf("scout: scrape auth: mode %q does not support browser auth", modeName)
		}

		session, err := auth.BrowserAuth(ctx, provider, opts)
		if err != nil {
			return fmt.Errorf("scout: scrape auth: %w", err)
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "authenticated as %s (provider: %s)\n", session.URL, session.Provider)

		if savePath != "" {
			if passphrase == "" {
				var readErr error
				passphrase, readErr = readPassphrase(cmd.ErrOrStderr(), "passphrase: ")
				if readErr != nil {
					return fmt.Errorf("scout: scrape auth: %w", readErr)
				}
			}

			if err := auth.SaveEncrypted(session, savePath, passphrase); err != nil {
				return fmt.Errorf("scout: scrape auth: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "session saved to %s\n", savePath)
		} else {
			data, _ := json.MarshalIndent(session, "", "  ")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		}

		return nil
	},
}

var scrapeRunCmd = &cobra.Command{
	Use:   "run <mode>",
	Short: "Run a scraper against an authenticated service",
	Long:  "Execute a scraper mode using a previously captured session. Results are streamed as NDJSON.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		modeName := args[0]

		mode, err := scraper.GetMode(modeName)
		if err != nil {
			return fmt.Errorf("scout: scrape run: %w", err)
		}

		sessionFile, _ := cmd.Flags().GetString("session-file")
		passphrase, _ := cmd.Flags().GetString("passphrase")
		if passphrase == "" {
			passphrase = os.Getenv("SCOUT_PASSPHRASE")
		}
		targets, _ := cmd.Flags().GetStringSlice("target")
		outputDir, _ := cmd.Flags().GetString("output-dir")
		limit, _ := cmd.Flags().GetInt("limit")
		timeout, _ := cmd.Flags().GetDuration("timeout")
		captureBody, _ := cmd.Flags().GetBool("body")

		var session *auth.Session
		if sessionFile != "" {
			if passphrase == "" {
				var readErr error
				passphrase, readErr = readPassphrase(cmd.ErrOrStderr(), "passphrase: ")
				if readErr != nil {
					return fmt.Errorf("scout: scrape run: %w", readErr)
				}
			}

			session, err = auth.LoadEncrypted(sessionFile, passphrase)
			if err != nil {
				return fmt.Errorf("scout: scrape run: %w", err)
			}

			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "loaded session for %s\n", session.Provider)
		}

		opts := scraper.DefaultScrapeOptions()
		opts.Headless = isHeadless(cmd)
		opts.Stealth = true
		opts.Timeout = timeout
		opts.Limit = limit
		opts.Targets = targets
		opts.OutputDir = outputDir
		opts.CaptureBody = captureBody
		opts.Progress = func(p scraper.Progress) {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "[%s] %s\n", p.Phase, p.Message)
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		// Handle Ctrl+C
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		go func() {
			<-sigCh
			signal.Stop(sigCh)
			cancel()
		}()

		results, err := mode.Scrape(ctx, session, opts)
		if err != nil {
			return fmt.Errorf("scout: scrape run: %w", err)
		}

		enc := json.NewEncoder(cmd.OutOrStdout())
		count := 0

		for result := range results {
			_ = enc.Encode(result)
			count++
		}

		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "scraped %d items\n", count)

		// Export collected results if output dir specified.
		if outputDir != "" {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "results exported to %s\n", outputDir)
		}

		_ = strings.Join(targets, ",") // suppress unused

		return nil
	},
}
