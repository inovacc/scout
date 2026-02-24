package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Portable browser identity (capture, load, show)",
}

var profileCaptureCmd = &cobra.Command{
	Use:   "capture <url>",
	Short: "Open browser, capture browser identity to a .scoutprofile file on Ctrl+C",
	Long: `Opens a visible browser to the given URL. Browse freely to establish your session.
Press Ctrl+C when done — all browser identity state is captured:
cookies, localStorage, sessionStorage, user agent, language, timezone, window size.

The output file can be used with 'scout profile load' to restore the identity.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		outFile, _ := cmd.Flags().GetString("output")
		if outFile == "" {
			outFile = "profile.scoutprofile"
		}

		name, _ := cmd.Flags().GetString("name")

		w := cmd.OutOrStdout()
		_, _ = fmt.Fprintf(w, "Launching browser to %s\n", args[0])
		_, _ = fmt.Fprintln(w, "Browse freely, then press Ctrl+C to capture profile.")

		opts := baseOpts(cmd)
		opts = append(opts, scout.WithHeadless(false))

		b, err := scout.New(opts...)
		if err != nil {
			return err
		}
		defer func() { _ = b.Close() }()

		page, err := b.NewPage(args[0])
		if err != nil {
			return err
		}

		if err := page.WaitLoad(); err != nil {
			return err
		}

		// Wait for signal.
		sigCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		<-sigCtx.Done()

		captureOpts := []scout.ProfileOption{}
		if name != "" {
			captureOpts = append(captureOpts, scout.WithProfileName(name))
		}

		prof, err := scout.CaptureProfile(page, captureOpts...)
		if err != nil {
			return err
		}

		if err := scout.SaveProfile(prof, outFile); err != nil {
			return err
		}

		_, _ = fmt.Fprintf(w, "\nProfile saved to: %s\n", outFile)
		_, _ = fmt.Fprintf(w, "  Name:           %s\n", prof.Name)
		_, _ = fmt.Fprintf(w, "  Browser:        %s\n", prof.Browser.Type)
		_, _ = fmt.Fprintf(w, "  User Agent:     %s\n", truncate(prof.Identity.UserAgent, 60))
		_, _ = fmt.Fprintf(w, "  Language:       %s\n", prof.Identity.Language)
		_, _ = fmt.Fprintf(w, "  Timezone:       %s\n", prof.Identity.Timezone)
		_, _ = fmt.Fprintf(w, "  Cookies:        %d\n", len(prof.Cookies))

		storageKeys := 0
		for _, s := range prof.Storage {
			storageKeys += len(s.LocalStorage) + len(s.SessionStorage)
		}

		_, _ = fmt.Fprintf(w, "  Storage keys:   %d\n", storageKeys)
		_, _ = fmt.Fprintf(w, "  Captured at:    %s\n", prof.CreatedAt.Format(time.RFC3339))

		return nil
	},
}

var profileLoadCmd = &cobra.Command{
	Use:   "load <file.scoutprofile> [url]",
	Short: "Restore a browser identity from a profile and navigate to a URL",
	Long: `Loads a profile from a .scoutprofile file and applies it to a new browser session.
Restores user agent, cookies, localStorage, sessionStorage, window size, and headers.
Optionally navigates to a URL after restoring.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		prof, err := scout.LoadProfile(args[0])
		if err != nil {
			return err
		}

		opts := baseOpts(cmd)
		opts = append(opts, scout.WithProfileData(prof))

		b, err := scout.New(opts...)
		if err != nil {
			return err
		}
		defer func() { _ = b.Close() }()

		targetURL := ""
		if len(args) > 1 {
			targetURL = args[1]
		}

		page, err := b.NewPage(targetURL)
		if err != nil {
			return err
		}

		if targetURL != "" {
			if err := page.WaitLoad(); err != nil {
				return err
			}
		}

		if err := page.ApplyProfile(prof); err != nil {
			return err
		}

		title, _ := page.Title()
		url, _ := page.URL()

		w := cmd.OutOrStdout()
		_, _ = fmt.Fprintf(w, "Profile loaded: %s (%s)\n", url, title)
		_, _ = fmt.Fprintf(w, "  Profile:        %s\n", prof.Name)
		_, _ = fmt.Fprintf(w, "  User Agent:     %s\n", truncate(prof.Identity.UserAgent, 60))
		_, _ = fmt.Fprintf(w, "  Cookies:        %d\n", len(prof.Cookies))

		return nil
	},
}

var profileShowCmd = &cobra.Command{
	Use:   "show <file.scoutprofile>",
	Short: "Display contents of a profile file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		format, _ := cmd.Flags().GetString("format")

		prof, err := scout.LoadProfile(args[0])
		if err != nil {
			return err
		}

		if format == "json" {
			data, err := json.MarshalIndent(prof, "", "  ")
			if err != nil {
				return fmt.Errorf("scout: profile: show: marshal: %w", err)
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		}

		w := cmd.OutOrStdout()
		_, _ = fmt.Fprintf(w, "Name:           %s\n", prof.Name)
		_, _ = fmt.Fprintf(w, "Version:        %d\n", prof.Version)
		_, _ = fmt.Fprintf(w, "Created:        %s\n", prof.CreatedAt.Format(time.RFC3339))
		_, _ = fmt.Fprintf(w, "Updated:        %s\n", prof.UpdatedAt.Format(time.RFC3339))
		_, _ = fmt.Fprintf(w, "Browser:        %s (%s/%s)\n", prof.Browser.Type, prof.Browser.Platform, prof.Browser.Arch)

		if prof.Browser.WindowW > 0 {
			_, _ = fmt.Fprintf(w, "Window:         %dx%d\n", prof.Browser.WindowW, prof.Browser.WindowH)
		}

		_, _ = fmt.Fprintf(w, "User Agent:     %s\n", prof.Identity.UserAgent)
		_, _ = fmt.Fprintf(w, "Language:       %s\n", prof.Identity.Language)
		_, _ = fmt.Fprintf(w, "Timezone:       %s\n", prof.Identity.Timezone)
		_, _ = fmt.Fprintf(w, "Locale:         %s\n", prof.Identity.Locale)

		if prof.Proxy != "" {
			_, _ = fmt.Fprintf(w, "Proxy:          %s\n", prof.Proxy)
		}

		_, _ = fmt.Fprintf(w, "Cookies:        %d\n", len(prof.Cookies))
		for _, c := range prof.Cookies {
			_, _ = fmt.Fprintf(w, "  %-30s  domain=%-20s  secure=%v  httpOnly=%v\n",
				truncate(c.Name, 30), c.Domain, c.Secure, c.HTTPOnly)
		}

		for origin, s := range prof.Storage {
			_, _ = fmt.Fprintf(w, "Storage [%s]:\n", origin)
			_, _ = fmt.Fprintf(w, "  localStorage:   %d keys\n", len(s.LocalStorage))

			for k := range s.LocalStorage {
				_, _ = fmt.Fprintf(w, "    %s\n", truncate(k, 60))
			}

			_, _ = fmt.Fprintf(w, "  sessionStorage: %d keys\n", len(s.SessionStorage))
			for k := range s.SessionStorage {
				_, _ = fmt.Fprintf(w, "    %s\n", truncate(k, 60))
			}
		}

		if len(prof.Extensions) > 0 {
			_, _ = fmt.Fprintf(w, "Extensions:     %d\n", len(prof.Extensions))
			for _, e := range prof.Extensions {
				_, _ = fmt.Fprintf(w, "  %s\n", e)
			}
		}

		if prof.Headers != nil && len(prof.Headers) > 0 {
			_, _ = fmt.Fprintf(w, "Headers:        %d\n", len(prof.Headers))
			for k, v := range prof.Headers {
				_, _ = fmt.Fprintf(w, "  %s: %s\n", k, truncate(v, 60))
			}
		}

		if prof.Notes != "" {
			_, _ = fmt.Fprintf(w, "Notes:          %s\n", prof.Notes)
		}

		return nil
	},
}

func init() {
	profileCaptureCmd.Flags().String("name", "", "profile name")
	profileCaptureCmd.Flags().StringP("output", "o", "", "output file (default: profile.scoutprofile)")

	profileShowCmd.Flags().String("format", "text", "output format: text or json")

	profileCmd.AddCommand(profileCaptureCmd, profileLoadCmd, profileShowCmd)
	rootCmd.AddCommand(profileCmd)
}
