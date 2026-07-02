package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/inovacc/scout/pkg/scout/tools"
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
	gatherCmd.Flags().Bool("report", false, "save report to ~/.scout/reports/")
}

var gatherCmd = &cobra.Command{
	Use:   "gather <url>",
	Short: "Collect all page intelligence in one shot: DOM, HAR, links, screenshots, cookies, metadata",
	Long: `Collect all page intelligence in one shot.

Also available as MCP tool ` + "`mcp__scout__gather`" + ` — both surfaces delegate to
pkg/scout/tools.Gather so flags and arguments behave identically.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := baseOpts(cmd)
		b, err := scout.New(opts...)
		if err != nil {
			return fmt.Errorf("scout: gather: %w", err)
		}
		defer func() { _ = b.Close() }()

		in := gatherInputFromFlags(cmd, args[0])

		result, err := tools.Gather(context.Background(), b, in)
		if err != nil {
			return err
		}

		if save, _ := cmd.Flags().GetBool("report"); save {
			r := &scout.Report{Type: "gather", URL: in.URL, Gather: result}
			id, saveErr := scout.SaveReport(r)
			if saveErr != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: save report: %v\n", saveErr)
			} else {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Report saved: %s\n", id)
			}
		}

		outFile, _ := cmd.Flags().GetString("output")
		saveScreenshot, _ := cmd.Flags().GetBool("save-screenshot")
		saveHAR, _ := cmd.Flags().GetBool("save-har")

		if saveScreenshot && result.Screenshot != "" {
			ssFile := "gather_screenshot.png"
			if outFile != "" && outFile != "-" {
				ssFile = outFile[:len(outFile)-len(filepath.Ext(outFile))] + "_screenshot.png"
			}
			data, derr := decodeBase64(result.Screenshot)
			if derr != nil {
				return fmt.Errorf("scout: gather: decode screenshot: %w", derr)
			}
			if werr := os.WriteFile(ssFile, data, 0o600); werr != nil {
				return fmt.Errorf("scout: gather: write screenshot %s: %w", ssFile, werr)
			}
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Screenshot saved: %s\n", ssFile)
		}

		if saveHAR && len(result.HAR) > 0 {
			harFile := "gather.har"
			if outFile != "" && outFile != "-" {
				harFile = outFile[:len(outFile)-len(filepath.Ext(outFile))] + ".har"
			}
			if werr := os.WriteFile(harFile, result.HAR, 0o600); werr != nil {
				return fmt.Errorf("scout: gather: write har %s: %w", harFile, werr)
			}
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "HAR saved: %s (%d entries)\n", harFile, result.HAREntries)
		}

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

// gatherInputFromFlags translates the cobra flag bag into the unified
// tools.GatherInput. Single source of truth for flag→input mapping; both
// the CLI handler above and any future shim (e.g. an `--input-file json`
// path) can re-use it.
func gatherInputFromFlags(cmd *cobra.Command, url string) tools.GatherInput {
	getBool := func(name string) bool { v, _ := cmd.Flags().GetBool(name); return v }
	timeoutStr := ""
	if d, _ := cmd.Flags().GetDuration("timeout"); d > 0 {
		timeoutStr = d.String()
	}
	return tools.GatherInput{
		URL:        url,
		HTML:       getBool("html"),
		Markdown:   getBool("markdown"),
		Screenshot: getBool("screenshot"),
		Snapshot:   getBool("snapshot"),
		HAR:        getBool("har"),
		Links:      getBool("links"),
		Cookies:    getBool("cookies"),
		Meta:       getBool("meta"),
		Frameworks: getBool("frameworks"),
		Console:    getBool("console"),
		All:        getBool("all"),
		Timeout:    timeoutStr,
	}
}

func decodeBase64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}
