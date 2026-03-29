package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/scraper/auth"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(authLoginCmd, authCaptureCmd, authReplayCmd, authShowCmd, authStatusCmd, authLogoutCmd, authProvidersCmd)

	authLoginCmd.Flags().String("provider", "", "auth provider name")
	authLoginCmd.Flags().String("workspace", "", "workspace domain (provider-specific)")
	authLoginCmd.Flags().Duration("timeout", 5*time.Minute, "max time to wait for login")

	authCaptureCmd.Flags().String("url", "", "URL to open in browser")
	authCaptureCmd.Flags().Duration("timeout", 5*time.Minute, "max time before auto-capture")
	authCaptureCmd.Flags().Bool("plaintext", false, "write unencrypted JSON output instead of AES-encrypted session file")
	authCaptureCmd.Flags().Bool("on-close", false, "capture when browser window is closed (instead of Ctrl+C)")
	authCaptureCmd.Flags().Bool("persist", false, "keep session directory after capture (use with --on-close)")

	authStatusCmd.Flags().String("input", "", "encrypted session file path")

	authLogoutCmd.Flags().String("input", "", "encrypted session file to delete")
}

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authentication and session management",
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Launch browser for provider-specific login and capture session",
	RunE: func(cmd *cobra.Command, _ []string) error {
		providerName, _ := cmd.Flags().GetString("provider")
		workspace, _ := cmd.Flags().GetString("workspace")
		timeout, _ := cmd.Flags().GetDuration("timeout")

		if providerName == "" {
			return fmt.Errorf("scout: --provider is required (use 'scout auth providers' to list)")
		}

		provider, err := auth.Get(providerName)
		if err != nil {
			return fmt.Errorf("scout: %w", err)
		}

		_ = workspace // reserved for provider-specific use

		passphrase, err := readPassphraseConfirm(cmd.ErrOrStderr())
		if err != nil {
			return err
		}

		outFile, _ := cmd.Flags().GetString("output")
		if outFile == "" {
			outFile = providerName + "-session.enc"
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			<-sigCh

			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "\ninterrupted, capturing session before exit...")

			cancel()
		}()

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "launching browser for %s login...\n", providerName)

		opts := auth.DefaultBrowserAuthOptions()
		opts.Timeout = timeout
		opts.CaptureOnClose = true
		opts.Progress = func(p auth.Progress) {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "  [%s] %s\n", p.Phase, p.Message)
		}

		session, err := auth.BrowserAuth(ctx, provider, opts)
		if err != nil {
			return fmt.Errorf("scout: auth login: %w", err)
		}

		if err := auth.SaveEncrypted(session, outFile, passphrase); err != nil {
			return fmt.Errorf("scout: save session: %w", err)
		}

		w := cmd.OutOrStdout()
		_, _ = fmt.Fprintf(w, "\nsession saved to: %s\n", outFile)
		_, _ = fmt.Fprintf(w, "  provider:  %s\n", session.Provider)
		_, _ = fmt.Fprintf(w, "  url:       %s\n", session.URL)
		_, _ = fmt.Fprintf(w, "  cookies:   %d\n", len(session.Cookies))
		_, _ = fmt.Fprintf(w, "  tokens:    %d\n", len(session.Tokens))
		_, _ = fmt.Fprintf(w, "  timestamp: %s\n", session.Timestamp.Format(time.RFC3339))

		return nil
	},
}

var authCaptureCmd = &cobra.Command{
	Use:   "capture",
	Short: "Launch browser to any URL, capture all data (cookies, storage) before close",
	Long: `Opens a browser to the specified URL. Interact freely — log in, navigate,
do anything you need. When you press Ctrl+C or the timeout expires, all session
data (cookies, localStorage, sessionStorage) is captured and saved.

By default the output is AES-encrypted. Use --plaintext to write unencrypted JSON
(same format accepted by 'scout auth replay').

Use --on-close to capture automatically when the browser window is closed instead
of waiting for Ctrl+C.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		targetURL, _ := cmd.Flags().GetString("url")
		if targetURL == "" {
			return fmt.Errorf("scout: --url is required")
		}

		timeout, _ := cmd.Flags().GetDuration("timeout")
		plaintext, _ := cmd.Flags().GetBool("plaintext")
		onClose, _ := cmd.Flags().GetBool("on-close")
		persist, _ := cmd.Flags().GetBool("persist")

		w := cmd.OutOrStdout()

		outFile, _ := cmd.Flags().GetString("output")

		if plaintext {
			// Plaintext JSON path: use scout.CaptureCredentials / scout.CaptureOnClose
			if outFile == "" {
				outFile = "credentials.json"
			}

			_, _ = fmt.Fprintf(w, "launching browser to %s\n", targetURL)

			browserOpts := baseOpts(cmd)
			browserOpts = append(browserOpts, scout.WithHeadless(false))

			var creds *scout.CapturedCredentials

			if onClose {
				_, _ = fmt.Fprintln(w, "log in manually, then close the browser to capture credentials.")

				captureOpts := []scout.CaptureOption{scout.WithCaptureSavePath(outFile)}
				if persist {
					captureOpts = append(captureOpts, scout.WithCapturePersist())
				}

				var err error

				creds, err = scout.CaptureOnClose(context.Background(), targetURL, browserOpts, captureOpts...)
				if err != nil {
					return err
				}
			} else {
				_, _ = fmt.Fprintln(w, "log in manually, then press Ctrl+C to capture credentials.")

				var err error

				creds, err = scout.CaptureCredentials(context.Background(), targetURL, browserOpts...)
				if err != nil {
					return err
				}

				if err := scout.SaveCredentials(creds, outFile); err != nil {
					return err
				}
			}

			_, _ = fmt.Fprintf(w, "\ncredentials saved to: %s\n", outFile)
			_, _ = fmt.Fprintf(w, "  url:              %s\n", creds.URL)
			_, _ = fmt.Fprintf(w, "  final url:        %s\n", creds.FinalURL)
			_, _ = fmt.Fprintf(w, "  browser:          %s\n", creds.Browser.Product)
			_, _ = fmt.Fprintf(w, "  user agent:       %s\n", truncate(creds.UserAgent, 60))
			_, _ = fmt.Fprintf(w, "  cookies:          %d\n", len(creds.Cookies))
			_, _ = fmt.Fprintf(w, "  localStorage:     %d keys\n", len(creds.LocalStorage))
			_, _ = fmt.Fprintf(w, "  sessionStorage:   %d keys\n", len(creds.SessionStorage))
			_, _ = fmt.Fprintf(w, "  captured at:      %s\n", creds.CapturedAt.Format(time.RFC3339))

			if persist {
				_, _ = fmt.Fprintln(w, "  session:          persisted")
			}

			return nil
		}

		// Encrypted path (default)
		if outFile == "" {
			outFile = "session.enc"
		}

		passphrase, err := readPassphraseConfirm(cmd.ErrOrStderr())
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			<-sigCh

			_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "\ncapturing session data...")

			cancel()
		}()

		_, _ = fmt.Fprintf(w, "launching browser to %s\n", targetURL)
		_, _ = fmt.Fprintln(w, "interact freely — press Ctrl+C when done to capture session")

		authOpts := auth.DefaultBrowserAuthOptions()
		authOpts.Timeout = timeout
		authOpts.Progress = func(p auth.Progress) {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "  [%s] %s\n", p.Phase, p.Message)
		}

		session, err := auth.BrowserCapture(ctx, targetURL, authOpts)
		if err != nil {
			return fmt.Errorf("scout: capture: %w", err)
		}

		if err := auth.SaveEncrypted(session, outFile, passphrase); err != nil {
			return fmt.Errorf("scout: save session: %w", err)
		}

		_, _ = fmt.Fprintf(w, "\nsession saved to: %s\n", outFile)
		_, _ = fmt.Fprintf(w, "  url:       %s\n", session.URL)
		_, _ = fmt.Fprintf(w, "  cookies:   %d\n", len(session.Cookies))
		_, _ = fmt.Fprintf(w, "  localStorage keys:   %d\n", len(session.LocalStorage))
		_, _ = fmt.Fprintf(w, "  sessionStorage keys: %d\n", len(session.SessionStorage))
		_, _ = fmt.Fprintf(w, "  timestamp: %s\n", session.Timestamp.Format(time.RFC3339))

		return nil
	},
}

var authReplayCmd = &cobra.Command{
	Use:   "replay <credentials.json> [url]",
	Short: "Restore a captured session and navigate to a URL",
	Long: `Loads credentials from a JSON file captured with 'scout auth capture --plaintext'.
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

		sessionState := creds.ToSessionState()

		if len(args) > 1 {
			sessionState.URL = args[1]
		}

		page, err := b.NewPage("")
		if err != nil {
			return err
		}

		if err := page.LoadSession(sessionState); err != nil {
			return err
		}

		title, _ := page.Title()
		url, _ := page.URL()

		w := cmd.OutOrStdout()
		_, _ = fmt.Fprintf(w, "session restored: %s (%s)\n", url, title)
		_, _ = fmt.Fprintf(w, "  cookies loaded:  %d\n", len(creds.Cookies))
		_, _ = fmt.Fprintf(w, "  localStorage:    %d keys\n", len(creds.LocalStorage))
		_, _ = fmt.Fprintf(w, "  sessionStorage:  %d keys\n", len(creds.SessionStorage))

		return nil
	},
}

var authShowCmd = &cobra.Command{
	Use:   "show <credentials.json>",
	Short: "Display contents of a plaintext credentials file",
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

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show info from an encrypted session file",
	RunE: func(cmd *cobra.Command, _ []string) error {
		input, _ := cmd.Flags().GetString("input")
		if input == "" {
			return fmt.Errorf("scout: --input is required")
		}

		passphrase, err := readPassphrase(cmd.ErrOrStderr(), "Enter passphrase: ")
		if err != nil {
			return err
		}

		session, err := auth.LoadEncrypted(input, passphrase)
		if err != nil {
			return fmt.Errorf("scout: load session: %w", err)
		}

		w := cmd.OutOrStdout()
		_, _ = fmt.Fprintln(w, "session info:")
		_, _ = fmt.Fprintf(w, "  provider:    %s\n", session.Provider)
		_, _ = fmt.Fprintf(w, "  version:     %s\n", session.Version)
		_, _ = fmt.Fprintf(w, "  url:         %s\n", session.URL)
		_, _ = fmt.Fprintf(w, "  cookies:     %d\n", len(session.Cookies))
		_, _ = fmt.Fprintf(w, "  tokens:      %d\n", len(session.Tokens))
		_, _ = fmt.Fprintf(w, "  localStorage keys:   %d\n", len(session.LocalStorage))
		_, _ = fmt.Fprintf(w, "  sessionStorage keys: %d\n", len(session.SessionStorage))
		_, _ = fmt.Fprintf(w, "  timestamp:   %s\n", session.Timestamp.Format(time.RFC3339))

		if !session.ExpiresAt.IsZero() {
			_, _ = fmt.Fprintf(w, "  expires:     %s\n", session.ExpiresAt.Format(time.RFC3339))
		}

		for k, v := range session.Extra {
			_, _ = fmt.Fprintf(w, "  %s: %s\n", k, truncate(v, 60))
		}

		return nil
	},
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Delete an encrypted session file",
	RunE: func(cmd *cobra.Command, _ []string) error {
		input, _ := cmd.Flags().GetString("input")
		if input == "" {
			return fmt.Errorf("scout: --input is required")
		}

		if _, err := os.Stat(input); os.IsNotExist(err) {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "no session file at %s\n", input)

			return nil
		}

		if err := os.Remove(input); err != nil {
			return fmt.Errorf("scout: remove session: %w", err)
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "session removed: %s\n", input)

		return nil
	},
}

var authProvidersCmd = &cobra.Command{
	Use:   "providers",
	Short: "List available auth providers",
	Run: func(cmd *cobra.Command, _ []string) {
		providers := auth.List()
		sort.Strings(providers)

		w := cmd.OutOrStdout()
		_, _ = fmt.Fprintln(w, "available providers:")

		for _, name := range providers {
			_, _ = fmt.Fprintf(w, "  - %s\n", name)
		}

		_, _ = fmt.Fprintln(w, "\nuse 'scout auth login --provider <name>' to authenticate")
		_, _ = fmt.Fprintln(w, "use 'scout auth capture --url <url>' for generic browser capture")
		_, _ = fmt.Fprintln(w, "use 'scout auth capture --url <url> --plaintext' for unencrypted JSON output")
	},
}
