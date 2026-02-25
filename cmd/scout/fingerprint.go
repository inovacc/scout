package main

import (
	"encoding/json"
	"fmt"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

var fingerprintCmd = &cobra.Command{
	Use:   "fingerprint",
	Short: "Browser fingerprint generation and application",
}

var fingerprintGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate a random browser fingerprint",
	RunE: func(cmd *cobra.Command, _ []string) error {
		osFlag, _ := cmd.Flags().GetString("os")
		mobile, _ := cmd.Flags().GetBool("mobile")
		locale, _ := cmd.Flags().GetString("locale")
		format, _ := cmd.Flags().GetString("format")

		var opts []scout.FingerprintOption
		if osFlag != "" {
			opts = append(opts, scout.WithFingerprintOS(osFlag))
		}
		if mobile {
			opts = append(opts, scout.WithFingerprintMobile(true))
		}
		if locale != "" {
			opts = append(opts, scout.WithFingerprintLocale(locale))
		}

		fp := scout.GenerateFingerprint(opts...)

		w := cmd.OutOrStdout()
		if format == "json" {
			data, err := json.MarshalIndent(fp, "", "  ")
			if err != nil {
				return fmt.Errorf("scout: fingerprint: marshal: %w", err)
			}
			_, _ = fmt.Fprintln(w, string(data))
		} else {
			_, _ = fmt.Fprintf(w, "User-Agent:    %s\n", fp.UserAgent)
			_, _ = fmt.Fprintf(w, "Platform:      %s\n", fp.Platform)
			_, _ = fmt.Fprintf(w, "Vendor:        %s\n", fp.Vendor)
			_, _ = fmt.Fprintf(w, "Languages:     %v\n", fp.Languages)
			_, _ = fmt.Fprintf(w, "Timezone:      %s\n", fp.Timezone)
			_, _ = fmt.Fprintf(w, "Screen:        %dx%d\n", fp.ScreenWidth, fp.ScreenHeight)
			_, _ = fmt.Fprintf(w, "Color Depth:   %d\n", fp.ColorDepth)
			_, _ = fmt.Fprintf(w, "Pixel Ratio:   %g\n", fp.PixelRatio)
			_, _ = fmt.Fprintf(w, "WebGL Vendor:  %s\n", fp.WebGLVendor)
			_, _ = fmt.Fprintf(w, "WebGL Render:  %s\n", fp.WebGLRenderer)
			_, _ = fmt.Fprintf(w, "CPU Cores:     %d\n", fp.HardwareConcurrency)
			_, _ = fmt.Fprintf(w, "Memory (GB):   %d\n", fp.DeviceMemory)
			_, _ = fmt.Fprintf(w, "Touch Points:  %d\n", fp.MaxTouchPoints)
			_, _ = fmt.Fprintf(w, "Do Not Track:  %s\n", fp.DoNotTrack)
		}
		return nil
	},
}

var fingerprintApplyCmd = &cobra.Command{
	Use:   "apply <url>",
	Short: "Navigate to a URL with a random fingerprint",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		osFlag, _ := cmd.Flags().GetString("os")
		mobile, _ := cmd.Flags().GetBool("mobile")
		locale, _ := cmd.Flags().GetString("locale")

		var fpOpts []scout.FingerprintOption
		if osFlag != "" {
			fpOpts = append(fpOpts, scout.WithFingerprintOS(osFlag))
		}
		if mobile {
			fpOpts = append(fpOpts, scout.WithFingerprintMobile(true))
		}
		if locale != "" {
			fpOpts = append(fpOpts, scout.WithFingerprintLocale(locale))
		}

		opts := append(baseOpts(cmd), scout.WithRandomFingerprint(fpOpts...))
		b, err := scout.New(opts...)
		if err != nil {
			return err
		}
		defer func() { _ = b.Close() }()

		page, err := b.NewPage(args[0])
		if err != nil {
			return err
		}
		defer func() { _ = page.Close() }()

		if err := page.WaitLoad(); err != nil {
			return err
		}

		title, _ := page.Title()
		pageURL, _ := page.URL()

		w := cmd.OutOrStdout()
		_, _ = fmt.Fprintf(w, "Navigated: %s\n", pageURL)
		_, _ = fmt.Fprintf(w, "Title:     %s\n", title)

		// Print the fingerprint that was used.
		_, _ = fmt.Fprintln(w, "\nFingerprint applied:")
		fp := scout.GenerateFingerprint(fpOpts...)
		_, _ = fmt.Fprintf(w, "  OS option: %s\n", osFlag)
		_, _ = fmt.Fprintf(w, "  Mobile:    %v\n", mobile)
		_, _ = fmt.Fprintf(w, "  Platform:  %s\n", fp.Platform)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(fingerprintCmd)
	fingerprintCmd.AddCommand(fingerprintGenerateCmd)
	fingerprintCmd.AddCommand(fingerprintApplyCmd)

	// Generate flags.
	fingerprintGenerateCmd.Flags().String("os", "", "Target OS: windows, mac, linux")
	fingerprintGenerateCmd.Flags().Bool("mobile", false, "Generate mobile fingerprint")
	fingerprintGenerateCmd.Flags().String("locale", "", "Locale (e.g. en-US, de-DE)")
	fingerprintGenerateCmd.Flags().String("format", "text", "Output format: text, json")

	// Apply flags.
	fingerprintApplyCmd.Flags().String("os", "", "Target OS: windows, mac, linux")
	fingerprintApplyCmd.Flags().Bool("mobile", false, "Generate mobile fingerprint")
	fingerprintApplyCmd.Flags().String("locale", "", "Locale (e.g. en-US, de-DE)")
}
