package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(testSiteCmd)

	testSiteCmd.Flags().Int("depth", 2, "maximum crawl depth")
	testSiteCmd.Flags().Int("concurrency", 3, "concurrent page limit")
	testSiteCmd.Flags().Bool("click", false, "click interactive elements to discover JS errors")
	testSiteCmd.Flags().Bool("json", false, "output as JSON")
	testSiteCmd.Flags().Duration("timeout", 60*time.Second, "overall timeout")
}

var testSiteCmd = &cobra.Command{
	Use:   "test-site <url>",
	Short: "Check site health: broken links, console errors, JS exceptions, network failures",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetURL := args[0]

		depth, _ := cmd.Flags().GetInt("depth")
		concurrency, _ := cmd.Flags().GetInt("concurrency")
		click, _ := cmd.Flags().GetBool("click")
		jsonOut, _ := cmd.Flags().GetBool("json")
		timeout, _ := cmd.Flags().GetDuration("timeout")

		opts := baseOpts(cmd)
		opts = append(opts, scout.WithNoSandbox())

		b, err := scout.New(opts...)
		if err != nil {
			return fmt.Errorf("scout: test-site: %w", err)
		}

		defer func() { _ = b.Close() }()

		var healthOpts []scout.HealthCheckOption

		healthOpts = append(healthOpts,
			scout.WithHealthDepth(depth),
			scout.WithHealthConcurrency(concurrency),
			scout.WithHealthTimeout(timeout),
		)

		if click {
			healthOpts = append(healthOpts, scout.WithHealthClickElements())
		}

		report, err := b.HealthCheck(targetURL, healthOpts...)
		if err != nil {
			return err
		}

		if jsonOut {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")

			return enc.Encode(report)
		}

		// Table output.
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Health Check: %s\n", report.URL)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Pages checked: %d  Duration: %s\n\n", report.Pages, report.Duration)

		if len(report.Issues) == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No issues found.")
			return nil
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "SEVERITY\tSOURCE\tURL\tMESSAGE")

		for _, issue := range report.Issues {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				issue.Severity, issue.Source, truncate(issue.URL, 50), truncate(issue.Message, 80))
		}

		_ = w.Flush()

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nSummary: errors=%d warnings=%d info=%d\n",
			report.Summary["error"], report.Summary["warning"], report.Summary["info"])

		if report.Summary["error"] > 0 {
			os.Exit(1)
		}

		return nil
	},
}
