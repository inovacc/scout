package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"github.com/inovacc/scout/scraper"
	"github.com/inovacc/scout/scraper/auth"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(authLoginCmd, authCaptureCmd, authStatusCmd, authLogoutCmd, authProvidersCmd)

	authLoginCmd.Flags().String("provider", "", "auth provider name")
	authLoginCmd.Flags().String("workspace", "", "workspace domain (provider-specific)")
	authLoginCmd.Flags().Duration("timeout", 5*time.Minute, "max time to wait for login")

	authCaptureCmd.Flags().String("url", "", "URL to open in browser")
	authCaptureCmd.Flags().Duration("timeout", 5*time.Minute, "max time before auto-capture")

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
		opts.Progress = func(p scraper.Progress) {
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
data (cookies, localStorage, sessionStorage) is captured and encrypted.

This is the generic "launch browser and capture everything" flow.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		targetURL, _ := cmd.Flags().GetString("url")
		if targetURL == "" {
			return fmt.Errorf("scout: --url is required")
		}

		timeout, _ := cmd.Flags().GetDuration("timeout")

		passphrase, err := readPassphraseConfirm(cmd.ErrOrStderr())
		if err != nil {
			return err
		}

		outFile, _ := cmd.Flags().GetString("output")
		if outFile == "" {
			outFile = "session.enc"
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

		w := cmd.OutOrStdout()
		_, _ = fmt.Fprintf(w, "launching browser to %s\n", targetURL)
		_, _ = fmt.Fprintln(w, "interact freely — press Ctrl+C when done to capture session")

		opts := auth.DefaultBrowserAuthOptions()
		opts.Timeout = timeout
		opts.Progress = func(p scraper.Progress) {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "  [%s] %s\n", p.Phase, p.Message)
		}

		session, err := auth.BrowserCapture(ctx, targetURL, opts)
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
	},
}
