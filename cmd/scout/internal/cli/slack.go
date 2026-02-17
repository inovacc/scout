package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/scraper"
	"github.com/inovacc/scout/scraper/slack"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(slackCmd)
	slackCmd.AddCommand(slackCaptureCmd, slackLoadCmd, slackDecryptCmd)

	slackCaptureCmd.Flags().String("workspace", "", "Slack workspace domain (e.g. myteam.slack.com or myteam)")
	slackCaptureCmd.Flags().Duration("timeout", 5*time.Minute, "max time to wait for login")

	slackLoadCmd.Flags().String("input", "slack-session.enc", "encrypted session file path")

	slackDecryptCmd.Flags().String("input", "slack-session.enc", "encrypted session file path")
}

var slackCmd = &cobra.Command{
	Use:   "slack",
	Short: "Slack workspace session management",
}

// tokenExtractionJS is the same JS used by the slack scraper to find xoxc tokens.
const tokenExtractionJS = `() => {
	// Try localConfig_v2
	try {
		const keys = Object.keys(localStorage);
		for (const key of keys) {
			if (key.startsWith('localConfig_v2')) {
				const val = localStorage.getItem(key);
				if (val) {
					const parsed = JSON.parse(val);
					const teams = parsed.teams || {};
					for (const teamId of Object.keys(teams)) {
						const token = teams[teamId].token;
						if (token && token.startsWith('xoxc-')) {
							return token;
						}
					}
				}
			}
		}
	} catch(e) {}

	// Try window.boot_data
	try {
		if (window.boot_data && window.boot_data.api_token) {
			const token = window.boot_data.api_token;
			if (token.startsWith('xoxc-')) {
				return token;
			}
		}
	} catch(e) {}

	// Try all localStorage keys for xoxc pattern
	try {
		for (let i = 0; i < localStorage.length; i++) {
			const key = localStorage.key(i);
			const val = localStorage.getItem(key);
			if (val && val.includes('xoxc-')) {
				const match = val.match(/(xoxc-[a-zA-Z0-9-]+)/);
				if (match) return match[1];
			}
		}
	} catch(e) {}

	return "";
}`

var slackCaptureCmd = &cobra.Command{
	Use:   "capture",
	Short: "Launch browser, log in to Slack, capture and encrypt session",
	RunE: func(cmd *cobra.Command, _ []string) error {
		workspace, _ := cmd.Flags().GetString("workspace")
		if workspace == "" {
			return fmt.Errorf("scout: --workspace is required")
		}

		timeout, _ := cmd.Flags().GetDuration("timeout")

		passphrase, err := readPassphraseConfirm(cmd.ErrOrStderr())
		if err != nil {
			return err
		}

		outFile, _ := cmd.Flags().GetString("output")
		if outFile == "" {
			outFile = "slack-session.enc"
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigCh
			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "\ninterrupted, shutting down...")
			cancel()
		}()

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "launching browser...")

		browser, err := scout.New(
			scout.WithHeadless(false),
			scout.WithNoSandbox(),
			browserOpt(cmd),
			scout.WithTimeout(timeout),
		)
		if err != nil {
			return fmt.Errorf("scout: launch browser: %w", err)
		}
		defer func() { _ = browser.Close() }()

		workspaceURL := normalizeWorkspaceURL(workspace)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "navigating to %s\n", workspaceURL)

		page, err := browser.NewPage(workspaceURL)
		if err != nil {
			return fmt.Errorf("scout: open page: %w", err)
		}

		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "please log in to Slack in the browser window...")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "waiting for xoxc token (polling every 2s)...")

		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return fmt.Errorf("scout: timeout: could not detect login within %v", timeout)
			case <-ticker.C:
				result, err := page.Eval(tokenExtractionJS)
				if err != nil {
					continue
				}

				token := result.String()
				if token == "" || !strings.HasPrefix(token, "xoxc-") {
					continue
				}

				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "token detected! capturing session...")

				session, err := slack.CaptureFromPage(page)
				if err != nil {
					return fmt.Errorf("scout: capture session: %w", err)
				}

				if err := slack.SaveEncrypted(session, outFile, passphrase); err != nil {
					return fmt.Errorf("scout: save session: %w", err)
				}

				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nsession saved to: %s\n", outFile)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  workspace: %s\n", session.WorkspaceURL)
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  token:     %s...\n", session.Token[:min(20, len(session.Token))])
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  cookies:   %d\n", len(session.Cookies))
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  timestamp: %s\n", session.Timestamp.Format(time.RFC3339))
				return nil
			}
		}
	},
}

var slackLoadCmd = &cobra.Command{
	Use:   "load",
	Short: "Load and display info from an encrypted session file",
	RunE: func(cmd *cobra.Command, _ []string) error {
		input, _ := cmd.Flags().GetString("input")

		passphrase, err := readPassphrase(cmd.ErrOrStderr(), "Enter passphrase: ")
		if err != nil {
			return err
		}

		session, err := slack.LoadEncrypted(input, passphrase)
		if err != nil {
			return fmt.Errorf("scout: load session: %w", err)
		}

		w := cmd.OutOrStdout()
		_, _ = fmt.Fprintln(w, "session info:")
		_, _ = fmt.Fprintf(w, "  version:    %s\n", session.Version)
		_, _ = fmt.Fprintf(w, "  workspace:  %s\n", session.WorkspaceURL)
		_, _ = fmt.Fprintf(w, "  token:      %s...\n", session.Token[:min(20, len(session.Token))])
		_, _ = fmt.Fprintf(w, "  d_cookie:   %s...\n", session.DCookie[:min(20, len(session.DCookie))])
		_, _ = fmt.Fprintf(w, "  cookies:    %d\n", len(session.Cookies))
		_, _ = fmt.Fprintf(w, "  localStorage keys: %d\n", len(session.LocalStorage))
		_, _ = fmt.Fprintf(w, "  sessionStorage keys: %d\n", len(session.SessionStorage))
		_, _ = fmt.Fprintf(w, "  timestamp:  %s\n", session.Timestamp.Format(time.RFC3339))

		if session.UserID != "" {
			_, _ = fmt.Fprintf(w, "  user_id:    %s\n", session.UserID)
		}
		if session.TeamName != "" {
			_, _ = fmt.Fprintf(w, "  team:       %s\n", session.TeamName)
		}

		return nil
	},
}

var slackDecryptCmd = &cobra.Command{
	Use:   "decrypt",
	Short: "Decrypt session file to plaintext JSON",
	RunE: func(cmd *cobra.Command, _ []string) error {
		input, _ := cmd.Flags().GetString("input")
		outFile, _ := cmd.Flags().GetString("output")
		if outFile == "" {
			outFile = "slack-session.json"
		}

		passphrase, err := readPassphrase(cmd.ErrOrStderr(), "Enter passphrase: ")
		if err != nil {
			return err
		}

		session, err := slack.LoadEncrypted(input, passphrase)
		if err != nil {
			return fmt.Errorf("scout: load session: %w", err)
		}

		if err := scraper.ExportJSON(session, outFile); err != nil {
			return fmt.Errorf("scout: export json: %w", err)
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "decrypted session written to: %s\n", outFile)
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "WARNING: this file contains sensitive credentials in plaintext")
		return nil
	},
}

func normalizeWorkspaceURL(workspace string) string {
	if strings.HasPrefix(workspace, "http://") || strings.HasPrefix(workspace, "https://") {
		return workspace
	}

	if !strings.Contains(workspace, ".") {
		workspace += ".slack.com"
	}

	return "https://" + workspace
}
