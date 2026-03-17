package main

import (
	"context"
	"fmt"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

var credentialsCmd = &cobra.Command{
	Use:   "credentials",
	Short: "Browser credential capture and replay",
}

var credentialsCaptureCmd = &cobra.Command{
	Use:   "capture <url>",
	Short: "Open browser for manual login, capture all auth state",
	Long: `Opens a visible browser to the given URL. Log in manually, navigate freely.

By default, press Ctrl+C when done. With --on-close, simply close the browser
window — credentials are captured automatically (snapshots taken every 2s).

Captures: cookies, localStorage, sessionStorage, user agent, browser version.
The output file can be used with 'scout credentials replay' to restore the session.

With --on-close, the session directory is deleted after capture unless --persist
is set.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		outFile, _ := cmd.Flags().GetString("output")
		if outFile == "" {
			outFile = "credentials.json"
		}

		onClose, _ := cmd.Flags().GetBool("on-close")
		persist, _ := cmd.Flags().GetBool("persist")

		w := cmd.OutOrStdout()
		_, _ = fmt.Fprintf(w, "Launching browser to %s\n", args[0])

		opts := baseOpts(cmd)
		opts = append(opts, scout.WithHeadless(false))

		var creds *scout.CapturedCredentials

		if onClose {
			_, _ = fmt.Fprintln(w, "Log in manually, then close the browser to capture credentials.")

			captureOpts := []scout.CaptureOption{
				scout.WithCaptureSavePath(outFile),
			}
			if persist {
				captureOpts = append(captureOpts, scout.WithCapturePersist())
			}

			var err error

			creds, err = scout.CaptureOnClose(context.Background(), args[0], opts, captureOpts...)
			if err != nil {
				return err
			}
		} else {
			_, _ = fmt.Fprintln(w, "Log in manually, then press Ctrl+C to capture credentials.")

			var err error

			creds, err = scout.CaptureCredentials(context.Background(), args[0], opts...)
			if err != nil {
				return err
			}

			if err := scout.SaveCredentials(creds, outFile); err != nil {
				return err
			}
		}

		_, _ = fmt.Fprintf(w, "\nCredentials saved to: %s\n", outFile)
		_, _ = fmt.Fprintf(w, "  URL:              %s\n", creds.URL)
		_, _ = fmt.Fprintf(w, "  Final URL:        %s\n", creds.FinalURL)
		_, _ = fmt.Fprintf(w, "  Browser:          %s\n", creds.Browser.Product)
		_, _ = fmt.Fprintf(w, "  User Agent:       %s\n", truncate(creds.UserAgent, 60))
		_, _ = fmt.Fprintf(w, "  Cookies:          %d\n", len(creds.Cookies))
		_, _ = fmt.Fprintf(w, "  LocalStorage:     %d keys\n", len(creds.LocalStorage))
		_, _ = fmt.Fprintf(w, "  SessionStorage:   %d keys\n", len(creds.SessionStorage))
		_, _ = fmt.Fprintf(w, "  Captured at:      %s\n", creds.CapturedAt.Format(time.RFC3339))

		if persist {
			_, _ = fmt.Fprintln(w, "  Session:          persisted")
		}

		return nil
	},
}

var credentialsReplayCmd = &cobra.Command{
	Use:   "replay <credentials.json> [url]",
	Short: "Restore a captured session and navigate to a URL",
	Long: `Loads credentials from a JSON file captured with 'scout credentials capture'.
Restores cookies, localStorage, sessionStorage, and user agent.
Optionally navigates to a different URL after restoring.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		creds, err := scout.LoadCredentials(args[0])
		if err != nil {
			return err
		}

		opts := baseOpts(cmd)
		if creds.UserAgent != "" {
			opts = append(opts, scout.WithUserAgent(creds.UserAgent))
		}

		b, err := scout.New(opts...)
		if err != nil {
			return err
		}

		defer func() { _ = b.Close() }()

		session := creds.ToSessionState()

		// If a URL argument is given, use it instead of the captured final URL.
		if len(args) > 1 {
			session.URL = args[1]
		}

		page, err := b.NewPage("")
		if err != nil {
			return err
		}

		if err := page.LoadSession(session); err != nil {
			return err
		}

		title, _ := page.Title()
		url, _ := page.URL()

		w := cmd.OutOrStdout()
		_, _ = fmt.Fprintf(w, "Session restored: %s (%s)\n", url, title)
		_, _ = fmt.Fprintf(w, "  Cookies loaded:  %d\n", len(creds.Cookies))
		_, _ = fmt.Fprintf(w, "  LocalStorage:    %d keys\n", len(creds.LocalStorage))
		_, _ = fmt.Fprintf(w, "  SessionStorage:  %d keys\n", len(creds.SessionStorage))

		return nil
	},
}

var credentialsShowCmd = &cobra.Command{
	Use:   "show <credentials.json>",
	Short: "Display contents of a credentials file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		creds, err := scout.LoadCredentials(args[0])
		if err != nil {
			return err
		}

		w := cmd.OutOrStdout()
		_, _ = fmt.Fprintf(w, "URL:              %s\n", creds.URL)
		_, _ = fmt.Fprintf(w, "Final URL:        %s\n", creds.FinalURL)
		_, _ = fmt.Fprintf(w, "Browser:          %s (%s/%s)\n", creds.Browser.Product, creds.Browser.OS, creds.Browser.Arch)
		_, _ = fmt.Fprintf(w, "User Agent:       %s\n", creds.UserAgent)
		_, _ = fmt.Fprintf(w, "Captured at:      %s\n", creds.CapturedAt.Format(time.RFC3339))
		_, _ = fmt.Fprintf(w, "Cookies:          %d\n", len(creds.Cookies))

		for _, c := range creds.Cookies {
			_, _ = fmt.Fprintf(w, "  %-30s  domain=%-20s  secure=%v  httpOnly=%v\n",
				truncate(c.Name, 30), c.Domain, c.Secure, c.HTTPOnly)
		}

		_, _ = fmt.Fprintf(w, "LocalStorage:     %d keys\n", len(creds.LocalStorage))
		for k := range creds.LocalStorage {
			_, _ = fmt.Fprintf(w, "  %s\n", truncate(k, 60))
		}

		_, _ = fmt.Fprintf(w, "SessionStorage:   %d keys\n", len(creds.SessionStorage))
		for k := range creds.SessionStorage {
			_, _ = fmt.Fprintf(w, "  %s\n", truncate(k, 60))
		}

		return nil
	},
}

func init() {
	credentialsCaptureCmd.Flags().Bool("on-close", false, "Capture when browser window is closed (instead of Ctrl+C)")
	credentialsCaptureCmd.Flags().Bool("persist", false, "Keep session directory after capture (use with --on-close)")

	credentialsCmd.AddCommand(credentialsCaptureCmd, credentialsReplayCmd, credentialsShowCmd)
	rootCmd.AddCommand(credentialsCmd)
}
