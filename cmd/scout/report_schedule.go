package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

func init() {
	reportCmd.AddCommand(reportScheduleCmd)

	reportScheduleCmd.Flags().Duration("every", time.Hour, "interval between health checks")
	reportScheduleCmd.Flags().Int("depth", 2, "maximum crawl depth")
	reportScheduleCmd.Flags().String("browser", "", "browser to use (chrome, brave, edge)")

	reportScheduleCmd.AddCommand(reportScheduleStopCmd)
}

var reportScheduleCmd = &cobra.Command{
	Use:   "schedule <url>",
	Short: "Run recurring health checks on a URL",
	Long:  "Runs in the foreground as a loop: executes a health check, saves a report, sleeps for the interval, and repeats. Stop with Ctrl-C.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetURL := args[0]

		every, _ := cmd.Flags().GetDuration("every")
		depth, _ := cmd.Flags().GetInt("depth")

		if every < time.Second {
			return fmt.Errorf("scout: report schedule: --every must be at least 1s")
		}

		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Scheduling health check for %s every %s (depth=%d)\n", targetURL, every, depth)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Press Ctrl-C to stop.\n\n")

		for {
			if err := runScheduledCheck(cmd, targetURL, depth); err != nil {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "error: %v\n", err)
			}

			select {
			case <-ctx.Done():
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "\nStopping scheduled health checks.")
				return nil
			case <-time.After(every):
			}
		}
	},
}

func runScheduledCheck(cmd *cobra.Command, targetURL string, depth int) error {
	opts := baseOpts(cmd)

	b, err := scout.New(opts...)
	if err != nil {
		return fmt.Errorf("scout: report schedule: %w", err)
	}

	defer func() { _ = b.Close() }()

	healthOpts := []scout.HealthCheckOption{
		scout.WithHealthDepth(depth),
		scout.WithHealthConcurrency(3),
		scout.WithHealthTimeout(60 * time.Second),
	}

	report, err := b.HealthCheck(targetURL, healthOpts...)
	if err != nil {
		return fmt.Errorf("scout: report schedule: health check: %w", err)
	}

	r := &scout.Report{
		Type:   "health_check",
		URL:    targetURL,
		Health: report,
	}

	id, saveErr := scout.SaveReport(r)
	if saveErr != nil {
		return fmt.Errorf("scout: report schedule: save: %w", saveErr)
	}

	issues := 0
	if report != nil {
		issues = len(report.Issues)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "[%s] Report %s — %d pages, %d issues, %s\n",
		time.Now().Format("15:04:05"), id, report.Pages, issues, report.Duration)

	return nil
}

var reportScheduleStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop a running scheduled health check",
	Run: func(cmd *cobra.Command, _ []string) {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Scheduled health checks run in the foreground.")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Send SIGINT (Ctrl-C) or SIGTERM to the running process to stop it.")
	},
}
