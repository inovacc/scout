package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/inovacc/scout/pkg/scout"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(jobsCmd)
	jobsCmd.AddCommand(jobsListCmd)
	jobsCmd.AddCommand(jobsStatusCmd)
	jobsCmd.AddCommand(jobsCancelCmd)

	jobsListCmd.Flags().String("status", "", "filter by status (pending, running, completed, failed, cancelled)")
}

func defaultJobsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".scout", "jobs")
	}
	return filepath.Join(home, ".scout", "jobs")
}

var jobsCmd = &cobra.Command{
	Use:   "jobs",
	Short: "Manage async jobs (batch, crawl, fetch)",
}

var jobsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all jobs",
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := scout.NewAsyncJobManager(defaultJobsDir())
		if err != nil {
			return fmt.Errorf("scout: jobs: %w", err)
		}

		statusFlag, _ := cmd.Flags().GetString("status")

		var jobs []*scout.AsyncJob
		if statusFlag != "" {
			jobs = m.List(scout.AsyncJobStatus(statusFlag))
		} else {
			jobs = m.List()
		}

		format, _ := cmd.Flags().GetString("format")
		if format == "json" {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(jobs)
		}

		if len(jobs) == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No jobs found.")
			return nil
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "ID\tTYPE\tSTATUS\tPROGRESS\tCREATED")

		for _, j := range jobs {
			progress := fmt.Sprintf("%d/%d", j.Progress.Completed, j.Progress.Total)
			if j.Progress.Failed > 0 {
				progress += fmt.Sprintf(" (%d failed)", j.Progress.Failed)
			}

			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				truncate(j.ID, 12),
				j.Type,
				j.Status,
				progress,
				j.CreatedAt.Format("2006-01-02 15:04:05"),
			)
		}

		return w.Flush()
	},
}

var jobsStatusCmd = &cobra.Command{
	Use:   "status <id>",
	Short: "Show job details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := scout.NewAsyncJobManager(defaultJobsDir())
		if err != nil {
			return fmt.Errorf("scout: jobs: %w", err)
		}

		j, err := m.Get(args[0])
		if err != nil {
			return fmt.Errorf("scout: jobs: %w", err)
		}

		format, _ := cmd.Flags().GetString("format")
		if format == "json" {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(j)
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "ID:        %s\n", j.ID)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Type:      %s\n", j.Type)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Status:    %s\n", j.Status)
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Created:   %s\n", j.CreatedAt.Format("2006-01-02 15:04:05"))

		if j.StartedAt != nil {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Started:   %s\n", j.StartedAt.Format("2006-01-02 15:04:05"))
		}

		if j.EndedAt != nil {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Ended:     %s\n", j.EndedAt.Format("2006-01-02 15:04:05"))
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Progress:  %d/%d completed, %d failed\n",
			j.Progress.Completed, j.Progress.Total, j.Progress.Failed)

		if j.Error != "" {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Error:     %s\n", j.Error)
		}

		return nil
	},
}

var jobsCancelCmd = &cobra.Command{
	Use:   "cancel <id>",
	Short: "Cancel a running job",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		m, err := scout.NewAsyncJobManager(defaultJobsDir())
		if err != nil {
			return fmt.Errorf("scout: jobs: %w", err)
		}

		if err := m.Cancel(args[0]); err != nil {
			return fmt.Errorf("scout: jobs: %w", err)
		}

		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Job %s cancelled.\n", args[0])
		return nil
	},
}
