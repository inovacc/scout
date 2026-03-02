package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(gatherCmd)

	gatherCmd.Flags().Bool("html", false, "include raw HTML")
	gatherCmd.Flags().Bool("markdown", false, "include markdown")
	gatherCmd.Flags().Bool("screenshot", false, "include screenshot (base64 PNG)")
	gatherCmd.Flags().Bool("snapshot", false, "include accessibility snapshot")
	gatherCmd.Flags().Bool("har", false, "include HAR network recording")
	gatherCmd.Flags().Bool("links", false, "include extracted links")
	gatherCmd.Flags().Bool("cookies", false, "include cookies")
	gatherCmd.Flags().Bool("meta", false, "include page metadata")
	gatherCmd.Flags().Bool("frameworks", false, "include detected frameworks")
	gatherCmd.Flags().Bool("console", false, "include console output")
	gatherCmd.Flags().Bool("all", false, "include everything (default if no flags set)")
	gatherCmd.Flags().Bool("save-screenshot", false, "save screenshot as PNG file alongside JSON")
	gatherCmd.Flags().Bool("save-har", false, "save HAR as separate file alongside JSON")
	gatherCmd.Flags().Duration("timeout", 30*time.Second, "page load timeout")
}

var gatherCmd = &cobra.Command{
	Use:   "gather <url>",
	Short: "Collect all page intelligence in one shot: DOM, HAR, links, screenshots, cookies, metadata",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetURL := args[0]

		opts := baseOpts(cmd)
		b, err := scout.New(opts...)
		if err != nil {
			return fmt.Errorf("scout: gather: %w", err)
		}

		defer func() { _ = b.Close() }()

		var gatherOpts []scout.GatherOption

		// Check if any specific flags are set.
		anySpecific := false
		flagChecks := []struct {
			name string
			opt  scout.GatherOption
		}{
			{"html", scout.WithGatherHTML()},
			{"markdown", scout.WithGatherMarkdown()},
			{"screenshot", scout.WithGatherScreenshot()},
			{"snapshot", scout.WithGatherSnapshot()},
			{"har", scout.WithGatherHAR()},
			{"links", scout.WithGatherLinks()},
			{"cookies", scout.WithGatherCookies()},
			{"meta", scout.WithGatherMeta()},
			{"frameworks", scout.WithGatherFrameworks()},
			{"console", scout.WithGatherConsole()},
		}

		for _, fc := range flagChecks {
			if v, _ := cmd.Flags().GetBool(fc.name); v {
				gatherOpts = append(gatherOpts, fc.opt)
				anySpecific = true
			}
		}

		// If --all or no specific flags, default behavior is all.
		_ = anySpecific

		if timeout, _ := cmd.Flags().GetDuration("timeout"); timeout > 0 {
			gatherOpts = append(gatherOpts, scout.WithGatherTimeout(timeout))
		}

		result, err := b.Gather(targetURL, gatherOpts...)
		if err != nil {
			return err
		}

		outFile, _ := cmd.Flags().GetString("output")
		saveScreenshot, _ := cmd.Flags().GetBool("save-screenshot")
		saveHAR, _ := cmd.Flags().GetBool("save-har")

		// Save screenshot as separate file if requested.
		if saveScreenshot && result.Screenshot != "" {
			ssFile := "gather_screenshot.png"
			if outFile != "" && outFile != "-" {
				ssFile = outFile[:len(outFile)-len(filepath.Ext(outFile))] + "_screenshot.png"
			}

			if data, err := decodeBase64(result.Screenshot); err == nil {
				if err := os.WriteFile(ssFile, data, 0o644); err == nil {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Screenshot saved: %s\n", ssFile)
				}
			}
		}

		// Save HAR as separate file if requested.
		if saveHAR && len(result.HAR) > 0 {
			harFile := "gather.har"
			if outFile != "" && outFile != "-" {
				harFile = outFile[:len(outFile)-len(filepath.Ext(outFile))] + ".har"
			}

			if err := os.WriteFile(harFile, result.HAR, 0o644); err == nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "HAR saved: %s (%d entries)\n", harFile, result.HAREntries)
			}
		}

		// For JSON output, omit large binary fields if saved separately.
		outputResult := *result
		if saveScreenshot {
			outputResult.Screenshot = ""
		}

		if saveHAR {
			outputResult.HAR = nil
		}

		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")

		if err := enc.Encode(outputResult); err != nil {
			return fmt.Errorf("scout: gather: encode: %w", err)
		}

		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Gathered %s in %s\n", result.URL, result.Duration)

		return nil
	},
}

func decodeBase64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
